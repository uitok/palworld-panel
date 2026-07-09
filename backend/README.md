# PalPanel Backend

Go/Gin backend for a Palworld dedicated-server panel. It manages a Windows edition Palworld Dedicated Server through either Windows SteamCMD or Docker + Wine, exposes REST APIs for lifecycle control, Palworld config, server mods, PalDefender, jobs, backups, and Palworld REST API proxying.

## Development

```powershell
cd backend
$env:PANEL_TOKEN="replace-with-a-random-32-byte-token"
$env:PALPANEL_REQUIRE_AUTH="true"
$env:PALPANEL_CORS_ORIGINS="http://127.0.0.1:63107,http://localhost:63107"
$env:HTTP_PROXY="socks5://127.0.0.1:10808"
$env:HTTPS_PROXY="socks5://127.0.0.1:10808"
$env:ALL_PROXY="socks5://127.0.0.1:10808"
go test ./...
go run ./cmd/palpanel
```

The API listens on `:8080` by default. Runtime data is stored in the repository `data` directory unless `PALPANEL_DATA_DIR` is set. `PANEL_TOKEN` is required by default and must not be empty or `change-me`. If the Vite frontend is opened from a LAN address such as `http://192.168.x.x:63107`, either include that exact origin in `PALPANEL_CORS_ORIGINS` or use `PALPANEL_CORS_ORIGINS=*` for local development.

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

## Runtime Modes

- `windows_steamcmd`: recommended for production Windows hosts. The backend downloads SteamCMD into `data/tools/steamcmd` when needed and installs the Windows dedicated server with `steamcmd +login anonymous +app_update 2394010 validate +quit`.
- `wine_docker`: keeps the existing Docker + Wine flow for Windows edition server mods and containerized operation. Official Palworld docs warn against Docker Desktop for production save-data IO, so update operations create backups first. Version checks use the existing Wine runner image; build or install once before checking remote version in this mode.

The Wine runner build uses `PALPANEL_DOCKER_RUNNER_BASE_IMAGE` as its base image and keeps the pinned digest by default. If Docker Hub metadata requests time out, the backend retries the same image through comma-separated `PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS` prefixes such as `docker.1ms.run` or `registry.cyou`.

Startup arguments are managed separately from `PalWorldSettings.ini` through `GET/PUT /api/server/startup`.

## Steam Workshop

Workshop search uses a backend-only Steam Web API key. `STEAM_WEB_API_KEY` is optional and overrides the embedded default key when set; the key is never returned by API responses. `PALPANEL_WORKSHOP_APP_ID` defaults to `1623730` and is used by both Workshop search metadata and the existing SteamCMD `workshop_download_item` flow.

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
