#!/usr/bin/env bash
set -euo pipefail

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "$name is required" >&2
    exit 2
  fi
}

require_env DOKPLOY_URL
require_env DOKPLOY_API_KEY

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

source_compose_id="${DOKPLOY_SOURCE_COMPOSE_ID:-bqmnXzfYoIuSln9Ndbx1x}"
cpa_usage_compose_id="${DOKPLOY_CPA_USAGE_COMPOSE_ID:-}"
cpa_usage_compose_name="${DOKPLOY_CPA_USAGE_COMPOSE_NAME:-cpa-usage}"
base_url="${DOKPLOY_URL%/}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

cpa_usage_compose_file="$tmpdir/cpa-usage.compose.yml"
CPA_USAGE_VERSION="${CPA_USAGE_VERSION:-v0.1.2}" scripts/render-dokploy-compose.sh "${CPA_USAGE_VERSION:-v0.1.2}" "$cpa_usage_compose_file"
scripts/verify-dokploy-compose.sh "$cpa_usage_compose_file"

api_get() {
  local path="$1"
  curl -fsS \
    -H "x-api-key: $DOKPLOY_API_KEY" \
    "$base_url/api/$path"
}

api_post() {
  local path="$1"
  local payload="$2"
  curl -fsS \
    -X POST \
    -H "x-api-key: $DOKPLOY_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary "@$payload" \
    "$base_url/api/$path"
}

migrate_env_file() {
  local input="$1"
  local output="$2"
  awk '
    BEGIN {
      old_key = "KEEPER_LOGIN_PASSWORD"
      new_key = "CPA_USAGE_LOGIN_PASSWORD"
      has_new = 0
      old_value = ""
    }
    /^[[:space:]]*#/ || /^[[:space:]]*$/ {
      lines[++n] = $0
      next
    }
    {
      key = $0
      sub(/=.*/, "", key)
      sub(/^[[:space:]]*export[[:space:]]+/, "", key)
      sub(/[[:space:]]+$/, "", key)
      if (key == new_key) {
        has_new = 1
      }
      if (key == old_key) {
        old_value = substr($0, index($0, "=") + 1)
        next
      }
      lines[++n] = $0
    }
    END {
      for (i = 1; i <= n; i++) {
        print lines[i]
      }
      if (!has_new && old_value != "") {
        print new_key "=" old_value
      }
    }
  ' "$input" > "$output"
}

source_json="$tmpdir/source-compose.json"
api_get "compose.one?composeId=$source_compose_id" > "$source_json"

environment_id="$(jq -r '.environmentId // empty' "$source_json")"
if [[ -z "$environment_id" ]]; then
  echo "source compose did not expose environmentId" >&2
  exit 1
fi

jq -r '.env // ""' "$source_json" > "$tmpdir/source.env"
migrate_env_file "$tmpdir/source.env" "$tmpdir/cpa-usage.env"

if [[ -z "$cpa_usage_compose_id" ]]; then
  jq -n \
    --arg name "$cpa_usage_compose_name" \
    --arg appName "$cpa_usage_compose_name" \
    --arg environmentId "$environment_id" \
    --rawfile composeFile "$cpa_usage_compose_file" \
    '{
      name: $name,
      appName: $appName,
      environmentId: $environmentId,
      composeType: "docker-compose",
      composeFile: $composeFile
    }' > "$tmpdir/create-cpa-usage.json"

  api_post "compose.create" "$tmpdir/create-cpa-usage.json" > "$tmpdir/create-response.json"
  cpa_usage_compose_id="$(jq -r '
    .composeId
    // .id
    // .compose.composeId
    // .compose.id
    // .data.composeId
    // .data.id
    // empty
  ' "$tmpdir/create-response.json")"

  if [[ -z "$cpa_usage_compose_id" ]]; then
    echo "compose.create did not return a compose id; set DOKPLOY_CPA_USAGE_COMPOSE_ID and rerun" >&2
    exit 1
  fi
fi

jq -n \
  --arg composeId "$cpa_usage_compose_id" \
  --rawfile env "$tmpdir/cpa-usage.env" \
  '{composeId: $composeId, env: $env}' > "$tmpdir/save-cpa-usage-env.json"
api_post "compose.saveEnvironment" "$tmpdir/save-cpa-usage-env.json" >/dev/null
echo "OK saved cpa-usage Dokploy environment"

jq -n \
  --arg composeId "$cpa_usage_compose_id" \
  --rawfile composeFile "$cpa_usage_compose_file" \
  '{
    composeId: $composeId,
    sourceType: "raw",
    composeType: "docker-compose",
    composeFile: $composeFile
  }' > "$tmpdir/update-cpa-usage-compose.json"
api_post "compose.update" "$tmpdir/update-cpa-usage-compose.json" >/dev/null
api_get "compose.getConvertedCompose?composeId=$cpa_usage_compose_id" >/dev/null
echo "OK updated cpa-usage Dokploy compose"

jq -n \
  --arg composeId "$source_compose_id" \
  --rawfile composeFile "deploy/dokploy/cpa-cliproxyapi.compose.yml" \
  '{
    composeId: $composeId,
    sourceType: "raw",
    composeType: "docker-compose",
    composeFile: $composeFile
  }' > "$tmpdir/update-source-compose.json"
api_post "compose.update" "$tmpdir/update-source-compose.json" >/dev/null
api_get "compose.getConvertedCompose?composeId=$source_compose_id" >/dev/null
echo "OK updated source Dokploy compose without cpa-usage"

echo "DOKPLOY_CPA_USAGE_COMPOSE_ID=$cpa_usage_compose_id"
