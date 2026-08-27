# CPA Usage Final Cutover Report

Status: historical one-time cutover template; not the current Dokploy release proof

Current release SoT: [Dokploy Release Chain](dokploy-release.md)

Date:

## Deployment Source

- Repository: `evenluo/cpa-usage`
- Commit:
- Image:
- Image ID:
- Image created at:

## Pre-Cutover Baseline

- Compose directory:
- Compose checksum before:
- Services before:
- CPA backend container ID/status:
- Database container ID/status:
- Previous usage service status before shutdown:

## Previous Usage Service Shutdown And Backup

- Previous usage service stopped at:
- Backup path:
- Backup SHA256:
- `/usage/healthz` after shutdown:
- Reason `/usage` downtime is acceptable:

## Data Migration

- Source volume:
- Target volume:
- Migration method:
- Tables preserved:
- Tables skipped and reason:
- Row-count checks:
- Representative record checks:

## New Service Deployment

- Compose change:
- `cpa-usage` service status:
- `APP_BASE_PATH`:
- `CPA_BASE_URL`:
- `REDIS_QUEUE_ADDR`:
- Auth enabled:

## Post-Deploy Smoke

- `GET /usage/healthz`:
- `GET /usage/`:
- Login:
- `GET /usage/api/v1/auth/session`:
- `GET /usage/api/v1/analytics/summary?range=7d&granularity=hour`:
- `GET /usage/api/v1/analytics/summary?range=7d&granularity=day`:
- Concurrent smoke: start `GET /usage/api/v1/analytics/summary?range=7d&granularity=hour`, then issue `GET /usage/api/v1/analytics/core?range=24h&granularity=hour`; core analytics should remain fast rather than waiting behind summary:
- `GET /usage/api/v1/status`:
- `GET /`:

## Ownership Evidence

- `cpa-usage` is the only usage consumer:
- Previous usage service stopped or absent:
- Adjacent services unchanged:

## Recovery Decision

- Recovery needed:
- Recovery source:
- Follow-up:

## Compatibility Decision

This cutover replaces the previous usage service behind the public `/usage` path. The public path, CPA root service, and adjacent infrastructure services remain compatible and unchanged unless stated above.

The analytics summary route remains a compatibility interface. Current analytics implementation and release proof are governed by [Analytics Raw/Rollup Convergence](../design/analytics-raw-rollup-convergence.md) and the [Dokploy Release Chain](dokploy-release.md), not this historical template.
