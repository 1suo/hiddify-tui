# Packaging and publication

GitHub Releases are the source of truth. A `vX.Y.Z` tag builds Linux, macOS,
and Windows archives for amd64 and arm64, native deb/rpm packages, Syft SBOMs,
checksums, build-provenance attestations, and rendered package-manager
manifests.

## Package roles

- `hiddify-tui` deb/rpm and all third-party manifests install the client,
  migration helper, manual page, and shell completions.
- `hiddify-tui-daemon` deb/rpm installs only the lifecycle wrapper and systemd
  unit. It does not enable, start, stop, or restart the unit. A compatible
  `hiddify-core` must be installed separately at
  `/usr/lib/hiddify/hiddify-core`.

This split prevents a client upgrade from touching an active VPN.

## Release

1. Push a signed `vX.Y.Z` tag.
2. Verify `checksums.txt`, SBOM files, and the GitHub artifact attestation.
3. Download `package-manifests.tar.gz` from that release.
4. Submit each rendered file to its package repository.

The renderer can also be run locally:

```sh
scripts/render-package-manifests.sh 1.2.3 dist/checksums.txt
```

## Repositories

- **AUR:** publish `aur/PKGBUILD` and `aur/.SRCINFO` to an AUR package Git
  repository. Regenerate `.SRCINFO` with `makepkg --printsrcinfo` if editing the
  PKGBUILD after release.
- **Homebrew:** put `homebrew/hiddify-tui.rb` in `Formula/` in a tap. A tap is the
  appropriate first target; Homebrew core expects source builds and broader
  project adoption.
- **Scoop:** put `scoop/hiddify-tui.json` in a bucket repository. Scoop is the
  simplest Windows publication target for these portable archives.
- **Winget:** submit the three files under `winget/` to
  `microsoft/winget-pkgs`.
- **Nix:** use `nix/package.nix` in a flake or overlay; submit to nixpkgs after
  the package has a stable release history.

Repository credentials are intentionally not stored here. Automatic pushes to
an AUR repo, Homebrew tap, Scoop bucket, Winget fork, or nixpkgs fork should be
added only after those repositories and narrowly scoped release credentials
exist.

## Platform behavior

Linux desktop clipboard paste tries Wayland (`wl-paste`), X11 (`xclip` or
`xsel`), then Windows interop for WSL. macOS uses `pbpaste`; Windows uses
Windows PowerShell or PowerShell 7. Clipboard reads are explicit, capped at
8 MiB, and time out after two seconds.

The macOS and Windows manual installers only start an optional bundled core
when port 17078 is free. Existing listeners are left untouched. Package-manager
installs are client-only.
