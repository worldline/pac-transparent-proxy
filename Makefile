VERSION ?= develop
build:
	go build -ldflags="-s -w -X main.version=${VERSION}"

clean:
	go clean

run: 
	go run . http://127.0.0.1/proxy.pac
