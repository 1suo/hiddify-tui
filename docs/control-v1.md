# Control API v1 contract

The source schema is [`proto/control/v1/control.proto`](../proto/control/v1/control.proto).
It is served only over the OS-authorized local endpoint; it is never exposed
over TCP by default.

## Versioning

- `Snapshot.api_major` must equal the client-supported major version. A client
  rejects any other major before rendering or mutating state.
- Additive minor changes are gated through `Snapshot.capabilities`. A client
  must ignore unknown fields, event variants, and capability names.
- Removed protobuf fields are reserved before a beta release. Field numbers are
  never reused.

## Snapshot and event recovery

The daemon returns a coherent `Snapshot` before a client starts
`WatchEvents(after_sequence = snapshot.event_sequence)`. Event `sequence` is
strictly increasing for one daemon lifetime and `revision` increases after
every externally visible state change.

If an event sequence is missing, an event contains `resync_required`, or a
stream reconnects after daemon restart, the client discards all derived state
and fetches a new snapshot. Per-client event queues are bounded in the daemon;
a slow client receives `resync_required` or is disconnected, never blocking
the daemon.

## Operation semantics and errors

`Connect`, `Disconnect`, `Restart`, refresh, selection, deletion, log clear,
and auto-connect changes return an `OperationResult`. Repeated lifecycle
requests are idempotent. `already_in_requested_state` is informational and
does not make the RPC fail.

The daemon maps expected failures to `ErrorCode`: no active profile, invalid
configuration, permission denied, incompatible version, missing user agent,
port conflict, not found, or an internal failure. Transport authentication and
authorization failures use the gRPC permission status as well as the typed
operation error where an operation result is available.

## Sensitive data

- Profile lists expose `redacted_url`, never a complete credential-bearing URL.
- `AddLocalProfile` accepts metadata and byte chunks only. It does not accept a
  local filesystem path, and the daemon applies a strict content limit.
- Settings export is redacted unless `include_secrets` is requested and the
  endpoint authorizes secret export.
- Logs and diagnostics must redact subscription URLs, authorization headers,
  profile bodies, private keys, Warp keys, and LAN credentials before they are
  buffered or emitted.

## Settings transaction

`ValidateSettings` validates the full candidate without mutation.
`ImportSettings` and `UpdateSettings` validate the complete candidate and
commit atomically only when valid. Unknown JSON keys are retained. A response
must identify restart-required changes through a capability or validation
message before the daemon performs one controlled restart.

## State transitions

The connection state is one of `stopped`, `starting`, `started`, `stopping`,
`reconnect-wait`, or `failed`. Lifecycle mutations are serialized. Explicit
disconnect cancels reconnect attempts; auto-connect is separately persisted
and defaults to false. Closing a client does not mutate connection state.
