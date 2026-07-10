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

The API listens on `127.0.0.1:8080` by default. Runtime data is stored in the repository `data` directory unless `PALPANEL_DATA_DIR` is set. `PANEL_TOKEN` is required by default and must not be empty or `change-me`. The Vite frontend port comes from `VITE_DEV_PORT` and defaults to `63107`; if it is opened from a LAN address such as `http://192.168.x.x:63107`, either include that exact origin in `PALPANEL_CORS_ORIGINS` or use `PALPANEL_CORS_ORIGINS=*` for local development.

Proxy environment variables such as `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY` are optional host-level settings. They are intentionally not enabled by default in `.env.example`.

## Production Package

The repository-level `scripts/package.sh` builds the supported Linux amd64 release. It includes native cgo/Oodle support in sav-cli, `palpanelctl`, and systemd units. `scripts/package.ps1` builds the unsigned Windows amd64 ZIP that is verified on a native runner and published from v1.0.2 onward.

In the Linux package the backend binary serves `frontend/dist` directly and `palpanelctl` sets package-local defaults:

- `PALPANEL_FRONTEND_DIST=<package>/frontend/dist`
- `PALPANEL_BACKEND_DIR=<package>/backend`
- `PALPANEL_DATA_DIR=<package>/data`
- `PALPANEL_RUNNER_DIR=<package>/backend/deployments/wine-runner`

`palpanelctl init` creates a private config and random `PANEL_TOKEN`. `palpanelctl install` uses `/opt/palpanel/<version>`, `/etc/palpanel`, and `/var/lib/palpanel` with separate systemd services for the web process and sav-cli.

## Auth

All management endpoints require:

```http
Authorization: Bearer <PANEL_TOKEN>
```

`GET /api/health` is public and returns `status`, `version`, `commit`, and `build_time`.

Optional role tokens:

- `PANEL_TOKEN`: admin, full access.
- `PANEL_OPERATOR_TOKEN`: operator, server/config/mod/player operations.
- `PANEL_VIEWER_TOKEN`: viewer, read-only.

`GET /api/auth/me` returns the authenticated role and permission names without returning any token. The destructive `world:reset` and `ai:config` permissions belong only to the admin role; Workshop translation uses the operator-compatible `mods:write` permission.

Set `PALPANEL_REQUIRE_AUTH=false` only for isolated local development.

## Production Web Serving

Build the frontend with `npm run build`, then set `PALPANEL_FRONTEND_DIST` to the frontend `dist` directory. The backend serves the SPA from this directory and falls back to `index.html` for routes such as `/dashboard`. For reverse proxies, forward `/api/*` to the backend and serve the frontend dist with HTTPS.

## Configuration And CLI

The backend accepts `--config <path>`, `--init-config`, and `--version`. Config files use strict `KEY=VALUE` parsing and are never sourced or executed. Process environment values take precedence over file values. Initial configuration uses a cryptographically random admin token and Unix mode `0600`.

Named provider settings are `PALPANEL_STEAM_API_BASE_URL`, `PALPANEL_STEAM_API_TIMEOUT_SECONDS` (15 seconds), and `PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS` (90 seconds). Optional `PALPANEL_LOG_LEVEL` accepts `debug`, `info`, `warn`, or `error`.

## Runtime Modes

- `windows_steamcmd`: recommended for production Windows hosts. The backend downloads SteamCMD into `data/tools/steamcmd` when needed and installs the Windows dedicated server with `steamcmd +login anonymous +app_update 2394010 validate +quit`.
- `wine_docker`: keeps the existing Docker + Wine flow for Windows edition server mods and containerized operation. Official Palworld docs warn against Docker Desktop for production save-data IO, so update operations create backups first. Version checks use the existing Wine runner image; build or install once before checking remote version in this mode.

The Wine runner build uses `PALPANEL_DOCKER_RUNNER_BASE_IMAGE` as its base image and keeps the pinned digest by default. If Docker Hub metadata requests time out, the backend retries the same image through comma-separated `PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS` prefixes such as `docker.1ms.run` or `registry.cyou`.

Startup arguments are managed separately from `PalWorldSettings.ini` through `GET/PUT /api/server/startup`. PalPanel always adds `-enable-gamedata-api`, `-log`, `-stdout`, and `-FullStdOutLogOutput` (case-insensitively deduplicated) so the bounded game-data proxy and live log collection work on Palworld 1.0 servers.

## Steam Workshop

Workshop search uses a backend-only Steam Web API key read exclusively from `STEAM_WEB_API_KEY`; no key is embedded in the binary. When the variable is absent, the status endpoint reports Workshop search as unconfigured. The key is never returned by API responses. `PALPANEL_WORKSHOP_APP_ID` defaults to `1623730` and is used by both Workshop search metadata and the existing SteamCMD `workshop_download_item` flow.

## AI Translation

Admins configure an OpenAI-compatible Base URL, model, and API key from Settings. Base URLs must use HTTPS; HTTP is accepted only for loopback addresses. Embedded URL credentials, redirects, and oversized responses are rejected. The API key is atomically stored in `data/secrets/ai-translation.key` with mode `0600` and is never returned by APIs, application logs, or audit records.

Workshop detail translation always fetches the authoritative description from Steam instead of accepting arbitrary browser text. Cached translations are keyed by Workshop ID, source SHA-256, target language, provider URL, and model, so source or model changes invalidate the cache automatically.

