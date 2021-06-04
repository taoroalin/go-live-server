# Go Live Server

Static server that reloads whenever a file changes. It serves static sites (html, css, ect) with an additional JS snippet that refreshes the browser whenever a file changes on disk.

Inspired by the package https://github.com/tapio/live-server

## Usage
```sh
cd website-directory
go-live-server -host=0.0.0.0
```

### Options

-   `-host=localhost` Hostname, such as mywebsite.com, 0.0.0.0, or localhost
-   `-port=9090` Port
-  `-browser=true` Whether to open link in browser on startup (default true)       
-  `-browser-path`
        relative path to open in browser
-   `-close=true` Whether to close the browser tab when the server closes (default true)
-   `-debounce=0` Time to wait after changes before reloading. Use this if it's reloading without all changes. This issue happens when a program saves a file multiple times in quick succession.
-   `-reconnect=false` Try to reconnect JS snippet to server if server is stopped and then started again

## Installation

You can install it as an executable from the GitHub [releases](https://github.com/taoroalin/go-live-server/releases) section, or compile the Go source code yourself.

Run from source (after installing [go](https://golang.org/doc/install))

```sh
git clone github.com/taoroalin/go-live-server
cd go-live-server
go mod tidy
go build
./main
```