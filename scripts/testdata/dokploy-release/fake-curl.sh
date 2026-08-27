#!/usr/bin/env bash
set -euo pipefail

state_dir="${FAKE_DOKPLOY_STATE_DIR:?set FAKE_DOKPLOY_STATE_DIR}"
scenario="${FAKE_DOKPLOY_SCENARIO:-success}"
fixtures="${FAKE_DOKPLOY_FIXTURES:?set FAKE_DOKPLOY_FIXTURES}"
compose_id="${DOKPLOY_CPA_USAGE_COMPOSE_ID:?set DOKPLOY_CPA_USAGE_COMPOSE_ID}"
release_id="${CPA_USAGE_RELEASE_ID:?set CPA_USAGE_RELEASE_ID}"
method="GET"
payload=""
output_file=""
write_format=""
url=""

while (($# > 0)); do
  case "$1" in
    -X)
      method="$2"
      shift 2
      ;;
    --data-binary)
      payload="${2#@}"
      shift 2
      ;;
    -H | --max-time)
      shift 2
      ;;
    -o)
      output_file="$2"
      shift 2
      ;;
    -w)
      write_format="$2"
      shift 2
      ;;
    -* )
      shift
      ;;
    *)
      url="$1"
      shift
      ;;
  esac
done

mkdir -p "$state_dir"
path="${url#*/api/}"
printf '%s %s\n' "$method" "$path" >> "$state_dir/calls.log"

render_deployment_fixture() {
  local fixture="$1"
  sed \
    -e "s/__RELEASE_ID__/$release_id/g" \
    -e "s/__COMPOSE_ID__/$compose_id/g" \
    "$fixtures/$fixture"
}

if [[ "$url" == "$DOKPLOY_CPA_USAGE_HEALTH_URL" ]]; then
  health_attempt_file="$state_dir/health-attempt"
  health_attempt=0
  if [[ -f "$health_attempt_file" ]]; then
    health_attempt="$(cat "$health_attempt_file")"
  fi
  health_attempt=$((health_attempt + 1))
  printf '%s' "$health_attempt" > "$health_attempt_file"

  health_code="200"
  health_fixture="health-ok.json"
  case "$scenario" in
    health-503)
      health_code="503"
      health_fixture="health-not-ok.json"
      ;;
    health-non-ok)
      health_fixture="health-not-ok.json"
      ;;
    health-retry)
      if [[ "$health_attempt" == "1" ]]; then
        health_code="503"
        health_fixture="health-not-ok.json"
      fi
      ;;
  esac
  cp "$fixtures/$health_fixture" "$output_file"
  if [[ -n "$write_format" ]]; then
    printf '%s' "$health_code"
  fi
  exit 0
fi

case "$path" in
  compose.one*)
    if [[ "$scenario" == "env-migration" ]]; then
      printf '{"env":"KEEPER_LOGIN_PASSWORD=fixture-secret-value\\n"}\n'
    else
      printf '{"env":"CPA_USAGE_LOGIN_PASSWORD=fixture-secret-value\\n"}\n'
    fi
    ;;
  deployment.allByCompose*)
    if [[ ! -f "$state_dir/deployed" ]]; then
      if [[ "$scenario" == "preflight-unsupported" ]]; then
        printf '{}\n'
      elif [[ "$scenario" == "preflight-existing-marker" ]]; then
        render_deployment_fixture "deployment-done.json"
      else
        printf '[]\n'
      fi
      exit 0
    fi

    poll_file="$state_dir/poll-attempt"
    poll_attempt=0
    if [[ -f "$poll_file" ]]; then
      poll_attempt="$(cat "$poll_file")"
    fi
    poll_attempt=$((poll_attempt + 1))
    printf '%s' "$poll_attempt" > "$poll_file"
    case "$scenario" in
      success | env-migration | health-503 | health-non-ok | health-retry)
        if [[ "$poll_attempt" == "1" ]]; then
          render_deployment_fixture "deployment-running.json"
        else
          render_deployment_fixture "deployment-done.json"
        fi
        ;;
      deployment-error)
        render_deployment_fixture "deployment-error.json"
        ;;
      deployment-cancelled)
        render_deployment_fixture "deployment-cancelled.json"
        ;;
      deployment-unknown)
        render_deployment_fixture "deployment-unknown.json"
        ;;
      deployment-missing-fields)
        render_deployment_fixture "deployment-missing-fields.json"
        ;;
      deployment-ambiguous)
        render_deployment_fixture "deployment-ambiguous.json"
        ;;
      deployment-missing-marker)
        printf '[]\n'
        ;;
      deployment-timeout)
        render_deployment_fixture "deployment-running.json"
        ;;
      *)
        render_deployment_fixture "deployment-done.json"
        ;;
    esac
    ;;
  compose.getConvertedCompose*)
    converted_image="$CPA_USAGE_IMAGE"
    if [[ "$scenario" == "converted-mismatch" ]]; then
      converted_image="ghcr.io/evenluo/cpa-usage:sha-bbbbbbbbbbbb"
    fi
    jq -n --arg image "$converted_image" '
      "services:\n  cpa-usage:\n    image: " + $image + "\n"
    '
    ;;
  compose.update | compose.saveEnvironment)
    printf '{}\n'
    ;;
  compose.deploy)
    cp "$payload" "$state_dir/deploy-payload.json"
    touch "$state_dir/deployed"
    printf '{}\n'
    ;;
  *)
    echo "unsupported fake curl URL: $url" >&2
    exit 2
    ;;
esac
