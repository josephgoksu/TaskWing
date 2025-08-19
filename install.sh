#!/bin/bash
set -e

# TaskWing Installation Script
# Usage: curl -sSfL https://raw.githubusercontent.com/josephgoksu/taskwing.app/main/install.sh | sh

REPO="josephgoksu/taskwing.app"
BINARY_NAME="taskwing"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case $ARCH in
    x86_64) ARCH="x86_64" ;;
    amd64) ARCH="x86_64" ;;
    arm64) ARCH="arm64" ;;
    aarch64) ARCH="arm64" ;;
    armv7l) ARCH="armv7" ;;
    armv6l) ARCH="armv6" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

case $OS in
    linux) OS="Linux" ;;
    darwin) OS="Darwin" ;;
    *) echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Get latest release version
echo "🔍 Fetching latest release..."
LATEST_VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | cut -d'"' -f4)

if [ -z "$LATEST_VERSION" ]; then
    echo "❌ Failed to get latest version"
    exit 1
fi

echo "📦 Latest version: $LATEST_VERSION"

# Construct download URL
ARCHIVE_NAME="${BINARY_NAME}.app_${OS}_${ARCH}"
if [ "$OS" = "Linux" ]; then
    ARCHIVE_NAME="${ARCHIVE_NAME}.tar.gz"
else
    ARCHIVE_NAME="${ARCHIVE_NAME}.tar.gz"
fi

DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/$ARCHIVE_NAME"

# Create temp directory
TMP_DIR=$(mktemp -d)
cd "$TMP_DIR"

echo "⬇️  Downloading $ARCHIVE_NAME..."
curl -sSfL "$DOWNLOAD_URL" -o "$ARCHIVE_NAME"

echo "📂 Extracting archive..."
tar -xzf "$ARCHIVE_NAME"

# Find install directory
INSTALL_DIR="/usr/local/bin"
if [ ! -w "$INSTALL_DIR" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
fi

echo "🚀 Installing to $INSTALL_DIR..."
mv "$BINARY_NAME" "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/$BINARY_NAME"

# Cleanup
cd - > /dev/null
rm -rf "$TMP_DIR"

echo "✅ TaskWing installed successfully!"
echo ""
echo "📍 Installed to: $INSTALL_DIR/$BINARY_NAME"
echo ""
echo "🏁 Quick start:"
echo "   $BINARY_NAME init      # Initialize in your project"
echo "   $BINARY_NAME add       # Add your first task"
echo "   $BINARY_NAME --help    # See all commands"
echo ""

# Check if install dir is in PATH
if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
    echo "⚠️  Note: $INSTALL_DIR is not in your PATH"
    echo "   Add this to your shell profile:"
    echo "   export PATH=\"$INSTALL_DIR:\$PATH\""
    echo ""
fi

echo "🤖 For AI integration, see: https://github.com/$REPO#ai-integration"