#!/usr/bin/env bash
set -euo pipefail

input="${1:?usage: smoke-palcalc-no-icu.sh <linux-archive-or-extracted-package>}"
tmp=""
if [[ -f "$input" ]]; then
  tmp="$(mktemp -d)"
  tar -xzf "$input" -C "$tmp"
  package_dir="$(find "$tmp" -mindepth 1 -maxdepth 1 -type d -print -quit)"
else
  package_dir="$input"
fi
package_dir="$(cd -- "$package_dir" && pwd -P)"
[[ -x "$package_dir/bin/palcalc-bridge" ]] || { printf 'palcalc-bridge not found in %s\n' "$package_dir" >&2; exit 1; }
command -v docker >/dev/null 2>&1 || { printf 'docker is required\n' >&2; exit 69; }

port="$((42000 + RANDOM % 10000))"
cid=""
cleanup() {
  [[ -z "$cid" ]] || docker rm -f "$cid" >/dev/null 2>&1 || true
  [[ -z "$tmp" ]] || rm -rf "$tmp"
}
trap cleanup EXIT

cid="$(docker run -d --rm --network host \
  -e DOTNET_SYSTEM_GLOBALIZATION_INVARIANT=1 \
  -e "PALCALC_BRIDGE_URLS=http://127.0.0.1:$port" \
  -e PALCALC_BRIDGE_CONCURRENCY=1 \
  -v "$package_dir:/app:ro" \
  --entrypoint /app/bin/palcalc-bridge \
  mcr.microsoft.com/dotnet/runtime-deps:9.0-noble-chiseled)"

for _ in $(seq 1 40); do
  curl --fail --silent "http://127.0.0.1:$port/health" >/dev/null 2>&1 && break
  docker inspect "$cid" >/dev/null 2>&1 || { printf 'PalCalc exited in the no-ICU container\n' >&2; exit 1; }
  sleep 0.25
done
curl --fail --silent "http://127.0.0.1:$port/health" | grep -F '"status":"ok"'
curl --fail --silent --header 'Content-Type: application/json' \
  --data '{"request_id":"no-icu-smoke","save_fingerprint":"smoke","owned_pals":[{"instance_id":"smoke-anubis","pal_id":"Anubis","nickname":"测试阿努比斯","level":50,"owner_player_id":"smoke","gender":"male","passives":[],"rank":1,"iv_hp":0,"iv_attack":0,"iv_defense":0,"container_id":"smoke-box","slot_index":0,"location_type":"palbox"}],"target":{"pal_id":"Anubis","gender":"wildcard","required_passives":[],"optional_passives":[],"iv_hp":0,"iv_attack":0,"iv_defense":0},"settings":{"max_breeding_steps":1,"max_solver_iterations":1,"max_wild_pals":0,"max_input_irrelevant_passives":0,"max_bred_irrelevant_passives":0,"max_threads":1,"max_gold_cost":0,"use_gender_reversers":false},"game_settings":{"breeding_time_seconds":1,"massive_egg_incubation_minutes":1,"multiple_breeding_farms":true,"multiple_incubators":true},"result_limit":1}' \
  "http://127.0.0.1:$port/v1/jobs" | grep -F '"id":"no-icu-smoke"'
for _ in $(seq 1 30); do
  job="$(curl --fail --silent "http://127.0.0.1:$port/v1/jobs/no-icu-smoke")"
  grep -Fq '"status":"completed"' <<<"$job" && break
  grep -Eq '"status":"(failed|canceled)"' <<<"$job" && { printf 'PalCalc solve failed: %s\n' "$job" >&2; exit 1; }
  sleep 1
done
curl --fail --silent "http://127.0.0.1:$port/v1/jobs/no-icu-smoke/result" | grep -F '"pal_id":"Anubis"'
printf 'PalCalc no-ICU smoke test passed\n'
