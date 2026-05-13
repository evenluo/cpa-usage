# Usage Intelligence Maxtap Rollout Report

Date: 2026-05-14

Scope: PRD #26 child issue #32, rollout of the refined Usage Intelligence dashboard to the maxtap `/cpa-usage` service.

## Deployment Source

- Repository: `evenluo/cpa-usage`
- Base before PRD integration: `72e178b446eca99fdc0cb5408b3fa0b228b63576`
- Deployed application commit: `100feeb02ca8a4ca5cb263e422f78277607b1587`
- Deployment target: maxtap Dokploy compose directory `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code`
- Service rebuilt and restarted: `cpa-usage`

## Baseline Before Final Rebuild

- Compose checksum: `0c054278a7e047a7135d385e01182bb13f648e202576c7b0cd7a9a953f257240  docker-compose.yml`
- Previous `cpa-usage:main` image: `sha256:ee6bb553e2b6fe9a3d0583cc02bf2ee60c69eb7487c675a3bc91ea69643b6b59`
- Previous image created at: `2026-05-14T02:24:53.962762089+08:00`
- Previous `cpa-usage` container status: healthy
- Baseline public smokes:
  - `https://cpa.maxtap.net/cpa-usage/healthz`: 200
  - `https://cpa.maxtap.net/cpa-usage/`: 200
  - `https://cpa.maxtap.net/usage/healthz`: 200

## Rebuild And Restart

Command shape:

```sh
cd /etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code
docker compose build --no-cache cpa-usage
docker compose up -d --no-deps cpa-usage
docker compose ps cpa-usage
```

Build evidence:

- Git source loaded by Docker build: `100feeb02ca8a4ca5cb263e422f78277607b1587 refs/heads/main`
- Only `cpa-usage` was recreated by compose output.
- New `cpa-usage:main` image: `sha256:36447344e1f38ea7984e3220491a041168ebc2ea9bd57cc9a2b465cd915fddfb`
- New image created at: `2026-05-14T02:34:33.425156907+08:00`
- New `cpa-usage` container status: healthy
- Compose checksum after deploy: `0c054278a7e047a7135d385e01182bb13f648e202576c7b0cd7a9a953f257240  docker-compose.yml`

## Protected Services

The rollout did not recreate or restart these services:

- `cliproxyapi`: created `2026-05-11T01:09:14.205056203Z`, started `2026-05-11T01:09:15.070800112Z`, running
- `cpa-usage-keeper`: created `2026-05-12T08:27:45.45747738Z`, started `2026-05-12T08:27:46.216534897Z`, running healthy
- `postgres`: created `2026-04-24T12:41:15.883474941Z`, started `2026-04-24T12:51:08.441410762Z`, running healthy

## Post-Deploy Smoke

Public smokes:

- `https://cpa.maxtap.net/cpa-usage/healthz`: 200, `{"status":"ok"}`
- `https://cpa.maxtap.net/cpa-usage/`: 200 HTML
- `https://cpa.maxtap.net/usage/healthz`: 200, `{"status":"ok"}`
- `https://cpa.maxtap.net/`: 200, CPA root JSON

Authenticated analytics smokes:

- Login: 204
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour`: 200
  - `granularity=hour`
  - `trend_len=3`
  - `heatmap_rows=8`
  - `has_summary=true`
  - `cost_status=unavailable`
- `GET /cpa-usage/api/v1/analytics/summary?range=7d&granularity=day`: 200
  - `granularity=day`
  - `trend_len=1`
  - `heatmap_rows=8`
  - `has_summary=true`
  - `cost_status=unavailable`
- Logout: 204

## Rollback Decision

Rollback was not required. If rollback had been needed, the recorded previous image was `sha256:ee6bb553e2b6fe9a3d0583cc02bf2ee60c69eb7487c675a3bc91ea69643b6b59`, and rollback scope would have remained limited to `cpa-usage` unless evidence implicated another service.

## Compatibility Decision

The rollout kept the existing `/usage` service, CPA root service, compose file, and login password unchanged. The analytics contract change remains additive, except unsupported `granularity` values intentionally fail with a 400 response rather than falling back.

Closes #32
