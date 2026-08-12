# Hiddify TUI implementation plan

Status: planning  
Target: Linux, macOS, and Windows desktop  
Primary language: Go  
UI framework: Bubble Tea v2 + Bubbles v2  

## 1. Product definition

Build a terminal frontend for Hiddify with a persistent background connection:

```text
hiddify-tui (thin client, target ~5-15 MB stripped)
    |
    | versioned local control API
    | Unix socket / Windows named pipe
    v
hiddify-core.service (always running, idle while disconnected)
    |
    +-- owns the core process and all mutable state
    +-- creates TUN when connected in TUN mode
    +-- exposes local mixed proxy ports in proxy modes
    +-- streams status, traffic, outbounds, and logs

hiddify-agent (user-session helper, system-proxy mode only)
    |
    +-- applies and restores proxy settings for the logged-in user
```

The TUI is a client, not the VPN process. Exiting, crashing, suspending, or
upgrading the TUI must not disconnect an active tunnel.

### V1 success criteria

- [ ] Install one package and obtain the daemon, TUI/CLI, session agent, and
      native service definitions.
- [ ] Add or migrate a Hiddify profile, select it, and connect using TUN,
      system-proxy, or local-proxy mode.
- [ ] Close the TUI and verify that the connection remains active.
- [ ] Reopen the TUI and reconstruct current state from the daemon without
      restarting the core.
- [ ] Enable optional auto-connect and reconnect the active profile after a
      reboot without opening a terminal.
- [ ] Disable auto-connect while leaving the daemon enabled.
- [ ] View live state, transfer rates, totals, connection counts, outbounds,
      delay tests, and bounded logs.
- [ ] Control every essential operation noninteractively and receive stable
      JSON output and exit codes.
- [ ] Reject unprivileged or non-designated local users at the control socket.
- [ ] Restore prior OS proxy state after disconnect, failure, or uninstall.

### V1 scope

- Profiles: add URL, add local content/file/stdin, list, inspect, rename,
  activate, refresh, and delete.
- Connection: connect, disconnect, restart, status, and selectable mode.
- Outbounds: inspect groups, select an outbound, test one/group/all, filter,
  and sort by delay.
- Observability: status, active profile/outbound, instantaneous and total
  traffic, memory, connection counts, and logs.
- Essential settings: service mode, region/balancer, IPv6, DNS endpoints and
  strategies, inbound ports, LAN sharing, TUN implementation/MTU/strict route,
  connection test URL/interval, log level, profile refresh, and auto-connect.
- Operations: native service installation, daemon autostart, user agent,
  migration from the current GUI, settings import/export, backups, and
  diagnostics.

### Deferred from V1

- Mobile platforms, GUI/tray UI, remote network administration, multi-user
  desktop ownership, QR scanning, per-app proxy, deep links, extension HTML UI,
  self-update UI, live database sharing with the Flutter app, and simultaneous
  GUI/TUI core ownership.
- Dedicated editors for arbitrary route rules, chains, Warp/Psiphon chains, and
  TLS tricks. Preserve these values during JSON import/export even when V1
  cannot edit them interactively.

## 2. Repository and ownership boundaries

This repository owns the thin clients, shared control protocol, packaging
metadata, migration tooling, and end-to-end tests. It must not vendor or link
the sing-box/Hiddify networking engine into `hiddify-tui`.

Planned layout:

```text
cmd/hiddify-tui/       interactive TUI and noninteractive CLI entrypoint
cmd/hiddify-agent/     user-session system-proxy helper
internal/client/       local RPC connection, retry, snapshot, event handling
internal/tui/          Bubble Tea screens and components
internal/cli/          command parsing, JSON/text output, exit-code mapping
internal/agent/        proxy apply/restore and daemon heartbeat logic
internal/migrate/      read-only Flutter GUI migration
internal/platform/     socket discovery and per-OS client/agent integration
proto/control/v1/      source-of-truth frontend control protocol
gen/                   generated Go protobuf/gRPC code
packaging/             systemd, launchd, Windows installer/service assets
test/                  integration fixtures and end-to-end harnesses
```

Required upstream work in `hiddify/hiddify-core`:

