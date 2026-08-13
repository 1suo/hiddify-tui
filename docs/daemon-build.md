# Publishing a correctly-tagged Hiddify Core daemon

This is the build-and-publish runbook for the companion `hiddify-core`
daemon that `hiddify-tui` talks to over the local control socket. It exists
because the daemon artifact currently in the field was built **without** the
protocol build tags, so profiles that import fine in the GUI are rejected with
errors like:

```
profile content is invalid: [V2rayParser] invalid sing-box config:
initialize outbound[0]: uTLS, which is required by reality is not included
in this build, rebuild with -tags with_utls
```

or, for Hysteria2/TUIC:

```
... QUIC is not included in this build, rebuild with -tags with_quic
```

The GUI core is built with the full tag set; the daemon must be too. Nothing
else is wrong with the daemon.

## 1. Source prerequisites

The daemon is `github.com/hiddify/hiddify-core` at `main`
(`db74dfc257d5becb4b4e9dbc7257a3dcdde20692`) plus the local control-daemon
adapter. The adapter source is checked into this repository under
[`contrib/hiddify-core-daemon/`](../contrib/hiddify-core-daemon/README.md) as a
coordination reference; it is not part of the TUI build.

```sh
git clone https://github.com/hiddify/hiddify-core
cd hiddify-core
git checkout db74dfc257d5becb4b4e9dbc7257a3dcdde20692
```

Initialize the pinned submodules:

```sh
# ray2sing is public but configured with an SSH URL; use HTTPS.
git config submodule.ray2sing.url https://github.com/hiddify/ray2sing.git
git submodule update --init --recursive hiddify-sing-box ray2sing

# Pins used by the audited build:
#   hiddify-sing-box  170d8315cab7a8695fd80469073ed2f1d07d63af
#   ray2sing          caf5e9ac03eaba54dc339319670748d32a073a39
```

## 2. Apply the daemon adapter

Copy the files from `contrib/hiddify-core-daemon/` over the checkout:

```sh
cp -r contrib/hiddify-core-daemon/v2 ./
cp    contrib/hiddify-core-daemon/cmd/cmd_daemon.go cmd/
cp    contrib/hiddify-core-daemon/proto/control/v1/control.proto proto/control/v1/
```

The adapter adds:

- `cmd/cmd_daemon.go` — the `hiddify-core daemon run` command.
- `v2/daemon/` — control.v1 server (`control.go`), socket lifecycle
  (`runtime.go`), and peer authorization (`auth_linux.go`, `auth_other.go`).
- `v2/hcore/connection_mode.go`, `outbounds.go`, `logs.go` — core wrappers
  exposed to the daemon.
- `v2/profile/profile_repository.go` — daemon-owned profile storage.
- `v2/config/builder.go`, `hiddify_option.go`, `v2/hcore/commands.go`,
  `buildconfighelper.go`, `start.go` — small daemon-integration edits.
- `proto/control/v1/control.proto` — the shared control protocol schema.

## 3. Regenerate the control.v1 bindings

The daemon imports `github.com/hiddify/hiddify-core/v2/controlv1`. Regenerate
it from the proto with buf, using `buf.gen.yaml`/`buf.yaml` at the repo root
(the same `go_package` as the proto):

```sh
buf generate   # emits v2/controlv1/control.pb.go and control_grpc.pb.go
```

## 4. Fix the go.mod replace directives

The pinned `hiddify-sing-box` revision does not contain its `replace/`
subdirectories, so the four local replace paths must point at published
modules instead. Change these lines in `go.mod`:

