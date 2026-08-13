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
    echo "installing core service"
    install -m0755 "$SRCDIR/hiddify-core" "$LIBDIR/hiddify-core"
    repo="$(cd "$(dirname "$0")/../.." && pwd)"
    install -m0644 "$repo/packaging/macos/com.github.hiddify.core.plist" "$DAEMONDIR/"
    launchctl load -w "$DAEMONDIR/com.github.hiddify.core.plist" 2>/dev/null || true
else
    echo "note: no standalone core for macOS; hiddify-tui connects to the Hiddify GUI's core on 127.0.0.1:17078"
fi

echo
echo "installed hiddify-tui to $BINDIR"
