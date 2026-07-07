# Usage Rollup read model and backfill

Usage Intelligence will use an hourly Usage Rollup read model for selected-window analytics instead of treating raw `usage_events` as the repeated first-screen aggregation source. The rollup lives in the existing repository layer; API handlers and service use cases receive analytics read models without choosing between raw and rollup sources.

## Decision

- Store hourly rollups keyed by bucket start, provider, model, auth type, auth index, and API key identity.
- Store request counts, success and failure counts, token sums, latency sum, latency sample count, and last event timestamp in rollup rows.
- Do not store Cost in rollups. Cost is calculated at read time from current Cost Rates so Reference Data edits immediately change historical interpretation.
- Do not store Key Alias or API Key presentation labels in rollups. Rollup rows keep identity keys and analytics reads enrich them from current Reference Data.
- Rebuild affected hourly buckets idempotently from raw facts after usage event insertion, in the same persistence transaction as the raw event write path.
- Create schema objects during migration, but run historical backfill outside schema migration so application startup is not blocked by a long rebuild.
- Track backfill progress in a persistent `usage_rollup_backfill_states` row with pending, running, completed, and failed states, target bucket, covered bucket, timestamps, and last error.
- Expose backfill batch-hours, idle-interval, and error-backoff configuration before enabling the runner so deployment defaults are reviewable before historical rebuild work begins.
- Allow rollup analytics reads only when the requested window is covered by completed backfill plus ingestion-maintained later buckets.
- The only raw analytics fallback is `backfill_incomplete`, and it must be logged with the requested window and reason.
- Do not introduce a new top-level `internal/analytics` package. Keep persistence and read-model selection in `internal/repository`, use-case assembly in `internal/service`, HTTP contracts in `internal/api`, and first-screen orchestration in `web/src/features/usage-intelligence`.

## Consequences

- First-screen analytics can move off repeated raw-event scans while preserving existing Cost, Metric Completeness, Cache Read Share, provider filtering, and Reference Data semantics.
- Backfill is observable through operations status and logs rather than through a Usage Intelligence UI surface.
- The first foundation slice establishes the backfill configuration and status contract; the historical backfill runner consumes that configuration in the later backfill slice.
- Raw `usage_events` remain the source for Request Evidence and compatibility paths that are explicitly outside the rollup coverage decision.
- The repository gains more read-model complexity, but the complexity is localized behind one persistence seam and can be tested against raw-event expectations.

## Compatibility

This is an additive read-model and status-surface decision. Existing ingestion semantics, Cost Rate storage, Key Alias semantics, Usage Intelligence response fields, Request Evidence, Live Capacity, full Usage Overview, authentication, and deployment topology remain compatible.

Unsupported or uncovered states do not silently select arbitrary fallbacks. The migration window permits only the observable `backfill_incomplete` raw fallback for analytics windows that are not yet covered.
