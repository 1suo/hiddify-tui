# macOS packaging

`hiddify-tui` is a client of a running `hiddify-core`. The packaged LaunchDaemon
runs the core headless (`run -c <config>`); the TUI connects to it at
`127.0.0.1:17078`.

```text
/Library/Application Support/Hiddify/hiddify-core   # core binary
/usr/local/bin/hiddify-tui                           # thin client
/Library/LaunchDaemons/com.github.hiddify.core.plist # headless core service
```

The core must be built with the protocol tags; see
[`docs/core-compatibility.md`](../../docs/core-compatibility.md).
