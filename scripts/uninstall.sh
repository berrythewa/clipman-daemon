#!/bin/bash
set -e

# Clipman Uninstallation Script
echo "Clipman Uninstaller"
echo "===================="
echo

# Check if we're running as root and re-execute if needed
if [ "$EUID" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    echo "Requesting elevated privileges to uninstall Clipman..."
    sudo "$0" "$@"
    exit $?
  else
    echo "Error: This script needs to be run as root. Please re-run with sudo."
    exit 1
  fi
fi

# Set paths
BINARY_PATH="/usr/local/bin/clipman"

# Check if clipman exists
if [ ! -f "$BINARY_PATH" ]; then
  echo "Clipman is not installed at $BINARY_PATH"
else
  echo "Removing Clipman binary from $BINARY_PATH..."
  rm -f "$BINARY_PATH"
  echo "Binary removed."
fi

# Check for service files and ask to remove them
echo "Checking for service installations..."

SERVICE_FOUND=false

# Check systemd service (Linux)
if command -v systemctl >/dev/null 2>&1; then
  if systemctl list-unit-files | grep -q clipman.service; then
    SERVICE_FOUND=true
    echo "Found system systemd service."
    systemctl stop clipman.service 2>/dev/null || true
    systemctl disable clipman.service 2>/dev/null || true
    rm -f /etc/systemd/system/clipman.service 2>/dev/null || true
    systemctl daemon-reload 2>/dev/null || true
    echo "System service removed."
  fi
  
  if [ -d "$HOME/.config/systemd/user" ] && [ -f "$HOME/.config/systemd/user/clipman.service" ]; then
    SERVICE_FOUND=true
    echo "Found user systemd service."
    systemctl --user stop clipman.service 2>/dev/null || true
    systemctl --user disable clipman.service 2>/dev/null || true
    rm -f "$HOME/.config/systemd/user/clipman.service" 2>/dev/null || true
    systemctl --user daemon-reload 2>/dev/null || true
    echo "User service removed."
  fi
fi

# Check launchd service (macOS)
if command -v launchctl >/dev/null 2>&1; then
  if [ -f "/Library/LaunchDaemons/com.berrythewa.clipman.plist" ]; then
    SERVICE_FOUND=true
    echo "Found system launchd service."
    launchctl unload /Library/LaunchDaemons/com.berrythewa.clipman.plist 2>/dev/null || true
    rm -f /Library/LaunchDaemons/com.berrythewa.clipman.plist
    echo "System service removed."
  fi
  
  if [ -f "$HOME/Library/LaunchAgents/com.berrythewa.clipman.plist" ]; then
    SERVICE_FOUND=true
    echo "Found user launchd service."
    launchctl unload "$HOME/Library/LaunchAgents/com.berrythewa.clipman.plist" 2>/dev/null || true
    rm -f "$HOME/Library/LaunchAgents/com.berrythewa.clipman.plist"
    echo "User service removed."
  fi
fi

# Check for scheduled tasks (Windows - this won't run in bash but keeping for reference)
if command -v schtasks.exe >/dev/null 2>&1; then
  if schtasks.exe /Query /TN "Clipman" >/dev/null 2>&1; then
    SERVICE_FOUND=true
    echo "Found Windows scheduled task."
    schtasks.exe /Delete /TN "Clipman" /F >/dev/null 2>&1 || true
    echo "Scheduled task removed."
  fi
fi

if [ "$SERVICE_FOUND" = false ]; then
  echo "No service installations found."
fi

# Optionally remove configuration and data
read -p "Do you want to remove all Clipman data and configuration? (y/N): " REMOVE_DATA

if [[ "$REMOVE_DATA" =~ ^[Yy]$ ]]; then
  echo "Removing Clipman data and configuration..."
  
  # Remove data directory
  if [ -d "$HOME/.clipman" ]; then
    rm -rf "$HOME/.clipman"
    echo "Data directory removed."
  fi
  
  # Remove config directory
  CONFIG_DIR=""
  if [ -d "$HOME/.config/clipman" ]; then
    CONFIG_DIR="$HOME/.config/clipman"
  elif [ -d "$HOME/Library/Application Support/clipman" ]; then
    CONFIG_DIR="$HOME/Library/Application Support/clipman"
  fi
  
  if [ -n "$CONFIG_DIR" ]; then
    rm -rf "$CONFIG_DIR"
    echo "Configuration directory removed."
  fi
else
  echo "Keeping user data and configuration."
  echo "Data directory: $HOME/.clipman"
  echo "To manually remove, run: rm -rf $HOME/.clipman"
fi

echo
echo "Clipman has been uninstalled from your system." 