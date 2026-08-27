#!/usr/bin/env bash
set -euo pipefail

compose_file="${1:-}"
tmpdir=""
expected_image="${CPA_USAGE_IMAGE:-}"

if [[ -z "$compose_file" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  compose_file="$tmpdir/cpa-usage.compose.yml"
  expected_image="ghcr.io/evenluo/cpa-usage:v0.0.0-rc.1"
  CPA_USAGE_IMAGE="$expected_image" scripts/render-dokploy-compose.sh "v0.0.0-rc.1" "$compose_file"
fi

scripts/verify-dokploy-compose-static.sh "$compose_file"

if [[ -z "$expected_image" ]]; then
  echo "CPA_USAGE_IMAGE is required when verifying an existing Compose file" >&2
  exit 2
fi

if ! command -v docker >/dev/null 2>&1; then
  echo "docker is required for canonical Dokploy Compose verification" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "jq is required for canonical Dokploy Compose verification" >&2
  exit 2
fi

config_json="$(env \
  "PUBLIC_HOST=example.com" \
  "MANAGEMENT_PASSWORD=example-management-password" \
  "CPA_USAGE_LOGIN_PASSWORD=example-login-password" \
  docker compose -f "$compose_file" config --format json)"

if ! jq -e --arg expected "$expected_image" '
  (.services | type == "object") and
  ((.services | keys) == ["cpa-usage"]) and
  (.services["cpa-usage"].image == $expected)
' >/dev/null <<<"$config_json"; then
  echo "canonical Compose must contain only services[\"cpa-usage\"] with the exact expected image" >&2
  exit 1
fi

echo "OK canonical Dokploy compose"
