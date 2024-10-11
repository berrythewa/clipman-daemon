.PHONY: build test clean

build:
	@echo "Building Clipman Daemon..."
	@go build -o bin/clipmand cmd/clipmand/main.go

test:
	@echo "Running tests..."
	@go test ./...

clean:
	@echo "Cleaning up..."
	@rm -rf bin
