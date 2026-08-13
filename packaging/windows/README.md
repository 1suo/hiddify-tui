# Windows packaging

`hiddify-tui` is a client of a running `hiddify-core`. `install.ps1` installs
the core as a `LocalSystem` headless service (`run -c <config>`); the TUI
connects to it at `127.0.0.1:17078`.

```text
%ProgramData%\Hiddify\hiddify-core.exe   # core binary
%ProgramData%\Hiddify\active-config.json # config the service runs
```

The core must be built with the protocol tags; see
[`docs/core-compatibility.md`](../../docs/core-compatibility.md).
