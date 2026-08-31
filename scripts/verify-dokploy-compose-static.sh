#!/usr/bin/env bash
set -euo pipefail

compose_file="${1:-}"
tmpdir=""

if [[ -z "$compose_file" ]]; then
  tmpdir="$(mktemp -d)"
  trap 'rm -rf "$tmpdir"' EXIT
  compose_file="$tmpdir/cpa-usage.compose.yml"
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

for forbidden_service in "postgres" "cliproxyapi"; do
  if grep -Eq "^  ${forbidden_service}:$" "$compose_file"; then
    echo "rendered compose contains forbidden service: $forbidden_service" >&2
    exit 1
  fi
done

echo "OK static-only Dokploy compose checks"
