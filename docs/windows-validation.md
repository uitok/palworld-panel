# Windows validation

PalPanel ships an unsigned native Windows ZIP containing the Launcher, backend, embedded
web UI, and `sav-cli`. The automated Windows build currently runs on the GitHub-hosted
Windows Server 2025 image. Windows 10/11 x64 and current Windows Server x64 releases are
the intended targets. The Launcher can run without a browser (`--no-browser`) on a server
host, but an interactive desktop is needed for its message boxes unless `--no-prompt` is
also used.

## Runtime layout

Source-tree development and validation use one managed root. The default is derived from
the script location, not the current PowerShell working directory:

```text
<repository>\dev-runtime\windows\
  config\
  data\database\
  data\backups\
  data\logs\
  data\saves\
  data\tasks\
  steamcmd\
  palworld\
  mods\downloads\
  mods\cache\
  mods\staging\
  mods\fixtures\
  ue4ss\
  paldefender\
  package\
  artifacts\
  temp\
  e2e\
```

The scripts reject the repository root, volume roots, source directories, paths outside
`dev-runtime\windows`, and managed paths containing a reparse point. Relative
`-RuntimeRoot` values are resolved against the repository. Relative `-TestRoot` values
are resolved against the selected runtime root. No validation staging uses the system
temporary directory.

The release ZIP remains portable: when no development runtime override is supplied, its
data stays beside the extracted application. Always extract the complete ZIP to a writable
directory before starting it. Do not run it from the ZIP viewer. The binaries are not
Authenticode-signed, so SmartScreen can show an unknown-publisher warning; verify the
published SHA-256 before choosing to run it.

## Build and offline validation

PowerShell 7 is recommended; Windows PowerShell 5.1 is also supported by the scripts.
Building requires the Go version in `backend/go.mod`, Node.js 22, npm, and MinGW-w64 GCC
for the CGO `sav-cli` build.

```powershell
$runtime = Join-Path $PSScriptRoot "..\dev-runtime\windows"
.\scripts\package.ps1 `
  -Version v0.0.0-windows-dev `
  -RuntimeRoot $runtime `
  -Clean `
  -MingwGcc C:\msys64\mingw64\bin\gcc.exe

.\scripts\e2e-windows.ps1 `
  -RuntimeRoot $runtime `
  -SkipGameDownload `
  -SkipLiveServer `
  -SkipPalDefender `
  -TimeoutMinutes 30
```

The offline command is the ordinary pull-request CI mode. It runs the path-isolation
contract, reuses or builds the Windows package, validates archive paths and checksums,
and runs the Launcher/process smoke test with an isolated runtime root. It does not claim
that SteamCMD, Palworld, UE4SS, or PalDefender were tested live.

`package.ps1 -RuntimeRoot` writes the directory and ZIP below `runtime\package`. Omitting
that option preserves release automation compatibility and writes to `dist\packages`,
while still keeping temporary files below the repository runtime root.

## Full live validation

The live run downloads a large Palworld Dedicated Server installation. Use a development
machine or self-hosted Windows runner with enough disk space and time:

```powershell
.\scripts\e2e-windows.ps1 `
  -RuntimeRoot ".\dev-runtime\windows" `
  -KeepArtifacts `
  -TimeoutMinutes 240
```

The script starts the packaged backend with authentication disabled only for its loopback
test process, selects `windows_steamcmd`, and calls PalPanel's own lifecycle APIs. It polls
the persisted job state and records progress. SteamCMD is retried by the backend, an
existing installation is reused, and `app_update 2394010 validate` checks or updates an
existing game rather than deleting it. A successful live run verifies `PalServer.exe`, the
current `PalServer-Win64-Shipping-Cmd.exe`, and the Steam app manifest. Runtime selection
prefers Shipping-Cmd and retains a fallback to the older `PalServer-Win64-Test-Cmd.exe`
name for existing installations. The run installs UE4SS before PalDefender, then starts,
restarts, and stops Palworld. Process identity, owned UDP endpoints, and a non-empty
PalPanel lifecycle log are required as live-start evidence; current Palworld builds do not
reliably emit server stdout. Dependency loading is checked separately in native
`UE4SS.log` and `PalDefender\Logs\*.log` files.

After an extracted Windows package and a complete game installation are present under the
runtime root, run the focused control-plane check as well:

```powershell
.\scripts\windows-live-game-check.ps1 `
  -RuntimeRoot ".\dev-runtime\windows"
