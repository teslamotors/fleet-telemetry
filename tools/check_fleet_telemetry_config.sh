#!/bin/bash
# Validates Fleet API fleet_telemetry_config responses so multi-app limit
# rejections are not mistaken for success (see GitHub issue #294).
#
# CREATE can return updated_vehicles >= 1 even when the vehicle rejected the
# config because too many apps already have telemetry configured. The GET
# response is authoritative: synced=true with config=null (or limit_reached=true)
# means the configuration was not applied.
#
# Usage:
#   ./check_fleet_telemetry_config.sh get path/to/get_response.json
#   ./check_fleet_telemetry_config.sh create path/to/create_response.json
#
# Responses may be either the raw Fleet API envelope:
#   {"response": {...}}
# or the inner response object itself.

set -euo pipefail

c_red="\033[0;31m"
c_green="\033[0;32m"
c_yellow="\033[0;33m"
c_reset="\033[0m"

success() { echo -e "${c_green}$1${c_reset}"; }
warning() { echo -e "${c_yellow}$1${c_reset}"; }
error() { echo -e "${c_red}$1${c_reset}"; }

die() {
  >&2 error "$*"
  exit 1
}

usage() {
  cat <<'EOF'
Usage:
  check_fleet_telemetry_config.sh get <get_response.json>
  check_fleet_telemetry_config.sh create <create_response.json>

get    Validate fleet_telemetry_config GET (authoritative acceptance check)
create Validate fleet_telemetry_config CREATE for max_configs skips
EOF
  exit 2
}

require_jq() {
  command -v jq >/dev/null 2>&1 || die "jq is required (https://jqlang.github.io/jq/)"
}

# Normalize to the inner response object whether or not an envelope is present.
response_body() {
  local file=$1
  jq -c 'if has("response") then .response else . end' "$file"
}

validate_get() {
  local file=$1
  local body
  body=$(response_body "$file")

  local synced config_null limit_reached
  synced=$(jq -r '.synced // false' <<<"$body")
  config_null=$(jq -r '.config == null' <<<"$body")
  limit_reached=$(jq -r '.limit_reached // false' <<<"$body")

  echo "synced=$synced config_null=$config_null limit_reached=$limit_reached"

  if [[ "$limit_reached" == "true" ]]; then
    die "limit_reached=true: vehicle is at the max number of telemetry apps. Free a slot (revoke another app / delete its config), then CREATE again — a previous rejected config is not applied automatically."
  fi

  if [[ "$config_null" == "true" ]]; then
    die "config is null: configuration was not applied for this app. Common cause is the multi-app telemetry limit (see #294). Do not treat synced=$synced alone as success."
  fi

  if [[ "$synced" != "true" ]]; then
    warning "synced is not true yet; vehicle has not adopted the target config. Retry GET until synced=true with a non-null config."
    exit 3
  fi

  success "fleet_telemetry_config GET looks applied (synced=true, config present)."
}

validate_create() {
  local file=$1
  local body
  body=$(response_body "$file")

  local updated
  updated=$(jq -r '.updated_vehicles // 0' <<<"$body")

  local max_configs_count
  max_configs_count=$(jq -r '(.skipped_vehicles.max_configs // []) | length' <<<"$body")

  echo "updated_vehicles=$updated skipped_max_configs=$max_configs_count"

  if [[ "$max_configs_count" != "0" ]]; then
    local vins
    vins=$(jq -r '(.skipped_vehicles.max_configs // []) | join(", ")' <<<"$body")
    die "CREATE skipped VIN(s) with max_configs: $vins. Vehicle already has the maximum number of telemetry configurations."
  fi

  if [[ "$updated" == "0" || "$updated" == "null" ]]; then
    die "CREATE updated_vehicles=$updated; no vehicles were configured. Inspect skipped_vehicles in the response."
  fi

  warning "CREATE reports updated_vehicles=$updated, but this alone is not proof of success."
  warning "A vehicle at the app limit can still look updated while GET returns synced=true with config=null."
  warning "Always run: $0 get <get_response.json>"
  success "CREATE response has no max_configs skips; confirm with GET before relying on streaming."
}

require_jq

MODE=${1:-}
FILE=${2:-}

[[ -n "$MODE" && -n "$FILE" ]] || usage
[[ -f "$FILE" ]] || die "file not found: $FILE"

case "$MODE" in
  get) validate_get "$FILE" ;;
  create) validate_create "$FILE" ;;
  *) usage ;;
esac
