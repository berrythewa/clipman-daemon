#!/bin/bash
set -e

# Clipman Installation Script
# This script installs Clipman on Linux and macOS

echo "Clipman Installer"
echo "===================="
echo

# Check if we're running as root and re-execute if needed
if [ "$EUID" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Requesting elevated privileges to install Clipman..."
    sudo "$0" "$@"
    exit $?
  else
    echo "Error: This script needs to be run as root. Please re-run with sudo."
    exit 1
  fi
fi

# Detect OS
OS=$(uname -s)
ARCH=$(uname -m)

case "$ARCH" in
  x86_64)
    ARCH="amd64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "Warning: Unsupported architecture $ARCH. Installation may fail."
    ARCH="amd64" # Default to amd64
    ;;
esac

# Set installation directory
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="clipman"
BINARY_PATH="$INSTALL_DIR/$BINARY_NAME"

echo "Installing Clipman for $OS ($ARCH)..."

# Check if the binary already exists
if [ -f "$BINARY_PATH" ]; then
  echo "Clipman is already installed. Replacing existing installation..."
  rm -f "$BINARY_PATH"
fi

# If a precompiled binary is provided locally
if [ -f "bin/clipman" ]; then
  echo "Using local binary..."
  install -m 755 bin/clipman "$BINARY_PATH"
elif [ -f "release/clipman-$OS-$ARCH" ]; then
  echo "Using prebuilt release binary..."
  install -m 755 "release/clipman-$OS-$ARCH" "$BINARY_PATH"
else
  # Download binary from GitHub releases
  # TODO: Replace with actual release URL when available
  DOWNLOAD_URL="https://github.com/berrythewa/clipman-daemon/releases/latest/download/clipman-$OS-$ARCH"
  
  echo "Downloading Clipman from $DOWNLOAD_URL..."
  if command -v curl >/dev/null 2>&1; then
    curl -L -o /tmp/clipman "$DOWNLOAD_URL"
  elif command -v wget >/dev/null 2>&1; then
    wget -O /tmp/clipman "$DOWNLOAD_URL"
  else
    echo "Error: Neither curl nor wget found. Please install one of them and try again."
    exit 1
  fi
  
  install -m 755 /tmp/clipman "$BINARY_PATH"
  rm -f /tmp/clipman
fi

echo "Clipman installed successfully to $BINARY_PATH"
echo

# Ask about service installation
read -p "Do you want to install Clipman as a service to start automatically? (y/N): " INSTALL_SERVICE

if [[ "$INSTALL_SERVICE" =~ ^[Yy]$ ]]; then
  read -p "Install as system service (requires admin privileges) or user service? (s/U): " SERVICE_TYPE
  
  if [[ "$SERVICE_TYPE" =~ ^[Ss]$ ]]; then
    echo "Installing system service..."
    "$BINARY_PATH" service install --system --start
  else
    echo "Installing user service..."
    "$BINARY_PATH" service install --start
  fi
  
  echo "Service installation complete."
else
  echo "Skipping service installation."
  echo "To start Clipman manually, run: $BINARY_PATH"
  echo "To install as a service later, run: $BINARY_PATH service install"
fi

echo
echo "Installation complete! Enjoy using Clipman."
echo "For documentation, visit: https://github.com/berrythewa/clipman-daemon" 