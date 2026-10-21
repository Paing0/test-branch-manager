#!/usr/bin/env bash
# Build script for all supported platforms.
# Usage: ./scripts/build.sh

set -euo pipefail

APP_NAME="test-branch-manager"
CMD="./cmd/test-branch-manager"
DIST="dist"
VERSION="${VERSION:-dev}"
LDFLAGS="-s -w -X main.version=${VERSION}"

mkdir -p "${DIST}"

echo "=== Building Git Branch Manager v${VERSION} ==="
echo ""

build() {
    local os="$1"
    local arch="$2"
    local ext="$3"
    local output="${DIST}/${APP_NAME}-${os}-${arch}${ext}"

    echo "  Building ${os}/${arch}..."
    CGO_ENABLED=0 GOOS="${os}" GOARCH="${arch}" go build -ldflags "${LDFLAGS}" -o "${output}" "${CMD}"
    echo "    -> ${output}"
}

build "windows" "amd64" ".exe"
build "darwin"  "amd64" ""
build "darwin"  "arm64" ""
build "linux"   "amd64" ""

echo ""
echo "=== Build complete ==="
echo "Artifacts:"
ls -lh "${DIST}/"
