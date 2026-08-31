#!/usr/bin/env bash
set -euo pipefail

if [[ "${1:-}" != "compose" ]]; then
  echo "fake docker only supports compose" >&2
  exit 2
fi

compose_file=""
while (($# > 0)); do
  case "$1" in
    -f)
      compose_file="$2"
      shift 2
      ;;
    *)
      shift
      ;;
  esac
done

if [[ -n "${FAKE_DOCKER_CONFIG_FIXTURE:-}" ]]; then
  cat "$FAKE_DOCKER_CONFIG_FIXTURE"
  exit 0
fi

image="$(awk '$1 == "image:" { print $2; exit }' "$compose_file")"
jq -n --arg image "$image" '{services: {"cpa-usage": {image: $image}}}'
