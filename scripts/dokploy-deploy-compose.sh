#!/usr/bin/env bash
set -euo pipefail

# This command has no cross-process lock or Dokploy CAS input. Its caller must
# provide single-writer serialization for the target Compose ID.

require_env() {
  local name="$1"
  if [[ -z "${!name:-}" ]]; then
    echo "$name is required" >&2
    exit 2
  fi
}

require_env DOKPLOY_URL
require_env DOKPLOY_API_KEY
require_env DOKPLOY_CPA_USAGE_COMPOSE_ID
require_env DOKPLOY_CPA_USAGE_HEALTH_URL
require_env CPA_USAGE_IMAGE
require_env CPA_USAGE_RELEASE_ID

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "curl is required" >&2
  exit 2
fi

base_url="${DOKPLOY_URL%/}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

compose_id="$DOKPLOY_CPA_USAGE_COMPOSE_ID"
health_url="$DOKPLOY_CPA_USAGE_HEALTH_URL"
release_id="$CPA_USAGE_RELEASE_ID"
poll_attempts="${DOKPLOY_DEPLOY_POLL_ATTEMPTS:-60}"
poll_interval="${DOKPLOY_DEPLOY_POLL_INTERVAL_SECONDS:-5}"
health_attempts="${DOKPLOY_HEALTH_ATTEMPTS:-12}"
health_interval="${DOKPLOY_HEALTH_INTERVAL_SECONDS:-5}"
request_timeout="${DOKPLOY_REQUEST_TIMEOUT_SECONDS:-20}"

for value_name in poll_attempts health_attempts request_timeout; do
  value="${!value_name}"
  if [[ ! "$value" =~ ^[1-9][0-9]*$ ]]; then
    echo "$value_name must be a positive integer" >&2
    exit 2
  fi
done

for value_name in poll_interval health_interval; do
  value="${!value_name}"
  if [[ ! "$value" =~ ^[0-9]+$ ]]; then
    echo "$value_name must be a non-negative integer" >&2
    exit 2
  fi
done

