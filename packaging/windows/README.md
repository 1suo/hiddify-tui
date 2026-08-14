# Windows packaging

`hiddify-tui` is a client of the Hiddify core. On Windows the core ships inside
the Hiddify GUI (serves Core gRPC at `127.0.0.1:17078`); there is no standalone
Windows core in the official releases, so the TUI normally attaches to the
GUI's core.

`install.ps1` installs the client (and puts it on `PATH`). If standalone
`hiddify-core.exe` and `hiddify-core-daemon.exe` binaries are placed in the
build dir, it also creates a SYSTEM startup task and starts it only if port
17078 is free. An existing core/VPN is left untouched. This keeps the optional
standalone core independent of both the terminal and GUI.

```powershell
# from an elevated PowerShell
.\packaging\windows\install.ps1 -BuildDir .\dist
.\packaging\windows\uninstall.ps1
```

Build the Windows binary first (from Linux or Windows):

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui.exe ./cmd/hiddify-tui
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-migrate.exe ./cmd/hiddify-migrate
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-core-daemon.exe ./cmd/hiddify-core-daemon
```
