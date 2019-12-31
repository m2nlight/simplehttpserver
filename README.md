# simplehttpserver

A simple static file http server

## Features

- Supports multiple paths mapping
- Supports angular router
- Supports custom index files
- Supports TLS (HTTPS)
- Supports basic authorize
- Supports compress
- Supports log file and colorful output
- Supports upload files

## Run

### Simple to run

Current file path mapping to web root, default port is 8080.

```sh
./simplehttpserver
```

Browse <http://localhost:8080>

### Supports angular router

The `dist` path mapping to web root, port to 4200, index file is index.html and supports fallback to index.html

```sh
./simplehttpserver -addr :4200 -path dist -indexnames index.html -fallback index.html
```

### Supports TLS

First, you can use [mkcert](https://github.com/FiloSottile/mkcert/releases) to create the cert-files for develop.

```sh
mkcert -install
mkcert -cert-file ssl-cert.pem -key-file ssl-cert.key localhost 127.0.0.1 ::1
```

Then run command

```sh
./simplehttpserver -addrtls :8081 -certfile ssl-cert.pem -keyfile ssl-cert.key -username admin -password admin -logfile 1.log
```

Browse <https://localhost:8081>

### Configuration file

1. Make a config file

    ```sh
    ./simplehttpserver -makeconfig config.yaml
    ```

2. Edit config.yaml

    You can add multiple paths in the config file.

    Line starts with `#` is a comment line.

    ```yaml
    addr: 0.0.0.0:8080
    #addrtls: 0.0.0.0:8081
    #certfile: ./ssl-cert.pem
    #keyfile: ./ssl-cert.key
    #username: admin
    #password: admin
    compress: false
    paths:
      #/c: "C:\\"
      #/d: "D:\\"
    indexnames:
      - index.html
      - index.htm
    verbose: true
    enablecolor: true
    enableupload: true
    ## maxrequestbodysize:0 to default size
    #maxrequestbodysize: 4294967296
    #logfile: ./simplehttpserver.log
    #fallback: ./index.html
    #HTTP_PROXY:
    #HTTPS_PROXY:
    #NO_PROXY: ::1,127.0.0.1,localhost
    ```

3. Run with the config file

    ```sh
    ./simplehttpserver -config config.yaml
    ```

### Get help

```sh
./simplehttpserver -help
```

## Build

```sh
export GOPATH="$(go env GOPATH)"
export PATH="$GOPATH/bin:$PATH"
go get -u -v github.com/mitchellh/gox
gox -output "bin/{{.Dir}}_{{.OS}}_{{.Arch}}" -os "linux darwin windows" -arch "amd64"
```

## Thanks

Powered by

- [Golang](https://golang.org)
- [fasthttp](https://github.com/valyala/fasthttp)
- [fatih/color](https://github.com/fatih/color)
- [go-yaml](https://github.com/go-yaml/yaml)