- [ ] Add an always-running daemon entrypoint that owns the core and state.
- [ ] Implement and serve the local control API defined in this repository.
- [ ] Complete and test profile persistence and configuration validation.
- [ ] Stop exposing unauthenticated management through TCP port `17078` in
      daemon mode; retain existing APIs only where compatibility requires them.
- [ ] Publish matching daemon artifacts for the supported platforms.

Protocol coordination policy:

- `proto/control/v1` is versioned and backward compatible after the first beta.
- The TUI repository generates client code from the protocol.
- Hiddify Core consumes the same tagged protocol module or generated schema.
- CI tests the TUI against the minimum supported and current core versions.
- Never read or write daemon storage directly from the TUI.

## 3. Milestone 0: feasibility and compatibility baseline

- [ ] Record the exact `hiddify-core` commit used for initial development.
- [ ] Build the current core and existing `HiddifyCli` on Linux x86-64.
- [ ] Exercise existing Core RPCs for start/stop, status, system stats,
      outbounds, URL tests, selection, parsing, settings changes, and logs.
- [ ] Document incomplete or unregistered APIs, especially ProfileService and
      currently unimplemented system-proxy RPCs.
- [ ] Capture fixtures from supported profile formats and Flutter GUI schema v5.
- [ ] Measure a minimal Bubble Tea + protobuf local-RPC client in release mode.
- [ ] Set a binary budget and make it visible in CI:
  - `hiddify-tui`: target <= 15 MB stripped, investigate at > 20 MB.
  - `hiddify-agent`: target <= 8 MB stripped, investigate at > 12 MB.
  - Report raw and archive-compressed sizes; do not fail solely on compression.
- [ ] Confirm whether upstream core can serve gRPC over Unix sockets and named
      pipes. Add an adapter if the transport assumes TCP.

Exit criteria:

- A spike client connects locally, receives a status stream, lists outbounds,
  and selects/tests an outbound without linking the core library.
- A written compatibility table identifies which existing Core RPCs can be
  wrapped and which application-level RPCs must be added.

## 4. Milestone 1: local control protocol

Define `control.v1.ControlService` around user intent rather than exposing raw
core internals.

### Snapshot and events

- [ ] `GetSnapshot(Empty) -> Snapshot`
- [ ] `WatchEvents(WatchRequest) -> stream Event`
- [ ] Snapshot contains daemon/core versions, compatibility state, connection
      state, active profile, requested/effective mode, selected outbound,
      traffic/system stats, agent health, and last actionable error.
- [ ] Event uses `oneof` variants for connection, profile, outbound, traffic,
      log, settings, agent, warning, and compatibility changes.
- [ ] Include monotonically increasing sequence numbers and snapshot revision.
- [ ] On sequence gaps, clients discard derived state and fetch a new snapshot.
- [ ] Bound each subscriber queue. A slow client receives a resync-required
      marker or is disconnected; it cannot block the daemon.

### Connection operations

- [ ] `Connect(ConnectRequest) -> OperationResult`
- [ ] `Disconnect(DisconnectRequest) -> OperationResult`
- [ ] `Restart(RestartRequest) -> OperationResult`
- [ ] Treat repeated requests idempotently.
- [ ] Return typed error codes for no active profile, invalid config, permission
      failure, incompatible version, agent unavailable, port conflict, and
      already-in-requested-state.
- [ ] Never accept an arbitrary filesystem path from an RPC client for the
      privileged daemon to open.

### Profiles

- [ ] `ListProfiles`, `GetProfile`, `AddRemoteProfile`, `AddLocalProfile`,
      `UpdateProfile`, `RefreshProfile`, `DeleteProfile`, `SetActiveProfile`.
- [ ] Use daemon-generated opaque profile IDs.
- [ ] Stream uploaded local content with a strict size limit rather than passing
      a local path.
- [ ] Redact subscription URLs and secrets by default.
- [ ] Return subscription usage, expiry, update interval, last successful
      refresh, last attempted refresh, and last refresh error.

### Outbounds and logs

- [ ] `ListOutboundGroups`, `SelectOutbound`, `TestOutbounds`.
- [ ] Test request identifies one outbound, one group, or all visible outbounds.
- [ ] `TailLogs` supports initial tail count, minimum level, and follow mode.
- [ ] `ClearLogs` clears the daemon's bounded application buffer, not external
      system service journals.

