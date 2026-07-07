# CPA Usage Final Cutover Report

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

- `GET /usage/healthz`:
- `GET /usage/`:
- Login:
- `GET /usage/api/v1/auth/session`:
- `GET /usage/api/v1/analytics/summary?range=7d&granularity=hour`:
- `GET /usage/api/v1/analytics/summary?range=7d&granularity=day`:
- Concurrent smoke: start `GET /usage/api/v1/analytics/summary?range=7d&granularity=hour`, then issue `GET /usage/api/v1/analytics/core?range=24h&granularity=hour`; core analytics should remain fast rather than waiting behind summary:
- `GET /usage/api/v1/status`:
- `GET /`:
- `GET /cpa-usage/healthz`:

## Ownership Evidence

- `cpa-usage` is the only usage consumer:
- `cpa-usage-keeper` stopped or absent:
- Adjacent services unchanged:

## Recovery Decision

- Recovery needed:
- Recovery source:
- Follow-up:

## Compatibility Decision

This cutover intentionally ends the old `/usage` dashboard path. CPA root service and adjacent infrastructure services remain compatible and unchanged unless stated above.

The analytics summary route remains a compatibility interface. Its response contract is preserved, but the backend implementation should read through the rollup-aware Usage Intelligence read models rather than maintaining a separate raw analytics implementation.
