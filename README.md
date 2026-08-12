# hiddify-tui

`hiddify-tui` is a thin, local terminal client for an always-running
`hiddify-core` daemon. It never embeds, starts, or owns the networking core.

## Current increment

This repository currently contains the first client-side foundation:

- a versioned protobuf/gRPC control API in `proto/control/v1`;
- Unix-domain-socket gRPC client, snapshot/event recovery, and deterministic fake server;
- stable `status` formatting and JSON/exit-code tests;
- an alternate-screen Bubble Tea v2 dashboard, opened by running
  `hiddify-tui` from a terminal.

The client connects to `/run/hiddify/control.sock` on Linux and
`/var/run/hiddify/control.sock` on macOS. Until the upstream daemon implements
that endpoint, `hiddify-tui status` correctly returns exit code `3`.

## Development

```sh
GOCACHE=/tmp/hiddify-tui-go-cache go test ./...
GOCACHE=/tmp/hiddify-tui-go-cache go run ./cmd/hiddify-tui --version
```

The complete delivery plan and cross-repository dependencies are in
[`TODO.md`](TODO.md).
