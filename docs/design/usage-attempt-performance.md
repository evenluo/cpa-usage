# Usage attempt performance

Status: current, slice C of Parent #140 / Issue #146 r1

Owner: `internal/repository` bounded raw aggregation, `internal/api` query and HTTP projection

Consumer: Usage Intelligence fixed-window supporting diagnostics and Request Evidence

## Selection and percentile method

`GET /api/v1/usage/performance` consumes the shared [diagnostic selection](usage-diagnostic-selection.md). It always reads one exact, inclusive 24-hour snapshot and returns the exact RFC3339Nano `window_start` and `window_end`. Provider, actual model, account, public endpoint, status, and inclusive `min_latency_ms` selection remain API-normalized and repository-applied once. No percentile rollup, sketch, cache, worker, lifetime scan, or frontend recomputation exists.

Every percentile uses the nearest-rank method. Sort the valid samples ascending, compute one-based rank `ceil(q * N)`, and return that observed value for q=0.50 and q=0.95. With no valid sample, p50 and p95 are `null`. Ties are not collapsed.

Every distribution returns:

- `population_count`: attempts eligible for that named result/execution population before metric validity checks;
- `sample_count`: attempts in that population with a valid metric sample;
- `coverage`: `sample_count / population_count`, or `null` when the population is empty;
- nullable `p50` and `p95`.

Missing or invalid evidence is unavailable, never zero.

## Result and execution populations

Latency keeps successful and failed attempts in separate distributions. A latency sample is valid only when `latency_ms > 0`; its denominator is the corresponding successful or failed attempt count.

TTFT and Output TPS describe successful generation only; failed attempts are explicitly excluded rather than mixed with completed successful output. Successful attempts form one exhaustive execution partition, with explicit false taking precedence over unknown:

1. `non_generating`: `generate=false`;
2. `non_streaming`: not explicitly non-generating and `stream=false`;
3. `generating_streaming`: `generate=true` and `stream=true`;
4. `unknown`: every remaining attempt, including historical missing flags.

TTFT is reported separately for `generating_streaming` and `unknown_execution`. A valid TTFT is positive, has positive total latency, and is not greater than total latency. Non-generating and non-streaming attempts do not enter TTFT percentiles.

Output TPS is reported separately for the same two execution populations. The repository consumes only `InterpretUsageAttempt(event).OutputTPS`; it does not reproduce the arithmetic, replace `UsageEvent.OutputTokens`, or substitute canonical total/non-reasoning output. Consequently nil/zero/inconsistent timing, explicit non-generation/non-streaming, invalid or non-complete canonical quality, and a legacy output scalar exceeding canonical output cannot produce false exact throughput. Historical absent canonical facts with unknown execution flags remain a separately labeled population. Provider-normalized output units are comparable only inside one provider: when no provider is selected, overall/model/account Output TPS remains unavailable while each provider breakdown retains its own distribution; selecting a provider enables Output TPS throughout that filtered response.

## Bounded comparisons and slow evidence

The overall summary and each provider, actual-model, and account comparison use the same selection. All statements run inside one repository read transaction, so delayed intake cannot split population counts, samples, Top-N membership, and `other_count` across snapshots. Within that snapshot each comparison dimension uses one SQL-ranked Top-N count statement and one bounded raw projection restricted to those returned groups. Each breakdown returns at most eight groups ordered by attempt count descending and value ascending. Blank dimensions and lower-ranked groups contribute to `other_count`, so visible item attempt counts plus `other_count` equal `total_attempts`. The frontend renders every returned item and the excluded count; it does not silently truncate rows.

Account values stay repository identities and receive the same display-safe API labeling as failure concentration. No endpoint query, fragment, unrestricted failure text, headers, credentials, or client metadata enters the response.

A successful or failed latency p95 may open Request Evidence with the same provider/model/account selection, exact returned `window_end`, matching `result`, and `min_latency_ms=p95`. Because the threshold is inclusive, ties can make the slow evidence set larger than five percent; the evidence page reports its actual paginated count and never claims an exact top-five-percent set.

## Compatibility and failure policy

This is an additive protected route, additive nullable execution evidence, and additive diagnostic selection. Existing non-diagnostic event-list provider/model/source/auth-index/result/range behavior, selected-window analytics, rollups, Cost, ingestion, auth/session, backup, release, and deployment behavior remain compatible.

Invalid selection returns HTTP 400 before any repository read. A successful zero-attempt selection returns HTTP 200 with empty breakdowns and unavailable percentiles. Repository/API failure remains HTTP 500; the frontend preserves stale complete data when available.

## Local bounded-read evidence (2026-09-07)

`BenchmarkUsageAttemptPerformanceHighCardinality` uses the existing deterministic 65,536-attempt synthetic fixture spread across 48 hours, with 32 providers, 512 models, 2,048 auth-index values, positive timing/output data, and known generate/stream flags. Its route-shaped exact 24-hour selection contains 32,760 attempts. Fixture construction and SQLite `ANALYZE` are outside the timer. On this Apple M4 with `GOMAXPROCS=1`, `-count=1`, and `-benchtime=1x`, the final transactionally consistent read took 322.35 ms with 24,890,880 B allocated across 378,529 allocations. `/usr/bin/time -l` reported 204,881,920 bytes maximum resident set size for the complete `go test` invocation.

An initial all-columns/all-groups implementation over the full 48-hour fixture took 2.28 s and allocated 1,285,473,624 B. It was rejected before delivery. That earlier run is directional rather than a like-for-like comparison with the corrected 24-hour benchmark. The delivered read restricts raw projections to interpretation fields and SQL-ranked visible Top-N groups without changing exact percentile, coverage, or `other_count` semantics.

This is representative local evidence, not a production SLA. Space remains linear in the valid overall Output TPS populations plus raw attempts for the returned Top-N groups inside the selected 24-hour window. No cache, sketch, background worker, or historical rollup hides that bound.
