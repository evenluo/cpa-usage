# Analytics raw/rollup convergence evidence

Status: accepted scoped implementation disposition for GitHub issue #126  
Parent authority: #117/r1, outcome 20  
Issue frozen repository base: `4aaf2ce7d8b76764cfa71c51affb0429a0939057`  
Implementation base: `0d0fdd9c7f91fd073fcb66cec3b9fa5c0707da66`  
Consumed owners: #124 (`7b55fd2`, `530ce1d`) and #125 (`3fc59c9`, `0d0fdd9`)  
Compatibility: public Analytics DTOs, HTTP behavior, SQLite facts, Cost/Metric Completeness, identity enrichment, ordering, and fallback reason remain unchanged.

## Disposition

`analyticsCoreWindowPlan` remains the explicit source plan. `BuildAnalyticsCoreWithFilter` now owns only plan construction, rollup coverage selection, and the observable `backfill_incomplete` fallback. One private plan path owns summary, trend, provider options, insights, time-breakdown projection, and final snapshot assembly.

Raw-only identity, API-key, and model top-N queries remain a local Adapter inside that plan path. They apply `LIMIT 20` in SQLite before returning rows. The all-groups source-plan candidate must materialize every high-cardinality group, merge it into Go maps, sort it, and only then retain 20 rows. The benchmark below found a 10.13x bytes/op and 17.23x allocations/op increase. This exceeds the predeclared 20% allocation/bytes retention threshold even though median elapsed time increased by only 4.45%.

Tight-design verdict: `ready`. Removing the raw top-N Adapter would protect no additional current behavior and would violate the current high-cardinality performance claim. Adding a public strategy, generic planner, or fallback would add unearned Interface/state. The benchmark-only shared candidate stays in `_test.go` and creates no production seam.

## Capability matrix

| Capability | Raw-only | Mixed/covered rollup | Disposition |
|---|---|---|---|
| Source and fallback selection | `analyticsCoreWindowPlan` | `analyticsCoreWindowPlan` | converged |
| Summary | events aggregate source | events + rollup aggregate sources | converged plan orchestration |
| Trend / Time Breakdown | events aggregate source | events + rollup aggregate sources | converged plan orchestration |
| Provider options | events segment | events + rollup segments | converged; duplicate raw provider query removed |
| Final snapshot / insights | one private builder | one private builder | converged |
| Key Alias identity top 20 | SQL-limited events Adapter | merge source segments, then top 20 | retained raw fast Adapter |
| API-key top 20 | SQL-limited canonical API-key facts | merge canonical source facts, then top 20 | retained raw fast Adapter |
| Model top 20 | SQL-limited events Adapter | merge source segments, then top 20 | retained raw fast Adapter |
| Heatmap | existing raw/fallback implementation | existing rollup-aware implementation | retained; explicit #126 non-goal |

Source additions therefore affect one core orchestration for summary, trend, provider options, and snapshot assembly. Identity/API-key/model source merging remains necessary only where a window can contain both events and rollups.

## Semantic parity ledger

The focused repository suite passed on the implementation range:

```text
env GOCACHE=/tmp/cpa-usage-issue126-go-cache go test ./internal/repository -count=1
ok  cpa-usage/internal/repository  1.350s
```

Evidence owners:

| Contract case | Proof |
|---|---|
| raw-only, hour/day, provider scoped/unscoped, no-full-hour, empty result | `TestAnalyticsCoreRawFastPathMatchesSharedPlan`; exact `reflect.DeepEqual` including nil/empty shape and UTC timestamps |
| no-full-hour source plan | `TestAnalyticsCoreWindowPlanKeepsNoFullHourRawOnly` |
| fully covered rollup summary/trend | `TestBuildAnalyticsCoreWithFilterUsesRollupsForSummaryAndTrend` |
| rollup identity/API-key/model plus current alias/pricing enrichment | `TestBuildAnalyticsCoreWithFilterUsesRollupsForBreakdownsAndReadTimeEnrichment` |
| selected/fixed-window compatibility and previous-period comparison | `TestBuildAnalyticsSummaryWithFilterMatchesCompatibilityReadModelsWhenRollupCovered`, `TestBuildAnalyticsSummaryWithFilterUsesRollupAwareReadModelsWhenCovered`, `TestBuildAnalyticsSummaryWithFilterReturnsPreviousPeriodComparison` |
| partial-hour prefix/full-bucket/tail merge | `TestBuildAnalyticsCoreWithFilterKeepsPartialHourWindowExact` |
| backfill-incomplete raw fallback and observable log | `TestBuildAnalyticsSummaryWithFilterMatchesCompatibilityReadModelsWhenBackfillIncomplete`, `TestBuildAnalyticsCoreWithFilterFallsBackWhenBackfillIncomplete` |
| ingestion-maintained buckets after completed target | `TestBuildAnalyticsCoreWithFilterAllowsIngestionMaintainedBucketsAfterCompletedTarget` |
| UTC/local day, hourly, repeated fall-back hour, spring-forward, daily DST | `TestBuildAnalyticsSummaryWithFilterBucketsDailyTrendByLocalDay`, `TestBuildAnalyticsSummaryWithFilterBucketsHourlyTrendWhenRequested`, `TestBuildAnalyticsSummaryWithFilterKeepsRepeatedDSTHoursSeparate`, `TestBuildAnalyticsSummaryWithFilterHandlesSpringForwardHeatmapHour`, `TestBuildAnalyticsSummaryWithFilterBucketsDailyTrendAcrossDSTChange` |
| OAuth identity, API-key identity, aliases, deleted/current enrichment and ranking | `TestBuildAnalyticsSummaryWithFilterAggregatesKeyAliasBreakdownByStableIdentity`, `TestBuildAnalyticsSummaryWithFilterReturnsAPIKeyBreakdownByClientKey`, `TestBuildAnalyticsKeyAliasTrendsRestrictsRowsToSelectedIdentities` |
| canonical API-key raw/rollup/Reference Data facts | `TestAPIKeyAggregateFactsMatchAcrossAnalyticsRollupAndAliasTargets` |
| model/provider order, multi-provider projection, deterministic insights | `TestBuildAnalyticsSummaryWithFilterReturnsModelAndTimeBreakdowns`, `TestBuildAnalyticsSummaryWithFilterReturnsProviderOptionsForCurrentScope`, `TestBuildAnalyticsSummaryWithFilterReturnsDeterministicInsights` |
| complete/partial/unavailable/zero-rate Cost | `TestBuildAnalyticsSummaryWithFilterAggregatesSummaryAndTrend`, `TestBuildAnalyticsSummaryWithFilterMarksCostUnavailableWhenNoPricedCostExists`, `TestBuildAnalyticsSummaryWithFilterMarksCostPartialWhenPricedRowsHaveZeroRates`, `TestBuildAnalyticsSummaryWithFilterOmitsCostComparisonWhenPricingIsIncomplete` |
| negative tokens, cached greater than input, cache share/savings | `TestBuildAnalyticsSummaryWithFilterClampsTokenFieldsBeforeCostCalculation`, `TestBuildAnalyticsCoreWithFilterPreservesRawPromptCostClamp`, `TestBuildAnalyticsSummaryWithFilterExposesCacheEfficiencyWhenPricingIsComplete`, `TestBuildAnalyticsSummaryWithFilterSplitsCacheUnavailableStates` |
| empty/no previous period | `TestBuildAnalyticsSummaryWithFilterReturnsEmptyState`, `TestBuildAnalyticsSummaryWithFilterReturnsMissingPreviousPeriodComparison` |