## World Reset

`GET /api/server/world` previews the active `DedicatedServerName`, save timestamp, and running state. Admin reset requests must include the previewed world ID and the exact confirmation phrase `RESET WORLD`. The asynchronous task rechecks the active world, saves/notifies and stops a running server, creates and verifies a `pre-world-reset` backup, then atomically stages only `SaveGames/0/<world-id>`.

When the server was running, the task starts it again and waits up to 180 seconds for a non-empty new `Level.sav`. Success removes the staged old world but keeps the verified backup. Start or generation failures stop the new process and retain both the backup and `.palpanel-world-reset/<job-id>` directory with recovery paths in the Job error. Server binaries, INI files, Workshop mods, PalDefender, and panel data are not removed.

## Server Logs

Wine mode mounts `data/logs` at `/data/logs` and mirrors PalServer stdout/stderr to both Docker stdout and `palserver.log`. Persistent files and Docker `json-file` output each use a 20 MiB limit with five backups. Windows SteamCMD mode uses the same bounded persistent-file policy.

`GET /api/server/logs` reads the persistent file first and falls back to `docker logs` only when the file is missing or unreadable. It remains available after the server stops and returns `source`, `available`, `reason`, and `updated_at` metadata alongside the compatible `logs` field.

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

Steam Build IDs remain the authoritative update signal for AppID `2394010`. The local Build ID comes from `data/server/steamapps/appmanifest_2394010.acf`; the latest public Build ID comes from SteamCMD `app_info_print 2394010`.

While the server is running, `GET /api/server/version` also reads the semantic game version from the official `/info` endpoint. The response reports the `1.0.0` compatibility target, a compatibility result, and warnings for enabled Workshop mods, PalDefender, and save-parser verification. An offline server still reports Build IDs without pretending the configuration schema version is the game version.

## Palworld 1.0 REST

The backend contracts cover the official `/info`, `/players`, `/settings`, `/metrics`, and `/game-data` response shapes. `GET /api/server/game-data` is deliberately excluded from periodic polling, has a 3-second default timeout, and rejects responses larger than 16 MiB. Configure those bounds with `PALPANEL_GAME_DATA_TIMEOUT_MS` and `PALPANEL_GAME_DATA_MAX_MB`.

Metrics retain the existing frontend fields and additionally map `basecampnum` to `active_bases` and expose `days`.

## Main Endpoints

- Lifecycle: `POST /api/server/install`, `POST /api/server/update`, `POST /api/server/update-if-needed`, `POST /api/server/start`, `POST /api/server/stop`, `POST /api/server/restart`, `POST /api/server/bootstrap`
- Safe restart: `POST /api/server/safe-restart`
- Setup: `GET /api/server/prerequisites`, `GET/PUT /api/server/runtime`, `GET/PUT /api/server/startup`, `POST /api/server/initialize-config`
- Session: `GET /api/auth/me`
- Status/logs/jobs: `GET /api/server/status`, `GET /api/server/version`, `POST /api/server/version/check`, `GET /api/server/metrics`, `GET /api/server/game-data`, `GET /api/server/logs?tail=200`, `GET /api/jobs`, `GET /api/jobs/{id}`
- World reset: `GET /api/server/world`, `POST /api/server/world/reset`
- Backups: `POST /api/server/backup`, `GET /api/backups`, `POST /api/backups/{name}/restore`
- Audit: `GET /api/audit-logs`
- Player access: `GET/POST/DELETE /api/players/bans`, `GET/PUT /api/players/whitelist`, `POST /api/players/{id}/kick`
- Palworld config: `GET /api/config/palworld`, `PUT /api/config/palworld`, `GET /api/config/palworld/schema`, `POST /api/config/palworld/validate`
- Mods: `GET /api/mods`, `GET /api/mods/workshop/search`, `GET /api/mods/workshop/{id}`, `POST /api/mods/workshop/{id}/translate`, `POST /api/mods/upload`, `POST /api/mods/workshop`, `POST /api/mods/{id}/enable`, `POST /api/mods/{id}/disable`, `DELETE /api/mods/{id}`
- AI translation: `GET/PUT /api/ai/translation/config`, `POST /api/ai/translation/test`
- PalDefender: `GET /api/security/paldefender/releases`, `GET /api/security/paldefender/status`, `POST /api/security/paldefender/install`, `POST /api/security/paldefender/update`, `POST /api/security/paldefender/rollback`, `GET/PUT /api/security/paldefender/config`, `POST /api/security/paldefender/apply-preset`, `POST /api/security/paldefender/rest-token`, `POST /api/security/paldefender/reload-config`

## Paths

- Palworld config: `data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini`
- Official server mods: `data/server/Mods/Workshop/<mod>/Info.json`
- Mod settings: `data/server/Mods/PalModSettings.ini`
- PalDefender binaries: `data/server/Pal/Binaries/Win64/PalDefender.dll` and `data/server/Pal/Binaries/Win64/d3d9.dll`
- PalDefender config after first server start: `data/server/Pal/Binaries/Win64/PalDefender/Config.json`
- Persistent PalServer log: `data/logs/palserver.log` (plus `.1` through `.5`)
- AI translation API key: `data/secrets/ai-translation.key`

Server files, Wine prefix, tools, mods, saves, backups, logs, PalDefender files, and SQLite data are kept outside the backend source tree.
