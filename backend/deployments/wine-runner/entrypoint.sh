#!/usr/bin/env bash
set -euo pipefail
export LC_ALL=C

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

  if grep -Eqi "App '2394010' state is 0x6|App '2394010' state is 0x[[:xdigit:]]*6" "$log_file"; then
    echo "[palpanel] SteamCMD left the app manifest in update-required state; preserving it and rebuilding install metadata." >&2
    local manifest="/data/server/steamapps/appmanifest_2394010.acf"
    local preserved_manifest=""
    if [[ -f "$manifest" ]]; then
      preserved_manifest="${manifest}.palpanel-stale.$(date -u +%Y%m%dT%H%M%SZ)"
      mv "$manifest" "$preserved_manifest"
    fi
    rm -f "$log_file"
    log_file="$(mktemp)"
    set +e
    run_steamcmd_logged "$log_file" \
      +@sSteamCmdForcePlatformType windows \
      +@sSteamCmdForcePlatformBitness 64 \
      +force_install_dir /data/server \
      "${steam_login_args[@]}" \
      +app_info_update 1 \
      +app_update 2394010 validate \
      +quit
    status=$?
    set -e
    if [[ "$status" -ne 0 && ! -f "$manifest" && -n "$preserved_manifest" && -f "$preserved_manifest" ]]; then
      mv "$preserved_manifest" "$manifest"
    fi
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

rotate_server_log() {
  local log_path="${1:?log path is required}"
  local keep="${2:?backup count is required}"
  local index
  rm -f "${log_path}.${keep}"
  for ((index = keep - 1; index >= 1; index--)); do
    if [[ -f "${log_path}.${index}" ]]; then
      mv "${log_path}.${index}" "${log_path}.$((index + 1))"
    fi
  done
  if [[ -f "$log_path" ]]; then
    mv "$log_path" "${log_path}.1"
  fi
  : > "$log_path"
}

stream_server_logs() {
  local log_path="/data/logs/palserver.log"
  local max_bytes=$((20 * 1024 * 1024))
  local keep=5
  local size=0
  local line
  mkdir -p /data/logs
  if [[ -f "$log_path" ]]; then
    size="$(stat -c '%s' "$log_path" 2>/dev/null || echo 0)"
  fi
  if ((size >= max_bytes)); then
    rotate_server_log "$log_path" "$keep"
    size=0
  fi
  while IFS= read -r line || [[ -n "$line" ]]; do
    local line_bytes=$((${#line} + 1))
    if ((size > 0 && size + line_bytes > max_bytes)); then
      rotate_server_log "$log_path" "$keep"
      size=0
    fi
    printf '%s\n' "$line"
    printf '%s\n' "$line" >> "$log_path"
    size=$((size + line_bytes))
  done
}

server_pid=""

forward_server_signal() {
  local signal="${1:?signal is required}"
  if [[ -n "$server_pid" ]] && kill -0 "$server_pid" 2>/dev/null; then
    kill -s "$signal" "$server_pid" 2>/dev/null || true
  fi
}

start_server() {
  mkdir -p /data/server /data/wineprefix /data/logs
  cd /data/server
  if [[ ! -f PalServer.exe ]]; then
    echo "PalServer.exe not found. Run install first." >&2
    exit 3
  fi
  # Prefer a game-local proxy DLL (used by UE4SS) while retaining Wine's
  # builtin fallback for installations that do not provide one.
  export WINEDLLOVERRIDES="${WINEDLLOVERRIDES:-dwmapi=n,b}"
  export HOME="${HOME:-/data/wineprefix}"
  export XDG_RUNTIME_DIR="${XDG_RUNTIME_DIR:-/tmp/palpanel-runtime-$(id -u)}"
  mkdir -p "$XDG_RUNTIME_DIR"
  chmod 700 "$XDG_RUNTIME_DIR" || true
  local fifo
  local logger_pid
  local status
  fifo="$(mktemp /tmp/palpanel-server-log.XXXXXX)"
  rm -f "$fifo"
  mkfifo "$fifo"
  stream_server_logs < "$fifo" &
  logger_pid=$!
  exec 3> "$fifo"
  printf '[palpanel] %s starting PalServer.exe\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >&3
  trap 'forward_server_signal TERM' TERM
  trap 'forward_server_signal INT' INT

  set +e
  wine PalServer.exe "$@" >&3 2>&1 &
  server_pid=$!
  wait "$server_pid"
  status=$?
  printf '[palpanel] %s PalServer.exe exited with status %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$status" >&3
  exec 3>&-
  wait "$logger_pid"
  set -e

  trap - TERM INT
  rm -f "$fifo"
  return "$status"
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