The exact raw fast/shared candidate proof uses identical DTO comparison. Existing mixed raw/rollup Cost proofs permit only `1e-9` absolute float tolerance where SQLite and Go merge order can differ; timestamps are compared as UTC instants. No public DTO field is excluded.

`backfill_incomplete` remains the only fallback reason. Natural raw-only windows (including a window with no complete hour) are a source plan, not a fallback. Coverage failure logs retain reason, detail, range, granularity, provider, start time, and end time. No other fallback or compatibility branch was added.

## Repeatable high-cardinality benchmark

Environment: `darwin/arm64`, Apple M4, Go `1.25.14`, `GOMAXPROCS=1`. The deterministic arithmetic fixture is generated outside the timer and contains 65,536 events over 48 hours, 32 providers, 512 models, 2,048 OAuth identities, 2,048 provider identities, 2,048 API keys, aliases, priced/unpriced/zero-rate models, negative token rows, and cached-greater-than-input rows. Each measured iteration returns 20 Key Alias rows, 20 API-key rows, 20 model rows, and 32 provider rows. A warm read runs before `ResetTimer`; setup and database creation are excluded from ns/op and allocation results.

Commands:

```text
env GOCACHE=/tmp/cpa-usage-issue126-go-cache GOMAXPROCS=1 go test ./internal/repository -run '^$' -bench='BenchmarkAnalyticsCoreRawFastHighCardinality$' -benchmem -count=3 -benchtime=1x
env GOCACHE=/tmp/cpa-usage-issue126-go-cache GOMAXPROCS=1 go test ./internal/repository -run '^$' -bench='BenchmarkAnalyticsCoreRawSharedPlanHighCardinality$' -benchmem -count=3 -benchtime=1x
env GOCACHE=/tmp/cpa-usage-issue126-go-cache GOMAXPROCS=1 go test ./internal/repository -run '^$' -bench='BenchmarkAnalyticsCoreCoveredRollupHighCardinality$' -benchmem -count=3 -benchtime=1x
```

`1x` is intentional: one raw iteration takes more than eight seconds, so `-benchtime=2s` also selects one iteration. Three independent fixture/process repetitions establish direction without multiplying database setup ten times.

Raw output summary:

| Benchmark | ns/op (min / median / max) | B/op (median) | allocs/op (median) |
|---|---:|---:|---:|
| raw SQL-limited fast Adapter | 8,532,191,292 / 8,543,145,709 / 8,569,129,667 | 734,464 | 7,567 |
| raw all-groups shared candidate | 8,430,056,583 / 8,923,190,750 / 9,215,967,625 | 7,441,216 | 130,373 |
| covered rollup shared plan | 824,891,500 / 849,738,375 / 896,542,834 | 7,326,992 | 128,290 |

Candidate versus retained raw median: elapsed `+4.45%`, bytes/op `+913.15%` (`10.13x`), allocations/op `+1,622.92%` (`17.23x`). Frozen decision thresholds were a stable elapsed regression above 10%, or bytes/allocations above 20%. The allocation and memory regressions independently require retention.

## Test locality ledger

No existing test was moved or renamed. `analytics_core_convergence_test.go` owns the new plan/candidate parity proof and deterministic fixture; `analytics_core_benchmark_test.go` owns repeatable performance evidence. Existing `analytics_test.go` fixtures and unique proofs remain intact. This is an owner-local addition, not a size-based reshuffle.
