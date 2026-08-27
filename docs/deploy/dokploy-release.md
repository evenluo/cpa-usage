# Dokploy Release Chain

## Goal

Dokploy is the source of truth for the production `cpa-usage` Compose app. Pushes to `main` and release tags build immutable GHCR images. The release job reports success only after it verifies the release contract before mutation, reads back the exact converted image, observes the correlated Dokploy deployment reach terminal success, and verifies the deployed health response.

## Production Compose

The authoritative template is:

```text
deploy/dokploy/cpa-usage.compose.yml
```

The production template contains only the `cpa-usage` service:

- external path: `/usage`
- data volume: external `cpa-dmit-us-usage-data`
- internal network: external `cpa-dmit-us-internal`
- internal CPA address: `http://cliproxyapi:8317`
- Redis queue address: `cliproxyapi:8317`
- no `postgres` or `cliproxyapi` services, and no `cpa-usage-keeper` or `KEEPER_LOGIN_PASSWORD`
- no hard-coded public host; `PUBLIC_HOST` must be supplied by the Dokploy environment

`deploy/dokploy/cpa-cliproxyapi.compose.yml` is kept only for the one-time split migration of the source Dokploy app. It contains `postgres` and `cliproxyapi` without the `cpa-usage` service, usage route labels, or `cpa-usage-data` volume declaration.

`cpa-usage` is rendered to a concrete GHCR version image, for example:

```text
ghcr.io/evenluo/cpa-usage:v0.1.0
```

Do not deploy production from `latest`, a branch-name tag, or a date tag. `main` deploys `sha-<12 hex>` and release tags deploy their SemVer tag.

## Required GitHub Configuration

GitHub Actions expects:

```text
secret: DOKPLOY_API_KEY
secret: DOKPLOY_URL
variable: DOKPLOY_CPA_USAGE_COMPOSE_ID=<new cpa-usage compose id>
variable: DOKPLOY_CPA_USAGE_HEALTH_URL=https://<production-host>/usage/healthz
```

Do not keep using `DOKPLOY_COMPOSE_ID` for this repository after the split. That variable points at the old full-stack compose app and would put `postgres` / `cliproxyapi` back into the release blast radius.

The workflow is `.github/workflows/release.yml` and runs on pushes to `main` plus tags matching `v*.*.*`. It accepts:

- stable: `v0.1.0`
- release candidate: `v0.2.0-rc.1`

Each workflow run supplies a unique, non-secret `CPA_USAGE_RELEASE_ID` containing the GitHub run, attempt, and commit identifiers. The release script writes it as the exact Dokploy deployment description and uses only an exact description match to correlate deployment status. It never treats the newest deployment as the requested one.

## Required Dokploy Environment

The Dokploy Compose environment must provide the runtime values referenced by the template:

```dotenv
PUBLIC_HOST=<production CPA host>
MANAGEMENT_PASSWORD=<existing CPA management password>
CPA_USAGE_LOGIN_PASSWORD=<usage dashboard login password>
AUTH_SESSION_SECRET=<random secret with at least 32 characters>
AUTH_SESSION_COOKIE_DOMAIN=<production CPA host>
```

The template defaults the current dmit-us runtime facts: `cpa-dmit-us-internal` and `cpa-dmit-us-usage-data`. Override these only when the Dokploy runtime topology changes.

The release script migrates `KEEPER_LOGIN_PASSWORD` to `CPA_USAGE_LOGIN_PASSWORD` once through `compose.saveEnvironment`, then removes the old key from the Dokploy env text. Runtime auth only reads `CPA_USAGE_LOGIN_PASSWORD`.

## One-time Dokploy Split

Prepare the new app and update the old app source without deploying the old app:

```bash
DOKPLOY_URL=https://<dokploy-host> \
DOKPLOY_API_KEY=<api-key> \
CPA_USAGE_VERSION=v0.1.25 \
make dokploy-migrate-cpa-usage-compose
```

The migration script:

