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
  "POSTGRES_PASSWORD=example-postgres-password" \
  "MANAGEMENT_PASSWORD=example-management-password" \
  "CPA_USAGE_LOGIN_PASSWORD=example-login-password" \
  docker compose -f "$compose_file" config >/dev/null

echo "OK dokploy compose"
