# PalPanel Windows Package

This ZIP contains the PalPanel backend with its embedded web UI, native `sav-cli`, and the
double-click Windows Launcher.

## Start

1. Extract the whole ZIP to a writable directory.
2. Double-click `PalPanel.exe`.
3. Confirm the first-run data location prompt.
4. Register the first administrator in the browser. The frontend and API use
   the same address, so no backend URL is required.

## Runtime locations

The release ZIP uses a portable sidecar layout. Unless `PALPANEL_RUNTIME_ROOT` or
`--runtime-root` is deliberately supplied, configuration lives in `config\` and all
managed runtime data lives in `data\` next to `PalPanel.exe`. This includes the database,
backups, logs, game installation, saves, Mods, UE4SS, and PalDefender files managed by the
panel. Do not point a release ZIP at a source-tree runtime by accident.

Source-tree development is different: a checkout uses
`<repository>\dev-runtime\windows\` as its managed runtime root. Development scripts
never need to write release data beside a packaged executable, and release maintenance
commands never delete source-tree files.

Do not run the Launcher from inside the ZIP. The Launcher prevents duplicate
instances, starts and health-checks `sav-cli` before the backend, supervises
both child processes, and stops them when it exits. The read-only sidecar maps
player-save inventory containers to their owners; one missing or damaged
player save produces a warning instead of failing the complete world index.

`windows_steamcmd` is the recommended runtime on a native Windows host. The
game server itself is installed from the web setup page and is not bundled in
this ZIP. Current Palworld releases run `PalServer-Win64-Shipping-Cmd.exe`;
PalPanel also retains compatibility with the older `PalServer-Win64-Test-Cmd.exe`
name. Because current builds may not emit stdout, PalPanel records its own bounded
lifecycle log and uses component-native logs for dependency load status.

## Steam Workshop login

Workshop search, details, translation, and downloads are shown only after the
local SteamCMD login cache is verified. The PalPanel dialog accepts only the
Steam account name and opens a separate local SteamCMD console. Enter the Steam
password and any Steam Guard challenge only in that console; PalPanel never
accepts them through the browser/API or stores them. Starting and verifying this
flow requires an administrator using the panel from the same Windows host.

GitHub, public HTTPS ZIP, local ZIP, and local Mod actions do not require this
Steam login. PalDefender is also outside the Workshop gate.

The backend embeds `PalDefender.dll` 1.8.1 with SHA-256
`18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03`.
Installation always checks and installs pinned UE4SS `v3.0.1` first. It then
downloads the `d3d9.dll` loader from the official PalDefender Release, but
always uses the local embedded, hash-pinned DLL instead of the Release DLL.
Load status is read from native `UE4SS.log` and `PalDefender\Logs\*.log`
evidence. After an admin installs and configures PalDefender, `/gm` provides player/inventory views,
2,455-item icon search, batch item grants, messages, kick and ban controls.

Keep PalDefender REST on loopback or a controlled trusted network and never
expose its Bearer Token. OpenAI-compatible translation accepts a Base URL,
model and API key in Settings; real keys must not be placed in package config,
logs, screenshots or support reports.

## Upgrade, uninstall, and recovery

Extract the candidate ZIP completely before upgrading. From the existing extracted release
directory, run its included maintenance script and give it the candidate ZIP path:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\maintenance\upgrade-windows.ps1 `
  -InstallRoot $PWD `
  -Archive C:\Downloads\palpanel_vNEXT_windows_amd64.zip
```

Use `-ExpectedSHA256 <published-64-character-sha256>` when a release checksum is available.
The upgrade stops only processes whose executable path belongs to this installation, stages
and verifies the ZIP, snapshots the previous program payload and managed database, then
starts the upgraded Launcher without a browser to validate the database migration. If that
validation fails, the previous payload and database snapshot are restored; the retained
transaction directory is `.palpanel-maintenance\tx\...`. Configuration, game
files, saves, Mods, UE4SS, PalDefender, and backups are never overlaid by the upgrade.

Normal uninstall removes only the packaged program payload and stops tracked PalPanel/game
processes. It preserves `config\`, `data\`, and recovery snapshots:

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\maintenance\uninstall-windows.ps1 `
  -InstallRoot $PWD
```

Complete cleanup is intentionally a separate, explicit action. Run it before normal
uninstall (or from a separately retained copy of the maintenance script):

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File .\maintenance\uninstall-windows.ps1 `
  -InstallRoot $PWD `
  -PurgeData `
  -ConfirmPurge PURGE_PALPANEL_MANAGED_DATA
```

That command removes only this verified installation's `config\`, `data\`, and
`.palpanel-maintenance\` children. It does not remove the installation root itself, the
ZIP archive, source directories, or configuration/data paths redirected outside the managed
sidecar.

After a damaged configuration prevents startup, first list retained snapshots:

```powershell
.\maintenance\recover-windows-config.ps1 -InstallRoot $PWD -ListBackups
```

Restore the latest snapshot only with confirmation, or generate a new secure configuration
while retaining the unreadable original:

```powershell
.\maintenance\recover-windows-config.ps1 -InstallRoot $PWD `
  -RestoreLatest -ConfirmRecovery RESTORE_PALPANEL_CONFIG

.\maintenance\recover-windows-config.ps1 -InstallRoot $PWD `
  -Recreate -ConfirmRecovery RECREATE_PALPANEL_CONFIG
```

These scripts validate archive paths, reject reparse points, and do not claim a real game
server was started. A package upgrade validates the Launcher and database migration only;
use the documented Windows live validation separately for SteamCMD and Palworld evidence.
If `palpanel.env` deliberately redirects a runtime or database path outside the release
`data\` sidecar, the upgrade refuses automatic startup validation; use
`-SkipStartupValidation` and validate/migrate that custom location manually.

## Signature warning

The executables are not Authenticode-signed. Windows SmartScreen may show an
unknown publisher warning. Verify the ZIP with the release `SHA256SUMS` before
running it.

## License

PalPanel is licensed under GPL-3.0-or-later; see `LICENSE`. Third-party
components retain their own licenses as listed in `THIRD_PARTY_LICENSES.txt`.
