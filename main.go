package main

import (
	"flag"
	"fmt"
	htmlTemplate "html/template"
	"io/fs"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"sync"
	"time"

	_ "embed"

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

*/

var websocketUpgrader = websocket.FastHTTPUpgrader{}

var connectionsMutex = sync.Mutex{}
var connectionsSoFar = 0
var connectionMap = map[int]*websocket.Conn{}
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

// go embed is a feature of go 1.16. it embeds files into the following string or []byte var in the Go binary

//go:embed inject.txt
var jsTemplateString string

var websocketJSTemplate, templateError = htmlTemplate.New("js").Parse(jsTemplateString)

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
	connectionsMutex.Lock()
	connectionIndex := connectionsSoFar
	connectionsSoFar++
	connectionMap[connectionIndex] = conn
	connectionsMutex.Unlock()
	// why callback? isn't the Go way to use goroutines?
	for {
		_, _, err := conn.ReadMessage()
		// println("got message " + string(p))
		if err != nil {
			// fmt.Printf("%v\n", err)
			connectionsMutex.Lock()
			delete(connectionMap, connectionIndex)
			connectionsMutex.Unlock()
			conn.Close()
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
	if requestedPath == "/go-live-server" {
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

func notifyReload(name string) {
	now := time.Now()
	fmt.Printf("%v modified at %v:%v:%v, reloading\n", name, now.Hour(), now.Minute(), now.Second())
	for _, conn := range connectionMap {
		conn.WriteMessage(websocket.TextMessage, []byte("reload"))
	}
}

var dotfileRegex = regexp.MustCompile(`(\/|^)\.[^\/\.]+$`)

func fileEventReadLoop(watcher *fsnotify.Watcher, debounce int, blindFor int, watchDotfileDirs bool) {
	var reloadTimer *time.Timer
	for {
		select {
		case event, ok := <-watcher.Events:
			// log.Println("event:", event)
			if !ok {
				return
			}
			if event.Op&fsnotify.Write == fsnotify.Write { // what is op 0?
				// log.Println("modified file:", event.Name)
				if watchDotfileDirs || dotfileRegex.FindString(event.Name) == "" {
					// fmt.Println(event.Name + " modified, reloading")
					if debounce == 0 && blindFor == 0 {
						notifyReload(event.Name)
					} else if debounce != 0 {
						if reloadTimer != nil {
							reloadTimer.Stop()
							reloadTimer = nil
						}
						reloadTimer = time.AfterFunc(time.Duration(debounce)*time.Millisecond, func() {
							notifyReload(event.Name)
						})
					} else {
						if reloadTimer == nil {
							notifyReload(event.Name)
							stime := time.Now()
							reloadTimer = time.AfterFunc(time.Duration(blindFor)*time.Millisecond, func() {
								reloadTimer = nil
							})
							fmt.Printf("took %v\n", time.Since(stime))
						}
					}
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				panic(err)
			}
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

	debounce := flag.Int("debounce", 0, "Time to wait after changes before reloading. Use this if it's reloading without all changes. This issue happens when software like formatters save files again whenever they're saved.")

	blindFor := flag.Int("blind-for", 50, "Time to wait after changes before detecting changes again")

	startupDelay := flag.Int("startup-delay", 500, "Time to wait after server start to look for changes. Exists because vscode (or something) is modifying files when they're first read.")

	host := flag.String("host", "localhost", "Hostname, such as mywebsite.com, 0.0.0.0, or localhost")

	port := flag.String("port", "9090", "Port. Defaults to 9090, public website is 80")

	useBrowser := flag.Bool("browser", true, "Whether to open link in browser on startup")

	flag.BoolVar(&jsOptions.Close, "close", true, "Whether to close the browser tab when the server closes")

	flag.BoolVar(&jsOptions.Reconnect, "reconnect", false, "Try to reconnect to server if server connection is lost")

	browserPath := flag.String("browser-path", "", "relative path to open in browser")

	nested := flag.Bool("nested", true, "Whether to watch for changes in nested directories. This requires traversing all those folders, so it won't work on gigantic directories")

	watchDotfileDirs := flag.Bool("dotfiles", false, "Whether to watch changes in dotfiles")

	if len(*browserPath) >= 1 && (*browserPath)[0] != '/' {
		newBrowserPath := "/" + *browserPath
		browserPath = &newBrowserPath
	}

	if flag.Arg(0) != "" {
		rootPath = flag.Arg(0)
	}
	flag.Parse() // --help will print here

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
	if *nested {
		stime := time.Now()
		filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
			// println(path)
			justName := d.Name()
			if !*watchDotfileDirs && justName[0] == '.' && len(justName) > 2 {
				return filepath.SkipDir
			}
			if d.IsDir() {
				addError := watcher.Add(path)
				// @TODO special handling of too many watches error
				if addError != nil {
					panic(addError)
				}
			}
			return nil
		})
		took := time.Since(stime)
		if took > 3*time.Millisecond {
			fmt.Printf("visiting nested dirs took %v\n", time.Since(stime))
		}
	} else {
		addError := watcher.Add(rootPath)
		if addError != nil {
			panic(addError)
		}
	}

	time.AfterFunc(time.Millisecond*time.Duration(*startupDelay), func() {
		fileEventReadLoop(watcher, *debounce, *blindFor, *watchDotfileDirs)
	})

	var tryAddress func()
	tryAddress = func() {
		addr := *host + ":" + *port

		if *useBrowser {
			openBrowserToLink("http://" + addr + *browserPath)
		}

		fmt.Println("Go live server listening on " + addr) // @TODO make this only print after successful listen
		serveError := fasthttp.ListenAndServe(addr, requestHandler)
		if serveError != nil {
			// isn't there a good way to check error?
			str := serveError.Error()
			if regexp.MustCompile(`Only one usage of each socket address`).FindString(str) != "" {
				portInt, _ := strconv.Atoi(*port)
				*port = fmt.Sprintf("%v", portInt+1)
				fmt.Println("Port taken, trying another port")
				tryAddress()
			} else {
				println(serveError.Error())
			}
			return
		}
	}
	tryAddress()
}