### Settings and operations

- [ ] `GetSettings`, `ValidateSettings`, `UpdateSettings`, `ResetSettings`.
- [ ] `ExportSettings` returns redacted output unless inclusion of secrets is
      explicitly requested and authorized.
- [ ] `ImportSettings` validates the entire candidate before atomic commit.
- [ ] `GetServiceInfo`, `SetAutoConnect`, `GetDiagnostics`.
- [ ] Add protocol-level validation and human-readable field errors.

### Compatibility rules

- [ ] Every snapshot includes API major/minor and daemon semantic version.
- [ ] Reject mismatched major versions with a clear upgrade instruction.
- [ ] Negotiate optional capabilities for minor-version additions.
- [ ] Reserve fields instead of reusing removed protobuf field numbers.
- [ ] Add a Buf/protobuf breaking-change check in CI.

Exit criteria:

- Protocol documentation includes every request, response, error, event, state
  transition, redaction rule, and version behavior.
- Fake server tests prove reconnection, resynchronization, and compatibility
  handling before daemon integration begins.

## 5. Milestone 2: core daemon

Implemented upstream in `hiddify-core`, tracked here as a release dependency.

### Lifecycle and state

- [ ] Add `hiddify-core daemon run` with no terminal dependency.
- [ ] Acquire an exclusive state-directory lock before initializing the core.
- [ ] Detect another Hiddify GUI/core process and return a remediation message.
- [ ] Initialize storage, settings, profile scheduler, core, control IPC, logs,
      event broadcaster, and agent coordination in a deterministic order.
- [ ] Run idle while disconnected without creating TUN or proxy listeners.
- [ ] Gracefully stop active networking, restore requested external state, flush
      storage, close IPC, and remove stale socket files on shutdown.

### Storage

- [ ] Linux state: `/var/lib/hiddify`.
- [ ] macOS state: `/Library/Application Support/Hiddify`.
- [ ] Windows state: `%ProgramData%\Hiddify`.
- [ ] Persist profiles, active profile ID, typed settings, selected mode,
      auto-connect, and schema version.
- [ ] Store validated generated configurations as owner-only files.
- [ ] Use atomic temp-file + fsync + rename replacement for mutable files.
- [ ] Create a backup before storage schema migration and retain the last known
      working profile config.
- [ ] Ensure exactly one active profile at the storage layer.

### Profile correctness

- [ ] Complete existing profile repository method/schema mismatches.
- [ ] Parse Hiddify subscription headers and local formats consistently with the
      current GUI.
- [ ] Validate downloaded/imported content through the core before committing.
- [ ] If refresh fails or generates an invalid config, retain the previous
      profile and running connection.
- [ ] Refresh remote profiles according to the explicit profile interval.
- [ ] Back off failed refreshes with jitter and cap repeated retries.
- [ ] Do not log full subscription URLs, embedded credentials, config bodies,
      Warp keys, LAN-sharing passwords, or authorization headers.

### Connection state machine

- [ ] Implement explicit states: stopped, starting, started, stopping,
      reconnect-wait, and failed.
- [ ] Serialize lifecycle mutations; never run start/stop/restart concurrently.
- [ ] Preserve the last requested operation and attach an operation ID to events.
- [ ] Keep current connection alive while validating settings/profile changes.
- [ ] Apply changes requiring restart as one controlled restart after commit.
- [ ] On core crash, publish failure and retry only when auto-connect or a still
      active connect request permits it.
- [ ] Retry auto-connect with jittered exponential backoff capped at five
      minutes. Explicit disconnect cancels retries until the next connect.

### Modes

- [ ] TUN: daemon creates and owns TUN/routes/DNS with required privileges.
- [ ] Local proxy: daemon binds mixed proxy to loopback by default.
- [ ] System proxy: daemon starts the loopback proxy, then asks the user agent to
      apply desktop proxy settings and verifies acknowledgment.
- [ ] LAN sharing requires explicit enablement, validated bind address, optional
      authentication, and a visible security warning.
- [ ] Validate port uniqueness and availability before changing a running mode.

