# Performance budget

Status: current, local stage 1 and stage 2 governance, 2026-09-08.

Owners: repository owns exact raw/rollup reads and atomic writes; service and poller own durable inbox processing; app owns HTTP/runtime observability and maintenance; the dashboard owns demand and refresh cadence.

## Scope and invariants

The authorized goal is to add sample-density performance charts while reducing redundant frontend demand and database occupancy under ingestion, reads and maintenance. Existing historical retention, event-key deduplication, separate inbox-pop attempt identities, late arrivals and atomic raw/rollup consistency remain unchanged. Response additions are compatible: existing routes and fields remain available; the dashboard consumes two new narrow endpoints. Exact percentiles still come from valid raw samples. No API read pool, TTL cache, asynchronous rollup, upstream pull backpressure, new queue, scheduler or retention policy is introduced.

## Stage 1: observe and control demand

`GET /metrics` keeps its original fields and adds bounded aggregate measurements:

- `database.pool`: open/in-use connections and cumulative wait count/seconds from `sql.DBStats`.
- `database.storage`: database and WAL byte sizes, with explicit availability. No paths are returned; an absent WAL is a real zero.
- `runtime.measurements`: cumulative allocations, current heap allocation, GC cycles and cumulative pause seconds.
- `http_requests`: method plus Gin route-template counts, errors, cancellations, cumulative duration and a fixed duration histogram. The p95 is a **bucket interval**, not an exact latency estimate. There are at most 128 series; excess observations increment the dropped counter. Raw URLs, query strings and identities are excluded.
- Inbox pending count plus oldest pending age share a single statement using the existing processable partial index. The age distinguishes `available`, `empty` and `unavailable`. Existing processed totals/rate remain ingestion signals; refreshing a page is not ingestion proof.

The production metrics above expose queueing and request duration. They do not claim to measure SQL transaction duration: the repeatable workload below measures whole ingestion and maintenance operations, which include connection waits and decoding as well as transactions.

Queries propagate AbortSignal, retain a 60-second freshness/refresh cadence for foreground dashboard data, and refetch on focus only when stale. Existing explicit mutation invalidations remain authoritative. TanStack Query's default background interval behavior is retained; hidden tabs do not receive a newly invented timer system.

`/usage/performance/providers` returns the complete fixed-24h provider catalog ranked by count then name. Its source planner uses valid hourly rollups and raw boundary hours without loading pricing or unrelated analytics. The selected provider remains stable on refresh. `/usage/model-mappings/summary` returns folded-card totals; expanded details use the existing endpoint and pin both summary and details to the summary's exact `window_end`. Closing the section restores the latest summary and disables detail fetching; reopening pins a new snapshot. Live Capacity starts identity reads when its section approaches the viewport and stays enabled after its first activation to preserve interactive refresh/task state.

## Stage 2: reduce database occupancy

Only genuinely inserted events contribute to hourly UPSERT deltas. All additive counts, token-quality/accounting measures and latency measures update in the same transaction as raw events. `last_event_at` uses the maximum timestamp, so late arrivals do not move it backwards. Replaying one inbox row remains deduplicated; identical payloads from distinct pops remain separate attempts.

Historical backfill aggregates with `INSERT ... SELECT ... GROUP BY` in SQLite instead of loading the complete bucket's raw events into Go. The existing 24-hour backfill batch and checkpoint semantics remain unchanged. Parity tests compare incremental results with a full rebuild, including boundaries, quality states and late arrivals. SQL dimension normalization uses the complete Unicode White_Space set to preserve Go TrimSpace semantics for historical rows, rather than SQLite's ASCII-space-only default.

The processor may consume four consecutive full batches before yielding for the existing five-second interval. A short batch, warning or failure ends the burst. `BatchLimitReached` means the returned batch filled its limit, not proof that more rows exist. Pull behavior is unchanged.

Cleanup checks actual SQLite freelist pages before VACUUM; a no-delete run can still reclaim previously freed pages. File-backed online backups open a dedicated temporary read-only source connection, releasing the application's one-connection pool for request and ingest work. This is solely a backup connection, not an API read pool. In-memory databases retain the supplied handle. Existing backup permissions, atomic completion, cancellation and restoration behavior remain covered.

## Reproducible evidence

