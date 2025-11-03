#!/usr/bin/env bash
#
# Doku CLI Installation Script
#
# This script installs the doku CLI to your system with proper version information.
# It builds from source with version, commit, and build date embedded.
#
# Usage:
#   ./scripts/install.sh           # Install to $GOPATH/bin
#   ./scripts/install.sh --local   # Build to ./bin/doku only
#

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Detect OS
OS="$(uname -s)"
ARCH="$(uname -m)"

echo -e "${CYAN}Doku CLI Installer${NC}"
echo ""

# Check if Go is installed
if ! command -v go &> /dev/null; then
    echo -e "${RED}✗ Go is not installed${NC}"
    echo "Please install Go from https://golang.org/dl/"
    exit 1
fi

echo -e "${GREEN}✓ Go detected: $(go version)${NC}"

# Get version information
VERSION=""
COMMIT=""
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

# Try to get version from git tag
if command -v git &> /dev/null && [ -d .git ]; then
    # Get latest tag
    TAG=$(git describe --tags --abbrev=0 2>/dev/null || echo "")

    if [ -n "$TAG" ]; then
        # Check if we're exactly on a tag
        if git describe --exact-match --tags HEAD &> /dev/null; then
            VERSION="$TAG"
        else
            # We're ahead of the tag
            COMMITS_AHEAD=$(git rev-list ${TAG}..HEAD --count)
            VERSION="${TAG}+${COMMITS_AHEAD}"
        fi
    else
        VERSION="v0.1.0-dev"
    fi

    COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

    echo -e "${GREEN}✓ Git detected${NC}"
    echo -e "  Version: ${CYAN}${VERSION}${NC}"
    echo -e "  Commit:  ${CYAN}${COMMIT}${NC}"
else
    VERSION="v0.1.0-dev"
    COMMIT="unknown"
    echo -e "${YELLOW}⚠ Git not detected, using default version${NC}"
fi

echo ""

# Build ldflags
LDFLAGS="-X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildDate=${BUILD_DATE}"

# Check for --local flag
if [ "$1" == "--local" ]; then
    echo -e "${CYAN}Building local binary...${NC}"
    mkdir -p bin
    go build -ldflags "${LDFLAGS}" -o bin/doku ./cmd/doku
    echo ""
    echo -e "${GREEN}✓ Build complete!${NC}"
    echo ""
    echo -e "Binary location: ${CYAN}./bin/doku${NC}"
    echo ""
    echo "To use the binary:"
    echo "  ${CYAN}./bin/doku version${NC}"
    echo ""
    echo "To install to PATH:"
    echo "  ${CYAN}./scripts/install.sh${NC}  (without --local flag)"
else
    echo -e "${CYAN}Installing doku CLI...${NC}"

    # Install to GOPATH/bin
    go install -ldflags "${LDFLAGS}" ./cmd/doku

    # Get install location
    GOPATH_BIN=$(go env GOPATH)/bin

    echo ""
    echo -e "${GREEN}✓ Installation complete!${NC}"
    echo ""
    echo -e "Installed to: ${CYAN}${GOPATH_BIN}/doku${NC}"
    echo ""

    # Check if GOPATH/bin is in PATH
    if [[ ":$PATH:" != *":${GOPATH_BIN}:"* ]]; then
        echo -e "${YELLOW}⚠ Warning: ${GOPATH_BIN} is not in your PATH${NC}"
        echo ""
        echo "Add this to your ~/.zshrc or ~/.bashrc:"
        echo "  ${CYAN}export PATH=\"\$PATH:${GOPATH_BIN}\"${NC}"
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
fi

echo ""
echo -e "${GREEN}Happy coding! 🚀${NC}"
