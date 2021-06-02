package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
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
			var protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
			var address = protocol + window.location.host + '/ws';
			var socket = new WebSocket(address);
			socket.addEventListener("open", (event)=>{
			socket.send("hi, I can send!")
			})
			socket.addEventListener("message",(msg)=> {
				console.log("got message!")
				console.log(msg.data)
				if (msg.data == 'reload') window.location.reload();
			})
			console.log('Live reload enabled.');
</script>
</head>
`)

func requestHandler(ctx *fasthttp.RequestCtx) {

	protocol := ctx.Request.Header.Protocol()
	if string(protocol) != "HTTP/1.1" {
		println(string(protocol))
	}
	path := "." + string(ctx.Path())

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
	if len(path) >= 5 && path[len(path)-5:] == ".html" {
		ctx.Response.Header.Set("Content-Type", "text/html")
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
	path, _ := os.Getwd()
	addError := watcher.Add(path)
	if addError != nil {
		panic(addError)
	}

	fmt.Println("Go live server listening on port 9090")
	openBrowserToLink("http://localhost:9090")

	go fileEventReadLoop(watcher)

	fasthttp.ListenAndServe(":9090", requestHandler)

}
