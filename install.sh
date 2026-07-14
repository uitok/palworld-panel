#!/usr/bin/env bash
set -euo pipefail
umask 077

repo="${PALPANEL_REPO:-uitok/palworld-panel}"
version="${PALPANEL_VERSION:-latest}"
listen_addr="${PALPANEL_LISTEN_ADDR:-127.0.0.1:8080}"
listen_explicit=0
[[ -v PALPANEL_LISTEN_ADDR ]] && listen_explicit=1
docker_mode="auto"
proxy_url="${PALPANEL_PROXY:-}"
github_token="${GITHUB_TOKEN:-${GH_TOKEN:-}}"

fail() {
  printf 'PalPanel installer: %s\n' "$*" >&2
  exit 1
}

usage() {
  cat <<'EOF'
Usage: install.sh [OPTIONS]

Downloads and installs the latest PalPanel Linux amd64 release.

Options:
  --version TAG        install a specific release tag (default: latest)
  --listen HOST:PORT   panel listener (default: 127.0.0.1:8080)
  --docker             grant the service account Docker socket access
  --no-docker          do not grant Docker socket access
  --proxy URL          proxy for GitHub downloads (for example socks5h://127.0.0.1:10808)
  --repo OWNER/REPO    GitHub repository (default: uitok/palworld-panel)
  -h, --help           show this help
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version)
      [[ $# -ge 2 ]] || fail "--version requires a tag"
      version="$2"
      shift 2
      ;;
    --version=*) version="${1#*=}"; shift ;;
    --listen)
      [[ $# -ge 2 ]] || fail "--listen requires HOST:PORT"
      listen_addr="$2"
      listen_explicit=1
      shift 2
      ;;
    --listen=*) listen_addr="${1#*=}"; listen_explicit=1; shift ;;
    --docker) docker_mode="enabled"; shift ;;
    --no-docker) docker_mode="disabled"; shift ;;
    --proxy)
      [[ $# -ge 2 ]] || fail "--proxy requires a URL"
      proxy_url="$2"
      shift 2
      ;;
    --proxy=*) proxy_url="${1#*=}"; shift ;;
    --repo)
      [[ $# -ge 2 ]] || fail "--repo requires OWNER/REPO"
      repo="$2"
      shift 2
      ;;
    --repo=*) repo="${1#*=}"; shift ;;
    -h|--help) usage; exit 0 ;;
    *) fail "unknown option: $1" ;;
  esac
done

[[ "$(uname -s)" == "Linux" ]] || fail "only Linux is supported"
case "$(uname -m)" in
  x86_64|amd64) ;;
  *) fail "only linux-amd64 is supported" ;;
