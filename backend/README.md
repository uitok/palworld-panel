# PalPanel Backend

Go/Gin backend for a Palworld dedicated-server panel. It manages a Windows edition Palworld Dedicated Server through either Windows SteamCMD or Docker + Wine, exposes REST APIs for lifecycle control, Palworld config, server mods, PalDefender, jobs, backups, and Palworld REST API proxying.

## Development

```powershell
cd backend
$env:PALPANEL_REQUIRE_AUTH="true"
$env:PALPANEL_CORS_ORIGINS="http://127.0.0.1:63107,http://localhost:63107"
go test -p=1 ./...
go run ./cmd/palpanel
```

The API listens on `127.0.0.1:8080` by default. Runtime data is stored in the repository `data` directory unless `PALPANEL_DATA_DIR` is set. On a new database, open the web UI and register the first administrator; later visits use the server-managed session. The Vite frontend port comes from `VITE_DEV_PORT` and defaults to `63107`; if it is opened from a LAN address such as `http://192.168.x.x:63107`, include that exact origin in `PALPANEL_CORS_ORIGINS` or use `PALPANEL_CORS_ORIGINS=*` only for isolated local development.

Proxy environment variables such as `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY` are optional host-level settings. They are intentionally not enabled by default in `.env.example`.

## Production Package

The repository-level `scripts/package.sh` builds the supported Linux amd64 release. It builds the frontend first and embeds it in the Go binary, then packages native cgo/Oodle support in sav-cli, `palpanelctl`, and systemd units. `scripts/package.ps1` builds the equivalent unsigned Windows amd64 ZIP.

In the Linux package the backend binary serves the embedded UI and `palpanelctl` sets package-local defaults:

- `PALPANEL_BACKEND_DIR=<package>/backend`
- `PALPANEL_DATA_DIR=<package>/data`
- `PALPANEL_RUNNER_DIR=<package>/backend/deployments/wine-runner`

`palpanelctl init` creates a private configuration and directs the user to browser registration. `palpanelctl install` uses `/opt/palpanel/<version>`, `/etc/palpanel`, and `/var/lib/palpanel` with separate systemd services for the web process and sav-cli. Installation enables and starts both units; portable mode and the Windows Launcher also start sav-cli before the backend and wait for its health endpoint.

## Auth

Browser authentication uses the `palpanel_session` HttpOnly, SameSite=Lax Cookie returned by registration or login. State-changing session requests must also be same-origin. Automation uses a revocable development key created in Settings:

```http
Authorization: Bearer ppk_...
```

The complete development key is returned only in its create response; SQLite stores only its digest. Revocation and `palpanel admin reset-password` invalidate credentials immediately. `GET /api/health`, `GET /api/ready`, `GET /api/auth/status`, `POST /api/auth/register`, and `POST /api/auth/login` are public as required by startup and login flows.

`GET /api/auth/me` returns the authenticated role and permission names without returning credentials. The destructive `world:reset` and `ai:config` permissions belong only to the admin role; Workshop translation uses the operator-compatible `mods:write` permission.

Set `PALPANEL_REQUIRE_AUTH=false` only for isolated local development.

## Production Web Serving

Release builds use the `embed_webui` build tag and serve the embedded SPA, including route fallback and immutable asset caching, from the API port. Normal development builds may set `PALPANEL_FRONTEND_DIST` to an external frontend `dist` directory; without either source, API routes continue to work and return JSON 404 responses. For reverse proxies, forward the single backend origin over HTTPS.

## Configuration And CLI

The backend accepts `--config <path>`, `--init-config`, and `--version`. Config files use strict `KEY=VALUE` parsing and are never sourced or executed. Process environment values take precedence over file values. Initial configuration uses Unix mode `0600` and does not contain authentication credentials. Use `palpanel admin reset-password [--username NAME]` for local account recovery; it revokes that account's sessions and development keys.

Static authentication variables from older releases are ignored. After upgrading such an installation, register the first administrator in the browser; existing configuration, SQLite data, Palworld server data, saves and Mods remain in place.

Named provider settings are `PALPANEL_STEAM_API_BASE_URL`, `PALPANEL_STEAM_API_TIMEOUT_SECONDS` (15 seconds), and `PALPANEL_AI_TRANSLATION_TIMEOUT_SECONDS` (90 seconds). Optional `PALPANEL_LOG_LEVEL` accepts `debug`, `info`, `warn`, or `error`.

## Runtime Modes

- `windows_steamcmd`: recommended for production Windows hosts. The backend downloads SteamCMD into `data/tools/steamcmd` when needed and installs the Windows dedicated server with `steamcmd +login anonymous +app_update 2394010 validate +quit`.
- `wine_docker`: keeps the existing Docker + Wine flow for Windows edition server mods and containerized operation. Official Palworld docs warn against Docker Desktop for production save-data IO, so update operations create backups first. Version checks use the existing Wine runner image; build or install once before checking remote version in this mode.

