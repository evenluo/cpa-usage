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
require_env DOKPLOY_CPA_USAGE_COMPOSE_ID

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required" >&2
  exit 2
fi

base_url="${DOKPLOY_URL%/}"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

compose_id="$DOKPLOY_CPA_USAGE_COMPOSE_ID"
compose_file="$tmpdir/cpa-usage.compose.yml"
scripts/render-dokploy-compose.sh "${CPA_USAGE_VERSION:-}" "$compose_file"
scripts/verify-dokploy-compose.sh "$compose_file"
deployment_title="${DOKPLOY_DEPLOYMENT_TITLE:-cpa-usage-$(date -u +%Y%m%dT%H%M%SZ)-$$}"
deployment_timeout_seconds="${DOKPLOY_DEPLOYMENT_TIMEOUT_SECONDS:-600}"
deployment_poll_seconds="${DOKPLOY_DEPLOYMENT_POLL_SECONDS:-5}"

if [[ ! "$deployment_timeout_seconds" =~ ^[1-9][0-9]*$ ]]; then
  echo "DOKPLOY_DEPLOYMENT_TIMEOUT_SECONDS must be a positive integer" >&2
  exit 2
fi
if [[ ! "$deployment_poll_seconds" =~ ^[1-9][0-9]*$ ]]; then
  echo "DOKPLOY_DEPLOYMENT_POLL_SECONDS must be a positive integer" >&2
  exit 2
fi

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

read_env_value() {
  local name="$1"
  local file="$2"
  local value
  value="$(awk -v wanted="$name" '
    /^[[:space:]]*#/ || /^[[:space:]]*$/ { next }
    {
      line = $0
      sub(/^[[:space:]]*export[[:space:]]+/, "", line)
      key = line
      sub(/=.*/, "", key)
      sub(/[[:space:]]+$/, "", key)
      if (key == wanted) {
        print substr(line, index(line, "=") + 1)
        exit
      }
    }
  ' "$file")"
  if [[ "$value" == \"*\" && "$value" == *\" ]]; then
    value="${value:1:${#value}-2}"
  elif [[ "$value" == \'*\' && "$value" == *\' ]]; then
    value="${value:1:${#value}-2}"
  fi
  printf '%s' "$value"
}

read_compose_service_image() {
  local file="$1"
  local service="$2"
  awk -v wanted="$service" '
    /^services:[[:space:]]*$/ {
      in_services = 1
      next
    }
    in_services && /^[^[:space:]]/ { exit }
    in_services && $0 ~ "^  " wanted ":[[:space:]]*$" {
      in_service = 1
      next
    }
    in_service && /^  [^[:space:]][^:]*:[[:space:]]*$/ { exit }
    in_service && /^    image:[[:space:]]*/ {
      value = $0
      sub(/^    image:[[:space:]]*/, "", value)
      sub(/[[:space:]]+#.*$/, "", value)
      if ((substr(value, 1, 1) == "\"" && substr(value, length(value), 1) == "\"") ||
          (substr(value, 1, 1) == "\047" && substr(value, length(value), 1) == "\047")) {
        value = substr(value, 2, length(value) - 2)
      }
      print value
      exit
    }
  ' "$file"
}

wait_for_deployment() {
  local deadline=$(( $(date +%s) + deployment_timeout_seconds ))
  local deployments_file="$tmpdir/deployments.json"
  local deployment_json status error_message

  while (( $(date +%s) < deadline )); do
    api_get "deployment.allByCompose?composeId=$compose_id" > "$deployments_file"
    if ! jq -e 'type == "array"' "$deployments_file" >/dev/null; then
      echo "Dokploy deployment list returned an unexpected payload" >&2
      exit 1
    fi
    deployment_json="$(jq -c --arg title "$deployment_title" 'map(select(.title == $title)) | sort_by(.createdAt) | last // empty' "$deployments_file")"
    if [[ -z "$deployment_json" ]]; then
      sleep "$deployment_poll_seconds"
      continue
    fi

    status="$(jq -r '.status // ""' <<<"$deployment_json")"
    case "$status" in
      done)
        echo "OK Dokploy deployment completed"
        return 0
        ;;
      error|cancelled)
        error_message="$(jq -r '.errorMessage // "no error message"' <<<"$deployment_json")"
        echo "Dokploy deployment $status: $error_message" >&2
        return 1
        ;;
      running)
        sleep "$deployment_poll_seconds"
        ;;
      *)
        echo "Dokploy deployment returned unknown status: ${status:-missing}" >&2
        return 1
        ;;
    esac
  done

  echo "Dokploy deployment did not complete within ${deployment_timeout_seconds}s" >&2
  return 1
}

current_json="$tmpdir/compose-one.json"
api_get "compose.one?composeId=$compose_id" > "$current_json"

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

converted_compose_response="$tmpdir/converted-compose.json"
converted_compose_file="$tmpdir/converted-compose.yml"
api_get "compose.getConvertedCompose?composeId=$compose_id" > "$converted_compose_response"
if ! jq -er 'if type == "string" then . else error("expected converted Compose YAML string") end' "$converted_compose_response" > "$converted_compose_file"; then
  echo "Dokploy converted compose returned an unexpected payload" >&2
  exit 1
fi
expected_image="$(read_compose_service_image "$compose_file" "cpa-usage")"
converted_image="$(read_compose_service_image "$converted_compose_file" "cpa-usage")"
if [[ -z "$expected_image" || -z "$converted_image" ]]; then
  echo "cpa-usage image is missing from the rendered or converted Compose" >&2
  exit 1
fi
if [[ "$converted_image" != "$expected_image" ]]; then
  echo "Dokploy converted compose image mismatch: expected $expected_image got $converted_image" >&2
  exit 1
fi
echo "OK Dokploy converted compose image $converted_image"

jq -n \
  --arg composeId "$compose_id" \
  --arg title "$deployment_title" \
  --arg description "${CPA_USAGE_IMAGE:-${CPA_USAGE_VERSION:-manual deployment}}" \
  '{composeId: $composeId, title: $title, description: $description}' > "$tmpdir/deploy.json"
api_post "compose.deploy" "$tmpdir/deploy.json" >/dev/null
echo "OK triggered Dokploy deployment"

wait_for_deployment

public_host="$(read_env_value PUBLIC_HOST "$tmpdir/migrated.env")"
login_password="$(read_env_value CPA_USAGE_LOGIN_PASSWORD "$tmpdir/migrated.env")"
if [[ -z "$public_host" ]]; then
  echo "PUBLIC_HOST is required in the Dokploy environment for release smoke" >&2
  exit 1
fi
if [[ -z "$login_password" ]]; then
  echo "CPA_USAGE_LOGIN_PASSWORD is required in the Dokploy environment for release smoke" >&2
  exit 1
fi
if [[ "$public_host" == http://* || "$public_host" == https://* ]]; then
  public_base_url="$public_host"
else
  public_base_url="https://$public_host"
fi
image_value="${CPA_USAGE_IMAGE:-}"
expected_version="${CPA_USAGE_VERSION:-${image_value##*:}}"
BASE_URL="$public_base_url" \
BASE_PATH="/usage" \
CPA_USAGE_LOGIN_PASSWORD="$login_password" \
EXPECTED_VERSION="$expected_version" \
EXPECTED_REVISION="${CPA_USAGE_EXPECTED_REVISION:-}" \
scripts/smoke-cpa-usage.sh
