#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-start}"
shift || true

steam_login_args=("+login" "anonymous")
if [[ -n "${STEAM_USERNAME:-}" && -n "${STEAM_PASSWORD:-}" ]]; then
  steam_login_args=("+login" "$STEAM_USERNAME" "$STEAM_PASSWORD")
fi

restore_host_ownership() {
  if [[ -z "${PALPANEL_HOST_UID:-}" || -z "${PALPANEL_HOST_GID:-}" ]]; then
    return 0
  fi
  if [[ ! "$PALPANEL_HOST_UID" =~ ^[0-9]+$ || ! "$PALPANEL_HOST_GID" =~ ^[0-9]+$ ]]; then
    echo "[palpanel] Invalid PALPANEL_HOST_UID/PALPANEL_HOST_GID; skipped ownership repair." >&2
    return 0
  fi
  for path in "$@"; do
    if [[ -e "$path" ]]; then
      chown -R "$PALPANEL_HOST_UID:$PALPANEL_HOST_GID" "$path" || true
    fi
  done
}

install_server() {
  mkdir -p /data/server /data/wineprefix
  local status=0
  local log_file
  log_file="$(mktemp)"
  if run_steamcmd_logged "$log_file" \
    +@sSteamCmdForcePlatformType windows \
    +@sSteamCmdForcePlatformBitness 64 \
    +force_install_dir /data/server \
    "${steam_login_args[@]}" \
    +app_info_update 1 \
    +app_update 2394010 validate \
    +quit; then
    rm -f "$log_file"
    restore_host_ownership /data/server /data/wineprefix
    return 0
  fi

  if grep -qi "Missing configuration" "$log_file"; then
    echo "[palpanel] SteamCMD reported missing app configuration; retrying after login with refreshed app info." >&2
    rm -f "$log_file"
    log_file="$(mktemp)"
    run_steamcmd_logged "$log_file" \
      "${steam_login_args[@]}" \
      +app_info_update 1 \
      +@sSteamCmdForcePlatformType windows \
      +@sSteamCmdForcePlatformBitness 64 \
      +force_install_dir /data/server \
      +app_update 2394010 validate \
      +quit
    status=$?
    rm -f "$log_file"
    restore_host_ownership /data/server /data/wineprefix
    return "$status"
  fi

  rm -f "$log_file"
  restore_host_ownership /data/server /data/wineprefix
  return 1
}

run_steamcmd_logged() {
  local log_file="${1:?log file is required}"
  shift
  set +e
  /opt/steamcmd/steamcmd.sh "$@" 2>&1 | tee "$log_file"
  local status=${PIPESTATUS[0]}
  set -e
  return "$status"
}

download_workshop() {
  local item_id="${1:?workshop item id is required}"
  local app_id="${PALPANEL_WORKSHOP_APP_ID:-1623730}"
  mkdir -p /data/workshop/.steamcmd
  /opt/steamcmd/steamcmd.sh \
    +@sSteamCmdForcePlatformType windows \
    +force_install_dir /data/workshop/.steamcmd \
    "${steam_login_args[@]}" \
    +workshop_download_item "$app_id" "$item_id" validate \
    +quit

  local src="/data/workshop/.steamcmd/steamapps/workshop/content/$app_id/$item_id"
  local dst="/data/workshop/$item_id"
  if [[ ! -d "$src" ]]; then
    echo "workshop item was not downloaded to $src" >&2
    exit 2
  fi
  rm -rf "$dst"
  mkdir -p "$dst"
  cp -a "$src"/. "$dst"/
  restore_host_ownership /data/workshop
}

app_info() {
  /opt/steamcmd/steamcmd.sh \
    +@sSteamCmdForcePlatformType windows \
    "${steam_login_args[@]}" \
    +app_info_update 1 \
    +app_info_print 2394010 \
    +quit
}

start_server() {
  mkdir -p /data/server /data/wineprefix
  cd /data/server
  if [[ ! -f PalServer.exe ]]; then
    echo "PalServer.exe not found. Run install first." >&2
    exit 3
  fi
  export HOME="${HOME:-/data/wineprefix}"
  export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp/palpanel-runtime-$(id -u)}"
  mkdir -p "$XDG_RUNTIME_DIR"
  chmod 700 "$XDG_RUNTIME_DIR" || true
  exec wine PalServer.exe "$@"
}

case "$cmd" in
  install|update)
    install_server
    ;;
  workshop)
    download_workshop "$@"
    ;;
  appinfo)
    app_info
    ;;
  start)
    start_server "$@"
    ;;
  *)
    echo "unknown command: $cmd" >&2
    exit 64
    ;;
esac
