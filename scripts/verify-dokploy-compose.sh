#!/usr/bin/env bash
set -euo pipefail

compose_file="${1:-}"
tmpdir=""

if [[ -z "$compose_file" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  compose_file="$tmpdir/cpa-cliproxyapi.compose.yml"
  scripts/render-dokploy-compose.sh "v0.0.0-rc.1" "$compose_file"
fi

if [[ ! -f "$compose_file" ]]; then
  echo "compose file not found: $compose_file" >&2
  exit 2
fi

for forbidden in "cpa-usage-keeper" "KEEPER_LOGIN_PASSWORD" ":latest"; do
  if grep -q -- "$forbidden" "$compose_file"; then
    echo "rendered compose contains forbidden token: $forbidden" >&2
    exit 1
  fi
done

if ! command -v docker >/dev/null 2>&1; then
  echo "docker not found; skipped docker compose config validation" >&2
  exit 0
fi

env \
  "PUBLIC_HOST=example.com" \
  "POSTGRES_IMAGE=postgres:16-alpine" \
  "POSTGRES_DB=cliproxyapi" \
  "POSTGRES_USER=cliproxyapi" \
  "POSTGRES_PASSWORD=example-postgres-password" \
  "POSTGRES_DATA_VOLUME=cliproxyapi-postgres-data" \
  "CLIPROXYAPI_IMAGE=eceasy/cli-proxy-api:v7.1.0" \
  "CLIPROXYAPI_PGSTORE_DSN=postgres://cliproxyapi:example-postgres-password@postgres:5432/cliproxyapi?sslmode=disable" \
  "CLIPROXYAPI_CONFIG_PATH=/opt/cliproxyapi/config.yaml" \
  "CLIPROXYAPI_AUTH_PATH=/opt/cliproxyapi/auths" \
  "CLIPROXYAPI_LOG_PATH=/opt/cliproxyapi/logs" \
  "MANAGEMENT_PASSWORD=example-management-password" \
  "CPA_USAGE_LOGIN_PASSWORD=example-login-password" \
  "AUTH_SESSION_SECRET=0123456789abcdef0123456789abcdef" \
  docker compose -f "$compose_file" config >/dev/null

echo "OK dokploy compose"