- reads the dmit-us source app `DOKPLOY_SOURCE_COMPOSE_ID`, defaulting to `qq0poZq0j2Rq3XJTUqH1c`
- creates or updates a Dokploy compose app named `cpa-usage`
- copies the source app env into the new app, migrating `KEEPER_LOGIN_PASSWORD` to `CPA_USAGE_LOGIN_PASSWORD`
- writes `deploy/dokploy/cpa-usage.compose.yml` into the new app
- writes `deploy/dokploy/cpa-cliproxyapi.compose.yml` into the source app so it no longer contains `cpa-usage`
- prints the `DOKPLOY_CPA_USAGE_COMPOSE_ID` value to set as the GitHub repo variable

The script does not deploy the source app. For cutover, back up `cpa-dmit-us-usage-data`, pre-pull the selected immutable image, stop the old source app's `cpa-usage` container, deploy the new `cpa-usage` app, verify it, and finally deploy the updated source app to remove the old service definition.

Cutover verification should confirm only one `cpa-usage` container is running, `cliproxyapi` and `postgres` kept their original `Created` / `StartedAt` timestamps, `https://<production-host>/` stays healthy, `https://<production-host>/usage/healthz` and `https://<production-host>/usage/` return 200, and `scripts/smoke-cpa-usage.sh` passes.

For Usage Intelligence performance releases, keep the smoke output lines with `time_total`. Compare `analytics core`, `activity heatmap`, `legacy analytics summary`, `request health`, `request evidence events`, and `status` separately. The production symptom to watch for is the old coupling where the first useful dashboard view waited on full overview or heatmap scans; after this rollout, a slower heatmap line should not hide the core dashboard timing.

## Local Verification

Render and validate a versioned Compose file:

```bash
CPA_USAGE_VERSION=v0.1.0 make render-dokploy-compose
CPA_USAGE_IMAGE=ghcr.io/evenluo/cpa-usage:v0.1.0 \
COMPOSE_FILE=.tmp/dokploy/cpa-usage.compose.yml \
make verify-dokploy-compose
make test-dokploy-release
```

`make verify-dokploy-compose` is the canonical gate. It requires Docker Compose and `jq`, runs `docker compose config --format json`, and requires the rendered JSON to contain only `services["cpa-usage"]` with the exact expected image. Missing parser/runtime support is a failure, not a skipped check.

`make verify-dokploy-compose-static` is a distinctly narrower static-only check for environments without Docker. It cannot satisfy the release gate. Both checks reject:

- `postgres` or `cliproxyapi` as services
- `cpa-usage-keeper`
- `KEEPER_LOGIN_PASSWORD`
- `:latest`

`make test-dokploy-release` uses only local fake Docker, Dokploy API, deployment, and health fixtures. It does not contact Dokploy or production.

## Release Proof and Failure Policy

The release command follows one ordered path:

1. Render the immutable image and pass the canonical Compose gate.
2. Read `compose.one` and `deployment.allByCompose` and validate their documented response shapes before any Dokploy mutation. The release marker must not already exist.
3. Apply the one-time environment-key migration if needed, then call `compose.update`.
4. Read `compose.getConvertedCompose`, require its response to be a Compose YAML string, and pass it through the same canonical exact-image gate.
5. Call `compose.deploy` with the unique release marker in `description`.
6. Poll official `deployment.allByCompose` for exactly one row with that marker. Only `running`, `done`, `error`, and `cancelled` are supported; success requires `done` within the bounded polling window.
7. Poll the configured HTTPS `/usage/healthz` URL within a bounded window and require HTTP 200 with JSON `status: "ok"`.

Missing or malformed control-plane responses, zero correlated rows until timeout, multiple correlated rows, an unknown status, `error`, `cancelled`, timeout, an unhealthy response, or an image mismatch all exit non-zero. A successful HTTP response from `compose.deploy` is only trigger acceptance and is never release success.

There is no automatic rollback or alternate API fallback. Recovery is an explicit operator redeploy of a selected immutable image after investigating the reported failed stage.

## Compatibility Decision

External compatibility is kept for the public path `/usage`, CPA management password semantics, CPA internal DNS, Redis usage queue address, and the existing `cpa-usage` SQLite data volume. The production release chain intentionally stops managing `postgres` and `cliproxyapi`. The old keeper service name and `KEEPER_LOGIN_PASSWORD` are not kept as runtime compatibility paths.

The fail-closed proof is an intentional operational tightening: automation that previously stopped after deployment trigger acceptance now fails unless the requested immutable image, the exact terminal deployment, and deployed health are all proven.
