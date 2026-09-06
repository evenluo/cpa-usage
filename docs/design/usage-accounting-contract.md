# Per-attempt accounting and execution facts

Status: current, slices F/G of Parent #140 / Issues #144 and #145 r1

CPA v7.2.62 (`3554b63721aac9b4202bf2ef88ba7a82b4e5caf8`) emits legacy token
facts. CPA v7.2.152 (`c76dfd4e0edabab9000628b1560ab8ab379eadb8`) retains the
v7.2.151 canonical accounting-v2 contract. Readable fields and their two schema
versions determine support; application version strings do not gate decoding.
The [pinned synthetic fixtures](../../internal/cpa/testdata/usage/README.md) give
official producer provenance and consumer examples.

## Owners and storage

`internal/cpa/usage_accounting.go` owns typed upstream extension DTOs.
`internal/service` adds them to the existing replay-safe allowlist and projects
typed facts to `entities.UsageAccounting`, embedded in `UsageEvent`.
`repository.InterpretUsageAttempt` owns per-attempt validity and throughput;
`UsageEventRecord.AttemptFacts` exposes its result on the existing bounded read.
G projects summary accounting and attempt evidence through `internal/api` and
the existing frontend token/evidence surfaces; C consumes execution facts for
its distributions.

G requires selected-window SQL aggregation plus the existing hourly rollup
owner; C requires bounded raw per-attempt reads. Nullable scalar columns support
both without parsing a JSON blob during SQL aggregation. No raw canonical JSON
is stored. `InsertUsageEvents` materializes `accounting_state` using the same
repository interpretation before insertion. SQL aggregators may use
`accounting_state = 'valid'` to select canonical buckets, and must separately
count/qualify `token_quality`. Direct fixture writes must use `InsertUsageEvents`
to exercise this contract. Immutable events have no competing update path.

The existing `input_tokens`, `output_tokens`, `reasoning_tokens`, `cached_tokens`,
explicit legacy cache fields, and `total_tokens` retain their meanings. Canonical
columns start with `canonical_`; they never replace or add to legacy columns in
this slice. Operator Cost Rates, three-rate Cost calculation, and completeness
remain unchanged. Response tiers do not establish billing or multipliers.

## Availability and quality

The read record retains typed reported values even when unavailable. A nil
number means missing or malformed, never zero. `State` qualifies the whole
canonical record. The first applicable rule in this table wins:

| State | Meaning |
| --- | --- |
| `malformed` | A supplied accounting field has an invalid JSON type, null, fractional or out-of-int64 number. |
| `unsupported_accounting_version` | A supplied top-level integer version is not 2. |
| `unsupported_schema_version` | A supplied nested integer version is not 2. |
| `absent` | Neither top-level version nor canonical breakdown is present. Historical records remain here. |
| `missing` | Either version, the breakdown, quality, or a required bucket is absent. |
| `unknown_quality` | Supplied string quality is outside the supported enum; the persisted category is `unknown`, never arbitrary upstream text. |
| `invalid` | A bucket is negative, a sum overflows or disagrees, or complete quality includes unclassified tokens. |
| `valid` | Both versions are 2 and all canonical structural invariants hold. |

For records with a quality value, unknown quality is checked before missing
numeric buckets. No state reconstructs canonical facts from legacy totals,
provider names, aliases, or tiers. Supported qualities are `complete`,
`inconsistent`, and `unclassified`. **Valid is not complete**: preserve the
upstream quality even if every sum holds or the unclassified bucket is zero.

Canonical buckets are mutually exclusive:

- input total = uncached + cache read + cache write;
- output total = non-reasoning + reasoning;
- total = input total + output total + unclassified.

All values must be non-negative int64, all nine numeric fields must be present,
and complete quality additionally requires zero unclassified tokens. Aggregate
consumers select only valid records for canonical bucket sums, report quality
and coverage independently, and define their denominators. F does not add
aggregate metrics or alter rollup units; G owns that implementation.

## Execution and throughput

`generate` and `stream` are nullable booleans. CPA's verified v2 producer emits
both, but CPA Usage never synthesizes them for historical or absent fields.
Malformed flags become unknown. The upstream SDK's defaulting rules do not
authorize consumer-side historical defaults.

