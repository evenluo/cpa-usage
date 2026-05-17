# CPA Usage Final Rollout Readiness Plan

Date: 2026-05-17

## Conclusion

CPA Usage is locally buildable and smoke-tested with `web-v2/` as the only frontend source, but it is not ready for final maxtap cutover yet.

The final goal is for this repository's `cpa-usage` service to become the single production owner of CPA Usage. The current `/usage` keeper should be backed up, migrated from if useful, and taken offline directly. The blocking gaps are deployment topology, data continuity, and Redis queue ownership: the current maxtap Dokploy compose directory `/etc/dokploy/compose/cpa-cliproxyapi-hazmcp/code` no longer contains a `cpa-usage` service, public `/cpa-usage` routes currently return 404, and the existing `/usage` keeper is still the active usage consumer.

## Current Evidence

- Local `make verify` passes after moving all frontend build, lint, typecheck, and Docker paths to `web-v2/`.
- Local `make verify-docker` builds image `cpa-usage:ci`.
- Local container smoke with dummy CPA settings returns:
  - `GET /healthz`: 200, `{"status":"ok"}`
  - `GET /`: 200 HTML
- Maxtap compose checksum: `3aaf19c66c36e1ed947b248828f617bb2ab817d41f89f9cefd8fee7e5cc4a71a`
- Maxtap compose services: `postgres`, `cliproxyapi`, `cpa-usage-keeper`.
- Maxtap public checks:
  - `https://cpa.maxtap.net/usage/healthz`: 200
  - `https://cpa.maxtap.net/`: 200
  - `https://cpa.maxtap.net/cpa-usage/healthz`: 404
  - `https://cpa.maxtap.net/cpa-usage/`: 404

## Rollout Decision

Use a direct cutover to this repository's `cpa-usage` service.

The final production state should have exactly one CPA usage consumer: `cpa-usage`. The existing `cpa-usage-keeper` is only a migration source; it does not need to remain available as a rollback target and should stop consuming Redis usage events before the new service goes live.

Compatibility decision: this rollout is intentionally incompatible with the old local `web/` source and with keeping `/usage` as a production usage dashboard. It remains compatible with CPA root service and PostgreSQL-backed CPA runtime. Temporary `/usage` downtime is acceptable after the keeper is stopped and before this project is online. The cutover must explicitly preserve usage history and Reference Data backups before the keeper is stopped.

## Gap Task Plan

### Task 0: Decide And Execute Data Continuity

Priority: P0

Goal: Preserve usable keeper data before taking the old consumer offline.

Required work:

- Inspect the current `cpa-usage-keeper` data volume and identify the SQLite database path.
- Create a keeper database backup before any service mutation.
- Decide whether to migrate the existing SQLite database directly or export/import selected tables.
- Preserve at minimum:
  - `usage_events`
  - `redis_usage_inboxes`, if needed for unprocessed or retryable local inbox rows
  - `usage_identities`
  - `key_aliases`
  - `model_price_settings`
  - migration metadata tables
- Stop `cpa-usage-keeper` after the backup has been verified, so it no longer consumes Redis usage events.
- Run migrations with the new binary against the migrated database.
- Verify row counts and representative records for Usage Events, Key Aliases, and Cost Rates.
- Confirm no plaintext secrets are copied into the repo or rollout report.

Acceptance checks:

- A timestamped keeper DB backup exists before cutover.
- `cpa-usage-keeper` is stopped after backup verification and before new service ingestion starts.
- The migrated `cpa-usage` database opens successfully under the new image.
- Usage Intelligence returns non-empty historical data for the expected recent windows.
- Reference Data shows existing Key Aliases and Cost Rates after migration.
- Any skipped table is listed with a reason.

### Task 1: Restore The `/cpa-usage` Deployment Slot

Priority: P0

Goal: Add the replacement maxtap `cpa-usage` service and prepare it to become the only usage dashboard.

Required work:

- Create a maxtap-specific compose patch or deployment note that adds service `cpa-usage`.
- Set `APP_BASE_PATH=/cpa-usage`.
- Set `CPA_BASE_URL=http://cliproxyapi:8317`.
- Set `REDIS_QUEUE_ADDR=cliproxyapi:8317`.
- Enable auth with the intended production login policy.
- Mount the migrated authoritative data volume at `/data`.
- Attach the service to `cliproxyapi-internal` and `dokploy-network`.
- Add Traefik routers for `Host(cpa.maxtap.net) && PathPrefix(/cpa-usage)`.
- Remove or disable `cpa-usage-keeper` in compose; do not keep it as a rollback service.

Acceptance checks:

- `docker compose config --services` includes `cpa-usage`.
- `docker compose up -d --no-deps cpa-usage` creates or recreates only `cpa-usage` during validation.
- `docker compose ps cpa-usage` reports healthy.
- `cpa-usage-keeper` is already stopped or removed before `cpa-usage` starts ingesting.
- New `/cpa-usage` smokes pass.
- `cliproxyapi` and `postgres` container IDs are unchanged after cutover.

### Task 2: Decide Image Source And Build Path

Priority: P0

Goal: Make the deploy source deterministic before touching maxtap.

Required work:

- Choose one deployment path:
  - GHCR image path: build and push `ghcr.io/evenluo/cpa-usage:<commit-sha>` and deploy that immutable tag.
  - Maxtap local build path: restore a git-backed source checkout on maxtap and build from a pinned commit.
- Do not deploy from a mutable local working tree.
- Record the deployed commit SHA in the rollout record.

Acceptance checks:

