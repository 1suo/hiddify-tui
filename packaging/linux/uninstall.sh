#!/usr/bin/env bash
# Remove the hiddify-tui client and the headless core service.
#
# Usage: sudo packaging/linux/uninstall.sh [--purge]

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "uninstall: run as root (sudo $0)" >&2
    exit 1
fi

PURGE=0
if [ "${1:-}" = "--purge" ]; then
    PURGE=1
fi

LIBDIR=/usr/lib/hiddify
BINDIR=/usr/local/bin
UNITDIR=/etc/systemd/system

systemctl disable --now hiddify-core.service 2>/dev/null || true
rm -f "$UNITDIR/hiddify-core.service"
systemctl daemon-reload

rm -f "$BINDIR/hiddify-tui" "$BINDIR/hiddify-migrate"
rm -rf "$LIBDIR"

if [ "$PURGE" -eq 1 ]; then
    echo "purging /var/lib/hiddify"
    rm -rf /var/lib/hiddify
fi

echo "uninstall complete"
