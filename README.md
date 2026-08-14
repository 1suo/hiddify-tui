# hiddify-tui

A terminal client for [`hiddify-core`](https://github.com/hiddify/hiddify-core).
It drives the core's `Core` gRPC service at `127.0.0.1:17078` and keeps profiles
client-side — attaching to an already-running core or spawning a headless one on
demand.

## Install

### Linux

```sh
curl -fsSL https://raw.githubusercontent.com/1suo/hiddify-tui/main/install.sh | sudo bash
```

Downloads and verifies the latest release, then installs the client and the
standalone core as a `hiddify-core.service` systemd unit. The Hiddify GUI is not
required. To install a specific release, set `HIDDIFY_TUI_VERSION`:

```sh
curl -fsSL https://raw.githubusercontent.com/1suo/hiddify-tui/main/install.sh | \
  sudo env HIDDIFY_TUI_VERSION=v0.1.0 bash
```

For client-only installation, download the deb/rpm package from GitHub Releases.

### macOS

```sh
# build from Linux or macOS
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui ./cmd/hiddify-tui
sudo ./packaging/macos/install.sh ./dist
```

Installs to `/usr/local/bin`. Official releases do not currently include a
standalone macOS core; when one is supplied in `dist`, the installer runs it
through the included headless daemon. Otherwise the TUI can only attach to an
existing compatible core, such as the GUI's.

### Windows

```powershell
# from an elevated PowerShell
.\packaging\windows\install.ps1 -BuildDir .\dist
```

Installs the client on `PATH`. A supplied standalone core can be used directly;
official releases do not currently publish one for Windows.

Prebuilt archives and deb/rpm packages are published with GitHub releases.
Release assets also contain rendered AUR, Homebrew, Scoop, Winget, and Nix
manifests. See [docs/packaging.md](docs/packaging.md) for publication details.

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

## License

MIT. See [LICENSE](LICENSE).
