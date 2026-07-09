# PalPanel Backend

Go/Gin backend for a Palworld dedicated-server panel. It manages a Windows edition Palworld Dedicated Server through either Windows SteamCMD or Docker + Wine, exposes REST APIs for lifecycle control, Palworld config, server mods, PalDefender, jobs, backups, and Palworld REST API proxying.

## Development

```powershell
cd backend
$env:PANEL_TOKEN="replace-with-a-random-32-byte-token"
$env:PALPANEL_REQUIRE_AUTH="true"
$env:PALPANEL_CORS_ORIGINS="http://127.0.0.1:63107,http://localhost:63107"
go test ./...
go run ./cmd/palpanel
```

The API listens on `:8080` by default. Runtime data is stored in the repository `data` directory unless `PALPANEL_DATA_DIR` is set. `PANEL_TOKEN` is required by default and must not be empty or `change-me`. The Vite frontend port comes from `VITE_DEV_PORT` and defaults to `63107`; if it is opened from a LAN address such as `http://192.168.x.x:63107`, either include that exact origin in `PALPANEL_CORS_ORIGINS` or use `PALPANEL_CORS_ORIGINS=*` for local development.

Proxy environment variables such as `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY` are optional host-level settings. They are intentionally not enabled by default in `.env.example`.

## Production Package

The repository-level `scripts/package.sh` and `scripts/package.ps1` build offline packages for Linux amd64 and Windows amd64. In those packages the backend binary serves `frontend/dist` directly and the start scripts set package-local defaults:

- `PALPANEL_FRONTEND_DIST=<package>/frontend/dist`
- `PALPANEL_BACKEND_DIR=<package>/backend`
- `PALPANEL_DATA_DIR=<package>/data`
- `PALPANEL_RUNNER_DIR=<package>/backend/deployments/wine-runner`

First startup creates `config/palpanel.env` from the example file and writes a random `PANEL_TOKEN`.

## Auth

All management endpoints require:

```http
Authorization: Bearer <PANEL_TOKEN>
```

`GET /api/health` is public.

Optional role tokens:

- `PANEL_TOKEN`: admin, full access.
- `PANEL_OPERATOR_TOKEN`: operator, server/config/mod/player operations.
- `PANEL_VIEWER_TOKEN`: viewer, read-only.

Set `PALPANEL_REQUIRE_AUTH=false` only for isolated local development.

## Production Web Serving

Build the frontend with `npm run build`, then set `PALPANEL_FRONTEND_DIST` to the frontend `dist` directory. The backend serves the SPA from this directory and falls back to `index.html` for routes such as `/dashboard`. For reverse proxies, forward `/api/*` to the backend and serve the frontend dist with HTTPS.

## Configuration Priority

The backend reads process environment variables only. For source development, export variables in the shell or load `backend/.env.example` with your own tooling. For offline packages, edit `config/palpanel.env`; the start scripts load that file and then fill package-local path defaults for variables that were not set.

## Runtime Modes

- `windows_steamcmd`: recommended for production Windows hosts. The backend downloads SteamCMD into `data/tools/steamcmd` when needed and installs the Windows dedicated server with `steamcmd +login anonymous +app_update 2394010 validate +quit`.
- `wine_docker`: keeps the existing Docker + Wine flow for Windows edition server mods and containerized operation. Official Palworld docs warn against Docker Desktop for production save-data IO, so update operations create backups first. Version checks use the existing Wine runner image; build or install once before checking remote version in this mode.

The Wine runner build uses `PALPANEL_DOCKER_RUNNER_BASE_IMAGE` as its base image and keeps the pinned digest by default. If Docker Hub metadata requests time out, the backend retries the same image through comma-separated `PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS` prefixes such as `docker.1ms.run` or `registry.cyou`.

Startup arguments are managed separately from `PalWorldSettings.ini` through `GET/PUT /api/server/startup`.

## Steam Workshop

Workshop search uses a backend-only Steam Web API key. `STEAM_WEB_API_KEY` is optional and overrides the embedded default key when set; the key is never returned by API responses. `PALPANEL_WORKSHOP_APP_ID` defaults to `1623730` and is used by both Workshop search metadata and the existing SteamCMD `workshop_download_item` flow.