### Autostart semantics

- [ ] Installing/enabling the service starts the daemon at boot.
- [ ] `auto_connect` is a separate setting and defaults to false.
- [ ] If enabled, connect the active profile after initialization.
- [ ] If no active profile exists, remain idle and emit an actionable warning.
- [ ] Service restart must not silently enable auto-connect.

Exit criteria:

- A daemon survives client detach/reconnect and reboot.
- Lifecycle operations, profile persistence, auto-connect, and all three modes
  pass integration tests without direct client access to daemon files.

## 6. Milestone 3: secure local IPC

### Endpoints

- [ ] Linux: `/run/hiddify/control.sock`.
- [ ] macOS: `/var/run/hiddify/control.sock`.
- [ ] Windows: `\\.\pipe\hiddify-control`.
- [ ] No TCP management listener by default.
- [ ] Remove stale Unix sockets only after verifying that no live daemon owns
      them.

### Authorization

- [ ] Installer records one designated controlling desktop user.
- [ ] Unix socket owner/group/mode admits only that user and root.
- [ ] Windows named-pipe DACL admits only the designated user, LocalSystem, and
      Administrators.
- [ ] Verify peer identity at connection time; do not trust a client-supplied UID
      or username.
- [ ] Authorize secret export and service mutations separately from read-only
      status operations where OS APIs permit it.
- [ ] Set request size, concurrent stream, connection, and deadline limits.
- [ ] Fuzz protobuf decoding and reject malformed/oversized messages cleanly.

Exit criteria:

- Authorized clients work without elevation.
- A second local user cannot read status, logs, profiles, or secrets and cannot
  mutate connection state.
- No management port appears in a TCP/UDP listener scan.

## 7. Milestone 4: thin CLI client

The `hiddify-tui` executable provides both interactive and scriptable behavior.

### Invocation rules

- [ ] With no subcommand, launch the TUI only if stdin and stdout are terminals.
- [ ] Without a terminal, print concise help and exit with usage status.
- [ ] Support `--socket`, `--timeout`, `--json`, `--no-color`, and `--version`.
- [ ] Never start or elevate the daemon implicitly.
- [ ] Print exact service remediation commands when the daemon is unavailable.

### Commands

- [ ] `hiddify-tui status [--watch]`
- [ ] `hiddify-tui connect [--profile ID] [--mode MODE]`
- [ ] `hiddify-tui disconnect`
- [ ] `hiddify-tui restart`
- [ ] `hiddify-tui profile list|show|add|refresh|rename|activate|delete`
- [ ] `hiddify-tui outbound list|select|test`
- [ ] `hiddify-tui logs [--follow] [--level LEVEL] [--tail N]`
- [ ] `hiddify-tui settings show|validate|set|reset|import|export`
- [ ] `hiddify-tui autoconnect status|enable|disable`
- [ ] `hiddify-tui service status|start|stop|enable|disable|install|uninstall`
- [ ] `hiddify-tui agent status`
- [ ] `hiddify-tui migrate gui [--dry-run] [--settings FILE]`
- [ ] `hiddify-tui diagnostics`

### Output contract

- [ ] Human output goes to stdout; actionable failures go to stderr.
- [ ] JSON mode returns one stable object for unary commands and JSON Lines for
      watch/follow commands.
- [ ] JSON includes a schema version and typed error code.
- [ ] Exit codes:
  - `0`: success.
  - `2`: usage or local validation error.
  - `3`: daemon unavailable or incompatible.
  - `4`: daemon rejected or failed the requested operation.
  - `5`: authorization/privilege failure.
- [ ] Confirm destructive interactive actions; require `--yes` when stdin is
      noninteractive.

Exit criteria:

- Shell scripts can manage profiles and connections without parsing presentation
  text.
- CLI conformance tests lock JSON fields and exit codes.

## 8. Milestone 5: terminal UI

### Global behavior

- [ ] Use Bubble Tea v2 with alternate-screen rendering and automatic terminal
      color downsampling.
- [ ] Reconstruct state from one snapshot plus the event stream.
- [ ] Reconnect with bounded backoff after daemon restart and resynchronize on
      missed events.
