# Linux packaging

`hiddify-tui` is a client of a running `hiddify-core`. The packaged systemd
unit runs the core headless; the TUI connects to it at `127.0.0.1:17078`.

```text
/usr/lib/hiddify/hiddify-core   # core binary (built with the full tag set)
/usr/local/bin/hiddify-tui      # thin client
/usr/local/bin/hiddify-migrate  # GUI migration helper
```

`install.sh` installs the binaries and the `hiddify-core.service` unit, which
runs `hiddify-core run -c /etc/hiddify/active-config.json`. Place the active
config there (the TUI hands configs to the core on connect; for a headless
service, the config is read from this file).

The core must be built with the protocol tags; see
[`docs/core-compatibility.md`](../../docs/core-compatibility.md).