Existing `service_tier` is the requested tier; `RequestServiceTier` presents it
as optional text. `response_service_tier` is an independent optional observed
response tier. Empty/malformed response tiers are unknown. Neither fills or
overwrites the other.

`InterpretUsageAttempt.OutputTPS` reuses the existing positive-output,
positive-TTFT, latency-greater-than-TTFT arithmetic and the existing
`event.OutputTokens` numerator. Valid complete v2 qualifies this reading; any
other supplied canonical evidence yields no TPS. A legacy output scalar
exceeding the canonical output total is inconsistent with that evidence and
also yields no TPS. Explicit non-generating or non-streaming
attempts yield no TPS. Absent canonical evidence keeps the existing historical
output interpretation, with absence and unknown flags visible in `AttemptFacts`.
The formula remains output tokens × 1000 / (latency_ms − ttft_ms).

Compatibility decision (root adjudicated, Parent r1): frozen
`internal/service/event_key.go:33-41` only fills missing total tokens; it does
not alter output. In pinned c76
[usage_helpers.go](https://github.com/router-for-me/CLIProxyAPI/blob/c76dfd4e0edabab9000628b1560ab8ab379eadb8/internal/runtime/executor/helps/usage_helpers.go),
lines 780-825 retain OpenAI output including its reasoning subset, lines 890-941
retain Anthropic raw output including thinking, while lines 944-977 retain
Gemini candidates separately from thoughts. Neither canonical output total nor
canonical non-reasoning output preserves the existing numerator for all three.
Positive-reasoning fixtures lock the unchanged numerator. This is not a newly
defined canonical-output throughput metric, and it must not be described as
excluding reasoning. C must qualify provider/actual-model/execution populations
and must not imply exact throughput comparability across provider semantics.
C owns the
explicit success/failure and known-execution sample populations and coverage;
the per-attempt seam does not infer a final client outcome.

## Replay, migration, and compatibility

Malformed extension fields do not discard a valid legacy attempt. The private
DTO field wrapper projects invalid supplied values as `[]`, a fixed invalid
type for the allowlisted scalars/objects; the decoder recognizes the same
malformed state on every replay. Unknown nested fields and excluded sensitive
fields never enter the inbox. This adds no decoder fallback or recovery owner.
Malformed base envelopes still follow the existing `decode_failed` lifecycle.

Migration `20260907_add_usage_accounting_fields` only adds columns, with null
facts, false presence/malformed markers and `absent` state for historical rows.
It does not update historical tokens, tiers, event identity, inbox payloads, or
rollups. Existing pending inboxes can only yield the facts already retained in
their projection. ADR 0008's destructive pop and persisted timestamp contract
and ADR 0009's inbox-row attempt identity/idempotence remain unchanged.

Compatibility is additive for SQLite and repository reads; legacy scalar/Cost
semantics and routes remain compatible. The intentional Output TPS correction
retains provider-specific numerator units and suppresses explicitly
non-stream/non-generating or qualified canonical samples. No canonical API/UI
surface is added by F alone. G's additive projection is described below. Main
integration owns whole-program gates and any production upgrade decision.

## Selected-window composition and supporting evidence (G)

The existing summary response adds `accounting`. `total_attempts` is the selected
window's attempt count after provider filtering; `valid_attempts` and `states`
count F's persisted availability states. `coverage_pct` is
`valid_attempts / total_attempts * 100`, or null when there are no attempts.
This measures attempt coverage, not token-volume coverage and not completeness
of all historical usage. Each state count has all selected attempts as its
denominator. `valid_quality` counts complete, inconsistent and unclassified
quality **among valid attempts**, with `valid_attempts` as that denominator.
Structural validity and quality are never merged into a single success verdict.

`composition` sums only `accounting_state = 'valid'` rows. It retains the input,
output and unclassified structure above, with no legacy scalar substitution or
addition. Valid inconsistent and unclassified quality still contributes its
reported canonical buckets, qualified by the independent quality counts. When
there are no valid attempts, zero aggregate sums are an empty population; the
UI displays canonical totals as unavailable. Reported zero tokens in a valid
population remain real zero observations.

Only the current summary needs these aggregates; trend and contributor scalar
contracts retain their existing units. The summary reuses the existing bounded
raw/hourly source plan, including partial-hour edges and provider filtering.
The existing rollup owner stores state/quality counts and canonical sums and
rebuilds affected buckets on ingestion. The additive rollup migration resets
the existing backfill checkpoint; the same bounded backfill runner reconstructs
rollups from persisted rows. Until coverage is complete, the existing observable
`backfill_incomplete` read path applies. It never fills historical canonical
columns or infers new upstream facts.

The Tokens KPI and trend continue to use the existing scalar total. A compact
caption and expandable composition inside Trend Workbench explain accounting
Metric Completeness and the disjoint buckets. Cost completeness retains its own
status and configured three-rate calculation, independent of accounting quality
or tiers. No new KPI group, pricing engine, query endpoint or worker is added.

Request Evidence adds `attempt_facts`, projected directly from F's repository
read record. Canonical values remain visible even when invalid or incomplete,
with the repository state and reported quality explaining their interpretation.
Optional numeric facts, generate/stream and requested/response tiers serialize
as null and display as `-`; historical records do not acquire implied flags,
response tiers or canonical buckets. The existing `service_tier` compatibility
field remains requested tier. Top-level `output_tps` uses the same
`AttemptFacts.OutputTPS`, preserving provider scalar units and F's eligibility
correction. Neither canonical output bucket is substituted into throughput.

Compatibility: API fields and SQLite rollup columns are additive. Existing
scalar metrics, Cost, cache-read share, event identity, auth/routes, and Redis
effects retain their contracts. API evidence now honors F's intentional TPS
qualification for invalid/incomplete canonical or non-generating/non-streaming
attempts. Browser rendering treats an unavailable additive payload explicitly
as unavailable; it never fabricates a canonical value from scalar fields.

Local bundle evidence (2026-09-07, identical installed dependencies and Vite
manifest reporting): compared with accepted E+F base `f536cab`, G changes total
JavaScript from 926,312 to 931,634 bytes (gzip 270,113 to 271,756), and CSS from
38,804 to 39,029 bytes (gzip 7,799 to 7,835). JavaScript chunk count stays 12;
there are no added package dependencies. The pre-existing large-chunk warning
occurs on both builds. This is a local artifact-size comparison, not runtime or
production latency evidence.

The existing deterministic analytics benchmark (65,536 attempts, 32 providers,
512 models, 2,048 identities per kind; `GOMAXPROCS=1`, `-benchtime=1x`) measured
the raw core snapshot at 8.460 s and the covered hourly snapshot at 0.788 s.
Both retain existing bounded top-N output. These are single local observations
on a shared host, not a baseline regression estimate or an SLA. Migration plan
evidence separately verifies indexed extrema reads; focused tests cover the
provider-scoped hybrid window and bounded backfill completion.

## Local intake performance evidence (2026-09-07)

The committed `BenchmarkRedisUsageAccountingBatchEndToEnd` and the identical
synthetic fixture files ran on the frozen `2e15df04` source archive and the F
implementation, sequentially on this Mac with `GOMAXPROCS=1`, `-count=3`, and
`-benchtime=1x`. Each timed operation ingests 1000 messages with 250 request IDs,
128 accounts, 32 models, and 64 keys. Timing includes replay projection, inbox
and event writes, the existing rollup rebuild, and processed marks. Database
creation and message construction are outside the timer. No production queue,
credential, DB, or provider request was used.

| Identical input | Baseline median | F median | Baseline / F DB bytes |
| --- | --- | --- | --- |
| v7.2.62 legacy | 46.09 ms | 58.85 ms | 1,748,992 / 1,765,376 |
| v7.2.152 complete | 47.57 ms | 81.11 ms | 1,748,992 / 2,351,104 |

The baseline drops v2 extensions. Preserving and validating them adds 33.54 ms
per 1000-message v2 batch (about 71% in this sample); legacy batches add about
28%. These are attributable local measurements, not an idle-host SLA or a
production throughput claim; other tasks were active on the machine. There is
no speculative performance cache or extra worker.
