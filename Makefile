.PHONY: build test clean install-local install uninstall release scripts

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date +%FT%T%z)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(DATE)"

build:
	@echo "Building Clipman v$(VERSION)..."
	@go build $(LDFLAGS) -o bin/clipman cmd/clipmand/main.go

install: build
	@echo "Installing Clipman v$(VERSION)..."
	@scripts/install.sh

install-go:
	@echo "Installing Clipman using Go install..."
	@go install $(LDFLAGS) -o $(GOPATH)/bin/clipman ./cmd/clipmand

uninstall:
	@echo "Uninstalling Clipman..."
	@scripts/uninstall.sh

test:
	@echo "Running tests..."
	@go test ./...

clean:
	@echo "Cleaning up..."
	@rm -rf bin release

release:
	@echo "Building release packages for Clipman v$(VERSION)..."
	@mkdir -p release
	@GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o release/clipman-linux-amd64 cmd/clipmand/main.go
	@GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o release/clipman-linux-arm64 cmd/clipmand/main.go
	@GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o release/clipman-darwin-amd64 cmd/clipmand/main.go
	@GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o release/clipman-darwin-arm64 cmd/clipmand/main.go
	@GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/clipman-windows-amd64.exe cmd/clipmand/main.go
	@echo "Release packages built in ./release/"

scripts:
	@echo "Setting execute permissions for scripts..."
	@chmod +x scripts/*.sh
