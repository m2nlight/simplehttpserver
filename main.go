package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/fatih/color"
	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v2"
)

const (
	// Version information
	Version = "SimpleHttpServer v1.2-beta.1"
	// HTTPProxy returns HTTP_PROXY
	HTTPProxy = "HTTP_PROXY"
	// HTTPSProxy returns HTTPS_PROXY
	HTTPSProxy = "HTTPS_PROXY"
	// NoProxy returns NO_PROXY
	NoProxy = "NO_PROXY"
)

var (
	version         = flag.Bool("version", false, "Output version only")
	addr            = flag.String("addr", "", "TCP address to listen. e.g.:0.0.0.0:8080")
	addrTLS         = flag.String("addrtls", "", "TCP address to listen to TLS (aka SSL or HTTPS) requests. Leave empty for disabling TLS")
	certFile        = flag.String("certfile", "", "Path to TLS certificate file")
	keyFile         = flag.String("keyfile", "", "Path to TLS key file")
	compress        = flag.String("compress", "", "Whether to enable transparent response compression. e.g.: true")
	username        = flag.String("username", "", "Username for basic authentication")
	password        = flag.String("password", "", "Password for basic authentication")
	path            = flag.String("path", "", "Local path to map to webroot. e.g.: ./")
	indexNames      = flag.String("indexnames", "", "List of index file names. e.g.: index.html,index.htm")
	configFile      = flag.String("config", "", "The config file path.")
	verbose         = flag.String("verbose", "", "Print verbose log. e.g.: false")
	logFile         = flag.String("logfile", "", "Output to logfile")
	fallback        = flag.String("fallback", "", "Fallback to some file. e.g.: If you serve a angular project, you can set it ./index.html")
	enableColor     = flag.String("enablecolor", "", "Enable color output by http status code. e.g.: false")
	makeconfig      = flag.String("makeconfig", "", "Make a config file. e.g.: config.yaml")
	config          = &Config{}
	fsMap           = make(map[string]fasthttp.RequestHandler)
	enableBasicAuth = false
	logMutex        sync.Mutex
)

// Config from config.yaml
type Config struct {
	Addr        string
	AddrTLS     string
	CertFile    string
	KeyFile     string
	Username    string
	Password    string
	Compress    bool
	Paths       map[string]string
	IndexNames  []string
	Verbose     bool
	LogFile     string
	Fallback    string
	EnableColor bool
	HTTPProxy   string `yaml:"HTTP_PROXY,omitempty"`
	HTTPSProxy  string `yaml:"HTTPS_PROXY,omitempty"`
	NoProxy     string `yaml:"NO_PROXY,omitempty"`
}

