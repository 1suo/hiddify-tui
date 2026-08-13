# hiddify-tui

`hiddify-tui` is a thin terminal client for a running
[`hiddify-core`](https://github.com/hiddify/hiddify-core). It talks to the
core's existing `Core` gRPC service over TCP (`127.0.0.1:17078`) and owns
profiles client-side; it never embeds or starts the networking core.

## Architecture

```text
hiddify-tui ──gRPC (Core service)──► hiddify-core (GUI, or HiddifyCli run)
             127.0.0.1:17078
```

- **Connection / status / outbounds / logs / settings** come from the core's
  `Core` gRPC service.
- **Profiles** are stored client-side in `~/.local/share/hiddify/profiles.json`
  and mirrored from the GUI's model: remote subscriptions (downloaded and
  header-parsed) and local configs. The active profile's content is handed to
  the core on connect via `Start(ConfigContent=...)`.

## Usage

Run `hiddify-tui` in a terminal for the interactive dashboard (profiles,
outbounds, and logs panes):

```sh
hiddify-tui                     # TUI, connecting to 127.0.0.1:17078
hiddify-tui --address 127.0.0.1:17078
```

Scriptable CLI:

```sh
hiddify-tui status
hiddify-tui --json status
hiddify-tui profile add https://sub.example.com/…
hiddify-tui profile add-file /path/to/config.json
hiddify-tui profile activate <id>
hiddify-tui connect
hiddify-tui outbound list
hiddify-tui logs --level warn
```

Exit codes: `0` success, `2` usage, `3` core unavailable, `4` rejected.

## Development

```sh
go build ./...
go test ./...
```

The core protocol schemas are vendored from `hiddify-core`
(`proto/hcore`, `proto/hcommon`) and generated with `buf` into `gen/`.
