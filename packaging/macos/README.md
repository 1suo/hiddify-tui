# macOS package contract

The privileged core daemon runs as a root `LaunchDaemon`; the system-proxy
agent runs as a per-user `LaunchAgent`. `hiddify-tui` only connects to the
daemon's local control socket and never owns networking itself.

## Layout

```text
/Library/Application Support/Hiddify/hiddify-core   # core daemon binary
/Library/Application Support/Hiddify/hiddify-agent   # session proxy agent
/usr/local/bin/hiddify-tui                           # thin client
/Library/LaunchDaemons/com.github.hiddify.core.plist # root core service
/Library/LaunchAgents/com.github.hiddify.agent.plist # per-user agent
/var/run/hiddify/control.sock                        # runtime socket (0750)
```

## Installer requirements

A signed package must:

1. Place `hiddify-core`, `hiddify-agent`, and `hiddify-tui` in the locations
   above, and copy the plists into the launchd directories.
2. Create `/var/run/hiddify` with mode `0750`, owned by `root:wheel`.
3. Record the designated desktop user; the daemon owns
   `/var/run/hiddify/control.sock` with mode `0600` and verifies the connecting
   peer's identity through `SO_PEERCRED` (or root) before accepting local RPCs.
   Never make the socket or state directory world-readable.
4. Load `com.github.hiddify.core.plist` with `launchctl load -w`, and install
   the agent plist into the designated user's launch agents directory.
5. Sign and notarize the package. Development CI may be unsigned.
6. On uninstall, ask before disconnecting, restore proxy settings through the
   agent, and retain profiles unless the user explicitly requests a purge.

The daemon runs as root because TUN, routes, and DNS integration need to be
measured against the completed core. A release package must replace root with
the smallest verified capability set compatible with every service mode.

Validate the plists with:

```sh
plutil -lint packaging/macos/com.github.hiddify.core.plist \
  packaging/macos/com.github.hiddify.agent.plist
```
