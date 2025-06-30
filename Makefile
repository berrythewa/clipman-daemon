.PHONY: build build-all test clean install-local install uninstall release scripts lint deps

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    ?= $(shell date +%FT%T%z)

# Build flags
LDFLAGS := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.buildTime=$(DATE)"
GCFLAGS := -gcflags=all="-N -l"  # Disable optimizations and inlining for debugging
BUILDFLAGS := $(LDFLAGS) $(GCFLAGS)

# Platform-specific build settings
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
    CGO_ENABLED := 1
    PLATFORM_FLAGS := 
endif
ifeq ($(UNAME_S),Darwin)
    CGO_ENABLED := 1
    PLATFORM_FLAGS := -ldflags "-framework Cocoa -framework Foundation"
endif
ifeq ($(OS),Windows_NT)
    CGO_ENABLED := 1
    PLATFORM_FLAGS := 
endif

# Export CGO setting
export CGO_ENABLED

# Build both CLI and daemon
build-all: build-cli build-daemon

# Build with X11 dependencies (Linux only)
build-x11: check-x11-deps build-all

# Check X11 dependencies
check-x11-deps:
	@echo "Checking X11 dependencies..."
	@if [ "$(UNAME_S)" != "Linux" ]; then \
		echo "X11 build only available on Linux"; \
		exit 1; \
	fi
	@if ! pkg-config --exists x11 xfixes; then \
		echo "Missing X11 dependencies. Please install:"; \
		echo "  Ubuntu/Debian: sudo apt-get install libx11-dev libxfixes-dev pkg-config"; \
		echo "  Arch Linux: sudo pacman -S libx11 libxfixes pkg-config"; \
		echo "  Fedora: sudo dnf install libX11-devel libXfixes-devel pkg-config"; \
		exit 1; \
	fi
	@echo "X11 dependencies found ✓"

# Build the CLI tool
build-cli:
	@echo "Building Clipman CLI v$(VERSION) for $(UNAME_S)..."
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@CGO_ENABLED=$(CGO_ENABLED) go build $(BUILDFLAGS) $(PLATFORM_FLAGS) -o bin/clipman cmd/clipman/main.go

# Build the daemon
build-daemon:
	@echo "Building Clipman Daemon v$(VERSION) for $(UNAME_S)..."
	@echo "CGO_ENABLED=$(CGO_ENABLED)"
	@CGO_ENABLED=$(CGO_ENABLED) go build $(BUILDFLAGS) $(PLATFORM_FLAGS) -o bin/clipmand cmd/clipmand/main.go

# Default build target (builds daemon)
build: build-daemon

# Debug builds with additional debugging information
build-debug: GCFLAGS := -gcflags=all="-N -l" -race
build-debug: build-all

# Install locally (no root required)
install-local: build-all
	@echo "Installing Clipman locally..."
	@mkdir -p $(HOME)/.local/bin
	@cp bin/clipman $(HOME)/.local/bin/
	@cp bin/clipmand $(HOME)/.local/bin/
	@echo "Installed to $(HOME)/.local/bin/"

# System-wide installation
install: build-all
	@echo "Installing Clipman v$(VERSION)..."
	@scripts/install.sh

# Install using Go
install-go:
	@echo "Installing Clipman using Go install..."
	@CGO_ENABLED=$(CGO_ENABLED) go install $(LDFLAGS) $(PLATFORM_FLAGS) ./cmd/clipman
	@CGO_ENABLED=$(CGO_ENABLED) go install $(LDFLAGS) $(PLATFORM_FLAGS) ./cmd/clipmand

# Uninstall
uninstall:
	@echo "Uninstalling Clipman..."
	@scripts/uninstall.sh

# Run tests
test:
	@echo "Running tests..."
	@CGO_ENABLED=$(CGO_ENABLED) go test -v ./...

# Run tests with race detection
test-race:
	@echo "Running tests with race detection..."
	@CGO_ENABLED=1 go test -race -v ./...

# Run CLI-based tests
test-config: build-cli
	@echo "Running config tests script..."
	@./scripts/test/test_config_cli.sh

# Run all tests (unit + CLI)
test-all: test test-config
	@echo "All tests completed!"

# Run linters
lint:
	@echo "Running linters..."
	@if command -v golangci-lint >/dev/null 2>&1; then \
		golangci-lint run ./...; \
	else \
		echo "golangci-lint not found. Installing..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(go env GOPATH)/bin v1.55.2; \
		golangci-lint run ./...; \
	fi

# Update dependencies
deps:
	@echo "Updating dependencies..."
	@go mod tidy
	@go mod verify

# Clean build artifacts
clean:
	@echo "Cleaning up..."
	@rm -rf bin release

# Build release packages with proper cross-compilation
release:
	@echo "Building release packages for Clipman v$(VERSION)..."
	@mkdir -p release
	
	# Linux builds
	@echo "Building for Linux..."
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o release/clipman-linux-amd64 cmd/clipman/main.go
	@CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o release/clipmand-linux-amd64 cmd/clipmand/main.go
	@CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o release/clipman-linux-arm64 cmd/clipman/main.go
	@CGO_ENABLED=1 GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o release/clipmand-linux-arm64 cmd/clipmand/main.go
	
	# macOS builds (requires macOS host for CGO with Objective-C)
	@echo "Building for macOS..."
	@if [ "$(UNAME_S)" = "Darwin" ]; then \
		CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -ldflags "-framework Cocoa -framework Foundation" -o release/clipman-darwin-amd64 cmd/clipman/main.go; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -ldflags "-framework Cocoa -framework Foundation" -o release/clipmand-darwin-amd64 cmd/clipmand/main.go; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -ldflags "-framework Cocoa -framework Foundation" -o release/clipman-darwin-arm64 cmd/clipman/main.go; \
		CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -ldflags "-framework Cocoa -framework Foundation" -o release/clipmand-darwin-arm64 cmd/clipmand/main.go; \
	else \
		echo "Skipping macOS builds (requires macOS host for CGO with Objective-C)"; \
	fi
	
	# Windows builds
	@echo "Building for Windows..."
	@CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/clipman-windows-amd64.exe cmd/clipman/main.go
	@CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o release/clipmand-windows-amd64.exe cmd/clipmand/main.go
	
	@echo "Release packages built in ./release/"

# Check build environment
check-env:
	@echo "Build Environment Check:"
	@echo "  OS: $(UNAME_S)"
	@echo "  CGO_ENABLED: $(CGO_ENABLED)"
	@echo "  Go version: $(shell go version)"
	@echo "  Git version: $(VERSION)"
	@echo "  Platform flags: $(PLATFORM_FLAGS)"

# Make scripts executable
scripts:
	@echo "Setting execute permissions for scripts..."
	@chmod +x scripts/*.sh
