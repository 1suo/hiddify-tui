# Hiddify Core compatibility baseline

Audited source: [`hiddify/hiddify-core`](https://github.com/hiddify/hiddify-core)
commit `db74dfc257d5becb4b4e9dbc7257a3dcdde20692` (2026-08-12).

## Build result

The parent source declares these nested repositories:

- `hiddify-sing-box` is a git submodule and supplies several local `replace`
  targets, including `wireguard-go` and `psiphon-tls`.
- `ray2sing` is a git submodule configured with a private SSH URL.

Both were initialized at their parent-pinned revisions for this audit:
`hiddify-sing-box` `170d8315cab7a8695fd80469073ed2f1d07d63af` and `ray2sing`
`caf5e9ac03eaba54dc339319670748d32a073a39`. At that official revision, the
full suite fails deterministically because the parent `go.mod` replaces dependencies with
`hiddify-sing-box/replace/psiphon-tls` and
`hiddify-sing-box/replace/wireguard-go`, but those directories do not exist at
the pinned `hiddify-sing-box` revision.

The development companion branch used by this repository restores those local
replacement paths and implements the daemon adapter described below. It is not
an upstream release artifact: package and release work must pin and publish
that exact compatible core revision before claiming general availability.

## Existing API assessment

| Current Core API | Existing behavior | control.v1 disposition |
| --- | --- | --- |
| `Core.Start`, `Stop`, `Restart` | Present; lifecycle is protected by a process-local lock. `StartRequest` can contain a filesystem path. | The companion daemon wraps stored, daemon-owned profile paths only. |
| `CoreInfoListener` | Existing stream of core state. | Normalize into `WatchEvents` connection events with sequence/revision. |
| `OutboundsInfo`, `MainOutboundsInfo`, `SelectOutbound`, `UrlTest*` | Present. | Wrap as outbound list/select/test operations. |
| `GetSystemInfo*`, `LogListener` | Present. | Map into snapshot and bounded event/log streams. |
| `ProfileService` | Proto and a repository implementation exist, but it is not registered in `grpc_server.go`; implementation method signatures/names do not match the generated service. | The companion daemon provides its own validated, daemon-owned profile API. |
| `GetSystemProxyStatus`, `SetSystemProxyEnabled` | Declared in the Core proto but implementation is commented out. | Do not expose directly; daemon must coordinate with `hiddify-agent`. |
| gRPC listener | `StartGrpcServer*` uses `net.Listen("tcp", ...)`, including insecure modes. | The companion daemon serves `control.v1` on a Unix socket and does not enable a TCP management listener. Windows named-pipe support remains release work. |

## Required daemon adapter work

1. Publish reviewed core artifacts that include the exact nested dependency
   revisions used for test and release builds.
2. Add and test the macOS socket and Windows named-pipe transports in the
   published core artifact.
3. Run the full compatibility matrix against the published minimum and current
   core versions before a beta release.
