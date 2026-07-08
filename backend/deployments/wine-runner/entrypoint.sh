#!/usr/bin/env bash
set -euo pipefail

cmd="${1:-start}"
shift || true

steam_login_args=("+login" "anonymous")
if [[ -n "${STEAM_USERNAME:-}" && -n "${STEAM_PASSWORD:-}" ]]; then
  steam_login_args=("+login" "$STEAM_USERNAME" "$STEAM_PASSWORD")
fi

install_server() {
  mkdir -p /data/server /data/wineprefix
  /opt/steamcmd/steamcmd.sh \
    +@sSteamCmdForcePlatformType windows \
    +force_install_dir /data/server \
    "${steam_login_args[@]}" \
    +app_update 2394010 validate \
    +quit
}

download_workshop() {
  local item_id="${1:?workshop item id is required}"
  mkdir -p /data/workshop/.steamcmd
  /opt/steamcmd/steamcmd.sh \
    +@sSteamCmdForcePlatformType windows \
    +force_install_dir /data/workshop/.steamcmd \
    "${steam_login_args[@]}" \
    +workshop_download_item 1623730 "$item_id" validate \
    +quit

  local src="/data/workshop/.steamcmd/steamapps/workshop/content/1623730/$item_id"
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
  exec xvfb-run -a wine PalServer.exe "$@"
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
