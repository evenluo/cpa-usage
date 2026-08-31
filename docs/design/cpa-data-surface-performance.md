# CPA data-surface performance evidence

Status: measured local acceptance evidence for the CPA data-surface expansion

Measured: 2026-08-31

Exact implementation base: `2c255cf1b5bb62f7dd4e0ea9313490bd99fe73d3`

Environment: `darwin/arm64`, Apple M4, Go `1.27.0`, Node `24.13.1`, npm `11.8.0`, `GOMAXPROCS=1` for Go comparisons.

## Decision

The local database is intentionally not treated as a production fixture. Performance is qualified with deterministic synthetic inputs and same-host comparison against the exact implementation base. No production CPA queue is popped, no production SQLite file is copied, and no timing assertion is added to the normal verification or CI path.

The measured paths do not justify a new runtime cache, query planner, composite index, worker, telemetry state, or frontend virtualization layer. Request Evidence elapsed time stays within 2.8% of the base at 65,536 events, the attempt and database costs scale approximately linearly through the live 1,000-message process limit, the schema-only migration is short at 65,536 legacy rows, Live Capacity transformation stays sublinear relative to the selected input increase, and total bundle gzip growth stays below the review envelope.

The 1,000-message Redis path has one explicit cost disposition. An initial measurement found avoidable replay JSON expansion, so the whitelist projection was compacted to omit fields that decode identically from zero, empty, or nil while retaining explicit nullable cache facts, including explicit zero. After that optimization, replay-safe JSON projection plus the wider attempt record raises median elapsed time by 17.0% and allocation bytes by 25.4% against the base, while SQLite bytes are 3.7% lower. The absolute current median is 46.7 ms for 1,000 fully populated attempts, below 1% of the fixed five-second local inbox processing interval. Removing the projection would reintroduce retention of provider failure bodies, response headers, and unknown fields; omitting the new columns would remove requested attempt evidence. The remaining cost is therefore retained and observable rather than hidden behind a more complex JSON pipeline or runtime optimization seam.

## Review envelopes

These are review triggers, not automated pass/fail thresholds:

- Same-work backend paths: investigate a stable median elapsed regression above 10%, or bytes/allocations above 20%.
- Live Capacity transformation: an 8x quota-row input increase should remain below 10x elapsed growth on the same host.
- Frontend bundles: investigate total gzip growth above 2%, or any affected logical chunk growing by more than 5 KiB gzip.
- Any result outside an envelope needs an evidence-backed disposition; it does not become a CI timing assertion.

## Repeatable command

The repository-owned manual Interface is:

```text
make benchmark-cpa-data-surface
```

It runs three one-iteration Go samples with fixture and database creation outside the timer, the Vitest Live Capacity benchmark on one worker, and a Vite manifest build followed by deterministic logical-chunk raw/gzip reporting. It is deliberately not a dependency of `make verify`.

For the dated base comparison, the exact base was expanded into an independent temporary directory. The compatible benchmark-only files were copied unchanged into that directory, both trees used the same Go/Node toolchain and `web/node_modules`, and the trees were run serially. The Redis comparison runs the equal-cardinality unique-request profile; the old implementation intentionally collapses repeated request IDs, so its repeated-attempt output is not the same work. The current-only replay projection, repeated-attempt path, migration, and all-row Live Capacity benchmark have no like-for-like base output and are reported as absolute/scaling costs.

## Deterministic fixtures

| Surface | Synthetic fixture | Timed work |
| --- | --- | --- |
| Redis replay projection | ordinary attempt, failed attempt with a large excluded body/header, invalid JSON | replay-safe projection only; JSON reflection cache warmed |
| Redis intake and processing | 1,000 fully populated attempts; one profile has 1,000 request IDs and one has 250 request IDs x 4 attempts; 32 providers, 512 models, every 17th failed | projection, inbox write, decode, usage-event write, rollup rebuild, processed mark |
| Request Evidence | 65,536 events over 48 hours, 32 providers, 512 models, 2,048 identities | filtered count, time/provider-scoped model options, and 100-row page projection |
| Attempt amplification | 250 request IDs at 1x, 2x, and 4x distinct attempts | usage-event and rollup writes through the live 1,000-attempt limit |
| Migration | 65,536 rows in the pre-attempt legacy table | six schema-only `ALTER TABLE ADD COLUMN` statements |
| Live Capacity | 100 accounts at 8 and 64 quota rows; render cases at 100x8 and 20x64 | view-model transformation and jsdom render/cleanup |
| Bundle | production Vite build with manifest | raw and Node-gzip bytes by stable manifest source/chunk owner |

## Backend results

Comparable base/current values below are medians of five serial `-benchtime=1x` samples. Current-only values are medians of three samples unless stated otherwise.

