# PalPanel Linux Package

This package contains PalPanel with its web UI embedded in the backend, the native cgo
`sav-cli` sidecar, the Wine runner resources, systemd units, and
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
and health-checks `sav-cli` before the backend; stopping or restarting the
panel manages both processes together.

## systemd installation

```bash
sudo ./palpanelctl install
```

Use `--listen HOST:PORT` to set the panel listener non-interactively. The
embedded frontend and API share the same origin. Sign in with the account
created during first-run registration.

The installer enables and starts both `palpanel-sav-cli.service` and
`palpanel.service`. The sidecar restarts automatically, reads saves without
writing them and tolerates a missing or damaged individual player save while
preserving the rest of the index.

Programs are installed under `/opt/palpanel/<version>`, with
`/opt/palpanel/current` selecting the active version. Configuration is stored
in `/etc/palpanel`, and state is stored in `/var/lib/palpanel`. Reinstalling a
new version preserves both locations.

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

## Security defaults

- Authentication is enabled.
- The web server binds to `127.0.0.1:8080`.
- Frontend API traffic uses same-origin `/api`.
- Steam Workshop search uses the bundled obfuscated key by default;
  `STEAM_WEB_API_KEY` in `palpanel.env` can override it.
- The env file is parsed as data and is never executed by a shell.
- OpenAI-compatible translation is configured in Settings with a Base URL,
  model and API key; never place a real key in `palpanel.env` or support logs.
- PalDefender REST must remain on loopback or a controlled trusted network,
  and its Bearer Token must not be exposed.

The backend embeds `PalDefender.dll` 1.8.1 with SHA-256
`18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03`.
Installation downloads the `d3d9.dll` loader from the official PalDefender
Release, but always uses the local embedded, hash-pinned DLL instead of the
Release DLL. The `/gm` page then
provides typed player/inventory access, 2,455-item icon search, batch grants,
messages, kick and ban controls subject to PalPanel permissions.

The native `sav-cli` includes the GPL-licensed vendored `gooz` decompressor and
supports PlM1/Oodle save containers. Its corresponding source is distributed
as a separate release asset. Per-player saves associate inventory containers
with their owners; unavailable player files produce warnings rather than
failing the whole index.

PalPanel is licensed under GPL-3.0-or-later; see `LICENSE`. Third-party
components retain their own licenses as listed in `THIRD_PARTY_LICENSES.txt`.
