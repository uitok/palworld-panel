# PalPanel Linux Package

This package contains PalPanel with its web UI embedded in the backend, the native cgo
`sav-cli` save sidecar, the self-contained `palcalc-bridge` breeding solver,
the Wine runner resources, systemd units, and
`palpanelctl`.

## Portable mode

```bash
./palpanelctl init
./palpanelctl start
./palpanelctl status
./palpanelctl logs -f
./palpanelctl stop
```

The first `init` creates `config/palpanel.env` with mode `0600`. Open the panel
URL to register the first administrator. Portable data, PID files, and bounded
logs remain inside the extracted package directory. `palpanelctl start` starts
and health-checks `sav-cli` and `palcalc-bridge` before the backend; stopping
or restarting the panel manages all three processes together.

## systemd installation

```bash
sudo ./palpanelctl install
```

Use `--listen HOST:PORT` to set the panel listener non-interactively. The
embedded frontend and API share the same origin. Sign in with the account
created during first-run registration.

The installer enables and starts `palpanel-sav-cli.service`,
`palpanel-palcalc.service`, and `palpanel.service`. The sidecars restart
automatically; the save parser reads saves without
writing them and tolerates a missing or damaged individual player save while
preserving the rest of the index.

Programs are installed under `/opt/palpanel/<version>`, with
`/opt/palpanel/current` selecting the active version. Configuration is stored
in `/etc/palpanel`, and state is stored in `/var/lib/palpanel`. Reinstalling a
new version preserves both locations.

The GitHub bootstrap installer also supports migration from an older
containerized PalPanel when its data directory is mounted on the host:

```bash
curl -fsSL https://raw.githubusercontent.com/uitok/palworld-panel/main/install.sh | \
  sudo bash -s -- --migrate-container palpanel
```

The named legacy container is stopped only after its configuration and data
mount are validated. It is retained for rollback, and an installation or
health-check failure automatically starts it again.

Default uninstall preserves configuration and data:

```bash
sudo /opt/palpanel/current/palpanelctl uninstall
```

Use `uninstall --purge` only when configuration and all PalPanel-managed data
should also be removed.

Wine Docker mode requires explicit Docker socket access:

```bash
sudo ./palpanelctl install --docker
```

Membership in the Docker group is effectively root-equivalent. Do not enable
it when using the Windows SteamCMD runtime without Docker.

## Docker/Wine server and Workshop configuration

Linux keeps the Palworld Windows server in the host data directory and runs it
through the bundled Docker/Wine runner. The game UDP ports are published as
configured. Palworld REST, RCON, and PalDefender REST are published only on the
host loopback interface. A custom `PALPANEL_REST_PORT` maps to the same internal
Palworld REST port; `PALPANEL_RCON_PORT` defaults to `25575`.
`PALPANEL_RCON_HOST` defaults to `127.0.0.1` and can be changed when a legacy
container network requires an explicit game-container or host-gateway address.

Runtime Debug logging can be toggled from the Monitor page. It writes bounded,
rotated diagnostics to `/var/lib/palpanel/logs/palpanel-debug.log` without
recording credentials, authorization headers, or request bodies.

Windows and Linux/Docker use the same Workshop login form in Mod Management.
Build the Linux runner once by installing or updating the server, then enter the
Steam account, password, and optional Steam Guard code in the panel. PalPanel
stores the password in the private `data/secrets/steam-workshop-credentials.json`
file (mode 0600 on Linux) and keeps Guard codes transient. Linux passes a private
temporary runscript into the Docker/Wine runner read-only, so credentials never
enter Docker process arguments or support logs. Downloads reuse the approved
SteamCMD cache without embedding the saved password and request reauthorization
when the machine grant expires. `palpanelctl steam-login` remains a recovery
fallback for administrators who need to repair the cache manually.

Server installation/update and community-server discovery proxies can be managed
independently from System Settings > Network & proxy. The managed configuration is
stored with the rest of PalPanel state under `data/secrets/network-proxy.json` in
portable mode or `/var/lib/palpanel/secrets/network-proxy.json` under systemd.
Docker/Wine receives proxy values through its environment; credentials are not
embedded in Docker arguments or job messages.

## Security defaults

- Authentication is enabled.
- The web server binds to `127.0.0.1:8080`.
- First initialization generates a random Palworld administrator password used
  by the official REST API and loopback-only Docker RCON mapping.
- Frontend API traffic uses same-origin `/api`.
- Steam Workshop search uses the bundled obfuscated key by default;
  `STEAM_WEB_API_KEY` in `palpanel.env` can override it.
- The env file is parsed as data and is never executed by a shell.
- OpenAI-compatible translation is configured in Settings with a Base URL,
  model and API key; never place a real key in `palpanel.env` or support logs.
- PalDefender REST must remain on loopback or a controlled trusted network,
  and its Bearer Token must not be exposed.

Installation first checks and installs hash-pinned UE4SS `v3.0.1`, then queries
the official PalDefender GitHub latest stable Release and downloads both
`d3d9.dll` and `PalDefender.dll` with their published SHA-256 digests. A
digest-verified ZIP is used only when the direct assets are absent. The current
official release is v1.8.3, but PalPanel follows GitHub Latest instead of
embedding or pinning that version. Docker/Wine gives
both native loaders precedence and records startup-log evidence. The `/gm` page then
provides typed player/inventory access, 2,455-item icon search, batch grants,
messages, kick and ban controls subject to PalPanel permissions.

The Real-time Map page polls online player coordinates every two seconds when
PalDefender REST is available. Offline players and other entities use the most
recent read-only save index, and the SVG background is a coordinate schematic
rather than extracted game terrain.

The native `sav-cli` includes the GPL-licensed vendored `gooz` decompressor and
supports PlM1/Oodle save containers. Its corresponding source is distributed
as a separate release asset. Per-player saves associate inventory containers
with their owners; unavailable player files produce warnings rather than
failing the whole index.

PalPanel is licensed under GPL-3.0-or-later; see `LICENSE`. Third-party
components retain their own licenses as listed in `THIRD_PARTY_LICENSES.txt`.