| Comparable path | Exact base | Current | Disposition |
| --- | ---: | ---: | --- |
| Redis 1,000 unique requests, elapsed | 39.895 ms | 46.695 ms | +17.0%; retained required projection/record cost |
| Redis 1,000 unique requests, B/op | 16,409,568 | 20,570,456 | +25.4%; retained and bounded |
| Redis 1,000 unique requests, allocs/op | 246,453 | 293,054 | +18.9% |
| Redis 1,000 unique requests, SQLite bytes | 2,129,920 | 2,052,096 | -3.7%; compact projection offsets wider rows |
| Request Evidence, elapsed | 42.680 ms | 43.862 ms | +2.8%; no material combined-filter latency regression |
| Request Evidence, B/op | 224,016 | 271,984 | +21.4%; wider 100-row evidence projection, elapsed stable |
| Request Evidence, allocs/op | 4,479 | 4,997 | +11.6% |

The query-plan regression test runs `ANALYZE` against the combined filtered count, time/provider-scoped distinct model options, and ordered page statements. Each must use an indexed `SEARCH`; a table or full-index `SCAN` fails. The proof does not pin a specific index and permits SQLite's temporary B-trees for `DISTINCT` or ordering.

Current-only replay projection medians were 2.375 microseconds / 1,336 B for an ordinary attempt, 9.375 microseconds / 9,464 B for a failed attempt containing a large excluded payload, and 1.542 microseconds / 1,232 B for invalid JSON.

The current repeated-attempt full pipeline preserved all 1,000 attempts from 250 request IDs in 47.421 ms and 2,060,288 SQLite bytes. Its cost is comparable to the 1,000-unique-request profile because both write 1,000 attempt rows; request ID is correlation, not event identity.

Attempt amplification remained approximately linear through the live processing limit:

| Attempts | Median elapsed | Median SQLite bytes |
| ---: | ---: | ---: |
| 250 (1x) | 7.404 ms | 401,408 |
| 500 (2x) | 13.321 ms | 577,536 |
| 1,000 (4x) | 26.837 ms | 966,656 |

The 65,536-row attempt-field migration median was 1.485 ms and the resulting database was 2,031,616 bytes, with zero file growth from the six schema-only additions. Fixture creation was outside the timer.

## Frontend results

Current Live Capacity means from one Vitest run:

| Case | Mean |
| --- | ---: |
| Build 100 accounts x 8 rows | 0.0977 ms |
| Build 100 accounts x 64 rows | 0.3122 ms |
| Render/cleanup 100 accounts x 8 rows | 172.83 ms |
| Render/cleanup 20 accounts x 64 rows | 214.12 ms |

The 8x per-account input increase costs 3.20x in view-model time. The two composite render stress cases create 800 and 1,280 metric meters respectively and observe 1.24x elapsed time for 1.6x emitted meters; because account count also changes, this is pressure coverage rather than a single-variable scaling claim. The harness writes build results to a module-level sink, verifies each fixture's exact meter count outside the timer, and fully clears Testing Library state and the document body after every render.

No raw base/current render delta is claimed. The base transformation retained at most three additional rows and the base card rendered only its two primary meters; the current card intentionally renders every returned quota row. Current-only scaling is the comparable evidence for this changed output contract.

Bundle comparison by logical manifest owner:

| Asset total | Exact base | Current | Delta |
| --- | ---: | ---: | ---: |
| JavaScript raw | 902,325 | 907,143 | +4,818 (+0.53%) |
| JavaScript gzip | 262,784 | 264,723 | +1,939 (+0.74%) |
| CSS raw | 32,560 | 32,862 | +302 (+0.93%) |
| CSS gzip | 6,850 | 6,930 | +80 (+1.17%) |

No affected logical chunk grew by 5 KiB gzip. The existing `index.lazy` chunk remains above Vite's 500 KiB raw warning in both base and current builds; this task did not create that warning.

## Evidence boundaries

- Synthetic data proves repeatability, scaling direction, query plans, allocations, and local SQLite/build costs. It does not establish the real CPA provider distribution, queue arrival rate, or production database contention.
- jsdom covers JavaScript transformation and DOM creation. It does not measure browser layout, paint, FLIP animation, low-end-device frame rate, or memory pressure.
- One host and three Go samples establish direction, not a universal service-level objective. Timing should be refreshed on the same idle host after material changes.
- A production release still needs the existing endpoint smoke timings. This evidence does not authorize a destructive production queue benchmark or claim that production is deployed.

## Deletion test

- Without the replay projection benchmark, sanitizer CPU/allocation cost becomes anecdotal.
- Without the 1,000-message benchmark, the bounded intake/process path has no end-to-end local throughput evidence.
- Without the Request Evidence benchmark and plan test, the new combined filter has neither high-cardinality timing nor full-scan regression coverage.
- Without attempt amplification, retry/failover storage growth is not observable.
- Without the migration benchmark, schema-only cost on an existing high-cardinality table is unmeasured.
- Without the Live Capacity benchmark and manifest reporter, all-row rendering and bundle growth return to informal build-log inspection.

No benchmark adds production state, a runtime branch, a fallback, a new dependency, or an automatic timing gate.
