#!/usr/bin/env bash
#
# Doku CLI Installation Script
#
# This script installs the doku CLI by downloading pre-built binaries from GitHub releases.
# Can also build from source with --source flag.
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash
#   ./scripts/install.sh                    # Install latest release
#   ./scripts/install.sh --source           # Build from source
#   ./scripts/install.sh --version v1.2.3   # Install specific version
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Default values
REPO="dokulabs/doku-cli"
VERSION="latest"
BUILD_FROM_SOURCE=false
INSTALL_DIR=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --source)
            BUILD_FROM_SOURCE=true
            shift
            ;;
        --version)
            VERSION="$2"
            shift 2
            ;;
        --dir)
            INSTALL_DIR="$2"
            shift 2
            ;;
        --help)
            echo "Doku CLI Installation Script"
            echo ""
            echo "Usage:"
            echo "  $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --source           Build from source instead of downloading binary"
            echo "  --version VERSION  Install specific version (default: latest)"
            echo "  --dir DIR          Install to specific directory"
            echo "  --help             Show this help message"
            echo ""
            echo "Examples:"
            echo "  $0                           # Install latest release"
            echo "  $0 --version v1.2.3          # Install specific version"
            echo "  $0 --source                  # Build from source"
            echo "  curl -fsSL https://raw.githubusercontent.com/dokulabs/doku-cli/main/scripts/install.sh | bash"
            exit 0
            ;;
        *)
            echo -e "${RED}Unknown option: $1${NC}"
            echo "Use --help for usage information"
            exit 1
            ;;
    esac
done

echo -e "${CYAN}╔════════════════════════════════════╗${NC}"
echo -e "${CYAN}║      Doku CLI Installer            ║${NC}"
echo -e "${CYAN}╚════════════════════════════════════╝${NC}"
echo ""

# Detect OS and architecture
OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
    Darwin*)
        OS="darwin"
        ;;
    Linux*)
        OS="linux"
        ;;
    MINGW*|MSYS*|CYGWIN*)
        OS="windows"
        ;;
    *)
        echo -e "${RED}✗ Unsupported operating system: $OS${NC}"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)
        ARCH="amd64"
        ;;
    aarch64|arm64)
        ARCH="arm64"
        ;;
    *)
        echo -e "${RED}✗ Unsupported architecture: $ARCH${NC}"
        exit 1
        ;;
esac

echo -e "${GREEN}✓ Detected platform: ${CYAN}${OS}/${ARCH}${NC}"

# Build from source if requested
if [ "$BUILD_FROM_SOURCE" = true ]; then
    echo ""
    echo -e "${CYAN}Building from source...${NC}"

    # Check if Go is installed
    if ! command -v go &> /dev/null; then
        echo -e "${RED}✗ Go is not installed${NC}"
        echo "Please install Go from https://golang.org/dl/"
        exit 1
    fi

    echo -e "${GREEN}✓ Go detected: $(go version)${NC}"

    # Get version information
    BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    if command -v git &> /dev/null && [ -d .git ]; then
        TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "v0.1.0-dev")
        COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

        if git describe --exact-match --tags HEAD &> /dev/null; then
            VERSION_INFO="$TAG"
        else
            COMMITS_AHEAD=$(git rev-list ${TAG}..HEAD --count)
            VERSION_INFO="${TAG}+${COMMITS_AHEAD}"
        fi
    else
        VERSION_INFO="v0.1.0-dev"
        COMMIT="unknown"
    fi

    LDFLAGS="-X main.Version=${VERSION_INFO} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}"

    echo -e "  Version: ${CYAN}${VERSION_INFO}${NC}"
    echo -e "  Commit:  ${CYAN}${COMMIT}${NC}"
    echo ""

    # Install
    go install -ldflags "${LDFLAGS}" ./cmd/doku

    INSTALL_PATH="$(go env GOPATH)/bin/doku"

    echo ""
    echo -e "${GREEN}✓ Build and installation complete!${NC}"
    echo -e "Installed to: ${CYAN}${INSTALL_PATH}${NC}"

