# Windows packaging

The one-command installer downloads the TUI release plus the official
`hiddify-core.dll` Windows runtime, verifies both, and installs a small host for
the DLL. A SYSTEM startup task keeps its gRPC service available at
`127.0.0.1:17078`, independently of a terminal or the Hiddify GUI.

From an elevated PowerShell:

```powershell
irm https://raw.githubusercontent.com/1suo/hiddify-tui/main/install.ps1 | iex
```

The task starts only when port 17078 is free; an existing core/VPN is left
untouched. The official core currently provides a Windows x64 runtime only.
The installer fetches the unmodified core from its official v4.1.0 release;
that component remains covered by the upstream GPLv3-derived license.

Uninstall from an elevated PowerShell:

```powershell
& "$env:ProgramData\hiddify-tui\uninstall.ps1"
```

For a local build, place `hiddify-core.dll` and `libcronet.dll` beside the four
executables in `dist`, then run `packaging\windows\install.ps1 -BuildDir dist`.

Build the Windows binary first (from Linux or Windows):

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui.exe ./cmd/hiddify-tui
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-migrate.exe ./cmd/hiddify-migrate
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-core-daemon.exe ./cmd/hiddify-core-daemon
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-core-host.exe ./cmd/hiddify-core-host
```