esac
[[ "${PALPANEL_TEST_MODE:-0}" == "1" || "$(id -u)" -eq 0 ]] || fail "run this installer as root (for example: curl ... | sudo bash)"
[[ "$repo" =~ ^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$ ]] || fail "invalid GitHub repository: $repo"
[[ "$listen_addr" =~ ^[^[:space:]#=]+:[0-9]+$ ]] || fail "invalid listen address: $listen_addr"
if [[ -n "$proxy_url" ]]; then
  [[ "$proxy_url" =~ ^(socks5h?|https?)://[^[:space:]]+$ ]] || fail "invalid proxy URL: $proxy_url"
fi
listen_host="${listen_addr%:*}"
listen_port="${listen_addr##*:}"
[[ -n "$listen_host" && "$listen_host" != *'/'* ]] || fail "invalid listen host: $listen_host"
(( ${#listen_port} <= 5 )) || fail "listen port must be between 1 and 65535"
listen_port_number=$((10#$listen_port))
(( listen_port_number >= 1 && listen_port_number <= 65535 )) || fail "listen port must be between 1 and 65535"

for command_name in curl sha256sum tar awk sed head mktemp seq sleep hostname; do
  command -v "$command_name" >/dev/null 2>&1 || fail "$command_name is required"
done

curl_headers=(-H 'Accept: application/vnd.github+json')
if [[ -n "$github_token" ]]; then
  curl_headers+=(-H "Authorization: Bearer $github_token")
fi
curl_proxy=()
if [[ -n "$proxy_url" ]]; then
  # An explicit proxy must win over a broad NO_PROXY inherited from the shell.
  curl_proxy=(--proxy "$proxy_url" --noproxy '')
fi

resolve_latest_version() {
  local resolved=""
  local effective_url=""
  local response=""
  if [[ -z "$proxy_url" ]] && command -v gh >/dev/null 2>&1; then
    resolved="$(gh api "repos/$repo/releases/latest" --jq .tag_name 2>/dev/null || true)"
  fi
  if [[ -z "$resolved" && -n "$github_token" ]]; then
    response="$(curl --fail --silent --show-error --location --retry 3 "${curl_proxy[@]}" "${curl_headers[@]}" \
      "https://api.github.com/repos/$repo/releases/latest")"
    resolved="$(printf '%s\n' "$response" | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n 1)"
  fi
  if [[ -z "$resolved" ]]; then
    effective_url="$(curl --fail --silent --show-error --location --retry 3 "${curl_proxy[@]}" -o /dev/null -w '%{url_effective}' \
      "https://github.com/$repo/releases/latest")"
    resolved="${effective_url##*/}"
  fi
  [[ "$resolved" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || fail "could not resolve the latest release tag"
  printf '%s\n' "$resolved"
}

if [[ "$version" == "latest" ]]; then
  version="$(resolve_latest_version)"
fi
[[ "$version" =~ ^v[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]] || fail "invalid release tag: $version"

temporary_dir="$(mktemp -d)"
cleanup() {
  rm -rf "$temporary_dir"
}
trap cleanup EXIT

archive_name="palpanel_${version}_linux_amd64.tar.gz"
checksums_name="SHA256SUMS"
if [[ -n "${PALPANEL_RELEASE_BASE_URL:-}" ]]; then
  release_base_url="${PALPANEL_RELEASE_BASE_URL%/}"
else
  release_base_url="https://github.com/$repo/releases/download/$version"
fi

download_asset() {
  local name="$1"
  printf 'Downloading %s\n' "$name"
  curl --fail --silent --show-error --location --retry 3 --connect-timeout 15 \
    "${curl_proxy[@]}" \
    "${curl_headers[@]}" "$release_base_url/$name" -o "$temporary_dir/$name"
}

download_asset "$archive_name"
download_asset "$checksums_name"

expected_checksum="$(awk -v name="$archive_name" '$2 == name || $2 == "*" name { print $1 }' "$temporary_dir/$checksums_name")"
[[ "$expected_checksum" =~ ^[0-9a-fA-F]{64}$ ]] || fail "release checksum for $archive_name is missing or invalid"
actual_checksum="$(sha256sum "$temporary_dir/$archive_name" | awk '{ print $1 }')"
[[ "$actual_checksum" == "$expected_checksum" ]] || fail "checksum verification failed for $archive_name"

mkdir -p "$temporary_dir/extracted"
tar -xzf "$temporary_dir/$archive_name" -C "$temporary_dir/extracted"
package_dir="$temporary_dir/extracted/palpanel_${version}_linux_amd64"
[[ -x "$package_dir/palpanelctl" ]] || fail "release package does not contain palpanelctl"

docker_access=0
case "$docker_mode" in
  enabled) docker_access=1 ;;
  disabled) ;;
  auto)
    if command -v docker >/dev/null 2>&1 && command -v getent >/dev/null 2>&1 \
      && getent group docker >/dev/null 2>&1 \
      && { [[ -S /var/run/docker.sock ]] || [[ -S /run/docker.sock ]]; }; then
      docker_access=1
    fi
    ;;
esac

etc_dir="${PALPANEL_ETC_DIR:-/etc/palpanel}"
config_path="$etc_dir/palpanel.env"
install_args=(install)
if (( listen_explicit )) || [[ ! -f "$config_path" ]]; then
  install_args+=(--listen "$listen_addr")
fi
if (( docker_access )); then
  install_args+=(--docker)
fi
printf 'Installing PalPanel %s\n' "$version"
"$package_dir/palpanelctl" "${install_args[@]}"

install_root="${PALPANEL_INSTALL_ROOT:-/opt/palpanel}"
installed_ctl="$install_root/current/palpanelctl"
[[ -x "$installed_ctl" ]] || fail "installation did not create $installed_ctl"

if (( ! listen_explicit )) && [[ -f "$config_path" ]]; then
  configured_listen="$(awk '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      equals = index($0, "=")
      if (equals == 0) next
      key = substr($0, 1, equals - 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", key)
      if (key != "PALPANEL_LISTEN_ADDR") next
      value = substr($0, equals + 1)
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", value)
      if (length(value) >= 2) {
        first = substr(value, 1, 1)
        last = substr(value, length(value), 1)
        if ((first == "\"" && last == "\"") || (first == "\047" && last == "\047")) {
          value = substr(value, 2, length(value) - 2)
        }
      }
      print value
      exit
    }
  ' "$config_path")"
  [[ -n "$configured_listen" ]] && listen_addr="$configured_listen"
fi
[[ "$listen_addr" =~ ^[^[:space:]#=]+:[0-9]+$ ]] || fail "installed listen address is invalid: $listen_addr"
listen_host="${listen_addr%:*}"
listen_port="${listen_addr##*:}"
[[ -n "$listen_host" && "$listen_host" != *'/'* ]] || fail "installed listen host is invalid: $listen_host"
(( ${#listen_port} <= 5 )) || fail "installed listen port is invalid: $listen_port"
listen_port_number=$((10#$listen_port))
(( listen_port_number >= 1 && listen_port_number <= 65535 )) || fail "installed listen port is invalid: $listen_port"

health_host="$listen_host"
case "$health_host" in
  0.0.0.0|'[::]'|'::') health_host="127.0.0.1" ;;
esac
if [[ "$health_host" == *:* && "$health_host" != \[*\] ]]; then
  health_host="[$health_host]"
fi
health_url="http://$health_host:$listen_port/api/health"
if [[ "${PALPANEL_SKIP_HEALTH_CHECK:-0}" != "1" ]]; then
  healthy=0
  for _ in $(seq 1 60); do
    if curl --noproxy '*' --fail --silent --show-error --max-time 2 "$health_url" >/dev/null 2>&1; then
      healthy=1
      break
    fi
    sleep 0.5
  done
  (( healthy )) || fail "services did not become healthy; run $installed_ctl logs"
fi

panel_host="$listen_host"
case "$panel_host" in
  0.0.0.0|'[::]'|'::')
    panel_host="$(hostname -I 2>/dev/null | awk '{ print $1 }')"
    [[ -n "$panel_host" ]] || panel_host="$(hostname -f 2>/dev/null || hostname)"
    ;;
esac
if [[ "$panel_host" == *:* && "$panel_host" != \[*\] ]]; then
  panel_host="[$panel_host]"
fi
panel_url="http://$panel_host:$listen_port/"

printf '\nPalPanel installation completed.\n'
printf 'Panel URL: %s\n' "$panel_url"
printf 'Open the panel URL to register the first administrator.\n'
printf 'Status: sudo %s status\n' "$installed_ctl"
printf 'Logs: sudo %s logs -f\n' "$installed_ctl"
if (( docker_access )); then
  printf 'Docker access: enabled (Docker group membership is root-equivalent).\n'
fi
