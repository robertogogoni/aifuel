#!/bin/bash
# AIFuel Installer Bootstrap
# Usage: curl -fsSL https://raw.githubusercontent.com/robertogogoni/aifuel/master/install.sh | bash
set -euo pipefail

REPO="robertogogoni/aifuel"
INSTALL_DIR="$HOME/.local/bin"

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

echo "⛽ AIFuel Installer"
echo "   Detecting system: ${OS}/${ARCH}"
echo ""

# Try to download prebuilt binary from GitHub releases
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4 2>/dev/null || echo "")

if [ -n "$LATEST" ]; then
    BIN_URL="https://github.com/${REPO}/releases/download/${LATEST}/aifuel-${OS}-${ARCH}"
    echo "   Downloading aifuel ${LATEST}..."
    mkdir -p "$INSTALL_DIR"
    if curl -fsSL "$BIN_URL" -o "${INSTALL_DIR}/aifuel" 2>/dev/null; then
        chmod +x "${INSTALL_DIR}/aifuel"
        echo "   Downloaded to ${INSTALL_DIR}/aifuel"
        echo ""
        exec "${INSTALL_DIR}/aifuel" install
    fi
fi

# Fallback: clone repo and build from source
echo "   No prebuilt binary available. Building from source..."
if ! command -v go &>/dev/null; then
    echo "   Error: Go is required to build from source."
    echo "   Install Go: https://go.dev/dl/"
    echo "   Or wait for a release with prebuilt binaries."
    exit 1
fi

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT
git clone --depth 1 "https://github.com/${REPO}.git" "$TMPDIR/aifuel"
cd "$TMPDIR/aifuel"
go build -o "${INSTALL_DIR}/aifuel" ./cmd/aifuel/
chmod +x "${INSTALL_DIR}/aifuel"
echo "   Built and installed to ${INSTALL_DIR}/aifuel"
echo ""
exec "${INSTALL_DIR}/aifuel" install
