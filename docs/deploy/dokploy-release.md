# Dokploy Release Chain

## Goal

Dokploy is the source of truth for the production Compose configuration. Release tags build immutable GHCR images, render the repository Compose template, update Dokploy through its API, and trigger a Dokploy deployment.

## Production Compose

The authoritative template is:

```text
deploy/dokploy/cpa-cliproxyapi.compose.yml
```

The template keeps the existing Dokploy app shape:

- services: `postgres`, `cliproxyapi`, `cpa-usage`
- external path: `/usage`
- `cpa-usage` data volume: `cpa-usage-data`
- no `cpa-usage-keeper` service, router, or volume declaration

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
variable: DOKPLOY_COMPOSE_ID=bqmnXzfYoIuSln9Ndbx1x
```

The workflow is `.github/workflows/release.yml` and runs on tags matching `v*.*.*`. It accepts:

- stable: `v0.1.0`
- release candidate: `v0.2.0-rc.1`

## Required Dokploy Environment

The Dokploy Compose environment must provide the runtime values referenced by the template:

```dotenv
PUBLIC_HOST=example.com
POSTGRES_IMAGE=<existing pinned postgres image>
POSTGRES_DB=<existing postgres database>
POSTGRES_USER=<existing postgres user>
POSTGRES_PASSWORD=<existing postgres password>
POSTGRES_DATA_VOLUME=<existing postgres data volume>
CLIPROXYAPI_IMAGE=<existing pinned cliproxyapi image>
CLIPROXYAPI_PGSTORE_DSN=<existing postgres DSN for cliproxyapi>
CLIPROXYAPI_CONFIG_PATH=<existing host config.yaml path>
CLIPROXYAPI_AUTH_PATH=<existing host auth directory>
CLIPROXYAPI_LOG_PATH=<existing host logs directory>
MANAGEMENT_PASSWORD=<existing CPA management password>
CPA_USAGE_LOGIN_PASSWORD=<usage dashboard login password>
AUTH_SESSION_SECRET=<at least 32 characters>
```

The release script migrates `KEEPER_LOGIN_PASSWORD` to `CPA_USAGE_LOGIN_PASSWORD` once through `compose.saveEnvironment`, then removes the old key from the Dokploy env text. Runtime auth only reads `CPA_USAGE_LOGIN_PASSWORD`.

## Local Verification

Render and validate a versioned Compose file:

```bash
CPA_USAGE_VERSION=v0.1.0 make render-dokploy-compose
make verify-dokploy-compose
```

The validation checks that the rendered Compose does not contain:

- `cpa-usage-keeper`
- `KEEPER_LOGIN_PASSWORD`
- `:latest`

It also runs `docker compose config` with sample non-secret values when Docker is available.

## Compatibility Decision

External compatibility is kept for the public path `/usage`, CPA management password semantics, CPA internal DNS, Redis usage queue address, and Postgres data. The old keeper service name and `KEEPER_LOGIN_PASSWORD` are not kept as runtime compatibility paths.
