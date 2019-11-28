# simplehttpserver

A simple static file http server

## Build

```sh
export GOPATH="$(go env GOPATH)"
export PATH="$GOPATH/bin:$PATH"
go get -u -v github.com/mitchellh/gox
gox -output "bin/{{.Dir}}_{{.OS}}_{{.Arch}}" -os "linux darwin windows" -arch "amd64"
cp config.yaml ./bin/
```
