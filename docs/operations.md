# Operations guide

`hiddify-tui` is a client of a running `hiddify-core`. Exiting or killing the
TUI does not disconnect an active connection. A core started on demand remains
alive, and packaged Linux installations use a persistent system service.

## Headless core

The TUI connects to the core's `Core` gRPC service at `127.0.0.1:17078`
(insecure). It first attaches to an existing core. If none is running and a
standalone `hiddify-core` binary is installed, the TUI and CLI start it without
the GUI. Recommended Linux setup:

```sh
make build
sudo ./packaging/linux/install.sh ./dist
```

Other supported arrangements are:

- an explicitly supplied binary through `--core-binary`, or
- any compatible external core already listening at `--address`.

`hiddify-tui install-core` installs the official standalone Linux core for the
current user. Official upstream releases currently provide standalone CLI
artifacts only for Linux; macOS and Windows require a separately supplied core
or an existing compatible process.

## Profiles

Profiles live in `~/.local/share/hiddify/profiles.json` (client-side). A remote
profile is a subscription URL; a local profile is a config document.

```sh
hiddify-tui profile add https://sub.example.com/…      # subscription
hiddify-tui profile add-file /path/to/config.json      # local config
hiddify-tui profile add-stdin < config.json            # paste
hiddify-tui profile list
hiddify-tui profile activate <id>
hiddify-tui profile refresh <id>                       # re-download subscription
hiddify-tui profile delete <id> --yes
```

## Connecting

```sh
hiddify-tui connect            # active profile
hiddify-tui connect --profile <id>
hiddify-tui disconnect
hiddify-tui restart
hiddify-tui status
hiddify-tui status --watch
```

Exit codes: `0` success, `2` usage, `3` core unavailable, `4` rejected.

## Logs, outbounds, settings

```sh
hiddify-tui logs --level warn
hiddify-tui outbound list
hiddify-tui outbound select <group> <outbound>
hiddify-tui outbound test <outbound>
hiddify-tui settings set /path/to/hiddify-settings.json
```

## Migration from the GUI

Exit the GUI first, then:

```sh
hiddify-tui migrate gui --database PATH --configs DIR --dry-run
hiddify-tui migrate gui --database PATH --configs DIR --apply
```

## SSH and noninteractive use

The TUI needs a terminal; the CLI does not. `--json` yields stable output and
`--no-color` disables ANSI color. Point `--address` at the core when it is not
on the default `127.0.0.1:17078`.
