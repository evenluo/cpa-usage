#!/usr/bin/env bash
set -euo pipefail

template="${DOKPLOY_COMPOSE_TEMPLATE:-deploy/dokploy/cpa-cliproxyapi.compose.yml}"
version_or_image="${1:-${CPA_USAGE_VERSION:-}}"
output="${2:-}"

if [[ -n "${CPA_USAGE_IMAGE:-}" ]]; then
  image="$CPA_USAGE_IMAGE"
else
  if [[ -z "$version_or_image" ]]; then
    echo "usage: $0 <version-tag|image> [output-file]" >&2
    exit 2
  fi
  if [[ "$version_or_image" == *"/"* ]]; then
    image="$version_or_image"
  else
    image="ghcr.io/evenluo/cpa-usage:$version_or_image"
  fi
fi

if [[ "$image" != ghcr.io/evenluo/cpa-usage:* ]]; then
  echo "CPA_USAGE_IMAGE must use ghcr.io/evenluo/cpa-usage:<version-tag>" >&2
  exit 2
fi

tag="${image##*:}"
if [[ ! "$tag" =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-rc\.[0-9]+)?$ ]]; then
  echo "CPA_USAGE_IMAGE tag must be SemVer, for example v0.1.0 or v0.2.0-rc.1" >&2
  exit 2
fi

if [[ ! -f "$template" ]]; then
  echo "template not found: $template" >&2
  exit 2
fi

image_escaped="${image//&/\\&}"
if [[ -n "$output" ]]; then
  mkdir -p "$(dirname "$output")"
  sed "s|__CPA_USAGE_IMAGE__|$image_escaped|g" "$template" > "$output"
else
  sed "s|__CPA_USAGE_IMAGE__|$image_escaped|g" "$template"
fi
