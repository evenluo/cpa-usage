#!/usr/bin/env bash
set -euo pipefail

BASE_URL="${BASE_URL:-http://127.0.0.1:8080}"
BASE_PATH="${BASE_PATH:-/usage}"
CPA_USAGE_LOGIN_PASSWORD="${CPA_USAGE_LOGIN_PASSWORD:-}"
EXPECT_KEEPER_STOPPED="${EXPECT_KEEPER_STOPPED:-false}"
CURL_INSECURE="${CURL_INSECURE:-false}"

curl_args=(-sS -L --max-time 20)
if [[ "$CURL_INSECURE" == "true" ]]; then
  curl_args+=(-k)
fi

tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT
cookie_jar="$tmpdir/cookies.txt"

json_escape() {
  printf '%s' "$1" | sed 's/\\/\\\\/g; s/"/\\"/g'
}

http_code() {
  local url="$1"
  curl "${curl_args[@]}" -o "$tmpdir/body" -w '%{http_code}' "$url" || true
}

require_code() {
  local label="$1"
  local url="$2"
  local expected="$3"
  local code
  code="$(http_code "$url")"
  if [[ "$code" != "$expected" ]]; then
    echo "FAIL $label expected HTTP $expected got $code" >&2
    sed -n '1,20p' "$tmpdir/body" >&2 || true
    exit 1
  fi
  echo "OK $label HTTP $code"
}

require_body_contains() {
  local label="$1"
  local pattern="$2"
  if ! grep -q "$pattern" "$tmpdir/body"; then
    echo "FAIL $label body missing pattern: $pattern" >&2
    sed -n '1,20p' "$tmpdir/body" >&2 || true
    exit 1
  fi
}

require_timed_json() {
  local label="$1"
  local path="$2"
  local pattern="$3"
  local output="$tmpdir/${label//[^A-Za-z0-9_.-]/_}.json"
  local metrics code duration
  metrics="$(curl "${curl_args[@]}" -b "$cookie_jar" -c "$cookie_jar" -o "$output" -w '%{http_code} %{time_total}' "$BASE_URL$BASE_PATH$path" || true)"
  code="${metrics%% *}"
  duration="${metrics#* }"
  if [[ "$code" != "200" ]]; then
    echo "FAIL $label HTTP $code time_total=${duration}s" >&2
    sed -n '1,20p' "$output" >&2 || true
    exit 1
  fi
  if ! grep -q "$pattern" "$output"; then
    echo "FAIL $label body missing pattern: $pattern" >&2
    sed -n '1,20p' "$output" >&2 || true
    exit 1
  fi
  echo "OK $label HTTP $code time_total=${duration}s"
}

require_code "cpa root" "$BASE_URL/" "200"
require_code "cpa-usage health" "$BASE_URL$BASE_PATH/healthz" "200"
require_body_contains "cpa-usage health" '"status":"ok"'
require_code "cpa-usage html" "$BASE_URL$BASE_PATH/" "200"
require_body_contains "cpa-usage html" '<html'

if [[ "$EXPECT_KEEPER_STOPPED" == "true" && "$BASE_PATH" != "/usage" ]]; then
  keeper_code="$(http_code "$BASE_URL/usage/healthz")"
  if [[ "$keeper_code" == "200" ]]; then
    echo "FAIL old keeper /usage/healthz is still HTTP 200" >&2
    exit 1
  fi
  echo "OK old keeper not serving /usage/healthz HTTP $keeper_code"
fi

session_code="$(curl "${curl_args[@]}" -b "$cookie_jar" -c "$cookie_jar" -o "$tmpdir/session-before" -w '%{http_code}' "$BASE_URL$BASE_PATH/api/v1/auth/session" || true)"
if [[ "$session_code" != "200" ]]; then
  echo "FAIL auth session before login HTTP $session_code" >&2
  sed -n '1,20p' "$tmpdir/session-before" >&2 || true
  exit 1
fi

if grep -q '"authenticated":true' "$tmpdir/session-before"; then
  echo "OK auth session already authenticated"
else
  if [[ -z "$CPA_USAGE_LOGIN_PASSWORD" ]]; then
    echo "FAIL CPA_USAGE_LOGIN_PASSWORD is required because session is not authenticated" >&2
    exit 1
  fi
  payload="{\"password\":\"$(json_escape "$CPA_USAGE_LOGIN_PASSWORD")\"}"
  login_code="$(curl "${curl_args[@]}" -b "$cookie_jar" -c "$cookie_jar" -H 'Content-Type: application/json' -d "$payload" -o "$tmpdir/login" -w '%{http_code}' "$BASE_URL$BASE_PATH/api/v1/auth/login" || true)"
  if [[ "$login_code" != "204" ]]; then
    echo "FAIL login expected HTTP 204 got $login_code" >&2
    sed -n '1,20p' "$tmpdir/login" >&2 || true
    exit 1
  fi
  echo "OK login HTTP 204"
fi

session_after_code="$(curl "${curl_args[@]}" -b "$cookie_jar" -c "$cookie_jar" -o "$tmpdir/session-after" -w '%{http_code}' "$BASE_URL$BASE_PATH/api/v1/auth/session" || true)"
if [[ "$session_after_code" != "200" ]] || ! grep -q '"authenticated":true' "$tmpdir/session-after"; then
  echo "FAIL authenticated session check failed HTTP $session_after_code" >&2
  sed -n '1,20p' "$tmpdir/session-after" >&2 || true
  exit 1
fi
echo "OK authenticated session"

require_timed_json "analytics core" "/api/v1/analytics/core?range=7d&granularity=day" '"summary":'
require_timed_json "activity heatmap" "/api/v1/analytics/heatmap?range=30d&granularity=day" '"heatmap":'
require_timed_json "legacy analytics summary" "/api/v1/analytics/summary?range=7d&granularity=day" '"heatmap":'
require_timed_json "request health" "/api/v1/usage/request-health?range=24h" '"service_health":'
require_timed_json "request evidence events" "/api/v1/usage/events?range=24h&page_size=10" '"events":'
require_timed_json "status" "/api/v1/status" '"timezone"'
