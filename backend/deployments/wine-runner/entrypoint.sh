#!/usr/bin/env bash
# Keep this script LF-only; the Dockerfile also normalizes copied sources so
# Windows worktrees cannot produce an unusable /usr/bin/env "bash\\r" shebang.
set -euo pipefail
export LC_ALL=C
steam_home="${HOME:-/root}/Steam"

cmd="${1:-start}"
shift || true

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
    +login anonymous \
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
      +login anonymous \
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
      +login anonymous \
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
  local restore_errexit=0
  [[ "$-" == *e* ]] && restore_errexit=1
  set +e
  /opt/steamcmd/steamcmd.sh "$@" 2>&1 | tee "$log_file"
  local status=${PIPESTATUS[0]}
  if [[ "$restore_errexit" -eq 1 ]]; then
    set -e
  else
    set +e
  fi
  return "$status"
}

download_workshop() {
  local item_id="${1:?workshop item id is required}"
  local account_name="${2:?Steam account name is required}"
  local app_id="${PALPANEL_WORKSHOP_APP_ID:-1623730}"
  mkdir -p /data/workshop/.steamcmd
  set +e
  /opt/steamcmd/steamcmd.sh \
    +@ShutdownOnFailedCommand 1 \
    +@NoPromptForPassword 1 \
    +@sSteamCmdForcePlatformType windows \
    +force_install_dir /data/workshop/.steamcmd \
    +login "$account_name" \
    +workshop_download_item "$app_id" "$item_id" validate \
    +quit
  local steam_status=$?
  set -e
  restore_host_ownership /data/workshop "$steam_home"
  if [[ "$steam_status" -ne 0 ]]; then
    return "$steam_status"
  fi

  local src="/data/workshop/.steamcmd/steamapps/workshop/content/$app_id/$item_id"
  local dst="/data/workshop/$item_id"
  if [[ ! -d "$src" ]]; then
    echo "workshop item was not downloaded to $src" >&2
    exit 2
  fi
  rm -rf "$dst"
  mkdir -p "$dst"
  cp -a "$src"/. "$dst"/
}

app_info() {
  /opt/steamcmd/steamcmd.sh \
    +@sSteamCmdForcePlatformType windows \
    +login anonymous \
    +app_info_update 1 \
    +app_info_print 2394010 \
    +quit
}

steam_auth_login() {
  local account_name="${1:?Steam account name is required}"
  if [[ ! "$account_name" =~ ^[A-Za-z0-9_]{3,64}$ ]]; then
    echo "invalid Steam account name" >&2
    return 64
  fi
  mkdir -p "$steam_home"
  set +e
  /opt/steamcmd/steamcmd.sh +login "$account_name"
  local status=$?
  set -e
  restore_host_ownership "$steam_home"
  return "$status"
}

steam_auth_verify() {
  local account_name="${1:?Steam account name is required}"
  local log_file
  log_file="$(mktemp)"
  set +e
  run_steamcmd_logged "$log_file" \
    +@ShutdownOnFailedCommand 1 \
    +@NoPromptForPassword 1 \
    +login "$account_name" \
    +quit
  local status=$?
  set -e
  restore_host_ownership "$steam_home"

  if [[ "$status" -eq 0 ]] && grep -Eqi \
    'Waiting for user info\.\.\.OK|Logged in OK|Login Successful|Logging in using cached credentials' \
    "$log_file" && ! grep -Eqi \
    'Invalid Password|Account Logon Denied|Steam Guard|Two-factor|No cached credentials|Cached credentials not found|Password required|Login Failure|Failed to log in|Not logged on' \
    "$log_file"; then
    rm -f "$log_file"
    return 0
  fi
  rm -f "$log_file"
  echo "[palpanel] Steam authentication cache is missing or expired; run palpanelctl steam-login for this account." >&2
  return 3
}

steam_auth_runscript() {
  local script_path="${1:?SteamCMD runscript path is required}"
  if [[ "$script_path" != /run/palpanel/* || ! -f "$script_path" ]]; then
    echo "invalid SteamCMD runscript path" >&2
    return 64
  fi
  set +e
  /opt/steamcmd/steamcmd.sh +runscript "$script_path"
  local status=$?
  set -e
  restore_host_ownership "$steam_home"
  return "$status"
}

proxy_test() {
  local target="${1:?proxy test target is required}"
  curl --fail --silent --show-error --location --max-time 12 --range 0-0 "$target" >/dev/null
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

palpanel_wine_dll_overrides() {
  local configured="${1:-}"
  configured="${configured%;}"
  if [[ ";$configured;" != *';dwmapi=n,b;'* ]]; then
    configured="${configured:+$configured;}dwmapi=n,b"
  fi
  if [[ ";$configured;" != *';d3d9=n,b;'* ]]; then
    configured="${configured:+$configured;}d3d9=n,b"
  fi
  printf '%s\n' "$configured"
}

start_server() {
  mkdir -p /data/server /data/wineprefix /data/logs
  cd /data/server
  if [[ ! -f PalServer.exe ]]; then
    echo "PalServer.exe not found. Run install first." >&2
    exit 3
  fi
  # PalDefender's game-local d3d9 loader and UE4SS's dwmapi proxy must win
  # over Wine's builtins while retaining any unrelated caller overrides.
  export WINEDLLOVERRIDES="$(palpanel_wine_dll_overrides "${WINEDLLOVERRIDES:-}")"
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

if [[ "${PALPANEL_ENTRYPOINT_SOURCE_ONLY:-0}" == "1" ]]; then
  return 0 2>/dev/null || exit 0
fi

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
  steam-auth-login)
    steam_auth_login "$@"
    ;;
  steam-auth-verify)
    steam_auth_verify "$@"
    ;;
  steam-auth-runscript)
    steam_auth_runscript "$@"
    ;;
  proxy-test)
    proxy_test "$@"
    ;;
  start)
    start_server "$@"
    ;;
  *)
    echo "unknown command: $cmd" >&2
    exit 64
    ;;
esac
