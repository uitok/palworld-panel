# PalPanel v1.1.0

PalPanel v1.1.0 brings the native Windows implementation and the Linux
Docker/Wine package to the same management surface. This is a feature release
and includes both amd64 packages.

## Highlights

- Native Windows SteamCMD installation, server lifecycle, isolated runtime
  paths, portable Launcher, and transactional upgrade/recovery tooling.
- Cross-platform local Mod scanning and actions for Workshop, Pak/LogicMods,
  and UE4SS layouts, including missing, duplicate, disabled, and manual files.
- Hardened Workshop flows: native Windows uses a local interactive SteamCMD
  cache; Linux Docker/Wine uses a complete mode-0600 environment credential
  pair without placing the password in Docker command arguments.
- PalDefender installation now checks and installs hash-pinned UE4SS `v3.0.1`
  first, preserves user configuration, supports rollback, and reports load
  evidence.
- `/gm` adds typed player and inventory operations, Chinese item names and
  icons, batch grants, messages, punishments, permissions, audit records, and
  idempotency protection.
- New production configurations generate a random Palworld administrator
  password. Palworld REST and RCON use the configured ports; Linux maps REST,
  RCON, and PalDefender REST only to loopback.
- OpenAI-compatible translation configuration has stricter proxy, timeout,
  cancellation, header, and secret-handling behavior.

## Linux notes

Install or upgrade with:

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | sudo bash
```

For the Docker/Wine runtime, install with `--docker`. Private Workshop downloads
require both `STEAM_USERNAME` and `STEAM_PASSWORD` in
`/etc/palpanel/palpanel.env`, followed by a PalPanel restart. Upgrades preserve
`/etc/palpanel` and `/var/lib/palpanel`; normal uninstall preserves them too.

## Windows notes

Extract the ZIP completely and run `PalPanel.exe`. The package is portable and
unsigned. Verify it with `SHA256SUMS`; SmartScreen may show an unknown-publisher
warning. Existing runtime data is not bundled in or overlaid by the ZIP.

## Validation boundary

Backend, save parser, frontend, package, install, upgrade, and smoke suites cover
both platforms. A real Windows server lifecycle plus REST, RCON, UE4SS,
PalDefender, and Workshop download were exercised. Live Linux game and GM item
grant evidence still requires a disposable server and online player. The supplied
`0509334144974424F0B1FA94541F4CD4.zip` is a local save rather than a dedicated
server player save and therefore cannot prove item delivery.

## Release assets

- `palpanel_v1.1.0_linux_amd64.tar.gz`
- `palpanel_v1.1.0_windows_amd64.zip` (unsigned)
- `palpanel_v1.1.0_source.tar.gz`
- `palpanel-sav-cli_v1.1.0_source.tar.gz`
- Linux SPDX SBOM, third-party inventory, and `SHA256SUMS`
