# Go Live Server

Go Live Server is a static website server that refreshes the browser whenever a file changes. It does this by adding a JS snippet that receives messages from the server through a websocket whenever a file changes on disk.

Inspired by the package https://github.com/tapio/live-server, but faster (0.5 second on startup, 50ms on reload), and in the Go ecosystem rather than the Node ecosystem.

## Usage

```sh
cd website-directory
go-live-server -host=0.0.0.0
```

### Options

- `-host=localhost` Hostname, such as mywebsite.com, 0.0.0.0, or localhost.
- `-port=9090` Port
- `-browser=true` Whether to open link in browser on startup (default true).    
- `-browser-path`
        relative path to open in browser
- `-close=true` Whether to close the browser tab when the server closes (default true)
- `-blind-for=50` Time to wait after changes before detecting changes again
- `-debounce=0` Time to wait after changes before reloading. Use this if it's reloading without all changes. This issue happens when a program saves a file multiple times in quick succession.
- `-reconnect=false` Try to reconnect JS snippet to server if server is stopped and then started again.
- `-nested=true` Watch nested directories. This requires listening to each subdirectory individually, so it won't work on gigantic directories.
- `-startup-delay=500` Time to wait after server start to look for changes. Exists because vscode (or something) is modifying files when they're first read.
- `-rigid-port` Don't try a new port if the specified one is taken.

## Installation

You can install it as an executable from the GitHub [releases](https://github.com/taoroalin/go-live-server/releases) section, or compile the Go source code yourself.

Run from source (after installing [go](https://golang.org/doc/install) version 1.16+)

```sh
git clone https://github.com/taoroalin/go-live-server
cd go-live-server
go mod tidy
go install
go-live-server
``c`

## How it works

It inserts this HTML (with optional extra bits) into the beginning of any HTML file it serves. This leads to 2 `<head>` tags, which is invalid, but browsers are totally fine with it. (because people often make their html invalid to save space, it isn't always possible to validly add to an html file)

```html
<head>
  <script>
    let protocol = window.location.protocol === 'https:' ? 'wss://' : 'ws://';
    let address = protocol + window.location.host + '/ws';
    let socket = new WebSocket(address);
    socket.onmessage = (msg)=> {
      if (msg.data === 'reload') window.location.reload();
    }
  </script>
</head>
```