# Dokploy Release Chain

## Goal

Dokploy is the source of truth for the production `cpa-usage` Compose app. Pushes to `main` and release tags build immutable GHCR images, render the repository Compose template, update only the `cpa-usage` Dokploy app through its API, and trigger a Dokploy deployment.

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
```

Do not keep using `DOKPLOY_COMPOSE_ID` for this repository after the split. That variable points at the old full-stack compose app and would put `postgres` / `cliproxyapi` back into the release blast radius.

The workflow is `.github/workflows/release.yml` and runs on pushes to `main` plus tags matching `v*.*.*`. It accepts:

- stable: `v0.1.0`
- release candidate: `v0.2.0-rc.1`

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
make verify-dokploy-compose
```

The validation checks that the rendered Compose does not contain:

- `postgres` or `cliproxyapi` as services
- `cpa-usage-keeper`
- `KEEPER_LOGIN_PASSWORD`
- `:latest`

It also runs `docker compose config` with sample non-secret values when Docker is available.

## Compatibility Decision

External compatibility is kept for the public path `/usage`, CPA management password semantics, CPA internal DNS, Redis usage queue address, and the existing `cpa-usage` SQLite data volume. The production release chain intentionally stops managing `postgres` and `cliproxyapi`. The old keeper service name and `KEEPER_LOGIN_PASSWORD` are not kept as runtime compatibility paths.
