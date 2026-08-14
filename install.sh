#!/usr/bin/env bash
# Install the latest hiddify-tui release and standalone Linux core service.
# Pin a release with HIDDIFY_TUI_VERSION=vX.Y.Z.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "install: run as root (curl ... | sudo bash)" >&2
    exit 1
fi

for command in awk curl install mktemp sha256sum tar uname; do
    if ! command -v "$command" >/dev/null 2>&1; then
        echo "install: required command not found: $command" >&2
        exit 1
    fi
done

case "$(uname -m)" in
    x86_64|amd64) archive_arch=amd64 ;;
    aarch64|arm64) archive_arch=arm64 ;;
    *) echo "install: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
esac

tag="${HIDDIFY_TUI_VERSION:-}"
if [ -z "$tag" ]; then
    latest_url="$(curl -fsSL -o /dev/null -w '%{url_effective}' \
        https://github.com/1suo/hiddify-tui/releases/latest)"
    tag="${latest_url##*/}"
fi
case "$tag" in
    v*) ;;
    *) tag="v$tag" ;;
esac
if ! [[ "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][0-9A-Za-z.-]+)?$ ]]; then
    echo "install: invalid release version: $tag" >&2
    exit 1
fi

version="${tag#v}"
archive="hiddify-tui_${version}_linux_${archive_arch}.tar.gz"
release_url="https://github.com/1suo/hiddify-tui/releases/download/${tag}"
source_url="https://raw.githubusercontent.com/1suo/hiddify-tui/${tag}"
tmpdir="$(mktemp -d /tmp/hiddify-tui-install.XXXXXX)"
trap 'rm -rf -- "$tmpdir"' EXIT

echo "downloading hiddify-tui $tag ($archive_arch)"
curl -fsSL "$release_url/$archive" -o "$tmpdir/$archive"
curl -fsSL "$release_url/checksums.txt" -o "$tmpdir/checksums.txt"

expected="$(awk -v file="$archive" '$2 == file { print $1; exit }' "$tmpdir/checksums.txt")"
actual="$(sha256sum "$tmpdir/$archive" | awk '{ print $1 }')"
if [ -z "$expected" ] || [ "$actual" != "$expected" ]; then
    echo "install: release checksum verification failed" >&2
    exit 1
fi

mkdir -p "$tmpdir/build" "$tmpdir/repo/packaging/linux" "$tmpdir/repo/packaging/systemd"
tar -xzf "$tmpdir/$archive" -C "$tmpdir/build"
curl -fsSL "$source_url/packaging/linux/install.sh" \
    -o "$tmpdir/repo/packaging/linux/install.sh"
curl -fsSL "$source_url/packaging/systemd/hiddify-core.service" \
    -o "$tmpdir/repo/packaging/systemd/hiddify-core.service"
chmod +x "$tmpdir/repo/packaging/linux/install.sh"

"$tmpdir/repo/packaging/linux/install.sh" "$tmpdir/build"
