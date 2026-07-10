# PalPanel Windows Package

This ZIP contains the PalPanel frontend, backend, native `sav-cli`, and the
double-click Windows Launcher.

## Start

1. Extract the whole ZIP to a writable directory.
2. Double-click `PalPanel.exe`.
3. Confirm the first-run data location prompt.
4. Enter the admin token shown by the Launcher in the browser. The frontend
   and API use the same address, so no backend URL is required.

Configuration, server data, backups, and logs stay next to `PalPanel.exe`.
Do not run the Launcher from inside the ZIP. The Launcher prevents duplicate
instances and stops its backend and `sav-cli` child processes when it exits.

`windows_steamcmd` is the recommended runtime on a native Windows host. The
game server itself is installed from the web setup page and is not bundled in
this ZIP.

## Signature warning

The executables are not Authenticode-signed. Windows SmartScreen may show an
unknown publisher warning. Verify the ZIP with the release `SHA256SUMS` before
running it.

## License

PalPanel is licensed under GPL-3.0-or-later; see `LICENSE`. Third-party
components retain their own licenses as listed in `THIRD_PARTY_LICENSES.txt`.
