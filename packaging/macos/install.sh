#!/usr/bin/env bash
# Install hiddify-tui on macOS. Run once with sudo.
#
# Usage: sudo packaging/macos/install.sh [BUILD-DIR]
#   BUILD-DIR must contain a darwin hiddify-tui (and hiddify-migrate).
#
# The core is bundled with the Hiddify GUI, which serves Core gRPC at
# 127.0.0.1:17078; no standalone macOS core is published. If BUILD-DIR also
# contains a standalone hiddify-core, a LaunchDaemon is installed to run it.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "install: run as root (sudo $0 [BUILD-DIR])" >&2
    exit 1
fi

SRCDIR="${1:-$(pwd)}"
BINDIR=/usr/local/bin
LIBDIR="/Library/Application Support/Hiddify"
DAEMONDIR=/Library/LaunchDaemons

if [ ! -f "$SRCDIR/hiddify-tui" ]; then
    echo "install: missing hiddify-tui in $SRCDIR (build with 'make build')" >&2
    exit 1
fi

echo "installing client"
mkdir -p "$BINDIR" "$LIBDIR" "$DAEMONDIR"
install -m0755 "$SRCDIR/hiddify-tui" "$BINDIR/hiddify-tui"
[ -f "$SRCDIR/hiddify-migrate" ] && install -m0755 "$SRCDIR/hiddify-migrate" "$BINDIR/hiddify-migrate"

if [ -f "$SRCDIR/hiddify-core" ]; then
	if [ ! -f "$SRCDIR/hiddify-core-daemon" ]; then
		echo "install: hiddify-core-daemon is required with a standalone core" >&2
		exit 1
	fi
	echo "installing core service"
	install -m0755 "$SRCDIR/hiddify-core" "$LIBDIR/hiddify-core"
	install -m0755 "$SRCDIR/hiddify-core-daemon" "$LIBDIR/hiddify-core-daemon"
    repo="$(cd "$(dirname "$0")/../.." && pwd)"
    install -m0644 "$repo/packaging/macos/com.github.hiddify.core.plist" "$DAEMONDIR/"
    if lsof -nP -iTCP:17078 -sTCP:LISTEN >/dev/null 2>&1; then
        echo "not loading or restarting the installed daemon: port 17078 is already in use"
        echo "the existing VPN/core process was not interrupted"
    elif launchctl print system/com.github.hiddify.core >/dev/null 2>&1; then
        echo "the LaunchDaemon is already loaded; it was not restarted"
    else
        launchctl bootstrap system "$DAEMONDIR/com.github.hiddify.core.plist"
        launchctl enable system/com.github.hiddify.core
        echo "installed and started the standalone headless core"
    fi
else
    echo "note: no standalone core for macOS; hiddify-tui connects to the Hiddify GUI's core on 127.0.0.1:17078"
fi

echo
echo "installed hiddify-tui to $BINDIR"
