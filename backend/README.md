# PalPanel Backend

Go/Gin backend for a Palworld dedicated-server panel. It manages a Windows edition Palworld Dedicated Server through either Windows SteamCMD or Docker + Wine, exposes REST APIs for lifecycle control, Palworld config, server mods, PalDefender, jobs, backups, and Palworld REST API proxying.

## Development

```powershell
cd D:\WL\me\pal\backend
$env:PANEL_TOKEN="change-me"
$env:HTTP_PROXY="socks5://127.0.0.1:10808"
$env:HTTPS_PROXY="socks5://127.0.0.1:10808"
$env:ALL_PROXY="socks5://127.0.0.1:10808"
go test ./...
go run ./cmd/palpanel
```

The API listens on `:8080` by default. Runtime data is stored in `D:\WL\me\pal\data` unless `PALPANEL_DATA_DIR` is set.

## Auth

All management endpoints require:

```http
Authorization: Bearer <PANEL_TOKEN>
```

`GET /api/health` is public.

## Runtime Modes

- `windows_steamcmd`: recommended for production Windows hosts. The backend downloads SteamCMD into `data/tools/steamcmd` when needed and installs the Windows dedicated server with `steamcmd +login anonymous +app_update 2394010 validate +quit`.
- `wine_docker`: keeps the existing Docker + Wine flow for Windows edition server mods and containerized operation. Official Palworld docs warn against Docker Desktop for production save-data IO, so update operations create backups first.

Startup arguments are managed separately from `PalWorldSettings.ini` through `GET/PUT /api/server/startup`.

## Main Endpoints

- Lifecycle: `POST /api/server/install`, `POST /api/server/update`, `POST /api/server/start`, `POST /api/server/stop`, `POST /api/server/restart`, `POST /api/server/bootstrap`
- Setup: `GET /api/server/prerequisites`, `GET/PUT /api/server/runtime`, `GET/PUT /api/server/startup`, `POST /api/server/initialize-config`
- Status/logs/jobs: `GET /api/server/status`, `GET /api/server/logs?tail=200`, `GET /api/jobs`, `GET /api/jobs/{id}`
- Backups: `POST /api/server/backup`, `GET /api/backups`
- Palworld config: `GET /api/config/palworld`, `PUT /api/config/palworld`, `GET /api/config/palworld/schema`, `POST /api/config/palworld/validate`
- Mods: `GET /api/mods`, `POST /api/mods/upload`, `POST /api/mods/workshop`, `POST /api/mods/{id}/enable`, `POST /api/mods/{id}/disable`, `DELETE /api/mods/{id}`
- PalDefender: `GET /api/security/paldefender/releases`, `GET /api/security/paldefender/status`, `POST /api/security/paldefender/install`, `POST /api/security/paldefender/update`, `POST /api/security/paldefender/rollback`, `GET/PUT /api/security/paldefender/config`, `POST /api/security/paldefender/apply-preset`, `POST /api/security/paldefender/rest-token`, `POST /api/security/paldefender/reload-config`

## Paths

- Palworld config: `data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini`
- Official server mods: `data/server/Mods/Workshop/<mod>/Info.json`
- Mod settings: `data/server/Mods/PalModSettings.ini`
- PalDefender binaries: `data/server/Pal/Binaries/Win64/PalDefender.dll` and `data/server/Pal/Binaries/Win64/d3d9.dll`
- PalDefender config after first server start: `data/server/Pal/Binaries/Win64/PalDefender/Config.json`

Server files, Wine prefix, tools, mods, saves, backups, logs, PalDefender files, and SQLite data are kept outside the backend source tree.
