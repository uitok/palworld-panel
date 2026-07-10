#!/usr/bin/env bash
set -euo pipefail

archive="${1:?usage: smoke-linux.sh <linux-archive>}"
tmp="$(mktemp -d)"
package_dir=""
cleanup() {
  if [[ -n "$package_dir" && -x "$package_dir/palpanelctl" ]]; then
    PALPANEL_LISTEN_ADDR="127.0.0.1:${panel_port:-18080}" PALPANEL_SAVE_INDEXER_PORT="${sav_port:-18090}" \
      "$package_dir/palpanelctl" stop >/dev/null 2>&1 || true
  fi
  rm -rf "$tmp"
}
trap cleanup EXIT

tar -xzf "$archive" -C "$tmp"
package_dir="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d -print -quit)"
[[ -n "$package_dir" ]] || { printf 'package root not found\n' >&2; exit 1; }
(cd "$package_dir" && sha256sum -c checksums.txt >/dev/null)

panel_port="$((20000 + RANDOM % 15000))"
sav_port="$((35001 + RANDOM % 15000))"
export PALPANEL_LISTEN_ADDR="127.0.0.1:$panel_port"
export PALPANEL_SAVE_INDEXER_PORT="$sav_port"

"$package_dir/bin/palpanel" --version | grep -F 'v1.0.0'
"$package_dir/bin/sav-cli" --version | grep -F 'v1.0.0'
"$package_dir/palpanelctl" init >"$tmp/init.txt"
[[ "$(stat -c '%a' "$package_dir/config/palpanel.env")" == "600" ]]
token="$("$package_dir/palpanelctl" token)"
[[ ${#token} -ge 32 ]]

"$package_dir/palpanelctl" start
"$package_dir/palpanelctl" status
curl --fail --silent "http://127.0.0.1:$panel_port/api/health" | grep -F '"version":"v1.0.0"'
curl --fail --silent "http://127.0.0.1:$sav_port/health" | grep -F '"build_version":"v1.0.0"'
unauthorized="$(curl --silent --output /dev/null --write-out '%{http_code}' "http://127.0.0.1:$panel_port/api/auth/me")"
[[ "$unauthorized" == "401" ]]
curl --fail --silent -H "Authorization: Bearer $token" "http://127.0.0.1:$panel_port/api/auth/me" | grep -F '"role":"admin"'
"$package_dir/palpanelctl" logs >/dev/null
"$package_dir/palpanelctl" restart
curl --fail --silent "http://127.0.0.1:$panel_port/api/health" >/dev/null
"$package_dir/palpanelctl" stop

[[ ! -e "$package_dir/run/backend.pid" && ! -e "$package_dir/run/sav-cli.pid" ]]
if pgrep -f "$package_dir/bin/(palpanel|sav-cli)" >/dev/null 2>&1; then
  printf 'orphan process remains after stop\n' >&2
  exit 1
fi
printf 'linux package smoke test passed\n'
