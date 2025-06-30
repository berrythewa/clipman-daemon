#!/bin/bash

# Script to install dependencies for Clipman Daemon with direct clipboard support

set -e

echo "🔧 Installing dependencies for Clipman Daemon with direct clipboard support..."

# Detect distribution
if [ -f /etc/os-release ]; then
    . /etc/os-release
    OS=$NAME
    VER=$VERSION_ID
else
    echo "❌ Could not detect OS distribution"
    exit 1
fi

echo "📦 Detected OS: $OS $VER"

# Install dependencies based on distribution
case $OS in
    "Ubuntu"|"Debian GNU/Linux"|"Linux Mint")
        echo "📦 Installing dependencies for Ubuntu/Debian..."
        sudo apt-get update
        sudo apt-get install -y \
            libx11-dev \
            libxfixes-dev \
            pkg-config \
            build-essential \
            git
        ;;
    "Arch Linux"|"Manjaro Linux")
        echo "📦 Installing dependencies for Arch Linux..."
        sudo pacman -S --noconfirm \
            libx11 \
            libxfixes \
            pkg-config \
            base-devel \
            git
        ;;
    "Fedora"|"Red Hat Enterprise Linux"|"CentOS Linux")
        echo "📦 Installing dependencies for Fedora/RHEL/CentOS..."
        if command -v dnf &> /dev/null; then
            sudo dnf install -y \
                libX11-devel \
                libXfixes-devel \
                pkg-config \
                gcc \
                git
        elif command -v yum &> /dev/null; then
            sudo yum install -y \
                libX11-devel \
                libXfixes-devel \
                pkg-config \
                gcc \
                git
        fi
        ;;
    "openSUSE"|"SUSE Linux")
        echo "📦 Installing dependencies for openSUSE..."
        sudo zypper install -y \
            libX11-devel \
            libXfixes-devel \
            pkg-config \
            gcc \
            git
        ;;
    *)
        echo "⚠️  Unsupported distribution: $OS"
        echo "Please install the following packages manually:"
        echo "  - libx11-dev / libX11-devel"
        echo "  - libxfixes-dev / libXfixes-devel"
        echo "  - pkg-config"
        echo "  - build tools (gcc, make)"
        echo "  - git"
        exit 1
        ;;
esac

# Verify installation
echo "🔍 Verifying installation..."

if ! pkg-config --exists x11; then
    echo "❌ X11 development libraries not found"
    exit 1
fi

if ! pkg-config --exists xfixes; then
    echo "❌ XFixes development libraries not found"
    exit 1
fi

if ! command -v pkg-config &> /dev/null; then
    echo "❌ pkg-config not found"
    exit 1
fi

if ! command -v gcc &> /dev/null; then
    echo "❌ gcc not found"
    exit 1
fi

echo "✅ All dependencies installed successfully!"
echo ""
echo "🎉 You can now build Clipman Daemon with direct clipboard support:"
echo "   make build-x11"
echo ""
echo "📋 Optional CLI tools for fallback support:"
echo "   - xclip: sudo apt-get install xclip (Ubuntu/Debian)"
echo "   - xsel: sudo apt-get install xsel (Ubuntu/Debian)"
echo "   - wl-clipboard: sudo apt-get install wl-clipboard (Ubuntu/Debian)" 