else
    # Download binary from GitHub releases
    echo ""
    echo -e "${CYAN}Downloading binary...${NC}"

    # Determine download tool
    if command -v curl &> /dev/null; then
        DOWNLOAD_CMD="curl -fsSL"
    elif command -v wget &> /dev/null; then
        DOWNLOAD_CMD="wget -qO-"
    else
        echo -e "${RED}✗ Neither curl nor wget found${NC}"
        echo "Please install curl or wget to continue"
        exit 1
    fi

    # Get latest release version if not specified
    if [ "$VERSION" = "latest" ]; then
        echo -e "  Fetching latest release version..."
        if command -v curl &> /dev/null; then
            VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        else
            VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        fi

        if [ -z "$VERSION" ]; then
            echo -e "${YELLOW}⚠ Could not determine latest version, using v0.1.0${NC}"
            VERSION="v0.1.0"
        fi
    fi

    echo -e "  Version: ${CYAN}${VERSION}${NC}"

    # Build download URL
    BINARY_NAME="doku"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME="doku.exe"
    fi

    # Expected asset name: doku-{version}-{os}-{arch}.tar.gz or doku-{os}-{arch}
    ASSET_NAME="doku-${OS}-${ARCH}"
    DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}.tar.gz"

    # Try to download
    echo -e "  Downloading from: ${DOWNLOAD_URL}"

    TMP_DIR=$(mktemp -d)
    trap "rm -rf $TMP_DIR" EXIT

    if command -v curl &> /dev/null; then
        if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/doku.tar.gz" 2>/dev/null; then
            # Try without .tar.gz extension (direct binary download)
            DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
            if ! curl -fsSL "$DOWNLOAD_URL" -o "$TMP_DIR/$BINARY_NAME" 2>/dev/null; then
                echo -e "${RED}✗ Failed to download binary${NC}"
                echo -e "${YELLOW}Build asset may not exist for this platform/version${NC}"
                echo ""
                echo "Try building from source instead:"
                echo "  ${CYAN}$0 --source${NC}"
                exit 1
            fi
        else
            # Extract tar.gz
            tar -xzf "$TMP_DIR/doku.tar.gz" -C "$TMP_DIR"
            # Rename if needed (for Windows)
            if [ "$BINARY_NAME" != "doku" ] && [ -f "$TMP_DIR/doku" ]; then
                mv "$TMP_DIR/doku" "$TMP_DIR/$BINARY_NAME"
            fi
        fi
    else
        if ! wget -qO "$TMP_DIR/doku.tar.gz" "$DOWNLOAD_URL" 2>/dev/null; then
            # Try without .tar.gz extension (direct binary download)
            DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET_NAME}"
            if ! wget -qO "$TMP_DIR/$BINARY_NAME" "$DOWNLOAD_URL" 2>/dev/null; then
                echo -e "${RED}✗ Failed to download binary${NC}"
                echo -e "${YELLOW}Build asset may not exist for this platform/version${NC}"
                echo ""
                echo "Try building from source instead:"
                echo "  ${CYAN}$0 --source${NC}"
                exit 1
            fi
        else
            # Extract tar.gz
            tar -xzf "$TMP_DIR/doku.tar.gz" -C "$TMP_DIR"
            # Rename if needed (for Windows)
            if [ "$BINARY_NAME" != "doku" ] && [ -f "$TMP_DIR/doku" ]; then
                mv "$TMP_DIR/doku" "$TMP_DIR/$BINARY_NAME"
            fi
        fi
    fi

    # Determine install location
    if [ -n "$INSTALL_DIR" ]; then
        INSTALL_PATH="$INSTALL_DIR/$BINARY_NAME"
    elif [ -w "/usr/local/bin" ]; then
        INSTALL_PATH="/usr/local/bin/$BINARY_NAME"
    else
        # Fall back to user's home bin or GOPATH
        if [ -n "$GOPATH" ]; then
            INSTALL_PATH="$(go env GOPATH 2>/dev/null || echo "$HOME/go")/bin/$BINARY_NAME"
        else
            INSTALL_PATH="$HOME/.local/bin/$BINARY_NAME"
        fi

        # Create directory if it doesn't exist
        mkdir -p "$(dirname "$INSTALL_PATH")"
    fi

    # Install binary
    if [ -w "$(dirname "$INSTALL_PATH")" ]; then
        cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_PATH"
    else
        echo -e "${YELLOW}⚠ Installation requires elevated permissions${NC}"
        sudo cp "$TMP_DIR/$BINARY_NAME" "$INSTALL_PATH"
    fi

    chmod +x "$INSTALL_PATH"

    echo ""
    echo -e "${GREEN}✓ Installation complete!${NC}"
    echo -e "Installed to: ${CYAN}${INSTALL_PATH}${NC}"
fi

echo ""

# Check if install directory is in PATH
INSTALL_BIN_DIR="$(dirname "$INSTALL_PATH")"
if [[ ":$PATH:" != *":${INSTALL_BIN_DIR}:"* ]]; then
    echo -e "${YELLOW}⚠ Warning: ${INSTALL_BIN_DIR} is not in your PATH${NC}"
    echo ""
    echo "Add this to your ~/.zshrc or ~/.bashrc:"
    echo "  ${CYAN}export PATH=\"\$PATH:${INSTALL_BIN_DIR}\"${NC}"
    echo ""
    echo "Then reload your shell:"
    echo "  ${CYAN}source ~/.zshrc${NC}"
    echo ""
fi

echo "Verify installation:"
echo "  ${CYAN}doku version${NC}"
echo ""
echo "Get started:"
echo "  ${CYAN}doku init${NC}"
echo ""
echo -e "${GREEN}Happy coding! 🚀${NC}"
