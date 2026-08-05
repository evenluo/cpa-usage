# Expose a public runtime metrics endpoint

CPA Usage exposes `GET /metrics` next to `/healthz` as an unauthenticated runtime snapshot endpoint. The endpoint returns aggregated numbers and status strings about queue consumption, rollup backfill, database backup, and process uptime, without request detail or identity information.

## Decision

- Serve `/metrics` on the app base path, outside the auth-protected `/api/v1` group, at the same visibility level as `/healthz`.
- The snapshot contains only aggregate counters, boolean runner states, status strings, and timestamps. It never includes raw usage events, API keys, aliases, or per-identity rows, so it carries no data that the protected workspace protects.
- `redis_inbox_pending` counts `pending` plus `process_failed` inbox rows, matching the retryable backlog definition used by the process loop.
- Redis inbox processing throughput is exposed as cumulative counters (`redis_events_processed_total`, `redis_events_processed_batches_total`, `redis_events_last_processed_at`) plus a derived `redis_events_processing_rate_per_minute` computed from the delta between adjacent snapshot requests. Failed batches and empty batches (no rows to process) are not counted, so retries and idle polling do not inflate throughput or keep `redis_events_last_processed_at` advancing while idle.
- Snapshot values are read on demand from runner state and lightweight repository queries rather than from pre-computed expvar counters, keeping the endpoint always consistent with the current process state. The per-request cost is two indexed COUNT/SELECT queries at most.
- If either database-backed read fails, the endpoint still returns the process-local portion of the snapshot and sets `db_unavailable: true`; the endpoint itself only returns an error when the metrics provider is absent or the process state is unreachable, so a database outage degrades the snapshot instead of hiding process health.
- Boolean runner states use JSON booleans (`poller_running`), matching the shape of the protected `/api/v1/status` response rather than introducing a second boolean encoding.
- Operators who want to hide the endpoint entirely can place the service behind a reverse proxy with basic auth or path rules; the service itself does not add a second auth surface.

## Consequences

- Monitoring systems and container probes can scrape runtime health without a session, including the Docker HEALTHCHECK pattern already used for `/healthz`.
- The endpoint duplicates a subset of `/api/v1/status` information; that duplication is accepted because the protected status response remains the operator-facing workspace surface, while `/metrics` is the machine-readable scrape surface.
- Snapshot-based rate derivation requires a scrape interval; a single request after a long idle period reports a rate over the full idle span, and the first request after startup reports no rate until the second sample.

## Compatibility

This is an additive endpoint decision. Existing API contracts, auth behavior, the protected status surface, ingestion semantics, and deployment topology remain compatible. The endpoint is new externally observable behavior and is deliberately unauthenticated; deployments that require authenticated access must enforce it at the reverse proxy.
