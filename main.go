package main

import (
	"github.com/fasthttp/websocket"
	"github.com/valyala/fasthttp"
)

var websocketUpgrader = websocket.FastHTTPUpgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// handshake duration?
}

func main() {

	fasthttp.ListenAndServe(":80", func(ctx *fasthttp.RequestCtx) {
		websocketReadLoop := func(conn *websocket.Conn) {
			// why callback? isn't the Go way to use goroutines?
			for {
				messageType, p, err := conn.ReadMessage()
				if err != nil {
					// log.Println(err)
					return
				}
				if err := conn.WriteMessage(messageType, p); err != nil {
					// log.Println(err)
					return
				}
			}
		}

		err := websocketUpgrader.Upgrade(ctx, websocketReadLoop)
		if err != nil {
			// log.Println(err)
			return
		}

	})
}
