#!/usr/bin/env bash
# Render package-manager templates from a GoReleaser checksums.txt.
# Usage: scripts/render-package-manifests.sh VERSION CHECKSUMS [OUTPUT-DIR]

set -euo pipefail

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
    echo "usage: $0 VERSION CHECKSUMS [OUTPUT-DIR]" >&2
    exit 2
fi

version="${1#v}"
checksums="$2"
output="${3:-dist/package-manifests}"

if ! [[ "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    echo "invalid release version: $version" >&2
    exit 2
fi
if [ ! -f "$checksums" ]; then
    echo "checksums file not found: $checksums" >&2
    exit 2
fi

checksum() {
    local filename="$1" value
    value="$(awk -v file="$filename" '$2 == file { print $1; exit }' "$checksums")"
    if ! [[ "$value" =~ ^[0-9a-fA-F]{64}$ ]]; then
        echo "missing SHA-256 for $filename" >&2
        exit 1
    fi
    printf '%s' "$value"
}

sha_linux_amd64="$(checksum "hiddify-tui_${version}_linux_amd64.tar.gz")"
sha_linux_arm64="$(checksum "hiddify-tui_${version}_linux_arm64.tar.gz")"
sha_darwin_amd64="$(checksum "hiddify-tui_${version}_darwin_amd64.tar.gz")"
sha_darwin_arm64="$(checksum "hiddify-tui_${version}_darwin_arm64.tar.gz")"
sha_windows_amd64="$(checksum "hiddify-tui_${version}_windows_amd64.zip")"
sha_windows_arm64="$(checksum "hiddify-tui_${version}_windows_arm64.zip")"

sri() { printf '%s' "$1" | xxd -r -p | base64 | tr -d '\n'; }

render() {
    local source="$1" destination="$2"
    mkdir -p "$(dirname "$destination")"
    sed \
        -e "s|@VERSION@|$version|g" \
        -e "s|@SHA_LINUX_AMD64@|$sha_linux_amd64|g" \
        -e "s|@SHA_LINUX_ARM64@|$sha_linux_arm64|g" \
        -e "s|@SHA_DARWIN_AMD64@|$sha_darwin_amd64|g" \
        -e "s|@SHA_DARWIN_ARM64@|$sha_darwin_arm64|g" \
        -e "s|@SHA_WINDOWS_AMD64@|$sha_windows_amd64|g" \
        -e "s|@SHA_WINDOWS_ARM64@|$sha_windows_arm64|g" \
        -e "s|@SRI_LINUX_AMD64@|$(sri "$sha_linux_amd64")|g" \
        -e "s|@SRI_LINUX_ARM64@|$(sri "$sha_linux_arm64")|g" \
        -e "s|@SRI_DARWIN_AMD64@|$(sri "$sha_darwin_amd64")|g" \
        -e "s|@SRI_DARWIN_ARM64@|$(sri "$sha_darwin_arm64")|g" \
        "$source" > "$destination"
}

render packaging/aur/PKGBUILD.tmpl "$output/aur/PKGBUILD"
render packaging/aur/.SRCINFO.tmpl "$output/aur/.SRCINFO"
render packaging/homebrew/hiddify-tui.rb.tmpl "$output/homebrew/hiddify-tui.rb"
render packaging/scoop/hiddify-tui.json.tmpl "$output/scoop/hiddify-tui.json"
render packaging/winget/1suo.HiddifyTUI.yaml.tmpl "$output/winget/1suo.HiddifyTUI.yaml"
render packaging/winget/1suo.HiddifyTUI.locale.en-US.yaml.tmpl "$output/winget/1suo.HiddifyTUI.locale.en-US.yaml"
render packaging/winget/1suo.HiddifyTUI.installer.yaml.tmpl "$output/winget/1suo.HiddifyTUI.installer.yaml"
render packaging/nix/package.nix.tmpl "$output/nix/package.nix"
render packaging/nix/flake.nix.tmpl "$output/nix/flake.nix"

if grep -R -n -E '@[A-Z0-9_]+@' "$output"; then
    echo "unresolved template token" >&2
    exit 1
fi

echo "rendered package manifests in $output"