func main() {
	// parse flags
	flag.Parse()
	if *version {
		fmt.Println(Version)
		return
	}
	// make config file
	if len(*makeconfig) > 0 {
		err := makeConfigFile(*makeconfig)
		if err != nil {
			log.Fatalf("error: %v\n", err)
		} else {
			log.Printf("The config file %s is created\n", *makeconfig)
		}
		return
	}
	// load config file
	fmt.Println(Version)
	if len(*configFile) > 0 {
		log.Println("Load config from:", *configFile)
		data, err := ioutil.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("error: %v\n", err)
		}
		// parse yaml
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Fatalf("error: %v\n", err)
		}
	}
	if config.Paths == nil {
		config.Paths = make(map[string]string)
	}
	// set output to logfile
	if len(*logFile) > 0 {
		config.LogFile = *logFile
	}
	if err := tryEnableLogFile(); err != nil {
		log.Fatalf("error: %v", err)
	}
	// overwrite config
	if len(*addr) > 0 {
		config.Addr = *addr
	}
	if len(*addrTLS) > 0 {
		config.AddrTLS = *addrTLS
	}
	if len(*certFile) > 0 {
		config.CertFile = *certFile
	}
	if len(*keyFile) > 0 {
		config.KeyFile = *keyFile
	}
	if len(*username) > 0 {
		config.Username = *username
	}
	if len(*password) > 0 {
		config.Password = *password
	}
	if len(config.Username) > 0 && len(config.Password) > 0 {
		enableBasicAuth = true
	}
	switch strings.ToLower(*compress) {
	case "true":
		config.Compress = true
	case "":
		fallthrough
	case "false":
		config.Compress = false
	default:
		log.Fatalf("error: %v", fmt.Errorf("argument compress error"))
	}
	if len(*path) > 0 {
		config.Paths["/"] = *path
	}
	if len(*indexNames) > 0 {
		config.IndexNames = strings.Split(*indexNames, ",")
	}
	switch strings.ToLower(*verbose) {
	case "":
		fallthrough
	case "true":
		config.Verbose = true
	case "false":
		config.Verbose = false
	default:
		log.Fatalf("error: %v", fmt.Errorf("argument verbose error"))
	}
	if len(*fallback) > 0 {
		config.Fallback = *fallback
	}
	switch strings.ToLower(*enableColor) {
	case "":
		fallthrough
	case "true":
		config.EnableColor = true
	case "false":
		config.EnableColor = false
	default:
		log.Fatalf("error: %v", fmt.Errorf("argument enablecolor error"))
	}
	// config proxy
	if len(config.HTTPProxy) > 0 {
		_ = os.Setenv(HTTPProxy, config.HTTPProxy)
	}
	printEnv(HTTPProxy)

	if len(config.HTTPSProxy) > 0 {
		_ = os.Setenv(HTTPSProxy, config.HTTPSProxy)
	}
	printEnv(HTTPSProxy)

	if len(config.NoProxy) > 0 {
		_ = os.Setenv(NoProxy, config.NoProxy)
	}
	printEnv(NoProxy)
	// run server and output config
	h := requestHandler
	if config.Compress {
		h = fasthttp.CompressHandler(h)
	}
	if len(config.Addr) == 0 && len(config.AddrTLS) == 0 {
		config.Addr = ":8080"
	}
	if len(config.Addr) > 0 {
		log.Println("Server address:", config.Addr)
		go func() {
			if err := fasthttp.ListenAndServe(config.Addr, h); err != nil {
				log.Fatalf("error in ListenAndServe: %s", err)
			}
		}()
	}
	if len(config.AddrTLS) > 0 {
		log.Println("Server address TLS:", config.AddrTLS)
		log.Println("CertFile:", config.CertFile)
		log.Println("KeyFile:", config.KeyFile)
		go func() {
			if err := fasthttp.ListenAndServeTLS(config.AddrTLS, config.CertFile, config.KeyFile, h); err != nil {
				log.Fatalf("error in ListenAndServeTLS: %s", err)
			}
		}()
	}
	log.Println("BasicAuth:", enableBasicAuth)
	log.Println("Compress:", config.Compress)
	if len(config.Fallback) > 0 {
		log.Println("Fallback:", config.Fallback)
	}
	log.Println("EnableColor:", config.EnableColor)
	indexNamesLen := len(config.IndexNames)
	if indexNamesLen > 0 {
		log.Printf("Have %d index name(s):\n", indexNamesLen)
		for _, v := range config.IndexNames {
			log.Println("  ", v)
		}
	} else {
		log.Println("No any index names")
	}
	// map paths
	if len(config.Paths) == 0 {
		config.Paths["/"] = "."
	}
	for k, v := range config.Paths {
		if !strings.HasPrefix(k, "/") {
			log.Printf("%s -> %s [ignored] URI path should start with '/'\n", k, v)
			continue
		}
		if strings.HasPrefix(v, ".") {
			if abs, err := filepath.Abs(v); err == nil {
				v = abs
				config.Paths[k] = v
			}
		}

		fs := &fasthttp.FS{
			Root:               v,
			IndexNames:         config.IndexNames,
			GenerateIndexPages: true,
			AcceptByteRange:    true,
		}
		if len(config.Fallback) > 0 {
			fs.PathNotFound = func(ctx *fasthttp.RequestCtx) {
				fallbackpath := filepath.Join(v, config.Fallback)
				if _, err := os.Stat(fallbackpath); err == nil {
					ctx.SendFile(fallbackpath)
				} else {
					ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
					if config.Verbose {
						statusCode := ctx.Response.StatusCode()
						go logInfo(statusCode, "%d | %s | %s | %s\n", statusCode, ctx.RemoteIP(), ctx.Method(), ctx.Path())
					}
				}
			}
		}
		if k != "/" {
			fs.PathRewrite = fasthttp.NewPathPrefixStripper(len(k))
		}
		fsMap[k] = fs.NewRequestHandler()
		// print map paths
		log.Printf("%s -> %s\n", k, v)
	}
	if len(fsMap) > 1 {
		if v, found := config.Paths["/"]; found {
			log.Printf("/ -> %s [ignored] root path overwrite /\n", v)
		}
	}

	// Wait forever.
	select {}
}

func printEnv(env string) {
	v := os.Getenv(env)
	if len(v) > 0 {
		log.Printf("%s: %s\n", env, v)
	}
}

func requestHandler(ctx *fasthttp.RequestCtx) {
	// auth
	if enableBasicAuth {
		user, pwd, ok := basicAuth(ctx)
		if !ok || user != config.Username || pwd != config.Password {
			ctx.Error(fasthttp.StatusMessage(fasthttp.StatusUnauthorized), fasthttp.StatusUnauthorized)
			ctx.Response.Header.Set("WWW-Authenticate", "Basic realm=Restricted")
			// log print
			if config.Verbose {
				statusCode := ctx.Response.StatusCode()
				go logInfo(statusCode, "%d | %s | %s | %s | %s | %s \n",
					statusCode, ctx.RemoteIP(), ctx.Method(), ctx.Path(), user, pwd)
			}
			return
		}
	}

	// router
	switch string(ctx.Method()) {
	case "POST":
		switch string(ctx.Path()) {
		case "/ping":
			{
				fmt.Fprintf(ctx, `{"message":"pong","time":"`+ctx.Time().String()+`"}`)
				ctx.SetContentType("application/json; charset=utf8")
			}
		default:
			ctx.Error(fasthttp.StatusMessage(fasthttp.StatusBadRequest), fasthttp.StatusBadRequest)
		}
	case "GET":
		fsHandler(ctx)
	default:
		ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
	}

	// log print
	if config.Verbose {
		statusCode := ctx.Response.StatusCode()
		go logInfo(statusCode, "%d | %s | %s | %s\n", statusCode, ctx.RemoteIP(), ctx.Method(), ctx.Path())
	}
}

