# Windows packaging

`hiddify-tui` is a client of the Hiddify core. On Windows the core ships inside
the Hiddify GUI (serves Core gRPC at `127.0.0.1:17078`); there is no standalone
Windows core in the official releases, so the TUI normally attaches to the
GUI's core.

`install.ps1` installs the client (and puts it on `PATH`). If a standalone
`hiddify-core.exe` is placed in the build dir, it also registers it as a
`LocalSystem` headless service.

```powershell
# from an elevated PowerShell
.\packaging\windows\install.ps1 -BuildDir .\dist
.\packaging\windows\uninstall.ps1
```

Build the Windows binary first (from Linux or Windows):

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui.exe ./cmd/hiddify-tui
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-migrate.exe ./cmd/hiddify-migrate
```
