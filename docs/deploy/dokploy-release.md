# Dokploy Release Chain

## Goal

Dokploy is the source of truth for the production `cpa-usage` Compose app. Release tags build immutable GHCR images, render the repository Compose template, update only the `cpa-usage` Dokploy app through its API, and trigger a Dokploy deployment.

## Production Compose

The authoritative template is:

```text
deploy/dokploy/cpa-usage.compose.yml
```

The production template contains only the `cpa-usage` service:

- external path: `/usage`
- data volume: external `cpa-cliproxyapi-hazmcp_cpa-usage-data`
- internal network: external `cpa-cliproxyapi-hazmcp_cliproxyapi-internal`
- public network: external `dokploy-network`
- internal CPA address: `http://cliproxyapi:8317`
- Redis queue address: `cliproxyapi:8317`
- no `postgres` or `cliproxyapi` services, and no `cpa-usage-keeper` or `KEEPER_LOGIN_PASSWORD`

`deploy/dokploy/cpa-cliproxyapi.compose.yml` is kept only for the one-time split migration of the source Dokploy app. It contains `postgres` and `cliproxyapi` without the `cpa-usage` service, usage route labels, or `cpa-usage-data` volume declaration.

`cpa-usage` is rendered to a concrete GHCR version image, for example:

```text
ghcr.io/evenluo/cpa-usage:v0.1.0
```

Do not deploy production from `latest` or a date tag.

## Required GitHub Configuration

GitHub Actions expects:

```text
secret: DOKPLOY_API_KEY
secret: DOKPLOY_URL
variable: DOKPLOY_CPA_USAGE_COMPOSE_ID=<new cpa-usage compose id>
```

Do not keep using `DOKPLOY_COMPOSE_ID` for this repository after the split. That variable points at the old full-stack compose app and would put `postgres` / `cliproxyapi` back into the release blast radius.

The workflow is `.github/workflows/release.yml` and runs on tags matching `v*.*.*`. It accepts:

- stable: `v0.1.0`
- release candidate: `v0.2.0-rc.1`

## Required Dokploy Environment

The Dokploy Compose environment must provide the runtime values referenced by the template:

```dotenv
MANAGEMENT_PASSWORD=<existing CPA management password>
CPA_USAGE_LOGIN_PASSWORD=<usage dashboard login password>
```

The template defaults the current Dokploy runtime facts: `example.com`, `dokploy-network`, `cpa-cliproxyapi-hazmcp_cliproxyapi-internal`, and `cpa-cliproxyapi-hazmcp_cpa-usage-data`. Override these only when the Dokploy runtime topology changes.

The release script migrates `KEEPER_LOGIN_PASSWORD` to `CPA_USAGE_LOGIN_PASSWORD` once through `compose.saveEnvironment`, then removes the old key from the Dokploy env text. Runtime auth only reads `CPA_USAGE_LOGIN_PASSWORD`.

## One-time Dokploy Split

Prepare the new app and update the old app source without deploying the old app:

```bash
DOKPLOY_URL=https://<dokploy-host> \
DOKPLOY_API_KEY=<api-key> \
CPA_USAGE_VERSION=v0.1.2 \
make dokploy-migrate-cpa-usage-compose
```

The migration script:

- reads the source app `DOKPLOY_SOURCE_COMPOSE_ID`, defaulting to `bqmnXzfYoIuSln9Ndbx1x`
- creates or updates a Dokploy compose app named `cpa-usage`
- copies the source app env into the new app, migrating `KEEPER_LOGIN_PASSWORD` to `CPA_USAGE_LOGIN_PASSWORD`
- writes `deploy/dokploy/cpa-usage.compose.yml` into the new app
- writes `deploy/dokploy/cpa-cliproxyapi.compose.yml` into the source app so it no longer contains `cpa-usage`
- prints the `DOKPLOY_CPA_USAGE_COMPOSE_ID` value to set as the GitHub repo variable

The script does not deploy the source app. For cutover, back up `cpa-cliproxyapi-hazmcp_cpa-usage-data`, pre-pull `ghcr.io/evenluo/cpa-usage:v0.1.2`, stop and remove `cpa-cliproxyapi-hazmcp-cpa-usage-1`, then deploy the new `cpa-usage` app.

Cutover verification should confirm only one `cpa-usage` container is running, `cliproxyapi` and `postgres` kept their original `Created` / `StartedAt` timestamps, `https://example.com/` stays healthy, `https://example.com/usage/healthz` and `https://example.com/usage/` return 200, and `scripts/smoke-cpa-usage.sh` passes.

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
