#!/bin/bash
# AIFuel Installer Bootstrap
# Usage: curl -fsSL https://raw.githubusercontent.com/robertogogoni/aifuel/master/install.sh | bash
set -euo pipefail

REPO="robertogogoni/aifuel"
INSTALL_DIR="$HOME/.local/bin"

# Colors (Catppuccin Mocha)
PEACH='\033[38;2;250;179;135m'
GREEN='\033[38;2;166;227;161m'
YELLOW='\033[38;2;249;226;175m'
RED='\033[38;2;243;139;168m'
DIM='\033[38;2;108;112;134m'
BOLD='\033[1m'
RESET='\033[0m'

info()  { printf "  ${GREEN}%s${RESET} %s\n" "$1" "$2"; }
warn()  { printf "  ${YELLOW}!${RESET} %s\n" "$1"; }
fail()  { printf "  ${RED}%s${RESET} %s\n" "x" "$1"; exit 1; }

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)       ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)            fail "Unsupported architecture: $ARCH" ;;
esac

printf "\n  ${PEACH}${BOLD}⛽ AIFuel Installer${RESET}\n\n"
info "System:" "${OS}/${ARCH}"

# Ensure install directory exists and is in PATH
mkdir -p "$INSTALL_DIR"
case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        warn "$INSTALL_DIR is not in your PATH"
        printf "     Add to your shell profile:\n"
        printf "     ${DIM}export PATH=\"\$HOME/.local/bin:\$PATH\"${RESET}\n\n"
        ;;
esac

# Check dependencies
for dep in jq curl; do
    if ! command -v "$dep" &>/dev/null; then
        warn "$dep not found (required by aifuel scripts)"
    fi
done

# Fetch latest release info
info "Fetching:" "latest release from GitHub..."
LATEST=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" 2>/dev/null \
    | grep -o '"tag_name": *"[^"]*"' | cut -d'"' -f4 || echo "")

if [ -n "$LATEST" ]; then
    BIN_NAME="aifuel-${OS}-${ARCH}"
    BIN_URL="https://github.com/${REPO}/releases/download/${LATEST}/${BIN_NAME}"
    CHECKSUM_URL="https://github.com/${REPO}/releases/download/${LATEST}/checksums.txt"

    info "Version:" "$LATEST"
    info "Binary:" "$BIN_NAME"

    # Download binary
    TMP=$(mktemp)
    trap 'rm -f "$TMP" "$TMP.checksums"' EXIT

    if curl -fsSL "$BIN_URL" -o "$TMP" 2>/dev/null; then
        # Verify checksum
        if curl -fsSL "$CHECKSUM_URL" -o "$TMP.checksums" 2>/dev/null; then
            EXPECTED=$(grep "$BIN_NAME" "$TMP.checksums" | awk '{print $1}')
            ACTUAL=$(sha256sum "$TMP" | awk '{print $1}')
            if [ -n "$EXPECTED" ] && [ "$EXPECTED" = "$ACTUAL" ]; then
                info "Checksum:" "verified"
            elif [ -n "$EXPECTED" ]; then
                fail "Checksum mismatch (expected: ${EXPECTED:0:16}..., got: ${ACTUAL:0:16}...)"
            fi
        fi

        mv "$TMP" "${INSTALL_DIR}/aifuel"
        chmod +x "${INSTALL_DIR}/aifuel"
        info "Installed:" "${INSTALL_DIR}/aifuel"
        printf "\n"
        exec "${INSTALL_DIR}/aifuel" install
    fi

    warn "Binary download failed, falling back to source build..."
fi

# Fallback: build from source
if ! command -v go &>/dev/null; then
    fail "Go is required to build from source. Install: https://go.dev/dl/"
fi

info "Building:" "from source..."
TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

git clone --depth 1 "https://github.com/${REPO}.git" "$TMPDIR/aifuel" 2>/dev/null
cd "$TMPDIR/aifuel"

# Get version from git tag for ldflags
GIT_VERSION=$(git describe --tags --always 2>/dev/null || echo "dev")
go build -ldflags "-s -w -X main.version=${GIT_VERSION}" \
    -o "${INSTALL_DIR}/aifuel" ./cmd/aifuel/

chmod +x "${INSTALL_DIR}/aifuel"
info "Built:" "${INSTALL_DIR}/aifuel (${GIT_VERSION})"
printf "\n"
exec "${INSTALL_DIR}/aifuel" install
