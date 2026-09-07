# Canonical usage accounting

Status: current
Authority: [accepted Accounting v2 direct cut](accounting-v2-direct-cut.md), superseding the additive accounting design from Parent #140 r1.

## One metric source

CPA v7.2.152 (`c76dfd4e0edabab9000628b1560ab8ab379eadb8`) supplies Accounting v2 on every queue message. The queue producer calls `EnsureTokenBreakdownForProvider` before serializing `accounting_version=2`, `token_breakdown`, `generate`, and `stream`.

The consumer admits that contract only. `internal/cpa` owns the typed external DTO; `internal/service` owns the replay-safe projection and admission; `internal/repository` owns canonical validation, per-attempt interpretation, metric reads and aggregation. All token consumers use canonical buckets, including selected-window KPI/trend/contributors, overview/analysis, mappings, Request Evidence and CSV. Stored historical scalar columns never supply an alternate metric.

Input = uncached + cache read + cache write. Output = non-reasoning + reasoning. Total = input + output + unclassified. All nine counters must be present, nonnegative int64 with overflow-safe consistent sums. Both versions must be 2 at intake; accepted versions are not persisted or repeated in read DTOs. Quality is `complete`, `inconsistent`, or `unclassified`; complete also requires zero unclassified tokens. Structural validity does not upgrade producer quality.

Cache Read Share is cache-read input divided by canonical input; its input coverage is scoped to canonical observations. Accounting attempt coverage separately identifies missing historical facts.

Historical events retain attempt counts, status and timing, with absent canonical facts. Malformed canonical facts are rejected before insertion. API/UI qualify these observations rather than converting absent tokens into a reported zero. A structurally valid observation reporting zero remains real zero. The repository's availability states are only `valid` and `absent`; detailed producer-version compatibility matrices are removed.

## Execution and Cost

Output TPS = canonical output total × 1000 / (latency_ms − ttft_ms). It includes reasoning tokens and requires complete-quality canonical facts, `generate=true`, `stream=true`, positive output/TTFT, and latency greater than TTFT. Unknown historical execution never supplies TPS. This is a token throughput observation; provider/model tokenization and workloads still affect comparisons.

Request Evidence exposes tokens, execution, Output TPS and service tiers only under `attempt_facts`, with no duplicate top-level projections or client fallbacks. Requested and response service tier are distinct optional observations. Neither establishes prices or billing multipliers.

Local three-rate Cost uses complete canonical facts only: uncached plus cache-write input at the prompt rate, cache-read input at the cache rate, and total canonical output at the completion rate. Existing operator rates and aliases remain untouched. Missing pricing, unavailable accounting or non-complete quality must qualify Cost as incomplete/unavailable. This defined local estimate is not an upstream bill.

## Intake and history

Malformed or unsupported incoming messages follow the existing explicit `decode_failed` inbox lifecycle. The consumer never admits them through a legacy decoder or encodes malformed fields into a secondary sentinel protocol. The allowlist excludes raw failure bodies, headers, credentials and client/session metadata even when admission fails.

The persisted inbox row and `PoppedAt` remain the owners of replay identity/time under ADR 0008/0009. Redis `LPOP` cannot reconstruct previously consumed history. Migration preserves historical raw rows and operator configuration; it does not infer their canonical values or consume the production queue.

## Aggregates and upgrade

Canonical sums and valid quality populations are projected into hourly rollups by the existing transactional rebuild owner. Complete-only prompt/cache-read/output sums and a complete-zero attempt count support the same Cost formula and known/unknown distinction for raw and covered rollup reads. A complete zero-token attempt has known zero Cost without pricing; mixed known and unknown costs are partial. Overview series carries Cost status for every observed bucket. No new worker, cache, version switch, or recovery owner exists.

For a first upgrade from published main, every pre-upgrade event is canonical-absent. The one-time rollup migration initializes `accounting_absent_attempts=request_count`, leaves all newly added canonical/quality sums zero and preserves every backfill checkpoint/lifecycle field. The same migration removes inactive scalar columns from the derived rollup table; only raw event scalars retain archival value. Subsequent incoming events rebuild only their affected hours. Unpublished intermediate binaries are not supported upgrade sources. Existing pending backfill work retains its original scope.

Required proof is strict v2 admission and malformed-input isolation, safe deterministic replay, historical absence versus actual zero, canonical parity across all read paths, Cost completeness, and upgrade initialization without checkpoint reset. See the PR for exact-head verification results.
