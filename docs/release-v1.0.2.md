# PalPanel v1.0.2

This release adds the first downloadable Windows amd64 package and applies
GPL-3.0-or-later consistently to the complete PalPanel project.

## Windows package

Download `palpanel_v1.0.2_windows_amd64.zip`, verify it against
`SHA256SUMS`, extract the whole ZIP, and double-click `PalPanel.exe`. The
Launcher initializes configuration, starts the backend and native `sav-cli`,
waits for both health checks, and opens the browser. Only the generated admin
token is required in the browser.

The executables are not Authenticode-signed. Windows SmartScreen may display
an unknown publisher warning. The ZIP is built and tested on GitHub's native
Windows runner with MinGW CGO before the Release job can publish it.

## Linux installation

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

The safe default listener remains `127.0.0.1:8080`. Existing configuration
and data are preserved during upgrades.

## License and source

Except for separately identified third-party materials, the backend,
frontend, Windows Launcher, management scripts, and sav-cli are licensed
under GPL-3.0-or-later. Every binary package includes `LICENSE` and the
third-party inventory.

The Release includes both a complete PalPanel corresponding-source archive
and the focused sav-cli archive with vendored gooz source and license.

## Assets

- `palpanel_v1.0.2_linux_amd64.tar.gz`
- `palpanel_v1.0.2_windows_amd64.zip` (unsigned)
- `palpanel_v1.0.2_source.tar.gz`
- `palpanel-sav-cli_v1.0.2_source.tar.gz`
- Linux SPDX SBOM, third-party inventory, and `SHA256SUMS`
