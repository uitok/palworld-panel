# PalPanel v1.0.0

PalPanel v1.0.0 is the first stable production release. This release publishes
the Linux amd64 package only.

## Security defaults

- Authentication is enabled and first-run initialization creates a random
  admin token in a `0600` config file.
- The panel binds to `127.0.0.1:8080` by default. LAN or public listeners must
  be configured explicitly.
- Steam Workshop search has no embedded API key. Set `STEAM_WEB_API_KEY` in
  `palpanel.env` to enable it.
- The frontend uses same-origin `/api` and contains no build-time panel token.

## Install and upgrade

Extract `palpanel_v1.0.0_linux_amd64.tar.gz`, then run:

```bash
sudo ./palpanelctl install
```

Versioned programs are installed under `/opt/palpanel/v1.0.0` and activated
through `/opt/palpanel/current`. Configuration in `/etc/palpanel` and data in
`/var/lib/palpanel` are preserved during upgrades and default uninstall.

Portable operation is also supported with `./palpanelctl init` followed by
`./palpanelctl start`.

## Assets

- Linux amd64 package with native cgo/Oodle save parsing.
- Corresponding GPL-3.0 sav-cli and vendored gooz source.
- SHA-256 checksums, SPDX SBOM, and third-party license inventory.

Windows Launcher and native MinGW CI verification are included in the source,
but unsigned Windows executables are intentionally deferred until an
Authenticode signing certificate is available.

The previously embedded Steam Web API key must be considered compromised and
revoked in Steam's administrative interface; this release does not use it.
