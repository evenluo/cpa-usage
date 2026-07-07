# Self-Hosted CPA Usage Cutover Runbook

Date:

## Goal

Move a self-hosted CPA Usage deployment to this repository's `cpa-usage` service while preserving usage history and avoiding plaintext secrets in git.

## Deployment Context

Fill these values before running the cutover:

- Compose directory: `<compose-directory>`
- Public host: `https://<your-cpa-host>`
- Public app path: `/usage`
- Existing keeper service name: `<old-usage-service>`
- Existing keeper data volume: `<old-usage-data-volume>`
- New app data volume: `<cpa-usage-data-volume>`
- CPA backend service DNS name: `<cpa-backend-service>`
- Backup directory: `<backup-directory>`

## Preflight

```sh
cd <compose-directory>
sha256sum docker-compose.yml
docker compose config --services
docker compose ps <cpa-backend-service> <old-usage-service> cpa-usage || true
docker ps -a --format '{{.Names}}|{{.Image}}|{{.Status}}' | grep -E 'cpa-usage|<cpa-backend-service>|<old-usage-service>' || true
```

Public checks:

```sh
curl -k -i https://<your-cpa-host>/
curl -k -i https://<your-cpa-host>/usage/healthz || true
curl -k -i https://<your-cpa-host>/usage/ || true
```

## Stop Old Keeper And Back Up Data

Run this only during the approved cutover window.

```sh
cd <compose-directory>
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup_dir=<backup-directory>/$stamp
mkdir -p "$backup_dir"

docker compose stop <old-usage-service>

keeper_data=/var/lib/docker/volumes/<old-usage-data-volume>/_data
tar -C "$keeper_data" -czf "$backup_dir/cpa-usage-keeper-data.tgz" .
sha256sum "$backup_dir/cpa-usage-keeper-data.tgz" > "$backup_dir/SHA256SUMS"
cat "$backup_dir/SHA256SUMS"
docker compose ps <old-usage-service> || true
```

After this point, `/usage` downtime is expected until `cpa-usage` is online.

## Prepare New Data Volume

Use the keeper backup as the migration source unless you have a reviewed export/import plan.

```sh
backup=<backup-directory>/<stamp>/cpa-usage-keeper-data.tgz
target=/var/lib/docker/volumes/<cpa-usage-data-volume>/_data

mkdir -p "$target"
tar -C "$target" -czf "<backup-directory>/pre-new-volume-$(date -u +%Y%m%dT%H%M%SZ).tgz" . 2>/dev/null || true
rm -rf "$target"/*
tar -C "$target" -xzf "$backup"
```

Do not commit copied database files or secrets to the repository.

## Add New Service

Use [deploy/example/cpa-usage.cutover.compose.yml](../../deploy/example/cpa-usage.cutover.compose.yml) as a starting point.

Set runtime secrets outside git:

```sh
export CPA_USAGE_IMAGE=ghcr.io/evenluo/cpa-usage:<immutable-commit-sha-tag>
export PUBLIC_HOST=<your-cpa-host>
export CPA_SERVICE_URL=http://<cpa-backend-service>:8317
export REDIS_QUEUE_ADDR=<cpa-backend-service>:8317
export MANAGEMENT_PASSWORD='<secret>'
export CPA_USAGE_LOGIN_PASSWORD='<secret>'
export AUTH_SESSION_SECRET='<secret>'
```

Run the overlay or merge it into your compose file:

```sh
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml config
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml up -d cpa-usage
docker compose -f docker-compose.yml -f cpa-usage.cutover.compose.yml ps cpa-usage
```

If the overlay is merged into `docker-compose.yml`, use plain `docker compose up -d cpa-usage`.

## Smoke

From a trusted machine:

```sh
BASE_URL=https://<your-cpa-host> \
BASE_PATH=/usage \
CPA_USAGE_LOGIN_PASSWORD='<secret>' \
scripts/smoke-cpa-usage.sh
```

The smoke script prints `time_total` for the optimized Usage Intelligence paths:

- `analytics core` should represent the first useful KPI/trend read without the fixed heatmap payload.
- `activity heatmap` should be checked independently because it was one of the observed production slow SQL symptoms.
- `legacy analytics summary` proves compatibility for bookmarked or scripted callers that still request the full summary.
- `request health`, `request evidence events`, and `status` prove the remaining first-screen and operations paths.

Compare these timings against the production symptoms by looking for the old shape to disappear: core dashboard timing should no longer track heatmap latency, heatmap timing should be isolated to its own line, and status/events/request-health should stay independently visible even if one analytics path is slower.

Manual checks:

```sh
curl -k -i https://<your-cpa-host>/usage/healthz
curl -k -i https://<your-cpa-host>/usage/
curl -k -i https://<your-cpa-host>/
```

Expected:

- `/usage/healthz`: 200
- `/usage/`: 200 HTML or login shell
- `/`: 200 CPA root response

## Seed OpenAI GPT Pricing

Seed GPT Cost Rates after the service has started and migrations have completed. The seed script is idempotent and only upserts the listed GPT rows in `model_price_settings`; it does not remove operator-managed prices for other models.

Verify current pricing sources before changing values:

- `https://openai.com/api/pricing/`
- `https://developers.openai.com/api/docs/pricing`

On the deployment host:

```sh
scp scripts/seed-openai-gpt-pricing.sh <deployment-host>:/tmp/seed-openai-gpt-pricing.sh

ssh <deployment-host>
cd <compose-directory>
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup=<backup-directory>/pre-gpt-pricing-$stamp.tgz
data_dir=/var/lib/docker/volumes/<cpa-usage-data-volume>/_data
tar -C "$data_dir" -czf "$backup" .

docker run --rm \
  -v <cpa-usage-data-volume>:/data \
  -v /tmp/seed-openai-gpt-pricing.sh:/seed-openai-gpt-pricing.sh:ro \
  alpine:3.20 \
  sh -lc 'apk add --no-cache sqlite >/dev/null && sh /seed-openai-gpt-pricing.sh /data/app.db'
```

## Recovery

Do not restart the old keeper as a normal rollback path after the new service starts ingesting. Recovery should use:

- the keeper backup tarball,
- a corrected `cpa-usage` image,
- and a redeploy of `cpa-usage`.

Only touch adjacent infrastructure services if direct evidence points to them.
