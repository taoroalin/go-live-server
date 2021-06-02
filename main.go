package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
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

var websocketJS = []byte(`
<script type="text/javascript">
			var protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
			var address = protocol + window.location.host + window.location.pathname + '/ws';
			var socket = new WebSocket(address);
			socket.onmessage = function(msg) {
				if (msg.data == 'reload') window.location.reload();
			};
			console.log('Live reload enabled.');
</script>
`)

func main() {
	/*

	 */
	watcher, watchError := fsnotify.NewWatcher()
	if watchError != nil {
		panic(errors.New("Go Live Server can't detect file changes on this oprating system"))
	}
	watcher.Add("./")

	fasthttp.ListenAndServe(":9090", func(ctx *fasthttp.RequestCtx) {

		protocol := ctx.Request.Header.Protocol()
		println(string(protocol))

		path := "." + string(ctx.Path())

		if path == "./ws" {
			websocketReadLoop := func(conn *websocket.Conn) {
				connectionListMutex.Lock()
				connectionList = append(connectionList, conn)
				connectionListMutex.Unlock()
				// why callback? isn't the Go way to use goroutines?
				for {
					messageType, p, err := conn.ReadMessage()
					println("got message " + string(p))
					if err != nil {
						fmt.Printf("%v\n", err)
						connectionListMutex.Lock()
						if len(connectionList) > 1 {
							// connectionList
						}
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

	})

	fmt.Println("Go live server listening on port 9090")
	openBrowserToLink("http://localhost:9090")

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Println("event:", event)
				if event.Op&fsnotify.Write == fsnotify.Write { // what is op 0?
					log.Println("modified file:", event.Name)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error:", err)
			}
		}
	}()
}