- [ ] Support resizing, UTF-8 and wide characters, no-color mode, and 80x24 as
      the minimum usable layout.
- [ ] Keep user-facing strings in one package for future localization; ship
      English only in V1.
- [ ] `q` and `Ctrl+C` detach and never disconnect.
- [ ] Use a separate confirmed action/key for disconnect and destructive actions.
- [ ] Show a persistent footer with navigation, connection state, daemon status,
      and active transient operation.

### Dashboard

- [ ] Show connection state, uptime, active profile, requested/effective mode,
      active outbound, IP when available, current up/down, totals, connection
      counts, memory, daemon/core versions, and agent health.
- [ ] Provide connect/disconnect/restart actions.
- [ ] Show the last actionable warning without hiding live status.

### Profiles

- [ ] Table/list with active marker, type, name, last refresh, expiry, remaining
      quota, refresh status, and update interval.
- [ ] Fuzzy filtering and stable sort modes.
- [ ] Add URL, paste content, or choose a local file.
- [ ] Refresh one/all, rename, activate, inspect, and confirmed delete.
- [ ] Never display a complete secret-bearing URL by default.

### Proxies

- [ ] Expand/collapse outbound groups.
- [ ] Show selected state, display tag, protocol/type, secure indicator, delay,
      last test time, endpoint summary, and traffic where available.
- [ ] Filter and sort; test one/group/all; select only selectable entries.
- [ ] Keep cursor selection stable as delay results stream in.

### Logs

- [ ] Bounded viewport with pause/follow, level filter, text filter, copy/export,
      and clear-buffer action.
- [ ] Make daemon-side redaction visible in documentation; do not attempt to
      reconstruct removed sensitive fields.

### Settings

- [ ] Group settings into General, Connection, DNS, Inbounds/LAN, TUN, Updates,
      and Service.
- [ ] Validate edits before saving and show field-level errors.
- [ ] Make restart-required changes explicit and offer one confirmed restart.
- [ ] Include redacted/full JSON import/export actions.

### Service and diagnostics

- [ ] Show service install/enable/running state, daemon and agent health, paths,
      API compatibility, privileges, active listeners, and last service error.
- [ ] Present commands to fix problems; do not prompt for passwords inside the
      alternate screen.

Exit criteria:

- Golden tests cover every screen at minimum and wide sizes.
- All primary workflows are keyboard-only and usable over SSH.
- Killing or quitting the TUI never changes daemon connection state.

## 9. Milestone 6: user-session system-proxy agent

The agent exists because system proxy settings belong to the logged-in desktop
session, while the daemon runs as root/LocalSystem.

- [ ] Keep `hiddify-agent` independent from the core and TUI renderer.
- [ ] Start only at user login; remain idle unless daemon requests system-proxy
      mode.
- [ ] Authenticate to the daemon using the same local OS identity controls.
- [ ] Save the exact previous proxy state before the first change.
- [ ] Apply proxy host/port/bypass settings using native per-user APIs.
- [ ] Acknowledge effective state and report platform errors to the daemon.
- [ ] Maintain a heartbeat/lease. Restore previous state if the lease expires,
      the daemon disconnects, mode changes, or the agent is shutting down.
- [ ] After user login, query desired daemon state and reapply system proxy when
      a connection was established before the session existed.
- [ ] Make restore idempotent and retain recovery state across agent crashes.
- [ ] On uninstall, restore prior state before deleting recovery data.

Platform implementations:

- [ ] Linux: support the selected desktop proxy backend(s); detect unsupported
      sessions and fall back to local-proxy instructions rather than claiming
      success.
- [ ] macOS: use SystemConfiguration/network service APIs in user context.
- [ ] Windows: update current-user Internet Settings and notify the system of
      configuration changes.

Exit criteria:

- Exact prior proxy state is restored after normal disconnect, agent crash,
  daemon crash/lease loss, reboot, mode change, and uninstall.

## 10. Milestone 7: native services and installation

### Linux

- [ ] Hardened systemd system service for `hiddify-core daemon run`.
- [ ] systemd user unit for `hiddify-agent`.
- [ ] Package post-install creates state/runtime directories and designated-user
      access without making them world-readable.
