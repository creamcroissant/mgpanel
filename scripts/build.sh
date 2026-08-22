#!/bin/bash
# MGPanel Build Script
# Builds frontend and backend with version info embedded

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Project root
PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

# Build variables
BINARY_NAME="mgpanel"
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"
COMMIT="${COMMIT:-$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")}"
BUILD_TIME="${BUILD_TIME:-$(date -u '+%Y-%m-%dT%H:%M:%SZ')}"
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.Commit=${COMMIT} -X main.BuildTime=${BUILD_TIME}"
GOFLAGS="-trimpath"

# Directories
DIST_DIR="dist"
USER_FRONTEND_DIR="web/user-vite"

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

# Print with color
info() {
    echo -e "${GREEN}==>${NC} $1"
}

warn() {
    echo -e "${YELLOW}==>${NC} $1"
}

error() {
    echo -e "${RED}==>${NC} $1"
    exit 1
}

# Show usage
usage() {
    echo "MGPanel Build Script"
    echo ""
    echo "Usage: $0 [command] [options]"
    echo ""
    echo "Commands:"
    echo "  all           Build for all platforms (default)"
    echo "  current       Build for current platform only"
    echo "  frontend      Build frontend assets only"
    echo "  backend       Build backend only (no frontend)"
    echo "  clean         Clean build artifacts"
    echo ""
    echo "Options:"
    echo "  --no-frontend Skip frontend build"
    echo "  --version     Show version info"
    echo "  --help        Show this help"
    echo ""
    echo "Environment Variables:"
    echo "  VERSION       Override version string"
    echo "  COMMIT        Override commit hash"
    echo "  BUILD_TIME    Override build time"
}

# Build frontend (unified User + Admin)
build_frontend() {
    info "Building User Frontend (includes Admin)..."
    cd "$PROJECT_ROOT/$USER_FRONTEND_DIR"
    npm ci --silent
    npm run build

    cd "$PROJECT_ROOT"
    info "Frontend build complete"
}

# Build backend for a specific platform
build_backend() {
    local os="$1"
    local arch="$2"
    local output="$3"

    info "Building for ${os}/${arch}..."

    # Try to find go in PATH or common locations
    GO_CMD="go"
    if ! command -v go &> /dev/null; then
        if [[ -x "/usr/local/go/bin/go" ]]; then
            GO_CMD="/usr/local/go/bin/go"
        elif [[ -x "/usr/bin/go" ]]; then
             GO_CMD="/usr/bin/go"
        else
             warn "Go not found in PATH. Attempting to proceed, but build might fail."
        fi
    fi

    GOOS="$os" GOARCH="$arch" $GO_CMD build $GOFLAGS -ldflags "$LDFLAGS" -o "$output" ./cmd/mgpanel
}

# Build for current platform
build_current() {
    local output="$BINARY_NAME"
    if [[ "$OSTYPE" == "msys" || "$OSTYPE" == "win32" ]]; then
        output="${BINARY_NAME}.exe"
    fi

    info "Building for current platform..."
    mkdir -p "$DIST_DIR"
    output="$DIST_DIR/$output"

    # Try to find go in PATH or common locations
    GO_CMD="go"
    if ! command -v go &> /dev/null; then
        if [[ -x "/usr/local/go/bin/go" ]]; then
            GO_CMD="/usr/local/go/bin/go"
        elif [[ -x "/usr/bin/go" ]]; then
             GO_CMD="/usr/bin/go"
        else
             warn "Go not found in PATH. Attempting to proceed, but build might fail."
        fi
    fi

    $GO_CMD build $GOFLAGS -ldflags "$LDFLAGS" -o "$output" ./cmd/mgpanel
    info "Build complete: ./${output}"
}

# Build for all platforms
build_all() {
    mkdir -p "$DIST_DIR"

    for platform in "${PLATFORMS[@]}"; do
        IFS='/' read -r os arch <<< "$platform"

        output="${DIST_DIR}/${BINARY_NAME}-${os}-${arch}"
        if [[ "$os" == "windows" ]]; then
            output="${output}.exe"
        fi

        build_backend "$os" "$arch" "$output"
    done

    info "Generating SHA256 checksums..."
    cd "$DIST_DIR"

    # Use sha256sum on Linux, shasum on macOS
    if command -v sha256sum &> /dev/null; then
        sha256sum ${BINARY_NAME}-* > SHA256SUMS.txt
    elif command -v shasum &> /dev/null; then
        shasum -a 256 ${BINARY_NAME}-* > SHA256SUMS.txt
    else
        warn "No sha256sum or shasum found, skipping checksum generation"
    fi

    cd "$PROJECT_ROOT"
    info "All platform builds complete"

    # Print summary
    echo ""
    info "Build Summary:"
    echo "  Version:    ${VERSION}"
    echo "  Commit:     ${COMMIT}"
    echo "  Build Time: ${BUILD_TIME}"
    echo ""
    echo "  Output files:"
    ls -lh "$DIST_DIR"/${BINARY_NAME}-*
    echo ""
    if [[ -f "$DIST_DIR/SHA256SUMS.txt" ]]; then
        echo "  Checksums:"
        cat "$DIST_DIR/SHA256SUMS.txt"
    fi
}

# Clean build artifacts
clean() {
    info "Cleaning build artifacts..."
    rm -f "$BINARY_NAME" "${BINARY_NAME}.exe"
    rm -rf "$DIST_DIR"
    rm -rf "$USER_FRONTEND_DIR/dist"
    info "Clean complete"
}

# Main
main() {
    local command="${1:-all}"
    local skip_frontend=false

    # Parse options
    for arg in "$@"; do
        case $arg in
            --no-frontend)
                skip_frontend=true
                ;;
            --version)
                echo "MGPanel Build Script"
                echo "Version: ${VERSION}"
                echo "Commit: ${COMMIT}"
                exit 0
                ;;
            --help|-h)
                usage
                exit 0
                ;;
        esac
    done

    case $command in
        all)
            if [[ "$skip_frontend" != true ]]; then
                build_frontend
            fi
            build_all
            ;;
        current)
            if [[ "$skip_frontend" != true ]]; then
                build_frontend
            fi
            build_current
            ;;
        frontend)
            build_frontend
            ;;
        backend)
            build_current
            ;;
        clean)
            clean
            ;;
        --*)
            # Skip options that were already handled
            ;;
        *)
            error "Unknown command: $command"
            usage
            exit 1
            ;;
    esac
}

main "$@"
