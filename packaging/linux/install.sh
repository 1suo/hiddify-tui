#!/usr/bin/env bash
# Install the hiddify-tui client and the headless core service. Run once with
# sudo; afterward hiddify-tui connects to the core at 127.0.0.1:17078.
#
# Usage: sudo packaging/linux/install.sh [BUILD-DIR]
#   BUILD-DIR contains the built hiddify-tui and hiddify-migrate binaries.
#   hiddify-core is located from BUILD-DIR, ~/.local/bin, or PATH.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "install: run as root (sudo $0 [BUILD-DIR])" >&2
    exit 1
fi

SRCDIR="${1:-$(pwd)}"
DESIGNATED_USER="${SUDO_USER:-}"

for binary in hiddify-tui; do
    if [ ! -f "$SRCDIR/$binary" ]; then
        echo "install: missing $binary in $SRCDIR (build it with 'make build')" >&2
        exit 1
    fi
done

CORE_BIN=""
USER_HOME="$(getent passwd "${DESIGNATED_USER:-root}" | cut -d: -f6)"
for candidate in "$SRCDIR/hiddify-core" "$USER_HOME/.local/bin/hiddify-core" "$(command -v hiddify-core 2>/dev/null)"; do
    if [ -n "$candidate" ] && [ -f "$candidate" ]; then
        CORE_BIN="$candidate"
        break
    fi
done

LIBDIR=/usr/lib/hiddify
BINDIR=/usr/local/bin
UNITDIR=/etc/systemd/system

echo "installing binaries"
install -d -m0755 "$LIBDIR" "$UNITDIR"
[ -n "$CORE_BIN" ] && install -m0755 "$CORE_BIN" "$LIBDIR/hiddify-core"
install -m0755 "$SRCDIR/hiddify-tui" "$BINDIR/hiddify-tui"
[ -f "$SRCDIR/hiddify-migrate" ] && install -m0755 "$SRCDIR/hiddify-migrate" "$BINDIR/hiddify-migrate"

repo="$(cd "$(dirname "$0")/../.." && pwd)"
install -m0644 "$repo/packaging/systemd/hiddify-core.service" "$UNITDIR/hiddify-core.service"

echo "enabling hiddify-core.service"
systemctl daemon-reload
systemctl enable hiddify-core.service

echo
echo "Installed. The core runs as a service; hiddify-tui connects to 127.0.0.1:17078."