- [ ] Evaluate required capabilities versus root; apply the smallest working
      privilege set and systemd hardening compatible with TUN and DNS/routes.

### macOS

- [ ] Root LaunchDaemon for core service.
- [ ] per-user LaunchAgent for proxy agent.
- [ ] Signed installer package places binaries, plists, state directories, and
      uninstall helper in stable locations.
- [ ] Production build is signed and notarized; development CI may be unsigned.

### Windows

- [ ] Automatic LocalSystem Windows Service for core daemon.
- [ ] Per-user scheduled startup task or approved equivalent for proxy agent.
- [ ] Named-pipe ACL references the installer-designated user SID.
- [ ] MSI installs service, task, binaries, runtime assets, and uninstall logic.
- [ ] Production binaries/installer are Authenticode-signed; development CI may
      be unsigned.

### Common service CLI behavior

- [ ] `install`, `enable`, `start`, `stop`, `restart`, `disable`, and `uninstall`
      are idempotent.
- [ ] Installation/elevation is explicit and occurs outside the TUI.
- [ ] Stop has a bounded graceful timeout followed by platform-standard service
      termination, with recovery cleanup on next start.
- [ ] Uninstall warns about an active connection, disconnects cleanly, restores
      proxy state, and preserves user profiles unless `--purge` is supplied.
- [ ] `--purge` is destructive, requires explicit confirmation, and reports the
      exact removed state directory.

Exit criteria:

- Clean VMs on all platforms pass install -> connect -> reboot -> status ->
  disconnect -> uninstall scenarios.

## 11. Milestone 8: migration from Hiddify GUI

- [ ] Require the GUI to be fully exited and confirm no GUI-owned core is active.
- [ ] Discover current per-platform GUI data locations without modifying them.
- [ ] Copy the SQLite database, profile directory, and optional settings export
      to a private temporary snapshot; read only the snapshot.
- [ ] Support current Drift profile schema v5:
  - IDs/types, active flag, name, URL, last update, update interval.
  - Subscription upload/download/total/expiry and support/web URLs.
  - Populated headers, profile override, and user override.
- [ ] Import local profile content and remote subscription metadata.
- [ ] Optionally import the GUI settings JSON, preserving unknown advanced keys.
- [ ] Validate each profile with the daemon before committing it.
- [ ] Detect exact duplicates and report them without adding a second copy.
- [ ] Create a destination backup and write migration version/result metadata.
- [ ] Implement `--dry-run` with no destination mutation.
- [ ] On partial failure, roll back the destination transaction and retain the
      diagnostic report.
- [ ] Never delete, rewrite, lock, or mark the GUI data as migrated.
- [ ] Clearly state that GUI and daemon must not run concurrently after import.

Exit criteria:

- Migration fixtures cover current GUI data, missing profile files, corrupt
  rows, duplicates, settings without secrets, and settings with private fields.
- A fixture imported by the current GUI can be migrated and connected by the
  daemon with equivalent active profile and essential settings.

## 12. Milestone 9: testing and quality gates

### Unit tests

- [ ] CLI parsing, output schemas, and exit-code mapping.
- [ ] TUI models/reducers, focus, resize, filtering, confirmation, and errors.
- [ ] Snapshot/event application, sequence-gap recovery, and reconnect logic.
- [ ] Settings validation and unknown-field preservation.
- [ ] Secret redaction for logs, URLs, configs, and exports.
- [ ] Migration mapping, duplicate detection, rollback, and dry-run.
- [ ] Agent proxy-state serialization, lease expiry, and idempotent restoration.

### Integration tests

- [ ] Fake control server for deterministic client tests.
- [ ] Real daemon over each local IPC transport.
- [ ] Multiple simultaneous CLI/TUI clients.
- [ ] Slow subscriber, daemon restart, core crash, corrupt event, and API version
      mismatch.
- [ ] Invalid profile/config, failed remote update, unavailable port, missing
      agent, insufficient privilege, and conflicting GUI/core process.
- [ ] Verify log/event memory stays bounded during long runs.

### TUI tests

- [ ] Golden render tests at 80x24, 120x40, and a wide layout.
- [ ] Resize, no-color, Unicode/wide glyphs, empty states, large lists, long
      profile names, reconnect overlays, and every confirmation dialog.
