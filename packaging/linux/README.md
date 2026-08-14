# Linux packaging

`hiddify-tui` is a client of a running `hiddify-core`. The packaged systemd
unit runs the core headless; the TUI connects to it at `127.0.0.1:17078`.

```text
/usr/lib/hiddify/hiddify-core   # core binary (built with the full tag set)
/usr/lib/hiddify/hiddify-core-daemon # persistent lifecycle wrapper
/usr/local/bin/hiddify-tui      # thin client
/usr/local/bin/hiddify-migrate  # GUI migration helper
```

`install.sh` installs the binaries and enables the `hiddify-core.service` unit.
It starts the service only if port 17078 is free; an existing core/VPN is not
stopped or restarted. The daemon supplies the core's required bootstrap
configuration and leaves the networking engine stopped until the TUI or CLI
connects a profile. Closing the TUI does not stop the service or tunnel.

The core must be built with the protocol tags; see
[`docs/core-compatibility.md`](../../docs/core-compatibility.md).
