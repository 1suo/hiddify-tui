# hiddify-tui

A terminal client for [`hiddify-core`](https://github.com/hiddify/hiddify-core).
It drives the core's `Core` gRPC service at `127.0.0.1:17078` and keeps profiles
client-side — attaching to an already-running core or spawning a headless one on
demand.

## Install

### Linux

```sh
make build                          # -> dist/hiddify-tui, dist/hiddify-migrate
sudo ./packaging/linux/install.sh ./dist
```

Installs the client and the core as a `hiddify-core.service` systemd unit.

### macOS

```sh
# build from Linux or macOS
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui ./cmd/hiddify-tui
sudo ./packaging/macos/install.sh ./dist
```

Installs to `/usr/local/bin`; attaches to the GUI's core (or a standalone core
via a root LaunchDaemon).

### Windows

```powershell
# from an elevated PowerShell
.\packaging\windows\install.ps1 -BuildDir .\dist
```

Installs the client on `PATH`; optionally registers a headless core service.

> Prebuilt binaries are published with GitHub releases. Package-manager
> packages (AUR, Homebrew, Scoop) are welcome contributions.

## Usage

```sh
hiddify-tui
```

All shortcuts are shown in the interface itself (pane titles, status line, and
footer).

## Development

```sh
go build ./...
go test ./...
```

Core protocol schemas are vendored from `hiddify-core` (`proto/hcore`,
`proto/hcommon`) and generated with `buf` into `gen/`.
