#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd -- "$script_dir/.." && pwd)"
packages_dir="$root_dir/dist/packages"
staging_dir="$packages_dir/staging"

version=""
targets="linux-amd64,windows-amd64"
skip_tests=0
clean=0

usage() {
  printf 'Usage: scripts/package.sh [--version VERSION] [--targets linux-amd64,windows-amd64] [--skip-tests] [--clean]\n'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version|-Version)
      version="${2:?--version requires a value}"
      shift 2
      ;;
    --version=*)
      version="${1#*=}"
      shift
      ;;
    --targets|-Targets)
      targets="${2:?--targets requires a value}"
      shift 2
      ;;
    --targets=*)
      targets="${1#*=}"
      shift
      ;;
    --skip-tests|-SkipTests)
      skip_tests=1
      shift
      ;;
    --clean|-Clean)
      clean=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      printf 'Unknown argument: %s\n' "$1" >&2
      usage >&2
      exit 64
      ;;
  esac
done

if [[ "$clean" -eq 1 ]]; then
  rm -rf "$packages_dir"
fi
mkdir -p "$packages_dir" "$staging_dir"

if [[ -z "$version" ]]; then
  if version="$(git -C "$root_dir" describe --tags --always --dirty 2>/dev/null)"; then
    :
  else
    version="$(date -u +%Y%m%d%H%M%S)"
  fi
fi
version="${version//\//-}"
version="${version// /-}"

run_checks_and_frontend_build() {
  if [[ "$skip_tests" -eq 0 ]]; then
    printf '[palpanel] Running backend tests\n'
    (cd "$root_dir/backend" && go test ./...)
    printf '[palpanel] Running sav-cli tests\n'
    (cd "$root_dir/sav-cli" && go test ./...)
    printf '[palpanel] Installing frontend dependencies\n'
    (cd "$root_dir/frontend" && npm ci)
    printf '[palpanel] Running frontend check\n'
    (cd "$root_dir/frontend" && npm run check)
  else
    printf '[palpanel] Skipping tests and checks\n'
    printf '[palpanel] Installing frontend dependencies\n'
    (cd "$root_dir/frontend" && npm ci)
    printf '[palpanel] Building frontend\n'
    (cd "$root_dir/frontend" && npm run build)
  fi
}

copy_common_files() {
  local pkg_dir="$1"

  mkdir -p "$pkg_dir/bin" "$pkg_dir/config" "$pkg_dir/scripts" "$pkg_dir/frontend" "$pkg_dir/backend/deployments"
  cp -R "$root_dir/frontend/dist" "$pkg_dir/frontend/dist"
  cp -R "$root_dir/backend/deployments/wine-runner" "$pkg_dir/backend/deployments/wine-runner"
  cp "$root_dir/scripts/palpanel.env.example" "$pkg_dir/config/palpanel.env.example"
  cp "$root_dir/scripts/package-README.md" "$pkg_dir/README.md"
  cp "$root_dir/scripts/start.sh" "$pkg_dir/scripts/start.sh"
  cp "$root_dir/scripts/start.ps1" "$pkg_dir/scripts/start.ps1"
  chmod +x "$pkg_dir/scripts/start.sh"
}

build_target() {
  local target="$1"
  local goos="${target%-*}"
  local goarch="${target#*-}"
  local exe_ext=""
  local archive=""

  if [[ "$target" != "$goos-$goarch" || -z "$goos" || -z "$goarch" ]]; then
    printf 'Invalid target: %s\n' "$target" >&2
    exit 64
  fi
  if [[ "$goos" == "windows" ]]; then
    exe_ext=".exe"
  fi

  local pkg_name="palpanel_${version}_${goos}_${goarch}"
  local pkg_dir="$staging_dir/$pkg_name"
  rm -rf "$pkg_dir"
  copy_common_files "$pkg_dir"

  printf '[palpanel] Building backend for %s\n' "$target"
  (cd "$root_dir/backend" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -o "$pkg_dir/bin/palpanel$exe_ext" ./cmd/palpanel)
  printf '[palpanel] Building sav-cli for %s\n' "$target"
  (cd "$root_dir/sav-cli" && CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build -trimpath -o "$pkg_dir/bin/sav-cli$exe_ext" ./cmd/sav_cli)

  (cd "$pkg_dir" && find . -type f ! -name checksums.txt -print0 | sort -z | xargs -0 sha256sum >checksums.txt)

  if [[ "$goos" == "windows" ]]; then
    archive="$packages_dir/$pkg_name.zip"
    rm -f "$archive"
    if command -v zip >/dev/null 2>&1; then
      (cd "$staging_dir" && zip -qr "$archive" "$pkg_name")
    elif command -v python3 >/dev/null 2>&1; then
      (cd "$staging_dir" && python3 -m zipfile -c "$archive" "$pkg_name")
    else
      printf 'zip or python3 is required to create %s\n' "$archive" >&2
      exit 69
    fi
  else
    archive="$packages_dir/$pkg_name.tar.gz"
    rm -f "$archive"
    tar -czf "$archive" -C "$staging_dir" "$pkg_name"
  fi

  printf '[palpanel] Wrote %s\n' "$archive"
}

run_checks_and_frontend_build

IFS=',' read -r -a target_list <<<"$targets"
for target in "${target_list[@]}"; do
  target="${target//[[:space:]]/}"
  if [[ -n "$target" ]]; then
    build_target "$target"
  fi
done

rm -rf "$staging_dir"
