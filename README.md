# hiddify-tui

A terminal client for [`hiddify-core`](https://github.com/hiddify/hiddify-core).
It drives the core's `Core` gRPC service (`127.0.0.1:17078`) and keeps profiles
client-side — either attaching to an already-running core or spawning a headless
one on demand.

## Install

```sh
# 1. build the clients
make build                     # -> dist/hiddify-tui, dist/hiddify-migrate

# 2. get the core (or point --core-binary at your own)
./dist/hiddify-tui install-core

# 3. (optional) system-wide install + auto-start service
sudo ./packaging/linux/install.sh ./dist
```

## Usage

Interactive dashboard:

```sh
hiddify-tui
```

| Key | Action |
|-----|--------|
| `c` | connect / disconnect (press twice to confirm disconnect) |
| `r` | restart connection |
| `s` | start / stop the core |
| `A` | toggle auto-start |
| `tab` / `1` `2` `3` | switch pane |
| `j`/`k` | move cursor |
| `enter` | use selected profile / outbound |
| `a` / `d` | add / delete profile (profiles pane) |
| `t` | test outbound (outbounds pane) |
| `q` | quit |

Scriptable CLI:

```sh
hiddify-tui status
hiddify-tui --json status
hiddify-tui connect | disconnect | restart
hiddify-tui profile add https://sub.example.com/…
hiddify-tui profile add-file /path/config.json
hiddify-tui profile activate <id>
hiddify-tui outbound list | select <group> <out> | test <out>
hiddify-tui logs --level warn
hiddify-tui settings validate /path/settings.json
hiddify-tui migrate gui --database … --configs … --apply
```

Exit codes: `0` success, `2` usage, `3` core unavailable, `4` rejected.

## Development

```sh
go build ./...
go test ./...
```

Core protocol schemas are vendored from `hiddify-core` (`proto/hcore`,
`proto/hcommon`) and generated with `buf` into `gen/`.
