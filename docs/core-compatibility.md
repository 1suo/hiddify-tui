# Hiddify Core compatibility baseline

Audited source: [`hiddify/hiddify-core`](https://github.com/hiddify/hiddify-core)
commit `db74dfc257d5becb4b4e9dbc7257a3dcdde20692` (2026-08-12).

`hiddify-tui` is a client of the core's existing `Core` gRPC service, served at
`127.0.0.1:17078` in the insecure modes (`SetupMode_GRPC_*_INSECURE`) that the
GUI and `HiddifyCli run` use.

## API surface used

| Core RPC | TUI use |
| --- | --- |
| `Start`, `Stop`, `Restart` | connect/disconnect/restart (config passed as `ConfigContent`) |
| `CoreInfoListener`, `GetSystemInfo`, `GetSystemInfoStream` | status snapshot + live stream |
| `OutboundsInfo`, `SelectOutbound`, `UrlTest` | outbound list/select/test |
| `LogListener` | log stream |
| `Parse` | validate imported configs |
| `ChangeHiddifySettings` | apply settings |

## Not used

- `ProfileService` — the proto and repository exist, but the service is not
  registered in `grpc_server.go` and its implementation does not match the
  generated service. Profiles are owned client-side (mirroring the GUI's model);
  the active profile's content is handed to `Start`.
- `GetSystemProxyStatus` / `SetSystemProxyEnabled` — present but not relied on.

## Build

The core must be built with the same protocol tag set as the GUI core, or
profiles that import in the GUI fail validation: Reality needs `with_utls`,
Hysteria2/TUIC need `with_quic`, WireGuard needs `with_wireguard`.

```sh
CGO_ENABLED=1 go build -trimpath -ldflags="-w -s -checklinkname=0" \
  -tags "with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,with_grpc,with_awg,tfogo_checklinkname0,with_naive_outbound,with_conntrack" \
  -o hiddify-core ./cmd/main
```

The pinned `hiddify-sing-box` submodule (`main`) has empty `replace/`
directories, so `go.mod`'s `./hiddify-sing-box/replace/*` directives do not
resolve. Build against the `extended` submodule branch or point the four
`replace` entries at the published `github.com/hiddify/*` forks.