func fsHandler(ctx *fasthttp.RequestCtx) {
	path := string(ctx.Path())
	if path == "/" && len(fsMap) > 1 {
		fmt.Fprintf(ctx, "<html><head><title>root</title></head><body><h1>root</h1><ul>")
		for k, v := range config.Paths {
			if !strings.HasPrefix(k, "/") || k == "/" {
				continue
			}
			fmt.Fprintf(ctx, `<li><a href="%s">%s</a> -> %s</li>`, k, k, v)
		}
		fmt.Fprintf(ctx, "</ul></body></html>")
		ctx.SetContentType("text/html; charset=utf8")
		return
	}

	for k, handler := range fsMap {
		if strings.HasPrefix(path, k) {
			handler(ctx)
			mimeType := staticFileGetMimeType(filepath.Ext(path))
			if len(mimeType) > 0 {
				ctx.SetContentType(mimeType)
			}
			return
		}
	}

	ctx.Error(fasthttp.StatusMessage(fasthttp.StatusNotFound), fasthttp.StatusNotFound)
}

func basicAuth(ctx *fasthttp.RequestCtx) (username, password string, ok bool) {
	auth := ctx.Request.Header.Peek("Authorization")
	if auth == nil {
		return
	}

	const prefix = "Basic "
	authStr := string(auth)
	if !strings.HasPrefix(authStr, prefix) {
		return
	}
	c, err := base64.StdEncoding.DecodeString(authStr[len(prefix):])
	if err != nil {
		return
	}
	cs := string(c)
	s := strings.IndexByte(cs, ':')
	if s < 0 {
		return
	}
	return cs[:s], cs[s+1:], true
}

func tryEnableLogFile() error {
	if len(config.LogFile) == 0 {
		return nil // no enable logfile and no returns error
	}
	path := filepath.Dir(config.LogFile)
	if err := os.MkdirAll(path, 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(config.LogFile, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0666)
	if err != nil {
		return err
	}
	_, err = file.WriteString(fmt.Sprintln(Version))
	if err != nil {
		return err
	}
	log.SetOutput(io.MultiWriter(file, os.Stdout))
	log.Println("LogFile:", config.LogFile)
	return nil
}

// fix https://github.com/golang/go/issues/32350
var builtinMimeTypesLower = map[string]string{
	".css":  "text/css; charset=utf-8",
	".gif":  "image/gif",
	".htm":  "text/html; charset=utf-8",
	".html": "text/html; charset=utf-8",
	".jpg":  "image/jpeg",
	".js":   "application/javascript",
	".wasm": "application/wasm",
	".pdf":  "application/pdf",
	".png":  "image/png",
	".svg":  "image/svg+xml",
	".xml":  "text/xml; charset=utf-8",
}

func staticFileGetMimeType(ext string) string {
	if v, ok := builtinMimeTypesLower[ext]; ok {
		return v
	}
	return ""
}

func getColor(statusCode int) color.Attribute {
	if statusCode >= 500 {
		return color.FgRed
	}
	if statusCode >= 400 {
		return color.FgMagenta
	}
	if statusCode >= 300 {
		return color.FgYellow
	}
	if statusCode >= 200 {
		return color.FgGreen
	}
	if statusCode >= 100 {
		return color.FgHiCyan
	}
	return color.FgBlue
}

func logInfo(statusCode int, format string, v ...interface{}) {
	if !config.EnableColor {
		log.Printf(format, v...)
	} else {
		logMutex.Lock()
		color.Set(getColor(statusCode))
		log.Printf(format, v...)
		color.Unset()
		logMutex.Unlock()
	}
}

func makeConfigFile(configfile string) error {
	if _, err := os.Stat(configfile); err == nil {
		return fmt.Errorf("The file %s exists", configfile)
	}

	file, err := os.Create(configfile)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.WriteString(
		`addr: 0.0.0.0:8080
#addrtls: 0.0.0.0:8081
#certfile: ./ssl-cert.pem
#keyfile: ./ssl-cert.key
#username: admin
#password: admin
compress: false
paths:
  # /c: "C:\\"
  # /d: "D:\\"
indexnames:
  - index.html
  - index.htm
verbose: true
enablecolor: true
#logfile: ./simplehttpserver.log
#fallback: ./index.html
#HTTP_PROXY:
#HTTPS_PROXY:
#NO_PROXY: ::1,127.0.0.1,localhost`)
	return err
}
