# hiddify-tui

`hiddify-tui` is a thin, local terminal client for an always-running
`hiddify-core` daemon. It never embeds, starts, or owns the networking core.

## Current increment

This repository currently contains the first client-side foundation:

- a versioned protobuf/gRPC control API in `proto/control/v1`;
- Unix-domain-socket gRPC client, snapshot/event recovery, and deterministic fake server;
- stable `status` formatting and JSON/exit-code tests;
- an alternate-screen Bubble Tea v2 dashboard, opened by running
  `hiddify-tui` from a terminal, with explicit connect/disconnect/restart
  controls;
- a standalone `hiddify-agent` that restores the logged-in user's system proxy
  settings after an expired lease or clean session shutdown, with platform
  backends for GNOME (Linux), System Configuration via `networksetup` (macOS),
  and the Internet Settings registry (Windows);
- `hiddify-tui migrate gui`, which produces a read-only migration plan for the
  current Hiddify GUI SQLite profile database and its `configs/` directory;
  the legacy `hiddify-migrate` wrapper remains available;
- native service assets for Linux (systemd), macOS (launchd), and Windows
  (service + scheduled task) under `packaging/`, and a release script that
  produces checksums and SBOMs (`scripts/release.sh`).

The client connects to `/run/hiddify/control.sock` on Linux and
`/var/run/hiddify/control.sock` on macOS. The companion core branch implements
the Linux endpoint; published cross-platform core artifacts remain a release
dependency.

## Development

```sh
GOCACHE=/tmp/hiddify-tui-go-cache go test ./...
GOCACHE=/tmp/hiddify-tui-go-cache go run ./cmd/hiddify-tui --version
```

Create a redacted, non-mutating GUI migration plan with explicit source paths:

```sh
go run ./cmd/hiddify-tui migrate gui --database /path/to/db --configs /path/to/configs --dry-run
```

After reviewing the plan, import it only after completely exiting the GUI and
its core process:

```sh
go run ./cmd/hiddify-tui migrate gui --database /path/to/db --configs /path/to/configs \
  --apply --yes --gui-exited
```

Applying requires the compatible daemon control endpoint. The source database
and config files remain read-only in both modes.

The complete delivery plan and cross-repository dependencies are in
[`TODO.md`](TODO.md). Operational guidance (install, connect, recovery, and
uninstall) is in [`docs/operations.md`](docs/operations.md).
