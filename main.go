package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/fsnotify/fsnotify"
	"github.com/valyala/fasthttp"
)

var websocketUpgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// handshake duration?
}

// list elements become nil when connections are disconnected. not as easy as I'd like to remove things from collections...
var connectionListMutex = sync.Mutex{}
var connectionList = []*websocket.Conn{}
var rootPath = "./"

var extRegex = regexp.MustCompile(`\.[a-z]+$`)

func openBrowserToLink(url string) {
	var err error

	switch runtime.GOOS {
	case "linux":
		err = exec.Command("xdg-open", url).Start()
	case "windows":
		err = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
	case "darwin":
		err = exec.Command("open", url).Start()
	default:
		err = fmt.Errorf("unsupported platform")
	}
	if err != nil {
		log.Fatal(err)
	}

}

// weird behavior where it automatically closes the connection if the client doesn't send any message?
var websocketJS = []byte(`
<!DOCTYPE html>
<head>
<script type="text/javascript">
			let protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
			let address = protocol + window.location.host + '/ws';
			let socket = new WebSocket(address);
			socket.addEventListener("message",(msg)=> {
				if (msg.data == 'reload') window.location.reload();
			})
			console.log('Go Live Server enabled.');
</script>
</head>
`)

var reconnecterJS = []byte(`
const tryReconnect = ()=>{
				try{
					socket = new WebSocket(address);
					document.removeEventListener(visEventListener)
				}catch(e){
				}
			}
			let reconnectInterval = null
			let visEventListener = ()=>{
					if(!document.hidden && reconnectInterval===null){
						reconnectInterval = setInterval(tryReconnect, 1000)
					}else if (reconnectInterval!==null){
						clearInterval(reconnectInterval)
						reconnectInterval = null
					}
				}
			socket.onclose = ()=>{
				console.log("socketclose")
				if(!document.hidden) reconnectInterval = setInterval(tryReconnect, 1000)
				document.addEventListener("visibilitychange", visEventListener)
			}`)

func fileNameToContentType(str string) string {
	table := map[string]string{
		".html":  "text/html; charset=UTF-8",
		".css":   "text/css; charset=UTF-8",
		".js":    "application/javascript; charset=UTF-8",
		".woff2": "font/woff2",
		".ico":   "image/x-icon"}
	extension := extRegex.FindString(str)
	contentType := table[extension]
	return contentType
}

func requestHandler(ctx *fasthttp.RequestCtx) {

	protocol := ctx.Request.Header.Protocol()
	if string(protocol) != "HTTP/1.1" {
		println(string(protocol))
	}
	path := rootPath[:len(rootPath)-1] + string(ctx.Path())

	if path == "./ws" {
		websocketReadLoop := func(conn *websocket.Conn) {
			connectionListMutex.Lock()
			connectionListIndex := len(connectionList)
			connectionList = append(connectionList, conn)
			connectionListMutex.Unlock()
			// why callback? isn't the Go way to use goroutines?
			for {
				messageType, p, err := conn.ReadMessage()
				println("got message " + string(p))
				if err != nil {
					fmt.Printf("%v\n", err)
					connectionListMutex.Lock()
					connectionList[connectionListIndex] = nil
					connectionListMutex.Unlock()
					conn.Close()
					return
				}
				if err := conn.WriteMessage(messageType, p); err != nil {
					fmt.Printf("%v\n", err)
					return
				}
			}
		}

		err := websocketUpgrader.Upgrade(ctx, websocketReadLoop)
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}
		return
	}

	uri := ctx.URI()

	if len(uri.LastPathSegment()) == 0 {
		path = string(path) + "index.html"
	}
	contentType := fileNameToContentType(path)
	if contentType != "" {
		ctx.Response.Header.Set("Content-Type", contentType)
	}
	println(string(path))

	if len(path) >= 5 && path[len(path)-5:] == ".html" {
		println("serving html with websocket js")
		fileBytes, err := ioutil.ReadFile(path)
		if err != nil {
			ctx.WriteString("404 Not Found")
			ctx.SetStatusCode(404)
			fmt.Printf("%v\n", err)
			return
		}
		fileBytesWithWebsocketJs := append(websocketJS, fileBytes...)
		ctx.Write(fileBytesWithWebsocketJs)
	} else {
		fileBytes, err := ioutil.ReadFile(path)
		if err != nil {
			ctx.WriteString("404 Not Found")
			ctx.SetStatusCode(404)
			return
		}
		ctx.Write(fileBytes)
	}
}

func fileEventReadLoop(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			log.Println("event:", event)
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write { // what is op 0?
				log.Println("modified file:", event.Name)
				for _, conn := range connectionList {
					if conn != nil {
						conn.WriteMessage(websocket.TextMessage, []byte("reload"))
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				fmt.Printf("%v\n", err)
				return
			}
			log.Println("error:", err)
		}
	}
}

func main() {
	watcher, watchError := fsnotify.NewWatcher()
	if watchError != nil {
		panic(errors.New("go live server can't detect file changes on this oprating system"))
	}

	argsWithoutProgram := os.Args[1:]
	if len(argsWithoutProgram) > 0 {
		rootPath = argsWithoutProgram[0]
		if rootPath[len(rootPath)-1] != '/' {
			rootPath += "/"
		}
	}

	println("watching " + rootPath)

	addError := watcher.Add(rootPath)
	if addError != nil {
		panic(addError)
	}

	fmt.Println("Go live server listening on port 9090")
	openBrowserToLink("http://localhost:9090")

	go fileEventReadLoop(watcher)

	fasthttp.ListenAndServe(":9090", requestHandler)

}
