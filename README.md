# hiddify-tui

`hiddify-tui` is a thin, local terminal client for an always-running
`hiddify-core` daemon. It never embeds, starts, or owns the networking core.

## Current increment

This repository currently contains the first client-side foundation:

- a versioned protobuf control API draft in `proto/control/v1`;
- transport-neutral snapshot/client contracts and deterministic fake server;
- stable `status` formatting and JSON/exit-code tests;
- a dependency-free dashboard renderer, ready for Bubble Tea integration.

There is no daemon transport yet. Consequently, `hiddify-tui status` correctly
returns exit code `3` until the upstream daemon is available.

## Development

```sh
GOCACHE=/tmp/hiddify-tui-go-cache go test ./...
GOCACHE=/tmp/hiddify-tui-go-cache go run ./cmd/hiddify-tui --version
```

The complete delivery plan and cross-repository dependencies are in
[`TODO.md`](TODO.md).