- [ ] PTY tests prove `q` and `Ctrl+C` detach without disconnecting.

### Platform/elevated tests

- [ ] TUN connectivity and cleanup on each OS.
- [ ] Local mixed proxy connectivity and loopback-only default binding.
- [ ] System-proxy apply and exact restoration for all required failure paths.
- [ ] IPC unauthorized-user rejection.
- [ ] Service install operations are idempotent.
- [ ] Auto-connect disabled default and enabled reboot behavior.

### CI gates

- [ ] Format, vet/static analysis, unit tests, race detector where supported,
      protobuf compatibility, generated-code cleanliness, and license scan.
- [ ] Build every release target reproducibly.
- [ ] Generate SBOMs and checksums.
- [ ] Report stripped binary sizes and dependency changes.
- [ ] Run minimum/current daemon compatibility matrices.
- [ ] Do not release if platform networking cleanup or proxy restoration tests
      fail.

## 13. Milestone 10: release and documentation

- [ ] Publish Linux x86-64/arm64 packages with systemd assets.
- [ ] Publish macOS universal amd64/arm64 package with launchd assets.
- [ ] Publish Windows x86-64 MSI and portable diagnostic client archive.
- [ ] Pin compatible Hiddify Core versions in release metadata.
- [ ] Generate SHA-256 checksums, SBOM, changelog, and upgrade notes.
- [ ] Document installation, service ownership, TUN permissions, system-proxy
      restoration, autostart versus auto-connect, migration, SSH use, CLI JSON,
      logs, diagnostics, recovery, and uninstall/purge.
- [ ] Document that stopping the service disconnects networking, while closing
      the TUI does not.
- [ ] Document recovery commands that restore proxy state and clean stale TUN or
      socket state without deleting profiles.

## 14. Recommended implementation order

1. [ ] Finish Milestone 0 and freeze the first protocol draft.
2. [ ] Implement generated client, fake server, CLI status, and TUI dashboard.
3. [ ] Implement upstream daemon snapshot/events and secure local IPC.
4. [ ] Add lifecycle operations, then profiles, then outbounds/logs/settings.
5. [ ] Complete the daily-driver TUI screens and scriptable CLI.
6. [ ] Add TUN and local proxy service packaging on Linux as the reference path.
7. [ ] Add the system-proxy agent and failure-safe restoration on Linux.
8. [ ] Port service/agent integration to macOS and Windows.
9. [ ] Add GUI migration after daemon storage and validation are stable.
10. [ ] Run platform acceptance suites, harden packaging, and publish beta.

## 15. Definition of done for V1

- [ ] `hiddify-tui` is demonstrably a thin local client and does not contain or
      load the Hiddify/sing-box networking core.
- [ ] The daemon is the single owner of profiles, settings, core lifecycle, and
      active networking.
- [ ] TUI exit never disconnects; service stop always cleans networking state.
- [ ] Daemon autostart and connection auto-connect are distinct controls.
- [ ] All three desktop platforms support TUN, local proxy, and automatic system
      proxy with tested restoration.
- [ ] Local management is OS-authorized and not exposed over TCP.
- [ ] Existing GUI data can be migrated explicitly without modifying its source.
- [ ] Essential operations work through both the TUI and stable JSON CLI.
- [ ] Required unit, integration, PTY, migration, security, platform, packaging,
      and reboot tests pass.
- [ ] Documentation allows a new user to install, migrate/add a profile, connect,
      enable auto-connect, diagnose failures, and uninstall safely.

## 16. Locked decisions and assumptions

- Separate local repository named `hiddify-tui`.
- Go implementation using Bubble Tea v2/Bubbles v2.
- Thin client plus an always-running privileged `hiddify-core.service`.
- Optional lightweight per-user agent for automatic system-proxy handling.
- Linux, macOS, and Windows are V1 targets.
- One designated desktop user per installation in V1.
- Local Unix socket/named pipe only; remote management is deferred.
- Auto-connect is opt-in and disabled by default.
- GUI data uses explicit one-time migration; there is no live shared state.
- The daemon remains running while disconnected but creates no TUN or proxy
  listeners until a connection is requested.
