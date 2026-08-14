#!/usr/bin/env bash
# Remove the macOS client and this project's optional LaunchDaemon.

set -euo pipefail

if [ "$(id -u)" -ne 0 ]; then
    echo "uninstall: run as root (sudo $0)" >&2
    exit 1
fi

label=system/com.github.hiddify.core
plist=/Library/LaunchDaemons/com.github.hiddify.core.plist

if launchctl print "$label" >/dev/null 2>&1; then
    launchctl bootout "$label"
fi

rm -f /usr/local/bin/hiddify-tui /usr/local/bin/hiddify-migrate "$plist"
rm -f "/Library/Application Support/Hiddify/hiddify-core" \
      "/Library/Application Support/Hiddify/hiddify-core-daemon"

echo "removed hiddify-tui and its optional LaunchDaemon"
