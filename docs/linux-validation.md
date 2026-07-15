# Linux validation

PalPanel Linux amd64 uses a native panel and `sav-cli` plus a Docker/Wine
Palworld runner. Validation must distinguish package/process checks from a live
game-server run.

## Automated release checks

Run on a clean Linux amd64 checkout with Go, Node.js 22, a C/C++ toolchain,
Docker, shellcheck, curl, tar, and sha256sum:

```bash
(cd backend && go test -p=1 ./...)
(cd sav-cli && CGO_ENABLED=1 go test -p=1 ./...)
(cd frontend && npm ci && npm run check)

scripts/package.sh --version v1.1.0 --targets linux-amd64 --skip-tests --clean
scripts/verify-release-contents.sh \
  dist/packages/palpanel_v1.1.0_linux_amd64.tar.gz \
  dist/packages/palpanel-sav-cli_v1.1.0_source.tar.gz \
  dist/packages/palpanel_v1.1.0_source.tar.gz
scripts/smoke-linux.sh dist/packages/palpanel_v1.1.0_linux_amd64.tar.gz
sudo scripts/test-install.sh dist/packages/palpanel_v1.1.0_linux_amd64.tar.gz
sudo scripts/test-bootstrap-install.sh dist/packages/palpanel_v1.1.0_linux_amd64.tar.gz
sudo scripts/test-upgrade.sh dist/packages/palpanel_v1.1.0_linux_amd64.tar.gz v1.0.2
```

These checks verify embedded frontend delivery, authentication, the native save
indexer, checksums, portable supervision, systemd installation, upgrade data
preservation, uninstall, and purge behavior. They do not prove a real Palworld
process or player session.

## Docker/Wine live checks

1. Install with `--docker`, open the panel, and select `wine_docker`.
2. Install the Palworld server and initialize `PalWorldSettings.ini`.
3. Confirm the generated configuration supplied a non-empty
   `PALWORLD_ADMIN_PASSWORD`; REST and RCON should be enabled with the configured
   ports.
4. Start, restart, and stop the server. Confirm no container remains after the
   managed stop and inspect the bounded Palworld log.
5. Probe Palworld REST and RCON through `127.0.0.1`. Docker must publish REST,
   RCON, and PalDefender REST only on loopback.
6. Put both `STEAM_USERNAME` and `STEAM_PASSWORD` in the mode-0600
   `palpanel.env`, restart PalPanel, and download one disposable Workshop item.
   Redacted logs must not contain the password.
7. Scan Workshop, Pak, and UE4SS Mod layouts. Exercise only non-destructive
   actions unless the target is disposable.
8. Install PalDefender, which installs pinned UE4SS first. Start the server and
   retain UE4SS and PalDefender load evidence from game-local or bounded logs.
9. Create the PalDefender REST token. Test `/gm` item grants only with an
   explicitly disposable online player.

The file `0509334144974424F0B1FA94541F4CD4.zip` is not valid evidence for step
9: it contains `PalLocalSaveData` only, has a zero `PlayerUId`, and lacks the
dedicated-server `Level.sav`, `LevelMeta.sav`, and `Players/*.sav` files.