## Performance Knobs

Read-only Palworld REST calls use a short timeout so offline save pages can fall back quickly when the official REST API is unavailable:

```bash
PALPANEL_PALWORLD_REST_READ_TIMEOUT_MS=1200
PALPANEL_PERF_SLOW_REQUEST_MS=500
```

`PALPANEL_PERF_SLOW_REQUEST_MS` controls slow-request logging. API responses also include `Server-Timing` and `X-Palpanel-Cache` headers where cached read paths are used.

## Save Indexer

Offline players, guilds, bases, pals, inventories, and map points come from a separate read-only save indexer. Start the sidecar from the repository root:

```bash
docker build -t palpanel-sav-cli:local ./sav-cli
docker run --rm --network host \
  -v "$PWD/data/server:$PWD/data/server:ro" \
  palpanel-sav-cli:local
```

Then set:

```bash
PALPANEL_SAVE_INDEXER_ENABLED=true
PALPANEL_SAVE_INDEXER_URL=http://127.0.0.1:8090
PALPANEL_SAVE_INDEX_CACHE_DIR=../data/save-index
```

The sidecar is the self-developed Go `sav-cli` in `sav-cli/` and never writes back to `.sav`. It reports unsupported save containers or schema changes as `parser_incompatible`; the backend keeps the last successful cache when available and does not block other panel features.

## Version Checks

Palworld server version is represented as the Steam Build ID for AppID `2394010`, not a semantic game version. Local Build ID comes from `data/server/steamapps/appmanifest_2394010.acf`; latest Build ID comes from SteamCMD `app_info_print 2394010`.

## Main Endpoints

- Lifecycle: `POST /api/server/install`, `POST /api/server/update`, `POST /api/server/update-if-needed`, `POST /api/server/start`, `POST /api/server/stop`, `POST /api/server/restart`, `POST /api/server/bootstrap`
- Safe restart: `POST /api/server/safe-restart`
- Setup: `GET /api/server/prerequisites`, `GET/PUT /api/server/runtime`, `GET/PUT /api/server/startup`, `POST /api/server/initialize-config`
- Status/logs/jobs: `GET /api/server/status`, `GET /api/server/version`, `POST /api/server/version/check`, `GET /api/server/logs?tail=200`, `GET /api/jobs`, `GET /api/jobs/{id}`
- Backups: `POST /api/server/backup`, `GET /api/backups`, `POST /api/backups/{name}/restore`
- Audit: `GET /api/audit-logs`
- Player access: `GET/POST/DELETE /api/players/bans`, `GET/PUT /api/players/whitelist`, `POST /api/players/{id}/kick`
- Palworld config: `GET /api/config/palworld`, `PUT /api/config/palworld`, `GET /api/config/palworld/schema`, `POST /api/config/palworld/validate`
- Mods: `GET /api/mods`, `GET /api/mods/workshop/search`, `GET /api/mods/workshop/{id}`, `POST /api/mods/upload`, `POST /api/mods/workshop`, `POST /api/mods/{id}/enable`, `POST /api/mods/{id}/disable`, `DELETE /api/mods/{id}`
- PalDefender: `GET /api/security/paldefender/releases`, `GET /api/security/paldefender/status`, `POST /api/security/paldefender/install`, `POST /api/security/paldefender/update`, `POST /api/security/paldefender/rollback`, `GET/PUT /api/security/paldefender/config`, `POST /api/security/paldefender/apply-preset`, `POST /api/security/paldefender/rest-token`, `POST /api/security/paldefender/reload-config`

## Paths

- Palworld config: `data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini`
- Official server mods: `data/server/Mods/Workshop/<mod>/Info.json`
- Mod settings: `data/server/Mods/PalModSettings.ini`
- PalDefender binaries: `data/server/Pal/Binaries/Win64/PalDefender.dll` and `data/server/Pal/Binaries/Win64/d3d9.dll`
- PalDefender config after first server start: `data/server/Pal/Binaries/Win64/PalDefender/Config.json`

Server files, Wine prefix, tools, mods, saves, backups, logs, PalDefender files, and SQLite data are kept outside the backend source tree.