```

This focused check does not download or install anything. It starts the packaged backend
and real dedicated server, then requires official REST `info/players`, RCON
`Info/ShowPlayers/Save`, an observed `.sav` update, a new process after restart, direct
official REST shutdown, and PalPanel safe-stop completion without the managed-force-stop
fallback. It also verifies the safe-stop audit record and writes compact evidence below
`dev-runtime/windows/artifacts/live-game-*`. `-PackageDir` can select a specific extracted
package; otherwise the newest valid extracted Windows package under the runtime root is
used.

On success, the per-run test and temp directories can be removed; the package, SteamCMD,
game installation, and reports remain cached. `-KeepArtifacts` also keeps the isolated
test root. On failure or timeout, the script terminates its tracked backend process tree
and retains logs, configuration, database, test root, and staging data for diagnosis.
SteamCMD and the game are never removed by E2E cleanup.

## Parameters

| Parameter | Meaning |
| --- | --- |
| `-RuntimeRoot` | Managed root; defaults to `<repository>\dev-runtime\windows`. |
| `-TestRoot` | Reusable marked test root below `RuntimeRoot`; defaults to a unique run directory. |
| `-KeepArtifacts` | Retain the successful test root and verification extraction. Reports are always retained. |
| `-SkipGameDownload` | Do not submit the SteamCMD/Palworld bootstrap job. |
| `-SkipLiveServer` | Do not start, restart, or stop a real Palworld process. |
| `-SkipPalDefender` | Do not download or install UE4SS and PalDefender. |
| `-Proxy` | Set `HTTP_PROXY`, `HTTPS_PROXY`, and `ALL_PROXY` for child processes. HTTP(S) and SOCKS5 URLs without embedded credentials are accepted. |
| `-TranslationBaseURL` | Configure and test an OpenAI-compatible endpoint through the backend API. |
| `-TranslationAPIKeyEnv` | Name of the environment variable containing the translation key; the value is redacted and never placed in command arguments. |
| `-TimeoutMinutes` | Overall deadline, including builds and live downloads. |

For the local SOCKS proxy documented for this repository:

```powershell
.\scripts\e2e-windows.ps1 `
  -Proxy "socks5://127.0.0.1:10808" `
  -TimeoutMinutes 240
```

To test a local OpenAI-compatible mock without writing a key to source or command logs:

```powershell
$env:PALPANEL_E2E_AI_KEY = "test-only-value"
.\scripts\e2e-windows.ps1 `
  -SkipGameDownload -SkipLiveServer -SkipPalDefender `
  -TranslationBaseURL "http://127.0.0.1:18080/v1" `
  -TranslationAPIKeyEnv PALPANEL_E2E_AI_KEY
```

The mock service must already be listening. Supplying no translation URL leaves this E2E
stage explicitly skipped; translation unit and API integration tests still run as part of
the normal backend test suite. A real provider is used only when both options are given.

## Evidence and test classes

Each run writes `dev-runtime\windows\artifacts\e2e-<timestamp>-<pid>-<id>` with:

- `events.jsonl`: structured stage and download-job progress events.
- `summary.json`: outcome, flags, classifications, durations, and any redacted failure.
- `paths.json`: the absolute repository, runtime, SteamCMD, game, and artifact paths.
- `commands\*.stdout.log` and `*.stderr.log`: checked external-command output.
- live JSON evidence for version, process, UDP endpoint, restart, and PalDefender status.
- `dependency-load-evidence.json`: native `ue4ss_log` and `paldefender_log` observations,
  including the exact component-log paths used for the decision.

Classifications are `unit`, `integration`, `windows-smoke`, `windows-e2e`, `windows-live`,
and `destructive-test-root-only`. Ordinary CI covers the smoke/E2E integration path and
uploads reports even when the job fails. `windows-live` requires an explicitly enabled
developer or self-hosted run.

## Operations notes

- Game downloads live at `runtime\palworld`; SteamCMD lives at `runtime\steamcmd`.
- Panel logs and the Palworld process log live at `runtime\data\logs`.
- Back up `runtime\data`, game saves, and user Mods before an upgrade.
- An overlay ZIP upgrade must preserve runtime data. Do not copy generated runtime files
  into a new release package.
- Normal removal should delete only application binaries. Keep game files, saves, backups,
  configuration, database, and user Mods unless a separately confirmed full cleanup is
  intended.
