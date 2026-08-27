#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

fixtures="$repo_root/scripts/testdata/dokploy-release"
tmpdir="$(mktemp -d)"
trap 'rm -rf "$tmpdir"' EXIT

expected_image="ghcr.io/evenluo/cpa-usage:sha-0123456789ab"
compose_file="$tmpdir/cpa-usage.compose.yml"
CPA_USAGE_IMAGE="$expected_image" scripts/render-dokploy-compose.sh "sha-0123456789ab" "$compose_file"

pass_count=0

pass() {
  echo "PASS $1"
  pass_count=$((pass_count + 1))
}

fail() {
  echo "FAIL $1" >&2
  exit 1
}

expect_success() {
  local name="$1"
  shift
  if "$@" > "$tmpdir/output.log" 2>&1; then
    pass "$name"
  else
    sed -n '1,120p' "$tmpdir/output.log" >&2
    fail "$name"
  fi
}

expect_failure() {
  local name="$1"
  shift
  if "$@" > "$tmpdir/output.log" 2>&1; then
    sed -n '1,120p' "$tmpdir/output.log" >&2
    fail "$name unexpectedly succeeded"
  fi
  pass "$name"
}

fake_docker_bin="$tmpdir/fake-docker-bin"
mkdir -p "$fake_docker_bin"
cp "$fixtures/fake-docker.sh" "$fake_docker_bin/docker"
chmod +x "$fake_docker_bin/docker"

verify_fixture() {
  local fixture="$1"
  PATH="$fake_docker_bin:$PATH" \
    FAKE_DOCKER_CONFIG_FIXTURE="$fixtures/$fixture" \
    CPA_USAGE_IMAGE="$expected_image" \
    scripts/verify-dokploy-compose.sh "$compose_file"
}

expect_success "canonical exact service image" verify_fixture "compose-exact.json"
for fixture in \
  compose-missing-service.json \
  compose-wrong-service.json \
  compose-nested-misleading.json \
  compose-non-exact.json; do
  expect_failure "canonical rejects ${fixture%.json}" verify_fixture "$fixture"
done

missing_runtime_bin="$tmpdir/missing-runtime-bin"
mkdir -p "$missing_runtime_bin"
for command_name in bash env grep jq; do
  command_path="$(command -v "$command_name")"
  ln -s "$command_path" "$missing_runtime_bin/$command_name"
done
expect_failure "canonical fails without Docker runtime" \
  env PATH="$missing_runtime_bin" CPA_USAGE_IMAGE="$expected_image" \
  scripts/verify-dokploy-compose.sh "$compose_file"
expect_success "static-only check remains explicit without runtime" \
  env PATH="$missing_runtime_bin" scripts/verify-dokploy-compose-static.sh "$compose_file"

release_bin="$tmpdir/release-bin"
mkdir -p "$release_bin"
cp "$fixtures/fake-docker.sh" "$release_bin/docker"
cp "$fixtures/fake-curl.sh" "$release_bin/curl"
chmod +x "$release_bin/docker" "$release_bin/curl"

run_release() {
  local scenario="$1"
  local state_dir="$tmpdir/state-$scenario"
  rm -rf "$state_dir"
  mkdir -p "$state_dir"
  PATH="$release_bin:$PATH" \
    FAKE_DOKPLOY_FIXTURES="$fixtures" \
    FAKE_DOKPLOY_STATE_DIR="$state_dir" \
    FAKE_DOKPLOY_SCENARIO="$scenario" \
    DOKPLOY_URL="https://dokploy.example.test" \
    DOKPLOY_API_KEY="fixture-api-secret" \
    DOKPLOY_CPA_USAGE_COMPOSE_ID="compose-fixture" \
    DOKPLOY_CPA_USAGE_HEALTH_URL="https://cpa.example.test/usage/healthz" \
    CPA_USAGE_IMAGE="$expected_image" \
    CPA_USAGE_RELEASE_ID="cpa-usage-run-123-attempt-1-sha-0123456789ab" \
    DOKPLOY_DEPLOY_POLL_ATTEMPTS=3 \
    DOKPLOY_DEPLOY_POLL_INTERVAL_SECONDS=0 \
    DOKPLOY_HEALTH_ATTEMPTS=3 \
    DOKPLOY_HEALTH_INTERVAL_SECONDS=0 \
    DOKPLOY_REQUEST_TIMEOUT_SECONDS=1 \
    scripts/dokploy-deploy-compose.sh
}

expect_success "running to done plus healthy release" run_release success
success_state="$tmpdir/state-success"
if ! jq -e '
  .composeId == "compose-fixture" and
  .description == "cpa-usage-run-123-attempt-1-sha-0123456789ab" and
  .title == "CPA Usage sha-0123456789ab"
