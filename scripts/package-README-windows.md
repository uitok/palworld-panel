# PalPanel Windows Package

This ZIP contains the PalPanel backend with its embedded web UI, native `sav-cli`, and the
double-click Windows Launcher.

## Start

1. Extract the whole ZIP to a writable directory.
2. Double-click `PalPanel.exe`.
3. Confirm the first-run data location prompt.
4. Register the first administrator in the browser. The frontend and API use
   the same address, so no backend URL is required.

Configuration, server data, backups, and logs stay next to `PalPanel.exe`.
Do not run the Launcher from inside the ZIP. The Launcher prevents duplicate
instances, starts and health-checks `sav-cli` before the backend, supervises
both child processes, and stops them when it exits. The read-only sidecar maps
player-save inventory containers to their owners; one missing or damaged
player save produces a warning instead of failing the complete world index.

`windows_steamcmd` is the recommended runtime on a native Windows host. The
game server itself is installed from the web setup page and is not bundled in
this ZIP.

The backend embeds `PalDefender.dll` 1.8.1 with SHA-256
`18b9f63eea2dd407f29b77a262f9d33b1dcd4b744328892c13d5822701418d03`.
Installation downloads the `d3d9.dll` loader from the official PalDefender
Release, but always uses the local embedded, hash-pinned DLL instead of the
Release DLL. After an admin
installs and configures PalDefender, `/gm` provides player/inventory views,
2,455-item icon search, batch item grants, messages, kick and ban controls.

Keep PalDefender REST on loopback or a controlled trusted network and never
expose its Bearer Token. OpenAI-compatible translation accepts a Base URL,
model and API key in Settings; real keys must not be placed in package config,
logs, screenshots or support reports.

## Signature warning

The executables are not Authenticode-signed. Windows SmartScreen may show an
unknown publisher warning. Verify the ZIP with the release `SHA256SUMS` before
running it.

## License

PalPanel is licensed under GPL-3.0-or-later; see `LICENSE`. Third-party
components retain their own licenses as listed in `THIRD_PARTY_LICENSES.txt`.
