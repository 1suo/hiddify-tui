# macOS packaging

`hiddify-tui` is a client of the Hiddify core. On macOS the core ships inside
the Hiddify GUI (it serves Core gRPC at `127.0.0.1:17078`); there is no
standalone macOS core in the official releases, so the TUI normally attaches to
the GUI's core.

`install.sh` installs the client to `/usr/local/bin`. If a standalone
`hiddify-core` is placed in the build dir, it also installs a root `LaunchDaemon`
to run it.

```sh
sudo packaging/macos/install.sh /path/to/build-dir
```

Build the darwin binaries first (from Linux or macOS):

```sh
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-tui ./cmd/hiddify-tui
GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags='-s -w' -o dist/hiddify-migrate ./cmd/hiddify-migrate
```
