# PalPanel v1.0.1

This release makes the production installation and first browser visit a
single-path experience on Linux amd64.

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

The installer resolves the latest GitHub Release, downloads the Linux package
and `SHA256SUMS`, verifies the archive, and installs the frontend, backend, and
native `sav-cli` together. It preserves existing configuration and data during
upgrades.

The safe default listener remains `127.0.0.1:8080`. Use
`--listen 0.0.0.0:8080` explicitly for LAN or public access. The installer
prints the panel URL and generated admin token after both services pass their
health checks.

## Usability changes

- The browser only asks for the panel token. Backend URL fields and browser
  storage for backend addresses have been removed.
- Frontend requests always use same-origin `/api`; development uses the Vite
  proxy target without exposing an address field to users.
- `palpanelctl install` accepts `--listen HOST:PORT` for unattended installs.
- The bootstrap installer can detect an existing Docker socket, while
  `--docker` and `--no-docker` keep that permission choice explicit.

## Assets

- Linux amd64 package with native cgo/Oodle save parsing.
- Corresponding GPL-3.0 sav-cli and vendored gooz source.
- SHA-256 checksums, SPDX SBOM, and third-party license inventory.

Windows Launcher and native MinGW CI verification remain in the source, but
unsigned Windows executables are not published.
