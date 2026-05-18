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
require_env DOKPLOY_COMPOSE_ID

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

base_url="${DOKPLOY_URL%/}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

compose_file="$tmpdir/cpa-cliproxyapi.compose.yml"
scripts/render-dokploy-compose.sh "${CPA_USAGE_VERSION:-}" "$compose_file"
scripts/verify-dokploy-compose.sh "$compose_file"

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

current_json="$tmpdir/compose-one.json"
api_get "compose.one?composeId=$DOKPLOY_COMPOSE_ID" > "$current_json"

jq -r '.env // ""' "$current_json" > "$tmpdir/current.env"
migrate_env_file "$tmpdir/current.env" "$tmpdir/migrated.env"
if ! cmp -s "$tmpdir/current.env" "$tmpdir/migrated.env"; then
  jq -n \
    --arg composeId "$DOKPLOY_COMPOSE_ID" \
    --rawfile env "$tmpdir/migrated.env" \
    '{composeId: $composeId, env: $env}' > "$tmpdir/save-env.json"
  api_post "compose.saveEnvironment" "$tmpdir/save-env.json" >/dev/null
  echo "OK migrated Dokploy env from KEEPER_LOGIN_PASSWORD to CPA_USAGE_LOGIN_PASSWORD"
else
  echo "OK Dokploy env did not require login password migration"
fi

jq -n \
  --arg composeId "$DOKPLOY_COMPOSE_ID" \
  --rawfile composeFile "$compose_file" \
  '{
    composeId: $composeId,
    sourceType: "raw",
    composeType: "docker-compose",
    composeFile: $composeFile
  }' > "$tmpdir/update-compose.json"
api_post "compose.update" "$tmpdir/update-compose.json" >/dev/null
echo "OK updated Dokploy compose"

api_get "compose.getConvertedCompose?composeId=$DOKPLOY_COMPOSE_ID" >/dev/null
echo "OK Dokploy converted compose"

jq -n --arg composeId "$DOKPLOY_COMPOSE_ID" '{composeId: $composeId}' > "$tmpdir/deploy.json"
api_post "compose.deploy" "$tmpdir/deploy.json" >/dev/null
echo "OK triggered Dokploy deployment"
