#!/usr/bin/env bash
set -euo pipefail

archive="${1:?usage: test-bootstrap-install.sh <linux-archive>}"
archive="$(readlink -f -- "$archive")"
archive_name="$(basename -- "$archive")"
[[ "$archive_name" =~ ^palpanel_(v[0-9]+\.[0-9]+\.[0-9]+)_linux_amd64\.tar\.gz$ ]] || {
  printf 'unexpected archive name: %s\n' "$archive_name" >&2
  exit 1
}
version="${BASH_REMATCH[1]}"
root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT

mkdir -p "$tmp/release" "$tmp/systemd"
cp "$archive" "$tmp/release/$archive_name"
(cd "$tmp/release" && sha256sum "$archive_name" >SHA256SUMS)

export PALPANEL_TEST_MODE=1
export PALPANEL_RELEASE_BASE_URL="file://$tmp/release"
export PALPANEL_INSTALL_ROOT="$tmp/opt/palpanel"
export PALPANEL_ETC_DIR="$tmp/etc/palpanel"
export PALPANEL_SYSTEM_DATA_DIR="$tmp/var/lib/palpanel"
export PALPANEL_SYSTEMD_DIR="$tmp/systemd"
PALPANEL_SERVICE_USER="$(id -un)"
export PALPANEL_SERVICE_USER
export PALPANEL_SKIP_SYSTEMD=1
export PALPANEL_SKIP_HEALTH_CHECK=1

"$root_dir/install.sh" --version "$version" --listen 127.0.0.1:18080 --no-docker >"$tmp/install.out"
grep -qx 'PALPANEL_LISTEN_ADDR=127.0.0.1:18080' "$PALPANEL_ETC_DIR/palpanel.env"
[[ -x "$PALPANEL_INSTALL_ROOT/current/bin/palpanel" ]]
[[ -x "$PALPANEL_INSTALL_ROOT/current/bin/sav-cli" ]]
[[ -f "$PALPANEL_INSTALL_ROOT/current/frontend/dist/index.html" ]]
grep -q '^Panel URL: http://127.0.0.1:18080/$' "$tmp/install.out"
grep -Eq '^Admin Token: [0-9a-f]{64}$' "$tmp/install.out"

"$root_dir/install.sh" --version "$version" --no-docker >"$tmp/upgrade.out"
grep -qx 'PALPANEL_LISTEN_ADDR=127.0.0.1:18080' "$PALPANEL_ETC_DIR/palpanel.env"
grep -q '^Panel URL: http://127.0.0.1:18080/$' "$tmp/upgrade.out"

printf '%064d  %s\n' 0 "$archive_name" >"$tmp/release/SHA256SUMS"
if "$root_dir/install.sh" --version "$version" --no-docker >"$tmp/bad.out" 2>"$tmp/bad.err"; then
  printf 'installer accepted an invalid checksum\n' >&2
  exit 1
fi
grep -q 'checksum verification failed' "$tmp/bad.err"

"$PALPANEL_INSTALL_ROOT/current/palpanelctl" uninstall --purge >/dev/null
printf 'GitHub bootstrap installer verification passed\n'
