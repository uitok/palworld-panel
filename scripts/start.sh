#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
package_dir="$(cd -- "$script_dir/.." && pwd)"
env_file="$package_dir/config/palpanel.env"
example_file="$package_dir/config/palpanel.env.example"

random_token() {
  if command -v openssl >/dev/null 2>&1; then
    openssl rand -hex 32
  else
    head -c 32 /dev/urandom | od -An -tx1 | tr -d ' \n'
  fi
}

if [[ ! -f "$env_file" ]]; then
  mkdir -p "$(dirname "$env_file")"
  if [[ -f "$example_file" ]]; then
    cp "$example_file" "$env_file"
  else
    touch "$env_file"
  fi
  token="$(random_token)"
  if grep -q '^PANEL_TOKEN=' "$env_file"; then
    sed -i.bak "s/^PANEL_TOKEN=.*/PANEL_TOKEN=$token/" "$env_file"
    rm -f "$env_file.bak"
  else
    printf '\nPANEL_TOKEN=%s\n' "$token" >>"$env_file"
  fi
  echo "[palpanel] Created config/palpanel.env"
  echo "[palpanel] PANEL_TOKEN=$token"
fi

set -a
# shellcheck disable=SC1090
. "$env_file"
set +a

export PALPANEL_FRONTEND_DIST="${PALPANEL_FRONTEND_DIST:-$package_dir/frontend/dist}"
export PALPANEL_BACKEND_DIR="${PALPANEL_BACKEND_DIR:-$package_dir/backend}"
export PALPANEL_DATA_DIR="${PALPANEL_DATA_DIR:-$package_dir/data}"
export PALPANEL_RUNNER_DIR="${PALPANEL_RUNNER_DIR:-$package_dir/backend/deployments/wine-runner}"

mkdir -p "$PALPANEL_DATA_DIR"

listen="${PALPANEL_LISTEN_ADDR:-:8080}"
display_port="${listen##*:}"
if [[ -z "$display_port" || "$display_port" == "$listen" ]]; then
  display_port="8080"
fi

echo "[palpanel] Frontend: http://127.0.0.1:$display_port/dashboard"
if [[ -n "${PANEL_TOKEN:-}" ]]; then
  echo "[palpanel] Token: $PANEL_TOKEN"
fi

exec "$package_dir/bin/palpanel"
