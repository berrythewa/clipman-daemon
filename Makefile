.PHONY: build test clean

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date +%FT%T%z)
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(DATE)"

build:
	@echo "Building Clipman Daemon v$(VERSION)..."
	@go build $(LDFLAGS) -o bin/clipmand cmd/clipmand/main.go

install:
	@echo "Installing Clipman Daemon v$(VERSION)..."
	@go install $(LDFLAGS) ./cmd/clipmand

test:
	@echo "Running tests..."
	@go test ./...

clean:
	@echo "Cleaning up..."
	@rm -rf bin