- The packaged `maintenance\upgrade-windows.ps1` performs a sidecar overlay upgrade from a
  fully extracted ZIP. It verifies ZIP paths and checksums, stops only processes whose
  executable paths are inside the selected release install, snapshots package payload and
  managed database files, verifies the new launcher startup, and restores the previous
  payload/database on failure. It does not overlay `config\`, `data\`, game files, saves,
  Mods, UE4SS, PalDefender, or backups. A configured runtime or database path outside the
  managed `data\` directory is not touched by this script; use `-SkipStartupValidation`
  and handle validation or migration of that custom location explicitly.
- The packaged `maintenance\uninstall-windows.ps1` defaults to removing only known package
  payload paths. `-PurgeData -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA` is required before
  it removes the verified install's `config\`, `data\`, and `.palpanel-maintenance\`
  children. It never deletes the installation root or paths redirected outside the sidecar.
- `maintenance\recover-windows-config.ps1` lists retained configuration snapshots, restores
  one only after an explicit confirmation token, or regenerates a secure configuration while
  retaining the damaged original. These offline maintenance flows do not assert that a real
  Palworld server, SteamCMD install, or player session was exercised.
- The current Mod sources are Workshop, GitHub/HTTPS packages, and local ZIP import. Local
  Mod detection, translation behavior, GM permissions, and rollback paths are covered by
  backend/frontend automated tests; they are not reported as real-player validation by the
  offline E2E command.
- PalDefender installation automatically checks and installs its pinned UE4SS dependency.
  `/gm` still requires a running server, confirmed dependency load, and an authorized test
  player before destructive player operations can be tested manually.

## Steam Workshop login gate

Workshop search, details, translation, and every Workshop download path stay hidden or
blocked until PalPanel verifies a reusable local SteamCMD login cache. The login dialog
accepts only the Steam account name. PalPanel then opens a separate SteamCMD console on the
same Windows desktop; enter the Steam password and any Steam Guard challenge only in that
console. Neither value is accepted by the browser/API, placed in environment variables, or
persisted by PalPanel. PalPanel stores only the validated account name, never reads
SteamCMD's credential configuration, and restricts the SteamCMD `config` directory ACL to
the current Windows account, SYSTEM, and Administrators.

Starting or verifying the login requires the admin-only `security:write` permission and a
real loopback TCP client; forwarded client-IP headers do not satisfy this restriction. A
backend restart reloads the selected account name and probes the existing cache with
non-interactive password prompting disabled. If verification fails or expires, return to
the login dialog and complete the local SteamCMD flow again.

This gate applies only to Steam Workshop. GitHub, public HTTPS ZIP, local ZIP, and local Mod
scan/action flows remain available without a Steam login. PalDefender is also an explicit
exception; installing it always checks and installs the pinned UE4SS dependency first.

Do not copy or publish `runtime\steamcmd\config`, `runtime\steamcmd\userdata`, SteamCMD
logs, or Workshop staging directories. The repository's development runtime is ignored by
Git, but evidence and support bundles still need a manual redaction review before sharing.

## GM protocol and live-player validation

The normal Go and frontend suites use a loopback mock PalDefender REST server. They verify
the player list/detail/inventory contracts, single and batch item grants, messages,
broadcasts, kick/ban/unban, backend permissions, strict JSON and identifier validation,
timeout and malformed-response mapping, offline/disconnected players, idempotent retries,
write audits, and confirmation UI. These are protocol integration tests, not evidence that
a real player joined a real server:

```powershell
Push-Location backend
$env:GOCACHE = Join-Path (Resolve-Path "..\dev-runtime\windows") "temp\go-build-cache"
go test ./internal/paldefender -run 'Test(REST|GMStatus)' -count=1
go test ./internal/api -run 'Test(PalDefenderGM|NewContractRoutes|OpenAPI)' -count=1
Pop-Location

Push-Location frontend
npm.cmd test -- --run src/api/paldefenderGM.test.ts src/pages/PalDefenderGM.test.tsx
Pop-Location
```

Live GM validation requires an isolated test server and a client account explicitly marked
as disposable test data. Confirm `/api/security/paldefender/gm/status` reports `ready`, then
record player list, detail and inventory responses. Against that test account only, verify
a single-item grant, a multi-item grant, direct message, broadcast, kick, ban and unban;
confirm every write appears in `/api/audit-logs`. Repeat one request with the same
`Idempotency-Key` and verify `Idempotency-Replayed: true` without a second game-side effect.
Finally disconnect the test client before and during a write and retain the returned
offline/not-found evidence. Do not run kick, ban, IP-ban or item grants against production
accounts, production servers or unconsenting players. Until those steps are performed and
their artifacts retained, report GM as `mock/protocol verified; live player pending`.

Common failures are recorded in `summary.json`. A missing MinGW compiler fails the package
stage; a blocked Steam CDN fails the game-install job with its retained backend error; a
port conflict fails the live start; and a missing translation environment variable fails
before any API request. Rerun with the same runtime root to reuse valid downloads, or pass a
new `-TestRoot` to isolate smoke configuration while keeping the shared game cache.
