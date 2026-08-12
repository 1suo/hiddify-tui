# Linux package contract

These systemd units are package assets for the target architecture: the
privileged core daemon owns network state and the user-session agent owns
desktop proxy state.  `hiddify-tui` only connects to the daemon's local control
socket; closing it must not affect an active connection.

The units intentionally name the target daemon entrypoint:

```text
/usr/lib/hiddify/hiddify-core daemon run --state-dir=/var/lib/hiddify --socket=/run/hiddify/control.sock
```

The upstream core does not implement that command yet, so these are not
installable release units.  See `docs/core-compatibility.md` for the upstream
work required before shipping a package.

## Installer requirements

A Linux package must:

1. Install `hiddify-core`, `hiddify-tui`, and `hiddify-agent` below
   `/usr/lib/hiddify` (or update these unit paths together).
2. Create `/var/lib/hiddify` with mode `0700`; systemd creates it from
   `StateDirectory=` at service start.
3. Create `/run/hiddify` with mode `0750`; systemd creates it from
   `RuntimeDirectory=` at service start.
4. Record one designated desktop user in `/etc/hiddify/core.env` as
   `HIDDIFY_ALLOWED_UID=<uid>`. The service passes it to the daemon, which
   owns the socket with mode `0600` and verifies `SO_PEERCRED` for that UID
   (or root) before accepting local RPCs. Never make the socket or state
   directory world-readable.
5. Enable `hiddify-core.service` for boot. Enable
   `hiddify-agent.service` only for the designated user's session.
6. On uninstall, ask before disconnecting, restore proxy settings through the
   agent, and retain profiles unless the user explicitly requests a purge.

The core currently runs as root because TUN, routes, and DNS integration need
to be measured against the completed daemon. The service limits filesystem
access, but does not claim a capability allow-list prematurely. A release
package must replace root with the smallest verified capability set compatible
with every supported service mode.

Validate these files with:

```sh
systemd-analyze verify packaging/systemd/hiddify-core.service \
  packaging/systemd/hiddify-agent.service
```
