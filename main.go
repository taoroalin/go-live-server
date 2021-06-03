package main

import (
	"flag"
	"fmt"
	htmlTemplate "html/template"
	"io/ioutil"
	"os/exec"
	"regexp"
	"runtime"
	"sync"

	"github.com/fasthttp/websocket"
	"github.com/fsnotify/fsnotify"
	"github.com/valyala/fasthttp"
)

/*
What would I need to do to make this project a legit open source thing?

Test thoroughly on Windows, Linux, and Mac

Add to a package manager

Make file server behavior match mature file servers

Documentation

benchmark vs live-server

give warning when under WSL 2 that file notifications don't propagate between the OSes

*/

var websocketUpgrader = websocket.FastHTTPUpgrader{}

// list elements become nil when connections are disconnected. not as easy as I'd like to remove things from collections...
var connectionListMutex = sync.Mutex{}

// this leaks memory, but wicked slow, 8 bytes per client that disconnects? sadly don't really have to worry about it
var connectionList = []*websocket.Conn{}
var rootPath = "./"

var extRegex = regexp.MustCompile(`\.[a-z]+$`)

var wslRegex = regexp.MustCompile(`microsoft`)

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
		panic(err)
	}
}

type JSOptions struct {
	Close     bool
	Reconnect bool
}

var jsOptions JSOptions

// weird behavior where it automatically closes the connection if the client doesn't send any message?
var websocketJSTemplate, templateError = htmlTemplate.New("js").Parse(`
<!DOCTYPE html>
<head>
<script type="text/javascript">
	let protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
	let address = protocol + window.location.host + '/ws';
	let socket = new WebSocket(address);
	socket.addEventListener("message",(msg)=> {
		if (msg.data == 'reload') window.location.reload();
	})
	{{- if .Close}}
	socket.addEventListener("close", ()=>{
		window.close()
	})
	{{- end}}
	console.log('Go Live Server enabled.');
	{{- if .Reconnect}}
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
	}
	{{- end}}
</script>
</head>
`)

var mimeTypes = map[string]string{
	// copy pasted from golang src/mime/type.go
	".css":  "text/css; charset=utf-8",
	".gif":  "image/gif",
	".htm":  "text/html; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".jpeg": "image/jpeg",
	".jpg":  "image/jpeg",
	".js":   "text/javascript; charset=utf-8",
	".json": "application/json",
	".mjs":  "text/javascript; charset=utf-8",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".wasm": "application/wasm",
	".webp": "image/webp",
	".xml":  "text/xml; charset=utf-8",

	// added by me
	".woff2": "font/woff2",
	".woff":  "font/woff",
	".ico":   "image/image/vnd.microsoft.icon",
	// ".ico":   "image/x-icon",

	// extra ones from https://developer.mozilla.org/en-US/docs/Web/HTTP/Basics_of_HTTP/MIME_types/Common_types bc why not
	".acc":  "audio/acc",
	".avi":  "video/x-msvideo",
	".bmp":  "image/bmp",
	".csv":  "text/csv",
	".epub": "application/epub+zip",
	".mp3":  "audio/mpeg",
	".mpeg": "video/mpeg",
	".mp4":  "video/mp4",
	".ttf":  "font/ttf",
}

func fileNameToContentType(str string) string {
	extension := extRegex.FindString(str)
	contentType := mimeTypes[extension]
	return contentType
}

func websocketCallback(conn *websocket.Conn) {
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

func requestHandler(ctx *fasthttp.RequestCtx) {

	protocol := ctx.Request.Header.Protocol()
	if string(protocol) != "HTTP/1.1" {
		println(string(protocol))
	}
	requestedPath := string(ctx.Path())
	path := rootPath[:len(rootPath)-1] + requestedPath
	if requestedPath == "/ws" {
		err := websocketUpgrader.Upgrade(ctx, websocketCallback)
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

	if len(path) >= 5 && path[len(path)-5:] == ".html" {
		fileBytes, err := ioutil.ReadFile(path)
		if err != nil {
			ctx.WriteString("404 Not Found")
			ctx.SetStatusCode(404)
			// fmt.Printf("%v\n", err)
			return
		}
		teError := websocketJSTemplate.Execute(ctx.Response.BodyWriter(), jsOptions)
		if teError != nil {
			panic(teError)
		}
		ctx.Write(fileBytes)
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
			// log.Println("event:", event)
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write { // what is op 0?
				// log.Println("modified file:", event.Name)
				fmt.Println("file modified, reloading")
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
			fmt.Println("error:", err)
		}
	}
}

func warnIfInWSL2() {
	if runtime.GOOS == "linux" {
		procversion, err := ioutil.ReadFile("/proc/version")
		if err != nil {
			panic(err)
		}
		if wslRegex.Find(procversion) != nil {
			fmt.Println("Warning: Cannot detect file changes from Windows in WSL")
		}
	}
}

func main() {
	if templateError != nil {
		panic(templateError)
	}
	/*
		what command line options do I want?

		When file is changed multiple times quickly, only reload on last one

		When file is changed multiple times quickly, only reload on first one

		When server closes, close all tabs

		port

		host

		no-browser

		open-path -- open to different path than server root

		--help

		--version
	*/

	host := flag.String("host", "localhost", "Hostname, such as mywebsite.com, 0.0.0.0, or localhost")

	port := flag.String("port", "9090", "Port. Defaults to 9090, public website is 80")

	useBrowser := flag.Bool("browser", true, "Whether to open link in browser on startup")

	flag.BoolVar(&jsOptions.Close, "close", true, "Whether to close the browser tab when the server closes")

	flag.BoolVar(&jsOptions.Reconnect, "reconnect", false, "Try to reconnect to server if server connection is lost")

	browserPath := flag.String("browser-path", "", "relative path to open in browser")

	if len(*browserPath) >= 1 && (*browserPath)[0] != '/' {
		newBrowserPath := "/" + *browserPath
		browserPath = &newBrowserPath
	}

	rootPath := flag.Arg(0)
	flag.Parse() // --help will print here

	fmt.Println(rootPath)
	fmt.Println(*host)
	fmt.Println(*port)
	fmt.Println(*useBrowser)
	fmt.Println(*browserPath)

	watcher, watchError := fsnotify.NewWatcher()
	if watchError != nil {
		panic(watchError)
		// panic(errors.New("go live server can't detect file changes on this oprating system"))
	}

	warnIfInWSL2()

	if len(rootPath) > 0 && rootPath[len(rootPath)-1] != '/' {
		rootPath += "/"
	}

	println("watching " + rootPath)

	addError := watcher.Add(rootPath)
	if addError != nil {
		panic(addError)
	}

	addr := *host + ":" + *port

	fmt.Println("Go live server listening on " + addr)
	if *useBrowser {
		openBrowserToLink("http://" + addr + *browserPath)
	}
	go fileEventReadLoop(watcher)

	fasthttp.ListenAndServe(addr, requestHandler)
}
