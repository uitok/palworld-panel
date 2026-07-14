#!/usr/bin/env bash
set -euo pipefail

root_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd -P)"
cd "$root_dir"

failed=0
check_forbidden() {
  local description="$1"
  local pattern="$2"
  local matches
  matches="$({
    while IFS= read -r -d '' path; do
      case "$path" in
        scripts/scan-secrets.sh|frontend/package-lock.json|backend/go.sum|sav-cli/go.sum) continue ;;
      esac
      grep -nH -I -E "$pattern" -- "$path" 2>/dev/null || true
    done < <(git ls-files --cached --others --exclude-standard -z)
  } | grep -v 'replace-with-a-random-32-byte-token' || true)"
  if [[ -n "$matches" ]]; then
    printf 'secret scan: %s\n%s\n' "$description" "$matches" >&2
    failed=1
  fi
}

check_forbidden "plaintext Steam credential code is forbidden" 'DefaultSteamWebAPIKey|SteamWebAPIKey[[:space:]]*=[[:space:]]*"[0-9A-Fa-f]{32}"'
check_forbidden "frontend build-time panel tokens are forbidden" 'VITE_PANEL_TOKEN'
check_forbidden "AWS access key pattern detected" 'AKIA[0-9A-Z]{16}'
check_forbidden "private key material detected" 'BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY'
check_forbidden "non-placeholder secret assignment detected" '(PANEL_TOKEN|STEAM_WEB_API_KEY|OPENAI_API_KEY)[[:space:]]*=[[:space:]]*[A-Za-z0-9_+/=-]{20,}'

if (( failed )); then
  exit 1
fi
printf 'secret scan: current source tree is clean\n'
