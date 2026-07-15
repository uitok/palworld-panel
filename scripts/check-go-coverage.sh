#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

check_package() {
  local module_dir="$1"
  local package="$2"
  local minimum="$3"
  local profile
  profile="$tmp_dir/$(basename "$module_dir")-$(basename "$package").cover"
  local coverage

  (
    cd "$repo_root/$module_dir"
    go test -coverprofile="$profile" "$package"
  )
  coverage="$( (cd "$repo_root/$module_dir" && go tool cover -func="$profile") | awk '/^total:/ {gsub("%", "", $3); print $3}')"
  if [[ -z "$coverage" ]]; then
    echo "unable to read coverage for $module_dir $package" >&2
    return 1
  fi
  if ! awk -v actual="$coverage" -v minimum="$minimum" 'BEGIN { exit !(actual + 0 >= minimum + 0) }'; then
    echo "$module_dir $package coverage ${coverage}% is below ${minimum}%" >&2
    return 1
  fi
  echo "$module_dir $package coverage ${coverage}% (minimum ${minimum}%)"
}

check_package backend ./internal/api 60
check_package backend ./internal/db 60
check_package backend ./internal/server 60
check_package backend ./internal/scheduler 70
check_package backend ./internal/monitor 70
check_package backend ./internal/palrest 70
CGO_ENABLED=1 check_package sav-cli ./internal/sidecar 70
