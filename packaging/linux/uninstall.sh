#!/usr/bin/env bash
# Remove the Hiddify services and binaries. Profiles are preserved unless
# --purge is supplied.
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
ETCDIR=/etc/hiddify
UNITDIR=/etc/systemd/system
USER_UNITDIR=/usr/lib/systemd/user

# Restore the user's system proxy state before removing the agent.
if [ -x "$LIBDIR/hiddify-agent" ]; then
    for user in $(getent passwd | cut -d: -f1); do
        home="$(getent passwd "$user" | cut -d: -f6)"
        if [ -f "$home/.local/state/hiddify/proxy-recovery.json" ]; then
            sudo -u "$user" "$LIBDIR/hiddify-agent" --restore 2>/dev/null || true
        fi
    done
fi

systemctl disable --now hiddify-core.service 2>/dev/null || true
rm -f "$UNITDIR/hiddify-core.service" "$USER_UNITDIR/hiddify-agent.service"
systemctl daemon-reload

rm -f "$BINDIR/hiddify-tui" "$BINDIR/hiddify-migrate"
rm -rf "$LIBDIR"
rm -rf "$ETCDIR"

if [ "$PURGE" -eq 1 ]; then
    echo "purging /var/lib/hiddify"
    rm -rf /var/lib/hiddify
else
    echo "profiles preserved in /var/lib/hiddify (use --purge to remove)"
fi

echo "uninstall complete"
