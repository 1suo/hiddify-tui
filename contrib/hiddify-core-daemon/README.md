# Recovered daemon adapter source (coordination reference)

These files are the control-daemon adapter that turns the upstream
`github.com/hiddify/hiddify-core` build into the `hiddify-core daemon run`
service that `hiddify-tui` connects to.

They were recovered from the Aug 12 session that originally implemented them.
They are a **reference copy for coordination**, not part of the TUI build; the
canonical home is the `hiddify-core` repository. See
[`docs/daemon-build.md`](../../docs/daemon-build.md) for the full build and
publish runbook.

## Contents

- `cmd/cmd_daemon.go` — `daemon run` command (reconstructed; register via
  `mainCommand`).
- `v2/daemon/control.go` — the control.v1 `ControlServer`.
- `v2/daemon/runtime.go` — state lock + Unix socket lifecycle.
- `v2/daemon/auth_linux.go`, `auth_other.go` — peer authorization.
- `v2/hcore/connection_mode.go`, `outbounds.go`, `logs.go` — core wrappers.
- `v2/profile/profile_repository.go` — daemon-owned profile storage.
- `v2/config/builder.go`, `hiddify_option.go`, `v2/hcore/commands.go`,
  `buildconfighelper.go`, `start.go` — daemon-integration edits.
- `proto/control/v1/control.proto` — shared control protocol schema.

## Not recovered (recreate per the runbook)

- `buf.yaml`, `buf.gen.yaml` (regenerate; `buf generate` from the proto).
- `go.mod`/`go.sum` replace changes (four lines, documented in the runbook).
- `hiddify-sing-box/experimental/libbox/oom_report.go` patch (documented).
- `hiddify-sing-box/daemon/instance.go` and `v2/hcore/standalone.go` edits —
  re-derive against the current core if the daemon fails to build.

The definitive protocol schema lives in [`proto/control/v1/control.proto`](../../proto/control/v1/control.proto);
keep the daemon's copy in sync with it.