if [[ ! "$health_url" =~ ^https://[^/?#]+/usage/healthz$ ]]; then
  echo "DOKPLOY_CPA_USAGE_HEALTH_URL must be an HTTPS /usage/healthz URL" >&2
  exit 2
fi

compose_file="$tmpdir/cpa-usage.compose.yml"
scripts/render-dokploy-compose.sh "${CPA_USAGE_VERSION:-}" "$compose_file"
scripts/verify-dokploy-compose.sh "$compose_file"

api_get() {
  local path="$1"
  curl -fsS --max-time "$request_timeout" \
    -H "x-api-key: $DOKPLOY_API_KEY" \
    "$base_url/api/$path"
}

api_post() {
  local path="$1"
  local payload="$2"
  curl -fsS --max-time "$request_timeout" \
    -X POST \
    -H "x-api-key: $DOKPLOY_API_KEY" \
    -H "Content-Type: application/json" \
    --data-binary "@$payload" \
    "$base_url/api/$path"
}

validate_deployments_response() {
  local input="$1"
  jq -e --arg composeId "$compose_id" '
    type == "array" and
    all(.[];
      (.deploymentId | type == "string" and length > 0) and
      (has("description") and (.description == null or (.description | type == "string"))) and
      (.composeId == $composeId) and
      (.status == "running" or .status == "done" or .status == "error" or .status == "cancelled")
    )
  ' "$input" >/dev/null
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

# All Dokploy reads in this block are preflight. No mutation is allowed before
# the local contract, API response shape, and release marker are proven usable.
current_json="$tmpdir/compose-one.json"
api_get "compose.one?composeId=$compose_id" > "$current_json"
if ! jq -e '
  type == "object" and
  ((.env // "") | type == "string")
' "$current_json" >/dev/null; then
  echo "unsupported Dokploy compose.one response" >&2
  exit 1
fi

deployments_json="$tmpdir/deployments.json"
api_get "deployment.allByCompose?composeId=$compose_id" > "$deployments_json"
if ! validate_deployments_response "$deployments_json"; then
  echo "unsupported Dokploy deployment.allByCompose response" >&2
  exit 1
fi

existing_marker_count="$(jq --arg marker "$release_id" '[.[] | select(.description == $marker)] | length' "$deployments_json")"
if [[ "$existing_marker_count" != "0" ]]; then
  echo "CPA_USAGE_RELEASE_ID is not unique in Dokploy deployment history" >&2
  exit 1
fi

jq -r '.env // ""' "$current_json" > "$tmpdir/current.env"
migrate_env_file "$tmpdir/current.env" "$tmpdir/migrated.env"
if ! cmp -s "$tmpdir/current.env" "$tmpdir/migrated.env"; then
  jq -n \
    --arg composeId "$compose_id" \
    --rawfile env "$tmpdir/migrated.env" \
    '{composeId: $composeId, env: $env}' > "$tmpdir/save-env.json"
  api_post "compose.saveEnvironment" "$tmpdir/save-env.json" >/dev/null
  echo "OK migrated Dokploy env from KEEPER_LOGIN_PASSWORD to CPA_USAGE_LOGIN_PASSWORD"
else
  echo "OK Dokploy env did not require login password migration"
fi

jq -n \
  --arg composeId "$compose_id" \
  --rawfile composeFile "$compose_file" \
  '{
    composeId: $composeId,
    sourceType: "raw",
    composeType: "docker-compose",
    composeFile: $composeFile
  }' > "$tmpdir/update-compose.json"
api_post "compose.update" "$tmpdir/update-compose.json" >/dev/null
echo "OK updated Dokploy compose"

api_get "compose.getConvertedCompose?composeId=$compose_id" > "$tmpdir/converted-compose.json"
if ! jq -er 'if type == "string" then . else error("expected Compose YAML string") end' \
  "$tmpdir/converted-compose.json" > "$tmpdir/converted-compose.yml"; then
  echo "unsupported Dokploy compose.getConvertedCompose response" >&2
  exit 1
fi
scripts/verify-dokploy-compose.sh "$tmpdir/converted-compose.yml"
echo "OK exact Dokploy converted compose image"

jq -n \
  --arg composeId "$compose_id" \
  --arg title "CPA Usage ${CPA_USAGE_IMAGE##*:}" \
  --arg description "$release_id" \
  '{composeId: $composeId, title: $title, description: $description}' > "$tmpdir/deploy.json"
api_post "compose.deploy" "$tmpdir/deploy.json" >/dev/null
echo "OK triggered Dokploy deployment"

deployment_done=false
for ((attempt = 1; attempt <= poll_attempts; attempt++)); do
  api_get "deployment.allByCompose?composeId=$compose_id" > "$deployments_json"
  if ! validate_deployments_response "$deployments_json"; then
    echo "unsupported Dokploy deployment.allByCompose response" >&2
    exit 1
  fi

  marker_count="$(jq --arg marker "$release_id" '[.[] | select(.description == $marker)] | length' "$deployments_json")"
  if [[ "$marker_count" == "0" ]]; then
    if ((attempt < poll_attempts)); then
      sleep "$poll_interval"
    fi
    continue
  fi
  if [[ "$marker_count" != "1" ]]; then
    echo "unsupported Dokploy deployment correlation: release marker is ambiguous" >&2
    exit 1
  fi

  deployment_status="$(jq -r --arg marker "$release_id" '.[] | select(.description == $marker) | .status' "$deployments_json")"
  case "$deployment_status" in
    running)
      if ((attempt < poll_attempts)); then
        sleep "$poll_interval"
      fi
      ;;
    done)
      deployment_done=true
      break
      ;;
    error | cancelled)
      echo "Dokploy deployment reached terminal failure: $deployment_status" >&2
      exit 1
      ;;
    *)
      echo "unsupported Dokploy deployment status" >&2
      exit 1
      ;;
  esac
done

if [[ "$deployment_done" != "true" ]]; then
  echo "Dokploy deployment did not reach terminal success before timeout" >&2
  exit 1
fi
echo "OK Dokploy deployment reached terminal success"

health_ok=false
for ((attempt = 1; attempt <= health_attempts; attempt++)); do
  health_code="$(curl -sS --max-time "$request_timeout" \
    -o "$tmpdir/health.json" \
    -w '%{http_code}' \
    "$health_url" || true)"
  if [[ "$health_code" == "200" ]] && jq -e 'type == "object" and .status == "ok"' "$tmpdir/health.json" >/dev/null 2>&1; then
    health_ok=true
    break
  fi
  if ((attempt < health_attempts)); then
    sleep "$health_interval"
  fi
done

if [[ "$health_ok" != "true" ]]; then
  echo "deployed /usage/healthz did not return HTTP 200 with status=ok before timeout" >&2
  exit 1
fi

echo "OK Dokploy release exact image, terminal deployment, and health"
