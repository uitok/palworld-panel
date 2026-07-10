#!/usr/bin/env bash
set -euo pipefail

archive="${1:?usage: test-install.sh <linux-archive>}"
tmp="$(mktemp -d)"
if [[ "$(id -u)" -eq 0 ]]; then
  service_user="palpanel-ci-$RANDOM"
else
  service_user="$(id -un)"
  export PALPANEL_TEST_MODE=1
fi
cleanup() {
  if [[ "$(id -u)" -eq 0 ]]; then
    userdel "$service_user" 2>/dev/null || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

mkdir -p "$tmp/package"
tar -xzf "$archive" -C "$tmp/package"
package_dir="$(find "$tmp/package" -mindepth 1 -maxdepth 1 -type d -print -quit)"
ctl="$package_dir/palpanelctl"
export PALPANEL_INSTALL_ROOT="$tmp/opt/palpanel"
export PALPANEL_ETC_DIR="$tmp/etc/palpanel"
export PALPANEL_SYSTEM_DATA_DIR="$tmp/var/lib/palpanel"
export PALPANEL_SYSTEMD_DIR="$tmp/systemd"
export PALPANEL_SERVICE_USER="$service_user"
export PALPANEL_SKIP_SYSTEMD=1

"$ctl" install >/dev/null
[[ -L "$PALPANEL_INSTALL_ROOT/current" ]]
[[ "$(stat -c '%a' "$PALPANEL_INSTALL_ROOT")" == "755" ]]
[[ "$(stat -c '%a' "$(readlink -f "$PALPANEL_INSTALL_ROOT/current")")" == "755" ]]
[[ "$(stat -c '%a' "$PALPANEL_ETC_DIR")" == "750" ]]
[[ "$(stat -c '%a' "$PALPANEL_ETC_DIR/palpanel.env")" == "600" ]]
installed_dir="$(readlink -f "$PALPANEL_INSTALL_ROOT/current")"
[[ -f "$installed_dir/LICENSE" ]]
[[ -f "$installed_dir/licenses/GPL-3.0.txt" ]]
[[ -f "$installed_dir/licenses/sav-cli-LICENSE.txt" ]]
[[ -f "$installed_dir/THIRD_PARTY_LICENSES.txt" ]]
[[ "$("$installed_dir/palpanelctl" config)" == "$PALPANEL_ETC_DIR/palpanel.env" ]]
printf '# preserve-config\n' >>"$PALPANEL_ETC_DIR/palpanel.env"
printf 'preserve-data\n' >"$PALPANEL_SYSTEM_DATA_DIR/preserve.marker"
config_hash="$(sha256sum "$PALPANEL_ETC_DIR/palpanel.env" | awk '{print $1}')"

"$ctl" install >/dev/null
[[ "$(sha256sum "$PALPANEL_ETC_DIR/palpanel.env" | awk '{print $1}')" == "$config_hash" ]]
[[ -f "$PALPANEL_SYSTEM_DATA_DIR/preserve.marker" ]]
"$ctl" uninstall >/dev/null
[[ ! -e "$PALPANEL_INSTALL_ROOT" ]]
[[ -f "$PALPANEL_ETC_DIR/palpanel.env" && -f "$PALPANEL_SYSTEM_DATA_DIR/preserve.marker" ]]

"$ctl" install >/dev/null
"$ctl" uninstall --purge >/dev/null
[[ ! -e "$PALPANEL_INSTALL_ROOT" && ! -e "$PALPANEL_ETC_DIR" && ! -e "$PALPANEL_SYSTEM_DATA_DIR" ]]
printf 'install, upgrade, uninstall, and purge verification passed\n'
