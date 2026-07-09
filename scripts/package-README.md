# PalPanel Offline Package

This package contains a self-contained PalPanel backend binary, built frontend assets, the `sav-cli` sidecar binary, and the Wine runner Dockerfile used by the backend.

## Start

Linux:

```bash
./scripts/start.sh
```

Windows PowerShell:

```powershell
.\scripts\start.ps1
```

On first run the start script copies `config/palpanel.env.example` to `config/palpanel.env`, generates a random `PANEL_TOKEN`, and prints the local dashboard URL and token. Runtime data is stored in the package-local `data/` directory unless you override `PALPANEL_DATA_DIR`.

## Layout

- `bin/palpanel[.exe]`: backend API and frontend static file server.
- `bin/sav-cli[.exe]`: read-only save indexer sidecar.
- `frontend/dist/`: built React frontend served by the backend.
- `backend/deployments/wine-runner/`: Docker + Wine runner resources.
- `config/palpanel.env.example`: editable environment template.
- `scripts/start.*`: package start entrypoints.
- `checksums.txt`: SHA-256 checksums for package files.

`sav-cli` in the default offline packages is built with `CGO_ENABLED=0`. It handles zlib-based save containers and reports `parser_incompatible` for `PlM1` Oodle containers that require a cgo-enabled `gooz` build.

## Configuration

Edit `config/palpanel.env` after the first run. The start scripts set these package-local defaults when they are not present in the env file:

- `PALPANEL_FRONTEND_DIST=<package>/frontend/dist`
- `PALPANEL_BACKEND_DIR=<package>/backend`
- `PALPANEL_DATA_DIR=<package>/data`
- `PALPANEL_RUNNER_DIR=<package>/backend/deployments/wine-runner`

Official Palworld ports remain at their upstream defaults unless changed: game `8211`, query `27015`, and REST `8212`.
