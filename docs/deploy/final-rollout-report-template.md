# CPA Usage Final Cutover Report

Date:

## Deployment Source

- Repository: `evenluo/cpa-usage`
- Commit:
- Image:
- Image ID:
- Image created at:

## Pre-Cutover Baseline

- Compose directory: `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code`
- Compose checksum before:
- Services before:
- `cliproxyapi` container ID/status:
- `postgres` container ID/status:
- `cpa-usage-keeper` status before shutdown:

## Keeper Shutdown And Backup

- Keeper stopped at:
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

- `GET /cpa-usage/healthz`:
- `GET /cpa-usage/`:
- Login:
- `GET /cpa-usage/api/v1/auth/session`:
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour`:
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=day`:
- `GET /cpa-usage/api/v1/status`:
- `GET /`:
- `GET /usage/healthz`:

## Ownership Evidence

- `cpa-usage` is the only usage consumer:
- `cpa-usage-keeper` stopped or absent:
- Protected services unchanged:

## Recovery Decision

- Recovery needed:
- Recovery source:
- Follow-up:

## Compatibility Decision

This cutover intentionally ends the old `/usage` dashboard path. CPA root service, `cliproxyapi`, and `postgres` remain compatible and unchanged unless stated above.
