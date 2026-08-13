# Operations guide

This document covers the behaviors a user or administrator needs to install,
connect, diagnose, and remove Hiddify safely. It complements the control
protocol contract in [`control-v1.md`](control-v1.md).

## Client vs. daemon vs. agent

Three processes cooperate:

- `hiddify-tui` is a thin local client. It reads the daemon snapshot and event
  stream and sends local control requests over an OS-authorized socket or pipe.
  It never owns the VPN process.
- `hiddify-core` (the daemon) owns profiles, settings, the core lifecycle, and
  active networking. It runs as a system service even while disconnected.
- `hiddify-agent` applies and restores the logged-in user's system proxy
  settings. It only matters in system-proxy mode.

**Closing the TUI never disconnects.** **Stopping the service always tears down
networking.** This is the key operational distinction.

## Installation

- Linux: install the distribution package. It ships the hardened
  `hiddify-core.service` (system) and `hiddify-agent.service` (user session).
  See [`packaging/linux/README.md`](../packaging/linux/README.md).
- macOS: install the signed package. It places a root `LaunchDaemon` for the
  core and a per-user `LaunchAgent` for the proxy agent. See
  [`packaging/macos/README.md`](../packaging/macos/README.md).
- Windows: run the MSI. It installs a `LocalSystem` service for the core and a
  per-user scheduled task for the agent. See
  [`packaging/windows/README.md`](../packaging/windows/README.md).

Every platform records **one designated desktop user**. Only that user (and
root/LocalSystem) can reach the control endpoint. The endpoint is never exposed
over TCP.

## Autostart vs. auto-connect

- Installing or enabling the service starts the daemon at boot (`autostart`).
- `auto_connect` is a separate, opt-in setting, disabled by default. When
  enabled, the daemon connects the active profile after initialization.
- If no active profile exists, the daemon remains idle and reports an
  actionable warning.
- `hiddify-tui autoconnect enable|disable|status` manages this without a
  terminal.

## Connecting and verifying

```sh
hiddify-tui status                 # human status
hiddify-tui status --json          # stable JSON + exit code
hiddify-tui connect --mode tun     # TUN, system-proxy, or local-proxy
hiddify-tui outbound list
hiddify-tui outbound test all
```

Exit codes: `0` success, `2` usage, `3` daemon unavailable/incompatible,
`4` operation rejected, `5` authorization failure.

## Logs and diagnostics

```sh
hiddify-tui logs --follow --level warn
hiddify-tui logs clear --yes
hiddify-tui diagnostics
```

Daemon-side redaction is authoritative. Logs never contain full subscription
URLs, credentials, config bodies, or keys.

## System-proxy restoration

The agent captures the exact prior proxy state before its first change and
restores it after a normal disconnect, an expired lease, an agent or daemon
crash, a mode change, reboot, or uninstall. If a desktop session was absent when
a connection started, the agent reapplies the proxy at login.

Manually restore proxy state with:

```sh
hiddify-agent --restore
```

## Migration from the GUI

Exit the GUI and its core completely, then review and apply:

```sh
hiddify-tui migrate gui --database PATH --configs PATH --dry-run
hiddify-tui migrate gui --database PATH --configs PATH --apply --yes --gui-exited
```

The source database and config files are read-only in both modes. The GUI and
daemon must not run concurrently after import.

## Recovery without deleting profiles

- Restore proxy state: `hiddify-agent --restore`.
- Restart the daemon after a crash: `systemctl restart hiddify-core` (Linux),
  `launchctl kickstart -k system/com.github.hiddify.core` (macOS), or
  `Restart-Service hiddify-core` (Windows).
- Remove a stale socket only when no daemon owns it; the daemon cleans its own
  socket on graceful shutdown.
- Delete a stale TUN interface with the platform tool (`ip link del`, `ifconfig
  utunX destroy`, or the Windows adapter UI) only after confirming the daemon is
  stopped.

## Uninstall and purge

Uninstall always disconnects cleanly, restores proxy state, and preserves
profiles. `--purge` additionally removes the state directory and is destructive:

```sh
hiddify-tui settings export --include-secrets --yes > settings.json  # optional backup
# then run the platform uninstaller, adding --purge only if intended
```

## SSH and noninteractive use

The TUI needs a terminal; the CLI does not. `--json` yields stable output for
scripting and `--no-color` disables ANSI color. When the daemon is unavailable,
the CLI prints the exact service remediation command for the current platform.
