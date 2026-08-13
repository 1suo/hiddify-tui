#!/usr/bin/env bash
# Build every release target and produce checksums and an SBOM.
#
# Usage: scripts/release.sh [VERSION]
#   VERSION defaults to the value of `git describe --tags --always`.
#
# Outputs into dist/:
#   hiddify-tui-<os>-<arch>[.exe]
#   hiddify-agent-<os>-<arch>[.exe]
#   hiddify-migrate-linux-amd64
#   checksums.txt          (SHA-256 for every artifact)
#   sbom/                  (go version -m module manifests per artifact)

set -euo pipefail

version="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
ldflags="-s -w -X main.version=${version}"

mkdir -p dist/sbom

build() {
    local bin="$1" package="$2" goos="$3" goarch="$4"
    local suffix=""
    if [ "$goos" = "windows" ]; then suffix=".exe"; fi
    local out="dist/${bin}-${goos}-${goarch}${suffix}"
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
        go build -trimpath -ldflags="$ldflags" -o "$out" "$package"
    go version -m "$out" > "dist/sbom/${bin}-${goos}-${goarch}.sbom"
}

for target in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64; do
    goos="${target%/*}"
    goarch="${target#*/}"
    build hiddify-tui ./cmd/hiddify-tui "$goos" "$goarch"
    build hiddify-agent ./cmd/hiddify-agent "$goos" "$goarch"
done

build hiddify-migrate ./cmd/hiddify-migrate linux amd64

(cd dist && find . -maxdepth 1 -type f -printf '%f\n' | sort | xargs sha256sum > checksums.txt)

echo "Release ${version} built into dist/"
wc -c dist/hiddify-tui-* | tail -n +1