```diff
-replace github.com/sagernet/wireguard-go => ./hiddify-sing-box/replace/wireguard-go
+replace github.com/sagernet/wireguard-go => github.com/hiddify/wireguard-go v0.0.0-20260207195137-b12022450359

-replace github.com/sagernet/tailscale => ./hiddify-sing-box/replace/tailscale
+replace github.com/sagernet/tailscale => github.com/hiddify/tailscale v1.92.4-sing-box-1.13-mod.6

-replace github.com/Psiphon-Labs/quic-go => ./hiddify-sing-box/replace/psiphon-quic-go
+replace github.com/Psiphon-Labs/quic-go => github.com/hiddify/psiphon-quic-go v0.0.0-20260212072127-47042a7c2475

-replace github.com/Psiphon-Labs/psiphon-tls => ./hiddify-sing-box/replace/psiphon-tls
+replace github.com/Psiphon-Labs/psiphon-tls => github.com/hiddify/psiphon-tls v0.0.0-20260205181946-4af85c2fb9f2
```

Also replace the `oom_profile.go` helper's private-runtime dependency in the
sing-box submodule so the daemon links against the public API:

```diff
--- hiddify-sing-box/experimental/libbox/oom_report.go
+++ hiddify-sing-box/experimental/libbox/oom_report.go
@@
 	"path/filepath"
 	"runtime"
+	"runtime/pprof"
 	"strings"
 	"time"
 
-	"github.com/sagernet/sing-box/experimental/libbox/internal/oomprofile"
 	"github.com/sagernet/sing-box/service/oomkiller"
@@
 func writeOOMProfile(destPath string, name string) {
-	filePath, err := oomprofile.WriteFile(destPath, name)
+	profile := pprof.Lookup(name)
+	if profile == nil {
+		return
+	}
+	filePath := filepath.Join(destPath, name+".pb")
+	file, err := os.Create(filePath)
 	if err != nil {
 		return
 	}
+	defer file.Close()
+	if err = profile.WriteTo(file, 0); err != nil {
+		_ = os.Remove(filePath)
+		return
+	}
 	chownReport(filePath)
 }
```

Then `go mod tidy`.

## 5. Build with the full protocol tag set

The canonical tag list is the `TAGS` variable in the `hiddify-core` Makefile:

```text
with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,with_grpc,with_awg,tfogo_checklinkname0,with_naive_outbound,with_conntrack
```

Build the standalone daemon binary (`./cmd/main`):

```sh
CGO_ENABLED=1 go build -trimpath -ldflags="-w -s -checklinkname=0" \
  -tags "with_gvisor,with_quic,with_wireguard,with_utls,with_clash_api,with_grpc,with_awg,tfogo_checklinkname0,with_naive_outbound,with_conntrack" \
  -o hiddify-core ./cmd/main
```

Notes:

- `with_naive_outbound` requires the cronet toolchain; see the Makefile
  `install_cronet` / `build-cronet` targets. If cronet is not available, drop
  `with_naive_outbound` (naive outbound only).
- A **minimal** build that fixes the reported protocol failures (Reality,
  Hysteria2, TUIC, WireGuard) only needs:

  ```sh
  CGO_ENABLED=1 go build -trimpath -ldflags="-w -s" \
    -tags "with_utls,with_quic,with_wireguard" -o hiddify-core ./cmd/main
  ```

- `go version -m hiddify-core` must show `CGO: enabled` and the module path
  `github.com/hiddify/hiddify-core/cmd/main`.

## 6. Verify before publishing

Import one link of each protocol through the daemon and confirm it validates:

```sh
hiddify-tui profile add-stdin <<< 'vless://...reality...'
hiddify-tui profile add-stdin <<< 'hysteria2://...'
hiddify-tui profile add-stdin <<< 'tuic://...'
hiddify-tui profile add-stdin <<< 'wireguard://...'
```

Each must succeed. A missing tag shows up as `rebuild with -tags with_XXX`.
Then smoke-test `daemon run`, connect, and a TUN-mode connect.

## 7. Publish

Ship the `hiddify-core` binary built above together with `hiddify-tui`,
`hiddify-agent`, and the platform service assets (see
[`packaging/`](../packaging/)). The daemon must be installed as a service with
the full tag set; never ship a daemon built without `with_utls` and
`with_quic`.
