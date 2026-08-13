#!/usr/bin/env bash
# Install the Hiddify core daemon, TUI/CLI, and session agent as native systemd
# services. Run once with sudo; afterward the daemon starts at boot and
# hiddify-tui connects to it without any manual daemon command.
#
# Usage: sudo packaging/linux/install.sh [BUILD-DIR]
#   BUILD-DIR defaults to the current directory and must contain the built
#   binaries: hiddify-core, hiddify-agent, hiddify-tui, hiddify-migrate.
#
# The designated controlling desktop user is taken from SUDO_USER.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "install: run as root (sudo $0 [BUILD-DIR])" >&2
    exit 1
fi

SRCDIR="${1:-$(pwd)}"
DESIGNATED_USER="${SUDO_USER:-}"

if [ -z "$DESIGNATED_USER" ]; then
    echo "install: SUDO_USER is empty; run via sudo so the controlling user can be recorded" >&2
    exit 1
fi

for binary in hiddify-agent hiddify-tui; do
    if [ ! -f "$SRCDIR/$binary" ]; then
        echo "install: missing $binary in $SRCDIR (build it with 'make build')" >&2
        exit 1
    fi
done

# hiddify-core is built by the companion core repository, not this one, so it
# is located rather than required. Look in the build dir, then the controlling
# user's ~/.local/bin, then PATH.
CORE_BIN=""
USER_HOME="$(getent passwd "$DESIGNATED_USER" | cut -d: -f6)"
for candidate in \
    "$SRCDIR/hiddify-core" \
    "$USER_HOME/.local/bin/hiddify-core" \
    "$(command -v hiddify-core 2>/dev/null)"; do
    if [ -n "$candidate" ] && [ -f "$candidate" ]; then
        CORE_BIN="$candidate"
        break
    fi
done
if [ -z "$CORE_BIN" ]; then
    echo "install: could not locate hiddify-core; place it in $SRCDIR or $USER_HOME/.local/bin" >&2
    exit 1
fi

LIBDIR=/usr/lib/hiddify
BINDIR=/usr/local/bin
ETCDIR=/etc/hiddify
UNITDIR=/etc/systemd/system
USER_UNITDIR=/usr/lib/systemd/user

DESIGNATED_UID="$(id -u "$DESIGNATED_USER")"

echo "installing binaries"
install -d -m0755 "$LIBDIR" "$ETCDIR" "$UNITDIR" "$USER_UNITDIR"
install -m0755 "$CORE_BIN" "$LIBDIR/hiddify-core"
install -m0755 "$SRCDIR/hiddify-agent" "$LIBDIR/hiddify-agent"
install -m0755 "$SRCDIR/hiddify-tui" "$BINDIR/hiddify-tui"
if [ -f "$SRCDIR/hiddify-migrate" ]; then
    install -m0755 "$SRCDIR/hiddify-migrate" "$BINDIR/hiddify-migrate"
fi

echo "recording designated user $DESIGNATED_USER (uid $DESIGNATED_UID)"
printf 'HIDDIFY_ALLOWED_UID=%s\n' "$DESIGNATED_UID" > "$ETCDIR/core.env"
chmod 0600 "$ETCDIR/core.env"

repo="$(cd "$(dirname "$0")/../.." && pwd)"
install -m0644 "$repo/packaging/systemd/hiddify-core.service" "$UNITDIR/hiddify-core.service"
install -m0644 "$repo/packaging/systemd/hiddify-agent.service" "$USER_UNITDIR/hiddify-agent.service"

echo "enabling and starting hiddify-core.service"
systemctl daemon-reload
systemctl enable --now hiddify-core.service

echo "installing the session agent user unit (used only for system-proxy mode)"
loginctl enable-linger "$DESIGNATED_USER" 2>/dev/null || true

echo
echo "Installed. The daemon now runs automatically at boot."
echo "Run 'hiddify-tui' as $DESIGNATED_USER to connect."
echo "To enable the optional system-proxy agent for this user, run:"
echo "  systemctl --user enable --now hiddify-agent.service"
