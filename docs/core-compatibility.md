# Hiddify Core compatibility baseline

Audited source: [`hiddify/hiddify-core`](https://github.com/hiddify/hiddify-core)
commit `db74dfc257d5becb4b4e9dbc7257a3dcdde20692` (2026-08-12).

## Build result

`go test ./...` cannot build this source checkout until its required nested
repositories are available:

- `hiddify-sing-box` is a git submodule and supplies several local `replace`
  targets, including `wireguard-go` and `psiphon-tls`.
- `ray2sing` is a git submodule configured with a private SSH URL.

The failure is deterministic: `go test ./...` reports missing
`hiddify-sing-box/replace/psiphon-tls/go.mod`. This document must be updated
with a green build and the exact nested dependency revisions before a beta can
claim compatibility.

## Existing API assessment

| Current Core API | Existing behavior | control.v1 disposition |
| --- | --- | --- |
| `Core.Start`, `Stop`, `Restart` | Present; lifecycle is protected by a process-local lock. `StartRequest` can contain a filesystem path. | Wrap with intent-specific requests; daemon must never accept a client filesystem path. |
| `CoreInfoListener` | Existing stream of core state. | Normalize into `WatchEvents` connection events with sequence/revision. |
| `OutboundsInfo`, `MainOutboundsInfo`, `SelectOutbound`, `UrlTest*` | Present. | Wrap as outbound list/select/test operations. |
| `GetSystemInfo*`, `LogListener` | Present. | Map into snapshot and bounded event/log streams. |
| `ProfileService` | Proto and a repository implementation exist, but it is not registered in `grpc_server.go`; implementation method signatures/names do not match the generated service. | Replace/repair behind the new daemon-owned profile API. |
| `GetSystemProxyStatus`, `SetSystemProxyEnabled` | Declared in the Core proto but implementation is commented out. | Do not expose directly; daemon must coordinate with `hiddify-agent`. |
| gRPC listener | `StartGrpcServer*` uses `net.Listen("tcp", ...)`, including insecure modes. | Replace with local Unix socket / Windows named-pipe transport; no TCP management listener by default. |

## Required daemon adapter work

1. Add `hiddify-core daemon run` with service-owned state and an exclusive
   state-directory lock.
2. Serve `proto/control/v1/control.proto` over OS-authorized local IPC.
3. Register and repair profile persistence/validation before mapping it into
   the control API.
4. Retire daemon-mode TCP management listeners and implement a bounded event
   broadcaster with snapshot revision and sequence recovery.
5. Publish artifacts that include the exact nested dependency revisions used
   for test and release builds.