' "$success_state/deploy-payload.json" >/dev/null; then
  fail "compose.deploy payload lacks the exact release marker"
fi
pass "compose.deploy carries exact non-secret marker"

success_calls="$(cat "$success_state/calls.log")"
case "$success_calls" in
  *"GET compose.one?composeId=compose-fixture"*"GET deployment.allByCompose?composeId=compose-fixture"*"POST compose.update"*"GET compose.getConvertedCompose?composeId=compose-fixture"*"POST compose.deploy"*"GET deployment.allByCompose?composeId=compose-fixture"*)
    pass "release call order is preflight then mutation then proof"
    ;;
  *)
    printf '%s\n' "$success_calls" >&2
    fail "release call order"
    ;;
esac

if grep -Eq 'fixture-api-secret|fixture-secret-value' "$tmpdir/output.log"; then
  fail "release output exposed fixture secrets"
fi
pass "release output omits secrets"

expect_success "environment migration stays after preflight" run_release env-migration
migration_calls="$(cat "$tmpdir/state-env-migration/calls.log")"
case "$migration_calls" in
  *"GET compose.one?composeId=compose-fixture"*"GET deployment.allByCompose?composeId=compose-fixture"*"POST compose.saveEnvironment"*"POST compose.update"*)
    pass "environment migration mutates only after complete preflight"
    ;;
  *)
    printf '%s\n' "$migration_calls" >&2
    fail "environment migration call order"
    ;;
esac
if grep -Eq 'fixture-api-secret|fixture-secret-value' "$tmpdir/output.log"; then
  fail "environment migration output exposed fixture secrets"
fi
pass "environment migration output omits secrets"

expect_success "nullable unrelated deployment history remains supported" run_release nullable-history
expect_success "nullable unrelated deployment status remains supported" run_release nullable-status-history

expect_failure "preflight unsupported shape fails" run_release preflight-unsupported
if grep -q '^POST ' "$tmpdir/state-preflight-unsupported/calls.log"; then
  fail "preflight failure called a mutation endpoint"
fi
pass "preflight failure performs no mutation"

expect_failure "existing release marker fails preflight" run_release preflight-existing-marker
if grep -q '^POST ' "$tmpdir/state-preflight-existing-marker/calls.log"; then
  fail "existing marker preflight called a mutation endpoint"
fi
pass "non-unique preflight marker performs no mutation"

expect_failure "converted image mismatch fails" run_release converted-mismatch
if ! grep -q '^POST compose.update$' "$tmpdir/state-converted-mismatch/calls.log" || \
   grep -q '^POST compose.deploy$' "$tmpdir/state-converted-mismatch/calls.log"; then
  fail "converted mismatch did not stop between update and deploy"
fi
pass "converted mismatch prevents compose.deploy"

for scenario in \
  deployment-error \
  deployment-cancelled \
  deployment-unknown \
  deployment-missing-fields \
  deployment-ambiguous \
  deployment-missing-marker \
  deployment-timeout \
  health-503 \
  health-non-ok; do
  expect_failure "release rejects $scenario" run_release "$scenario"
done
expect_success "health retry reaches exact success" run_release health-retry

render_step="$(grep -n '^      - name: Render and verify Dokploy compose$' .github/workflows/release.yml | cut -d: -f1)"
deploy_step="$(grep -n '^      - name: Deploy via Dokploy API$' .github/workflows/release.yml | cut -d: -f1)"
if [[ -z "$render_step" || -z "$deploy_step" || "$render_step" -ge "$deploy_step" ]]; then
  fail "release workflow does not order canonical verification before Dokploy mutation"
fi
if ! grep -q 'CPA_USAGE_RELEASE_ID:' .github/workflows/release.yml || \
   ! grep -q 'DOKPLOY_CPA_USAGE_HEALTH_URL:' .github/workflows/release.yml; then
  fail "release workflow does not supply release proof inputs"
fi
pass "workflow orders canonical gate and supplies proof inputs"

# The literal GitHub expression must not be expanded by this shell.
# shellcheck disable=SC2016
if ! grep -Fq 'group: cpa-usage-dokploy-compose-${{ vars.DOKPLOY_CPA_USAGE_COMPOSE_ID }}' .github/workflows/release.yml || \
   ! grep -Fq 'cancel-in-progress: false' .github/workflows/release.yml || \
   ! grep -Fq 'queue: max' .github/workflows/release.yml; then
  fail "release workflow does not serialize the shared Dokploy Compose mutation path"
fi
pass "workflow serializes and retains interleaving releases for one Dokploy Compose"

echo "OK $pass_count Dokploy release fixture checks"
