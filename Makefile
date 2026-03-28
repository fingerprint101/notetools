BINARY := nt
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build install clean test

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

install: build
	install -m 755 $(BINARY) /usr/local/bin/nt

test:
	go test ./...

clean:
	rm -f $(BINARY) nt-linux-* nt-darwin-*

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o nt-linux-amd64 .

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o nt-darwin-arm64 .
