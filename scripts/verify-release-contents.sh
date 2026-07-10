#!/usr/bin/env bash
set -euo pipefail

archive="${1:?usage: verify-release-contents.sh <linux-archive> [source-archive]}"
source_archive="${2:-}"
[[ -f "$archive" ]] || { printf 'archive not found: %s\n' "$archive" >&2; exit 1; }

listing="$(tar -tzf "$archive")"
if grep -E '(^|/)(data|logs|run)/|(^|/)palpanel\.env$|(^|/)\.env($|\.)|\.db$|\.sqlite$|\.log$' <<<"$listing"; then
  printf 'release archive contains runtime data, secrets, database, or logs\n' >&2
  exit 1
fi

required=(
  '/bin/palpanel'
  '/bin/sav-cli'
  '/palpanelctl'
  '/config/palpanel.env.example'
  '/systemd/palpanel.service'
  '/systemd/palpanel-sav-cli.service'
  '/THIRD_PARTY_LICENSES.txt'
  '/licenses/GPL-3.0.txt'
  '/checksums.txt'
)
for item in "${required[@]}"; do
  grep -Fq "$item" <<<"$listing" || { printf 'release archive is missing %s\n' "$item" >&2; exit 1; }
done

tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
tar -xzf "$archive" -C "$tmp"
package_dir="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d -print -quit)"
(cd "$package_dir" && sha256sum -c checksums.txt >/dev/null)
if grep -RInI -E '(PANEL_TOKEN|STEAM_WEB_API_KEY)[[:space:]]*=[[:space:]]*[A-Za-z0-9_+/=-]{20,}' "$package_dir" --exclude='checksums.txt'; then
  printf 'release archive contains a configured secret\n' >&2
  exit 1
fi

if [[ -n "$source_archive" ]]; then
  [[ -f "$source_archive" ]] || { printf 'source archive not found: %s\n' "$source_archive" >&2; exit 1; }
  source_listing="$(tar -tzf "$source_archive")"
  if grep -E '/sav-cli/(data|logs|run|dist)/|/sav-cli/\.env($|\.)|\.(db|sqlite|log|sav|zip|exe|dll|o|a)$' <<<"$source_listing"; then
    printf 'source archive contains runtime data, secrets, database, logs, or build artifacts\n' >&2
    exit 1
  fi
  grep -Fq '/sav-cli/LICENSE' <<<"$source_listing" || { printf 'source archive is missing sav-cli/LICENSE\n' >&2; exit 1; }
  grep -Fq '/sav-cli/vendor/github.com/oriath-net/gooz/COPYING' <<<"$source_listing" || {
    printf 'source archive is missing vendored gooz license\n' >&2
    exit 1
  }
  grep -Fq '/sav-cli/vendor/github.com/oriath-net/gooz/kraken.cpp' <<<"$source_listing" || {
    printf 'source archive is missing vendored gooz source\n' >&2
    exit 1
  }
fi
printf 'release content verification passed\n'
