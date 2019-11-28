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

	"github.com/valyala/fasthttp"
	"gopkg.in/yaml.v3"
)

const (
	// Version information
	Version = "SimpleHttpServer v1.1-beta.1"
	// HTTPProxy returns HTTP_PROXY
	HTTPProxy = "HTTP_PROXY"
	// HTTPSProxy returns HTTPS_PROXY
	HTTPSProxy = "HTTPS_PROXY"
	// NoProxy returns NO_PROXY
	NoProxy = "NO_PROXY"
)

var (
	version         = flag.Bool("version", false, "output version only")
	addr            = flag.String("addr", "", "TCP address to listen to")
	addrTLS         = flag.String("addrtls", "", "TCP address to listen to TLS (aka SSL or HTTPS) requests. Leave empty for disabling TLS")
	certFile        = flag.String("certfile", "", "Path to TLS certificate file")
	keyFile         = flag.String("keyfile", "", "Path to TLS key file")
	compress        = flag.String("compress", "", "Whether to enable transparent response compression. exp: true")
	username        = flag.String("username", "", "Username for basic authentication")
	password        = flag.String("password", "", "Password for basic authentication")
	path            = flag.String("path", "", "local path to map to webroot. exp: ./")
	configFile      = flag.String("configfile", "config.yaml", "config file path.")
	verbose         = flag.String("verbose", "", "print verbose log. exp: true")
	logFile         = flag.String("logfile", "", "output to logfile")
	config          = &Config{}
	fsMap           = make(map[string]fasthttp.RequestHandler)
	enableBasicAuth = false
)

// Config from config.yaml
type Config struct {
	Addr       string
	AddrTLS    string
	CertFile   string
	KeyFile    string
	Username   string
	Password   string
	Compress   bool
	Paths      map[string]string
	Verbose    bool
	LogFile    string
	HTTPProxy  string `yaml:"HTTP_PROXY,omitempty"`
	HTTPSProxy string `yaml:"HTTPS_PROXY,omitempty"`
	NoProxy    string `yaml:"NO_PROXY,omitempty"`
}

func main() {
	// parse flags
	flag.Parse()
	if *version {
		fmt.Println(Version)
		return
	}
	// load config file
	fmt.Println(Version)
	if len(*configFile) > 0 {
		data, err := ioutil.ReadFile(*configFile)
		if err != nil {
			log.Fatalf("error: %v", err)
		}
		// parse yaml
		err = yaml.Unmarshal(data, &config)
		if err != nil {
			log.Fatalf("error: %v", err)
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
	case "":
	case "true":
		config.Compress = true
	case "false":
		config.Compress = false
	default:
		log.Fatalf("error: %v", fmt.Errorf("argument compress error"))
	}
	if len(*path) > 0 {
		config.Paths["/"] = *path
	}
	switch strings.ToLower(*verbose) {
	case "":
	case "true":
		config.Verbose = true
	case "false":
		config.Verbose = false
	default:
		log.Fatalf("error: %v", fmt.Errorf("argument verbose error"))
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
			IndexNames:         []string{"index.html"},
			GenerateIndexPages: true,
			AcceptByteRange:    true,
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
				log.Printf("%d | %q | %q | %q | %s | %s \n",
					ctx.Response.StatusCode(), ctx.RemoteIP(), ctx.Method(), ctx.Path(), user, pwd)
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
		log.Printf("%d | %q | %q | %q\n", ctx.Response.StatusCode(), ctx.RemoteIP(), ctx.Method(), ctx.Path())
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