`make benchmark-performance-budget` is manual evidence, separate from CI and without hardware-dependent pass/fail timing thresholds. Run comparisons sequentially on the same idle host and unchanged fixture; fixture construction is outside timing.

`TestPerformanceBudgetMixedWorkload` runs real HTTP handlers for analytics, exact performance and request evidence concurrently with durable inbox pull/process, a full-day backfill and an online backup. It asserts successful responses, raw/rollup count equality and a drained inbox. It starts no remote clients or background runners. The benchmark uses 32,768 valid canonical seed attempts, four 250-attempt ingestion batches, and three requests to each of three endpoints. Its nine-observation nearest-rank HTTP p95 is their maximum; DB wait is the sum across concurrent operations and can exceed elapsed time. B/op counts all concurrently running workload goroutines.

Local Apple M4 / Go darwin-arm64 / `GOMAXPROCS=1`, three independent `-benchtime=1x` runs, medians per column:

| Equal-work measurement | Baseline `b8ec07e` | Candidate | Interpretation |
| --- | ---: | ---: | --- |
| Insert 1,000 attempts into an hour containing 32,768 | 307.189 ms | 36.498 ms | 88.1% lower operation time |
| Same insert, allocations | 154,860,896 B | 8,510,872 B | 94.5% fewer allocated bytes |
| Rebuild 24 hours / 32,768 attempts | 286.760 ms | 143.387 ms | 50.0% lower operation time |
| Same rebuild, allocations | 129,865,024 B | 32,872 B | Raw rows stay inside SQLite |
| Complete mixed workload | 1,769.716 ms | 1,598.413 ms | 9.7% lower elapsed time |
| Mixed HTTP p95 (nine observations) | 1,652 ms | 1,524 ms | Small-sample diagnostic only |
| Mixed four-batch ingestion | 1,616 ms | 1,523 ms | 5.8% lower elapsed time |
| Mixed allocated bytes | 654,180,568 B | 499,611,976 B | 23.6% lower allocations |
| Mixed cumulative connection wait | 4,977 ms | 4,966 ms | Ranges overlap; no stable wait reduction claimed |
| Exact high-cardinality performance read | 380.410 ms | 357.325 ms | Ranges overlap; histogram does not demonstrate a speedup |
| Same exact performance read, allocations | 22,895,928 B | 22,917,224 B | Approximately 0.1% more allocated bytes |

The performance-read baseline and candidate share the corrected canonical accounting fixture. Its original archival-token-only fixture failed its pre-timer validity guard after the accounting direct cut; that fixture repair is included in this change. The 65,536-attempt fixture spans 48 hours and selects 32,760 attempts. SQL count remains 13 for a populated provider-scoped read; bins are constructed in existing sample passes.

The mixed workload deliberately keeps the same nine HTTP reads on both revisions, so it isolates backend changes and does not credit fewer frontend requests. Read-side raw projection/sort costs still dominate this mix: reducing insertion and backfill cost does not imply the same percentage reduction for complete pages. Bytes are Go allocations, not process RSS or SQLite native allocation. DB wait is cumulative queue time, not transaction duration. No universal latency budget is inferred from three local runs.

These are local synthetic evidence, not authenticated production behavior, release proof or a production SLA.

## Completion checks

- Full backend tests and `go vet` passed on the integrated tree. The subsequent backup filename boundary fix also passed the affected four-package check.
- Frontend dependency install, ESLint, all 47 unit-test files (397 tests), TypeScript and production build passed. One additional regression test was then added, and its five-test chart suite passed in full.
- The complete 69-case Playwright suite across mobile/tablet/desktop initially passed 51 cases. All 18 failures were corrected, then all 33 cases in their four affected specs passed. Failures covered the explicit scroll-before-read lazy contract and focused heatmap details surviving adjacent layout changes; no failed case was waived.
- Real embedded Go UI with 1,200 canonical synthetic attempts was inspected at desktop, 390px mobile and 768px tablet sizes, including dark mode, full histogram tables and no horizontal overflow or console errors.
- Focused race tests cover route counters, inbox metrics and concurrent backfill/ingestion. Incremental, Go rebuild and SQL rebuild match across quality states, replay, late arrivals and Unicode whitespace. Mixed-load metrics readback confirms three observed HTTP routes, no request errors, available pool/runtime data and an empty inbox.
- Independent integration review closed its snapshot and whitespace findings and found no remaining material issue. Final diff whitespace validation passed.
