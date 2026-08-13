# Windows package contract

The core daemon runs as an automatic `LocalSystem` service; the system-proxy
agent runs as a per-user scheduled task at logon. `hiddify-tui` only connects to
the daemon's local control pipe and never owns networking itself.

## Layout

```text
%ProgramData%\Hiddify\hiddify-core.exe   # core daemon binary
%ProgramData%\Hiddify\hiddify-agent.exe   # session proxy agent
%ProgramData%\Hiddify\state\              # daemon-owned state
%ProgramData%\Hiddify\runtime\            # named-pipe ACL / runtime
\\.\pipe\hiddify-control                  # local control pipe
```

## Installer requirements

An MSI must:

1. Install the binaries above and run `install.ps1` with the installer-designated
   user SID as `DesignatedUser`.
2. Configure the named-pipe DACL to admit only the designated user,
   `LocalSystem`, and `Administrators`. Never make state world-readable.
3. Sign binaries and the installer with Authenticode. Development CI may be
   unsigned.
4. On uninstall, run `uninstall.ps1`, which restores the user's proxy state,
   disconnects cleanly, and preserves profiles unless `--purge` is requested.

## Deferred transport note

The named-pipe transport (`\\.\pipe\hiddify-control`) requires matching support
in the published core artifact; see `docs/core-compatibility.md`. Until that
support ships, the agent and client fall back to the Unix-domain socket path
where the platform provides it.
