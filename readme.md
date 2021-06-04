# Go Live Server

Go Live Server is a static website server that refreshes the browser whenever a file changes. It does this by adding a JS snippet that receives messages from the server through a websocket whenever a file changes on disk.

Inspired by the package https://github.com/tapio/live-server

## Usage

```sh
cd website-directory
go-live-server -host=0.0.0.0
```

### Options

- `-host=localhost` Hostname, such as mywebsite.com, 0.0.0.0, or localhost
- `-port=9090` Port
- `-browser=true` Whether to open link in browser on startup (default true)       
- `-browser-path`
        relative path to open in browser
- `-close=true` Whether to close the browser tab when the server closes (default true)
- `-debounce=0` Time to wait after changes before reloading. Use this if it's reloading without all changes. This issue happens when a program saves a file multiple times in quick succession.
- `-reconnect=false` Try to reconnect JS snippet to server if server is stopped and then started again
- `-nested=false` Watch nested directories. This requires listening to each subdirectory individually, so it won't work on gigantic directories.

## Installation

You can install it as an executable from the GitHub [releases](https://github.com/taoroalin/go-live-server/releases) section, or compile the Go source code yourself.

Run from source (after installing [go](https://golang.org/doc/install))

```sh
git clone github.com/taoroalin/go-live-server
cd go-live-server
go mod tidy
go build
./go-live-server
```

## How it works

It inserts this HTML (with optional extra bits) into the beginning of any HTML file it serves. This leads to 2 `<head>` tags, which is invalid, but browsers are totally fine with it. (because people often make their html invalid to save space, it isn't always possible to make valid html)

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