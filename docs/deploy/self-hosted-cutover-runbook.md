# Self-Hosted CPA Usage Cutover Runbook

Status: current migration and Accounting v2 cutover runbook; routine releases remain owned by the release chain

Routine release SoT: [Dokploy Release Chain](dokploy-release.md)

Date:

## Accounting v2 cutover

For an existing published CPA Usage installation, use this section with the [Dokploy release chain](dokploy-release.md). The keeper-to-CPA-Usage migration below is a separate operation and is not required for this upgrade. The supported database source is published main, not an unpublished intermediate PR build.

1. Prepare the intended CPA image before the maintenance window. Set its Dokploy `CLIPROXYAPI_IMAGE` to an explicit verified Accounting v2 tag or digest; the CPA compose template requires this setting. The tested contract is CPA v7.2.152, described in [CPA data contract](../design/cpa-data-contract.md). Verify that version's offline fixture or a non-production queue capture, including both versions, canonical buckets and explicit execution flags. Do not pop the production queue merely to inspect a payload.
2. Before restarting CPA, pause ordinary requests and let in-flight requests finish. The published CPA queue is in memory: restarting it with unconsumed records loses those attempts. Keep the published consumer running and use its existing authenticated `POST /api/v1/sync` to process real queued work. Repeat at least one second apart until HTTP 200 reports `last_status: "empty"`, `sync_running: false`, and no `last_error`/`last_warning`. With traffic paused, this observes both an empty producer pop and finished local processing. Errors or warnings do not establish drainage.
3. Through a read-only SQLite connection, confirm `SELECT COUNT(*) FROM redis_usage_inboxes WHERE status IN ('pending', 'process_failed');` returns zero. The published main UI does not expose this counter. Stop the published consumer and recheck the same query to close the observation/stop race. If nonzero, resume it and finish those rows before proceeding. Take a coherent offline copy of the database directory, including any WAL files, verify the copy with SQLite `PRAGMA integrity_check`, and retain it outside the live volume. Scheduled backups start after migrations and cannot supply this copy.
4. Upgrade the independently managed CPA application to the prepared image, then deploy the integrated CPA Usage artifact through the [existing release chain](dokploy-release.md). Keep ordinary requests paused. Database migrations run before ingestion; the accounting migration refuses a processable inbox without deleting its rows. Historical raw events stay intact and their canonical facts remain absent. Canonical coverage starts with ingestion by the integrated consumer. The four migrations retain their existing transaction and ledger ownership.
5. Verify exact deployed artifacts and terminal deployment results, run authenticated smoke, then send one controlled normal CPA request while ordinary traffic remains paused. Confirm its Request Evidence has valid canonical facts and newly produced messages do not increase the existing inbox `decode_failed` count. A healthy process alone does not prove successful ingestion. Resume ordinary requests after these checks pass. If cutover fails, stop the new consumer and inspect the explicit failure before any restore; never run the published binary against the migrated database as a rollback shortcut.

These are operator steps, not production actions performed by this PR. CPA and CPA Usage are separate applications; deploying CPA Usage does not upgrade CPA.

### Correct existing cache-column constraints

Some existing installations have `usage_events.cache_read_tokens` and `cache_creation_tokens` declared `NOT NULL DEFAULT 0`. Accounting v2 intake leaves these archival scalar fields absent and writes canonical buckets instead. Such installations reject otherwise valid messages with `NOT NULL constraint failed: usage_events.cache_read_tokens` (or `cache_creation_tokens`), then mark them `discarded` after five processing failures. A healthy process does not establish successful ingestion.

The forward migration `20260907_make_usage_event_cache_columns_nullable` corrects these two columns to the existing nullable entity contract. It preserves historical values, event IDs, indexes, triggers, table constraints and the autoincrement high-water mark in the existing migration transaction. Already-nullable databases need no table rebuild. This is a compatible schema correction; it does not change canonical accounting, infer historical tokens, or upgrade CPA.

For an affected installation:

1. Take a coherent SQLite backup outside the live volume, restrict its permissions, and verify `PRAGMA integrity_check` before releasing the correction through the existing release chain.
2. Verify the exact deployed consumer image and migration ledger, check both columns have `notnull = 0` in `PRAGMA table_info(usage_events)`, and confirm new v2 attempts reach `usage_events` with valid canonical facts.
3. Before recovery, take another coherent backup and record the exact retained inbox IDs for this constraint failure. Confirm their persisted JSON still contains the supported v2 envelope. Do not reset unrelated `discarded` or `decode_failed` rows.
4. In one operator-controlled transaction, return only those identified constraint-failure rows that remain `discarded` to `pending`, reset their processing attempt count and error, and let the existing consumer process them. Preserve each row's ID, payload and `popped_at`; these own deterministic replay and the `redis-inbox:<id>` event key. No producer queue pop is needed for recovery.
5. Match every selected ID to its canonical usage event, check affected hourly aggregates and authenticated dashboard reads, and confirm fresh traffic continues to ingest without this failure. Failure rows are retained for only seven days; a deployment alone does not retry discarded rows. Records no longer present in the inbox or a verified backup cannot be reconstructed by this procedure.

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
tar -C "$target" -czf "<backup-directory>/pre-new-volume-$(date -u +%Y%m%dT%H%M%SZ).tgz" .
tar -tzf "$backup" >/dev/null
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

## Seed Model Pricing

Seed model Cost Rates after the service has started and migrations have completed. The seed script is idempotent and only inserts missing rows in `model_price_settings`; existing prices (including custom values and explicit zero prices) are preserved. Run it explicitly against the deployed database: deploying an image does not seed prices.

Verify current pricing sources before changing values:

- `https://openai.com/api/pricing/`
- `https://developers.openai.com/api/docs/pricing`

New entries verified on 2026-09-06 (USD per 1M tokens):

| Model | Input | Output | Cache read |
| --- | ---: | ---: | ---: |
| `gpt-6-astra` | 10.00 | 50.00 | 1.00 |
| `kimi-k3` | 3.00 | 15.00 | 0.30 |

Sources: [GPT-6 Astra](https://developers.openai.com/api/docs/models/gpt-6-astra), [Kimi API](https://platform.kimi.ai/). These are reference token rates. GPT-6 uses Standard pricing for at most 272K input tokens; the current three-rate schema does not represent long-context multipliers, Fast/Batch/Flex tiers, or separate cache-write pricing.

On the deployment host:

```sh
scp scripts/seed-model-pricing.sh <deployment-host>:/tmp/seed-model-pricing.sh

ssh <deployment-host>
cd <compose-directory>
stamp=$(date -u +%Y%m%dT%H%M%SZ)
backup=<backup-directory>/pre-model-pricing-$stamp.tgz
data_dir=/var/lib/docker/volumes/<cpa-usage-data-volume>/_data
tar -C "$data_dir" -czf "$backup" .

docker run --rm \
  -v <cpa-usage-data-volume>:/data \
  -v /tmp/seed-model-pricing.sh:/seed-model-pricing.sh:ro \
  alpine:3.20 \
  sh -lc 'apk add --no-cache sqlite >/dev/null && sh /seed-model-pricing.sh /data/app.db'
```

## Recovery

Do not restart the old keeper as a normal rollback path after the new service starts ingesting. Recovery should use:

- the keeper backup tarball,
- a corrected `cpa-usage` image,
- and a redeploy of `cpa-usage`.

Only touch adjacent infrastructure services if direct evidence points to them.