- The image tag or local build commit exactly matches the reviewed commit.
- `docker image inspect` on maxtap records image ID and creation time before and after rollout.
- Rollback can target the previous image ID or previous immutable tag.

### Task 3: Write A Maxtap Cutover Runbook

Priority: P0

Goal: Convert the migration, deploy, validation, and keeper shutdown sequence into copyable, low-risk steps.

Required work:

- Add a runbook under `docs/deploy/` for the final maxtap cutover.
- Include preflight commands:
  - compose checksum
  - service list
  - container IDs and image IDs
  - keeper data volume and database path
  - public `/`, `/usage/healthz`, `/cpa-usage/healthz`, and `/cpa-usage/` checks
- Include database backup and migration commands.
- Include commands that stop or remove `cpa-usage-keeper` after backup verification.
- Include validation commands for the new service after keeper has been stopped.
- Do not include keeper rollback commands; recovery should use database backup and a fixed `cpa-usage` deploy.
- Include post-deploy smoke commands.

Acceptance checks:

- A reviewer can execute the runbook without inferring paths, service names, or URLs.
- The runbook explicitly says not to restart `cliproxyapi` or `postgres` unless evidence implicates them.
- The runbook defines the exact point where `cpa-usage-keeper` stops consuming Redis usage events and states that `/usage` downtime is acceptable until `cpa-usage` is online.

### Task 4: Add Repeatable Smoke Verification

Priority: P0

Goal: Make final acceptance mechanical instead of manual.

Required work:

- Add a small smoke script for local and maxtap checks.
- Check unauthenticated endpoints:
  - `/cpa-usage/healthz`
  - `/cpa-usage/`
  - `/`
- Check authenticated CPA Usage endpoints after login:
  - `/cpa-usage/api/v1/auth/session`
  - `/cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour`
  - `/cpa-usage/api/v1/analytics/summary?range=7d&granularity=day`
  - `/cpa-usage/api/v1/status`
- Redact cookies and passwords from output.

Acceptance checks:

- Script exits non-zero on HTTP failure, missing JSON fields, or failed login.
- Script output is safe to paste into a rollout report.
- Script can confirm keeper is stopped or absent after final cutover.
- Script does not require `/usage/healthz` to stay healthy after keeper shutdown.

### Task 5: Document Production Environment Values

Priority: P1

Goal: Prevent `.env.example` from being mistaken for maxtap truth.

Required work:

- Add a redacted maxtap environment matrix to the rollout runbook.
- Record required values and sources:
  - `APP_BASE_PATH=/cpa-usage`
  - `CPA_BASE_URL=http://cliproxyapi:8317`
  - `REDIS_QUEUE_ADDR=cliproxyapi:8317`
  - `AUTH_ENABLED=true`
  - `LOGIN_PASSWORD=<secret>`
  - `WORK_DIR=/data` or equivalent container path
  - backup/log retention values
- Keep secret values redacted.
- Record old keeper env values only as migration evidence, not as future production truth.

Acceptance checks:

- The runbook distinguishes `.env.example` defaults from maxtap production values.
- No plaintext secret is committed.
- The future production env matrix contains `cpa-usage`, not `cpa-usage-keeper`, as the usage owner.

### Task 6: Add Final Rollout Report Template

Priority: P1

Goal: Keep the final launch evidence consistent with the previous maxtap rollout report.

Required work:

- Add a template covering:
  - deployed commit
  - image ID
  - compose checksum before and after
- `cliproxyapi` and `postgres` container IDs before and after
  - keeper shutdown/removal evidence
  - database backup and migration evidence
  - public smoke results
  - authenticated smoke results
  - recovery decision based on the keeper database backup
  - compatibility decision

Acceptance checks:

- The report can be filled during rollout without editing structure.
- It has an explicit section proving this service became the only usage consumer.
- It records that `/usage` downtime was intentional once keeper was stopped.

## Suggested Execution Order

1. Complete Task 0 first; backup and keeper shutdown are the cutover gate.
2. Complete Task 1 and Task 2 together because the compose service needs a real image/build source.
3. Complete Task 3 before remote mutation.
4. Complete Task 4 before final deploy so the same script verifies preflight and post-deploy.
5. Complete Task 5 and Task 6 before merge or deployment approval.
6. Execute final maxtap cutover only after `make verify`, `make verify-docker`, smoke script dry-run, data migration rehearsal, and runbook review pass.

## Plan Review

Review conclusion: actionable after data continuity, deployment topology, and image source decisions are made explicit in Tasks 0, 1, and 2.

Issues found during review:

- The current maxtap state does not have a `cpa-usage` service, so a simple rebuild command would be invalid.
- The current `/usage` keeper is the active usage consumer; it must be stopped before this service starts ingesting because running both services against the same Redis queue would split Usage Events.
- A clean new `cpa-usage` volume would not preserve historical Usage Events, Key Aliases, or Cost Rates.
- The repo has a generic `docker-compose.example.yml`, but not a maxtap-specific rollout artifact.
- The previous rollout report is useful historical evidence, but it is stale because maxtap service topology changed by 2026-05-17.

Corrections applied in this plan:

- The first task is now data backup, keeper shutdown, and ownership, not service creation.
- The service creation task now leads to direct replacement, not long-term coexistence or keeper rollback.
- The plan requires immutable image/build provenance before rollout.
- The cutover protects `cliproxyapi` and `postgres`, backs up and migrates from `/usage`, stops or removes `cpa-usage-keeper`, and then brings `cpa-usage` online as the only usage consumer.
