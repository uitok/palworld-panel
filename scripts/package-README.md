# PalPanel Linux Package

This package contains the PalPanel backend and frontend, the native cgo
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

The first `init` creates `config/palpanel.env` with a cryptographically random
admin token and mode `0600`. The token is printed only when the file is first
created; `./palpanelctl token` reads it again. Portable data, PID files, and
bounded logs remain inside the extracted package directory.

## systemd installation

```bash
sudo ./palpanelctl install
```

Use `--listen HOST:PORT` to set the panel listener non-interactively. The
frontend and API share the same origin, so the browser only asks for the admin
token.

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
- Steam Workshop search remains disabled until `STEAM_WEB_API_KEY` is added to
  `palpanel.env`.
- The env file is parsed as data and is never executed by a shell.

The native `sav-cli` includes the GPL-licensed vendored `gooz` decompressor and
supports PlM1/Oodle save containers. Its corresponding source is distributed
as a separate release asset.
