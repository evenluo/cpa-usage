# CPA Usage Final Cutover Report

Date: 2026-05-17

## Deployment Source

- Repository: `evenluo/cpa-usage`
- Commit: `2cd0b87346a1bf22c30350a21dfc19fa60681add`
- Image: `cpa-usage:2cd0b87346a1`
- Image ID: `sha256:1615e74d66497b4e3fa0372fda7459acea4b64229782cc90984af12363888a5f`
- Image created at: `2026-05-17T23:30:05.113832895+08:00`

## Pre-Cutover Baseline

- Compose directory: `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code`
- Compose checksum before: `3aaf19c66c36e1ed947b248828f617bb2ab817d41f89f9cefd8fee7e5cc4a71a  docker-compose.yml`
- Services before: `postgres`, `cliproxyapi`, `cpa-usage-keeper`
- `cliproxyapi`: running, image `eceasy/cli-proxy-api:v7.1.6`
- `postgres`: running healthy, image `postgres:16-alpine`
- `cpa-usage-keeper`: running healthy before shutdown, image `ghcr.io/willxup/cpa-usage-keeper:latest`

## Keeper Shutdown And Backup

- Keeper stopped at: `2026-05-17T15:22:41Z`
- Backup path: `/root/cpa-usage-cutover-backups/20260517T152241Z/cpa-usage-keeper-data.tgz`
- Backup SHA256: `d0091456fc28b8e8a2bcfc8781223bec8ff233039836d88906916f53901f6506`
- `/usage/healthz` after shutdown: HTTP 404
- Reason `/usage` downtime is acceptable: old keeper is no longer a rollback target; this repository's `cpa-usage` is the replacement owner.

## Data Migration

- Source volume: `cpa-cliproxyapi-hazmcp_cpa-usage-keeper-data`
- Target volume: `cpa-cliproxyapi-hazmcp_cpa-usage-data`
- Pre-existing target volume backup: `/root/cpa-usage-cutover-backups/pre-new-volume-20260517T153116Z.tgz`
- Migration method: restore keeper backup tarball into `cpa-usage-data`; run new image migrations on startup.
- Tables preserved:
  - `usage_events`
  - `redis_usage_inboxes`
  - `usage_identities`
  - `key_aliases`
  - `model_price_settings`
  - `schema_migrations`
- Tables skipped and reason: none identified.
- Row-count checks after startup:
  - `usage_events=9317`
  - `redis_usage_inboxes=753`
  - `usage_identities=2`
  - `key_aliases=0`
  - `model_price_settings=12`
  - `schema_migrations=19`
- Startup migration evidence: `20260513_create_key_aliases` applied; earlier migrations skipped as already present.

## New Service Deployment

- Compose overlay: `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code/cpa-usage.cutover.compose.yml`
- Overlay checksum: `1451d6e73c71933f2aa86815ef23f7ceb1a38c4034ce61058caca3c8a3925a27`
- `cpa-usage` service status: running healthy
- Container ID: `9d3ac4efdf0d88f6e3ee4a1b8235785b2dd616e5d4a4789487c3133fd9f272f4`
- `APP_BASE_PATH`: `/cpa-usage`
- `CPA_BASE_URL`: `http://cliproxyapi:8317`
- `REDIS_QUEUE_ADDR`: `cliproxyapi:8317`
- Auth enabled: true

## Post-Deploy Smoke

Automated smoke command:

```sh
BASE_URL=https://cpa.maxtap.net \
BASE_PATH=/cpa-usage \
LOGIN_PASSWORD='<redacted>' \
EXPECT_KEEPER_STOPPED=true \
scripts/smoke-cpa-usage.sh
```

Results:

- `GET /`: HTTP 200
- `GET /cpa-usage/healthz`: HTTP 200
- `GET /cpa-usage/`: HTTP 200
- `GET /usage/healthz`: HTTP 404
- Login: HTTP 204
- `GET /cpa-usage/api/v1/auth/session`: authenticated
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour`: HTTP 200
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=day`: HTTP 200
- `GET /cpa-usage/api/v1/status`: HTTP 200

## OpenAI GPT Pricing Seed

- Source check date: 2026-05-17
- Source pages:
  - `https://openai.com/api/pricing/`
  - `https://developers.openai.com/api/docs/pricing`
- Pricing basis: OpenAI API Standard tier, short context, USD per 1M tokens.
- Seed script: `scripts/seed-openai-gpt-pricing.sh`
- Backup before pricing write: `/root/cpa-usage-cutover-backups/pre-gpt-pricing-20260517T154052Z.tgz`
- Backup SHA256: `61f1be0b2c3e86d486cc48508d7cebd38f9b01a240c54085f97b5c83368b222f`
- Seeded rows:
  - `gpt-5.5`: input `5.00`, output `30.00`, cached input `0.50`
  - `gpt-5.4`: input `2.50`, output `15.00`, cached input `0.25`
  - `gpt-5.4-mini`: input `0.75`, output `4.50`, cached input `0.075`
  - `gpt-5.4-nano`: input `0.20`, output `1.25`, cached input `0.02`
  - `gpt-5.3-codex`: input `1.75`, output `14.00`, cached input `0.175`
- Production GPT usage models at seed time:
  - `gpt-5.5`: `8486` events
  - `gpt-5.4`: `695` events
  - `gpt-5.4-mini`: `122` events
- Verification after seed:
  - `GET /`: HTTP 200
  - `GET /cpa-usage/healthz`: HTTP 200
  - `GET /cpa-usage/`: HTTP 200
  - `GET /usage/healthz`: HTTP 404
  - Login: HTTP 204
  - `GET /cpa-usage/api/v1/auth/session`: authenticated
  - `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour`: HTTP 200
  - `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=day`: HTTP 200
  - `GET /cpa-usage/api/v1/status`: HTTP 200

## Auth Base Path Hotfix

- Issue: browser users on `/cpa-usage/login` could bounce between `Checking session...` and login redirect because the frontend router was not mounted with the `/cpa-usage` base path.
- Fix commit: `0572ad9f2890`
- Image: `cpa-usage:0572ad9f2890`
- Image ID: `sha256:399c0494bc390c75b451acd4af5a3aee92fde6e09b009e27d2e3fc226b113059`
- Deployment time: 2026-05-17 23:58 Asia/Shanghai
- Frontend bundle after deploy: `./assets/index-DU8wGBLX.js`
- Verification:
  - container image is `cpa-usage:0572ad9f2890`
  - container health is `healthy`
  - automated smoke passed for root, health, login, session, hour/day analytics, status, and stopped keeper path
  - browser check on `https://cpa.maxtap.net/cpa-usage/login` shows the `Sign in` form with links under `/cpa-usage`

## Ownership Evidence

- `cpa-usage` is the only running usage consumer.
- `cpa-usage-keeper` is stopped and `/usage/healthz` is no longer served.
- `cliproxyapi` remains running:
  - container ID: `486bac4156c60c46800a7d9d65dd26dbc6dd9d34efde81d48a19875e0c110bb0`
- `postgres` remains running healthy:
  - container ID: `7fd82d3c99b5923c8186d878ee51e2488045ad573c2b2242ac18d00ef1438a72`

## Recovery Decision

- Recovery needed: no
- Recovery source if needed later: keeper backup tarball plus corrected `cpa-usage` image
- Follow-up: merge the compose overlay into Dokploy-managed compose configuration or keep the overlay path documented for future `docker compose -f` operations.

## Compatibility Decision

This cutover intentionally ends the old `/usage` dashboard path. CPA root service, `cliproxyapi`, and `postgres` remain compatible and unchanged.