The Wine runner build uses `PALPANEL_DOCKER_RUNNER_BASE_IMAGE` as its base image and keeps the pinned digest by default. If Docker Hub metadata requests time out, the backend retries the same image through comma-separated `PALPANEL_DOCKER_RUNNER_BASE_IMAGE_MIRRORS` prefixes such as `docker.1ms.run` or `registry.cyou`.

Startup arguments are managed separately from `PalWorldSettings.ini` through `GET/PUT /api/server/startup`. PalPanel always adds `-enable-gamedata-api`, `-log`, `-stdout`, and `-FullStdOutLogOutput` (case-insensitively deduplicated) so the bounded game-data proxy and live log collection work on Palworld 1.0 servers.

## Steam Workshop

Workshop search uses a byte-wise XOR-obfuscated key bundled in release binaries by default. `STEAM_WEB_API_KEY` overrides it at runtime. Item details use the public `ISteamRemoteStorage/GetPublishedFileDetails` endpoint without a key. API responses expose only the `bundled` or `environment` source and never the key itself. XOR obfuscation is not encryption; a bundled value remains recoverable from a public binary. `PALPANEL_WORKSHOP_APP_ID` defaults to `1623730` and is used by search metadata and the SteamCMD `workshop_download_item` flow.

Native Windows Workshop access uses SteamCMD's own cached login session. PalPanel persists only the validated Steam account name; it never accepts a Steam password or Steam Guard code and never reads SteamCMD credential configuration. `POST /api/mods/workshop/auth/start` opens a separate local SteamCMD console where the user completes login, and `POST /api/mods/workshop/auth/verify` verifies the cache with `NoPromptForPassword`. Both operations require the admin-only `security:write` permission and a loopback TCP client; forwarded client-IP headers cannot bypass that restriction. Before use, the backend restricts the SteamCMD `config` tree ACL to the current Windows account, SYSTEM, and Administrators without changing the SteamCMD binary or `steamapps` tree. `GET /api/mods/workshop/auth/status` reloads the saved account name and probes an existing cache after a backend restart. Workshop search, detail, translation, and every Workshop download path remain closed until verification succeeds. GitHub, public HTTPS ZIP, local ZIP, UE4SS, and PalDefender flows do not use this gate.

Unified Mod import accepts a Workshop ID/URL, a public GitHub repository or Release URL, a public HTTPS ZIP, or a local ZIP. Every source is inspected before installation. Remote downloads ignore proxy variables and reject credentials, redirects to non-public addresses, oversized payloads, unsafe ZIP paths, links, special files, multiple `Info.json` files, and missing `PackageName`. New Mods install disabled; updates preserve record identity and enabled state, and enabled updates set `pending_restart` without restarting PalServer.

## AI Translation

Admins configure an OpenAI-compatible Base URL, model, and API key from Settings. Base URLs must use HTTPS; HTTP is accepted only for loopback addresses. Embedded URL credentials, redirects, and oversized responses are rejected. The API key is atomically stored in `data/secrets/ai-translation.key` with mode `0600` and is never returned by APIs, application logs, or audit records.

Workshop detail translation always fetches the authoritative description from Steam instead of accepting arbitrary browser text. Cached translations are keyed by Workshop ID, source SHA-256, target language, provider URL, and model, so source or model changes invalidate the cache automatically.

## PalDefender 1.8.1 And GM

The release binary embeds `PalDefender.dll` 1.8.1 with SHA-256 `18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03`. The official v1.8.1 Release publishes `d3d9.dll`, `PalDefender.dll`, and `PalDefender.zip`. Installation obtains the `d3d9.dll` loader from that Release, using the ZIP when present, but always installs the local embedded, hash-pinned `PalDefender.dll` instead of the DLL published in the Release.

The `/gm` frontend uses typed PalPanel DTOs instead of calling PalDefender from the browser. The backend proxy covers version/status, player lists and individual player details, six inventory containers, batches of up to 100 item grants, direct messages, broadcasts, alerts, kick, ban, IP-ban and unban operations. The local catalog contains 2,455 ItemIDs with Chinese names and item-identification WebP artwork for search suggestions.

GM reads require `read`; item grants, messages and punishments require `players:write`. Every GM write also requires an `Idempotency-Key` containing 8-128 safe ASCII characters. Repeating the same key and normalized request within ten minutes replays the first result without contacting PalDefender again; reusing it for another route or payload returns `409 idempotency_key_reused`. This bounded cache belongs to the current backend process, so after a process restart an operator must inspect the audit log and player state before retrying an operation whose outcome was uncertain.

PalDefender installation, configuration and REST Token creation require `security:write`, which is admin-only. GM calls are rejected until the DLLs exist, startup-log loading is verified, and the PalDefender REST API is enabled. The REST API defaults to `http://127.0.0.1:17993`; the backend rejects non-loopback endpoints, inherited proxies and redirects so the bearer token cannot be forwarded to another host. Timeout, unavailable, invalid-response, player-offline and player-not-found failures have separate API codes. Never publish the PalDefender Bearer Token in source, logs, screenshots, proxy configuration or browser code.

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
PALPANEL_MONITOR_RETENTION_DAYS=7
```

`PALPANEL_PERF_SLOW_REQUEST_MS` controls slow-request logging. API responses also include `Server-Timing` and `X-Palpanel-Cache` headers where cached read paths are used.
Monitor samples are pruned hourly after seven days by default. Set `PALPANEL_MONITOR_RETENTION_DAYS=0` to disable monitor history persistence.

`GET /api/ready` is the public readiness probe. It verifies SQLite, the applied schema version, and required data directories; `GET /api/health` remains a process liveness probe.

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

The sidecar is the self-developed Go `sav-cli` in `sav-cli/` and never writes back to `.sav`. Packaged launchers start it automatically with the panel. It reads per-player `Players/*.sav` files and associates `InventoryInfo` container IDs with `OwnerType=player`; a missing or damaged individual player save adds a warning while the rest of the world index continues. Unsupported world containers or schema changes are reported as `parser_incompatible`; the backend keeps the last successful cache when available and does not block other panel features.

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
- Authentication: `GET /api/auth/status`, `POST /api/auth/register`, `POST /api/auth/login`, `POST /api/auth/logout`, `GET /api/auth/me`, `GET/POST/DELETE /api/auth/api-keys`
- Status/logs/jobs: `GET /api/server/status`, `GET /api/server/version`, `POST /api/server/version/check`, `GET /api/server/metrics`, `GET /api/server/game-data`, `GET /api/server/logs?tail=200`, `GET /api/jobs`, `GET /api/jobs/{id}`
- World reset: `GET /api/server/world`, `POST /api/server/world/reset`
- Backups: `POST /api/server/backup`, `GET /api/backups`, `POST /api/backups/{name}/restore`
- Audit: `GET /api/audit-logs`
- Player access: `GET/POST/DELETE /api/players/bans`, `GET/PUT /api/players/whitelist`, `POST /api/players/{id}/kick`
- Palworld config: `GET /api/config/palworld`, `PUT /api/config/palworld`, `GET /api/config/palworld/schema`, `POST /api/config/palworld/validate`
- Mods: `GET /api/mods`, `POST /api/mods/import/inspect`, `POST /api/mods/import/inspect/{id}/select`, `POST /api/mods/import`, `GET /api/mods/workshop/auth/status`, `POST /api/mods/workshop/auth/start`, `POST /api/mods/workshop/auth/verify`, `GET /api/mods/workshop/search`, `GET /api/mods/workshop/{id}`, `POST /api/mods/workshop/{id}/translate`, plus compatible `/api/mods/upload` and `/api/mods/workshop` endpoints
- AI translation: `GET/PUT /api/ai/translation/config`, `POST /api/ai/translation/test`
- PalDefender: `GET /api/security/paldefender/releases`, `GET /api/security/paldefender/status`, `POST /api/security/paldefender/install`, `POST /api/security/paldefender/update`, `POST /api/security/paldefender/rollback`, `GET/PUT /api/security/paldefender/config`, `POST /api/security/paldefender/apply-preset`, `POST /api/security/paldefender/rest-token`, `POST /api/security/paldefender/reload-config`
- PalDefender GM: `GET /api/security/paldefender/gm/status`, `GET /api/security/paldefender/gm/players`, `GET /api/security/paldefender/gm/players/{id}`, `GET /api/security/paldefender/gm/items`, `GET /api/security/paldefender/gm/players/{id}/inventory`, item/message/kick/ban/unban writes under `/api/security/paldefender/gm/players/{id}`, and `POST /api/security/paldefender/gm/broadcast`

## Paths

- Palworld config: `data/server/Pal/Saved/Config/WindowsServer/PalWorldSettings.ini`
- Official server mods: `data/server/Mods/Workshop/<mod>/Info.json`
- Mod settings: `data/server/Mods/PalModSettings.ini`
- PalDefender binaries: `data/server/Pal/Binaries/Win64/PalDefender.dll` and `data/server/Pal/Binaries/Win64/d3d9.dll`
- PalDefender config after first server start: `data/server/Pal/Binaries/Win64/PalDefender/Config.json`
- Persistent PalServer log: `data/logs/palserver.log` (plus `.1` through `.5`)
- AI translation API key: `data/secrets/ai-translation.key`

Server files, Wine prefix, tools, mods, saves, backups, logs, PalDefender files, and SQLite data are kept outside the backend source tree.
