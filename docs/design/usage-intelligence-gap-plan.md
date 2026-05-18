# Usage Intelligence Gap Closure Plan

## Conclusion

The `/cpa-usage/` page should evolve from the current functional analytics view into a Usage Intelligence workspace that matches the target dashboard at the level of information architecture, data-backed visual shape, and operational density. The goal is not pixel-perfect replication. The goal is to make the primary analytics page feel complete: hourly-first trends, real previous-period comparisons, a date-by-hour heatmap, deterministic insights, and cost/token leaderboards.

This plan applies to the main `/cpa-usage/` analytics page. It does not target `/cpa-usage/events`, which remains the request event inspection surface.

## Current Gap

- The main trend is effectively a fixed bucket chart and does not expose Time Granularity as a user-facing capability.
- The default dashboard does not show true previous-period comparisons in the KPI cards.
- The heatmap is not a date-by-hour usage intensity view like the target design.
- The main trend lacks the target dashboard's bucket summary strip: average cost, average tokens, peak cost, peak tokens, total cost, and total tokens.
- The right rail needs to be more purposeful: deterministic insights first, then a compact Key Alias leaderboard.
- Model mix and request health exist conceptually, but they need denser presentation and clearer Cost/token semantics.

## Domain Decisions

- **Usage Intelligence** is the aggregate dashboard surface for usage, Cost, request health, and time patterns.
- Request events are drill-down evidence, not the primary dashboard structure.
- Default selected range remains Last 7 days in this refinement.
- Default Time Granularity is Hour.
- Supported Time Granularity in this refinement is Hour and Day.
- Heatmap default measure is Tokens.
- Heatmap V1 uses date-by-hour buckets for the fixed 30-day **Fixed Operational Window**, not weekday averages and not the **Selected Analysis Window**.
- KPI comparison uses the immediately previous period for the same selected range.
- Missing previous-period data is shown explicitly. It is not inferred.
- Leaderboards sort by Cost when Cost is complete; if Cost is unavailable, token volume becomes the ordering measure.
- Insights remain deterministic metrics and warnings, not AI-generated summaries.
- Production rollout updates only `/cpa-usage`; the existing `/usage` service and CPA root service must remain intact.

## Data Contract

Extend `GET /api/v1/analytics/summary` with a `granularity` query parameter:

```text
granularity=hour | day
```

For this refinement:

- Default `granularity` is `hour`.
- Unsupported values return `400`.
- `range=7d` remains the default selected range.
- `provider` continues to scope the whole analytics response.
- The response `timezone` remains the server business timezone, currently derived from `time.Local`.
- All bucket labels and date/hour grouping use the server business timezone, not the browser timezone.
- Time filters continue to use the existing inclusive end boundary behavior in `parseUsageFilterQuery`; this refinement should not change existing range semantics.
- Previous-period comparison uses the parsed current `StartTime` and `EndTime`; the previous period is shifted backward by the same duration and must not overlap the current inclusive boundary.

Add response fields:

```json
{
  "range": "7d",
  "granularity": "hour",
  "previous_range_start": "2026-05-01T00:00:00Z",
  "previous_range_end": "2026-05-07T23:59:59.999999999Z",
  "comparison": {
    "total_cost_change_pct": 12.4,
    "total_tokens_change_pct": 8.7,
    "request_count_change_pct": 15.3,
    "success_rate_change_pp": 0.38,
    "has_previous_period": true
  },
  "heatmap": {
    "measure": "tokens",
    "max_tokens": 120000,
    "max_cost": 4.2,
    "max_requests": 18,
    "max_failures": 1,
    "rows": [
      {
        "date": "2026-05-12",
        "label": "Tue 05/12",
        "cells": [
          {
            "hour": 0,
            "bucket_start": "2026-05-11T16:00:00Z",
            "bucket_end": "2026-05-11T17:00:00Z",
            "total_tokens": 120000,
            "total_cost": 4.2,
            "request_count": 18,
            "failure_count": 0,
            "cost_available": true,
            "cost_status": "available"
          }
        ]
      }
    ]
  }
}
```

The existing `trend` array should follow `granularity`. The heatmap remains hourly regardless of the selected main trend granularity because it represents time-of-day usage intensity.

### Schema Rules

- `granularity` is always present and is either `hour` or `day`.
- `comparison.has_previous_period=false` when the previous period has no requests.
- When `comparison.has_previous_period=false`, percentage and point deltas are omitted from JSON.
- When a previous-period denominator is zero for a specific metric, that metric's change field is omitted instead of returning `0`.
- Heatmap rows must cover each local date in the fixed 30-day **Fixed Operational Window** used for Activity Heatmap.
- Each heatmap row must contain exactly 24 cells, one for each local hour.
- Empty heatmap cells use zero counts and `cost_status="available"` with `cost_available=true`.
- Heatmap intensity normalization is a frontend responsibility using `max_tokens`, `max_cost`, `max_requests`, or `max_failures`.
- Heatmap `bucket_start` and `bucket_end` are UTC timestamps for the local-hour bucket.
- Heatmap Cost fields preserve Metric Completeness; if any billable event in a cell lacks pricing, the cell is `partial` or `unavailable` using the existing Cost status rules.

## Implementation Touchpoints

| Layer | Files | Required changes |
| --- | --- | --- |
| API parsing | `internal/api/analytics.go`, `internal/api/usage_filter.go` | Parse `granularity`; default to `hour`; reject unsupported values with `400`; include `granularity`, previous range, comparison, and heatmap in the response. |
| Service filter | `internal/service/dto/usage.go` | Add `Granularity string` to `UsageFilter`; pass it through analytics service only. |
| Repository filter | `internal/repository/dto/usage_query_filter.go` | Add `Granularity string`; keep non-analytics callers unaffected. |
| Repository analytics | `internal/repository/analytics.go` | Make trend bucketing honor `Granularity`; build previous-period summary; build date-by-hour heatmap; keep SQL aggregation as the primary mechanism. |
| Repository DTO | `internal/repository/dto/analytics.go` | Add comparison and heatmap DTOs to `AnalyticsSummarySnapshot`. |
| Service analytics | `internal/service/analytics_service.go` | Pass analytics filters to the repository and return repository-owned analytics read models without duplicating service-layer analytics DTOs. |
| API DTO | `internal/api/analytics.go` | Publish stable JSON names for comparison and heatmap. |
| Frontend types/state | `web/src/routes/index.lazy.tsx`, `web/src/hooks/useAnalytics.ts`, `web/src/types/api.ts` | Add `TimeGranularity` state; request `granularity`; map comparison and heatmap payloads. |
| Frontend charts | `web/src/components/charts/*` | Upgrade the primary Cost/Tokens chart, add KPI sparklines if needed, and add a heatmap component. |
| Frontend verification | `web/` | Cover feature-level Usage Intelligence and Reference Data behavior with Vitest, alongside lint, typecheck, and build in the frontend verification gate. |

## Time and Boundary Rules

- The canonical timezone for analytics buckets is the server business timezone exposed by the API `timezone` field.
- The browser must render labels from backend-provided bucket labels and heatmap row labels; it must not regroup buckets by browser timezone.
- Existing preset range parsing remains anchored by the API request time.
- The current period is the parsed inclusive `timestamp >= StartTime AND timestamp <= EndTime` interval used by the existing repository filter.
- Previous period duration is `EndTime - StartTime`.
- Previous period start is `StartTime - duration`.
- Previous period end is `StartTime - 1ns` to avoid double-counting an event exactly at the current start boundary.
- Provider filtering must apply identically to current summary, previous summary, trend, heatmap, insights, model mix, and Key Alias leaderboard.
- DST behavior should follow `time.Local`: local date rows may still contain 24 displayed hour slots, but bucket timestamps should reflect actual local-hour conversions.

## Implementation Plan

### Phase 1: Data Contract

Backend:

- Parse and validate `granularity` in the analytics summary API and propagate it through API, service, and repository filters.
- Add hourly and daily trend bucket selection in the analytics repository using explicit `Granularity`, not the current range-length heuristic.
- Keep `hour` as the default.
- Add previous-period filtering using the same range length and provider scope.
- Build `comparison` from current summary versus previous summary.
- Add heatmap aggregation grouped by server-business local date and local hour.
- Return complete 24-cell rows for each local date in the fixed 30-day **Fixed Operational Window**.
- Preserve Metric Completeness semantics for Cost fields in trend, comparison, leaderboard, model mix, and heatmap cells.

Tests:

- API defaults to `granularity=hour`.
- API rejects unsupported granularity values.
- Hour granularity returns hourly buckets for Last 7 days.
- Day granularity returns daily buckets.
- Previous-period comparison is scoped by the same provider.
- Missing previous-period data returns `has_previous_period=false`.
- Heatmap rows are local date rows with 24 hourly cells.
- Empty heatmap buckets are present with zero values.
- Heatmap Cost status follows existing Metric Completeness rules.
- DST/local-time cases are covered at repository level.

### Phase 2: Dashboard UI

Top KPI cards:

- Add mini sparklines backed by current trend data.
- Add previous-period comparison text.
- Show `No previous data` when comparison is unavailable.
- Keep KPI order: Total Cost, Total Tokens, Requests, Success Rate.

Primary trend:

- Add Hour/Day segmented control.
- Default to Hour.
- Replace the current fixed trend usage of `TokenCostCompareChart` with a granularity-aware primary trend.
- Render Cost and Tokens as peer measures on the same chart.
- Add a tooltip with bucket label, Cost, tokens, requests, and failures.
- Add bucket summary strip:
  - Avg Cost / Hour or Avg Cost / Day
  - Avg Tokens / Hour or Avg Tokens / Day
  - Peak Cost / Hour or Peak Cost / Day
  - Peak Tokens / Hour or Peak Tokens / Day
  - Total Cost
  - Total Tokens

Heatmap:

- Replace the current `Time` tab content in the Breakdown Workbench with a date-by-hour token heatmap.
- Rows are dates in the fixed 30-day **Fixed Operational Window**.
- Columns are hours from `00` through `23`.
- Default measure is Tokens.
- Cell intensity is normalized in the frontend using `heatmap.max_tokens`.
- Tooltip shows date, hour, tokens, Cost, requests, and failures.

Right rail:

- Keep deterministic insights.
- Prioritize Metric Completeness and health risks before cost, token, and contributor movements.
- Move the compact Key Alias Leaderboard into the right rail below insights:
  - rank
  - Key Alias
  - Cost or token measure
  - percent of total
- Do not keep a duplicate always-visible Key Alias ranking list below the main chart.

Lower panels:

- Keep Model Mix and Request Health as compact lower panels.
- The date-by-hour heatmap replaces the previous `Time Breakdown` tab; it is not duplicated as another lower panel in the same screen.
- Model Mix defaults to Cost share when Cost is complete; otherwise it uses token share and labels the Cost state.
- Request Health remains a supporting stability panel, not the primary story.

### Current Component Mapping

- `MetricCard`: keep, but add comparison and mini-sparkline props.
- `TokenCostCompareChart`: evolve into the primary trend chart or wrap it with a new granularity-aware component.
- `TrendPointDetail`: replace with tooltip plus bucket summary strip; do not keep the current button grid as the main interaction.
- `InsightRail`: keep, but tighten ordering and visual density.
- `Breakdown Workbench`: keep segmented structure, but `Time` becomes heatmap rather than another line chart plus list.
- `AliasRankingChart`: either remove from the main flow or repurpose for a compact right-rail leaderboard; avoid duplicate Key Alias surfaces.
- `HealthTimeline`: keep as a compact request health panel.

### Phase 3: Verification and Rollout

Local verification:

- Run backend analytics tests: `go test ./internal/repository ./internal/api ./internal/service`.
- Run frontend lint, feature tests, typecheck, and build through `make verify-frontend`.
- Run `make verify` if the narrower checks pass.

Self-hosted rollout:

- Implement on a feature branch and review before merging to `main`.
- After merge, push `main`.
- Before rebuild, record the current remote baseline:
  - `docker compose ps cpa-usage`
  - current `cpa-usage` image ID/digest from `docker image inspect cpa-usage:main`
  - current compose checksum from `<compose-directory>/docker-compose.yml`
  - current public smoke results for `/cpa-usage/healthz`, `/cpa-usage/`, and `/usage/healthz`
- Rebuild the `cpa-usage` service through the existing compose setup.
- Do not change `/usage`.
- Do not change the CPA root service.
- Do not change the login password.
- If the remote deployment fails, roll back only the `cpa-usage` service:
  - restore the recorded compose file if compose changed
  - retag or redeploy the recorded previous `cpa-usage` image
  - restart only `cpa-usage`
  - re-run `/cpa-usage/healthz`, `/cpa-usage/`, and `/usage/healthz`
  - do not restart `cliproxyapi`, `postgres`, or `cpa-usage-keeper` unless the failure explicitly involves them

Public smoke:

```text
https://<your-cpa-host>/cpa-usage/healthz
https://<your-cpa-host>/cpa-usage/api/v1/analytics/summary?range=7d&granularity=hour
https://<your-cpa-host>/cpa-usage/api/v1/analytics/summary?range=7d&granularity=day
https://<your-cpa-host>/cpa-usage/
https://<your-cpa-host>/usage/healthz
```

Browser verification:

- Confirm the primary chart defaults to Hour.
- Confirm switching to Day changes the trend and summary labels.
- Confirm the heatmap is date-by-hour, not a daily block list.
- Confirm KPI cards show previous-period comparison or explicit missing previous data.
- Confirm static assets load under `/cpa-usage`.

## Compatibility Decision

This is an additive analytics contract change. Existing fields remain available. Existing ingestion, Cost calculation, Key Alias semantics, pricing configuration, `/usage` service, and CPA root service remain compatible.

No fallback is introduced for unsupported granularity. Invalid values should fail fast with a `400` response so callers do not silently read the wrong aggregation.

## Open Follow-Ups

- Date range picker remains out of scope for this refinement.
- Minute or 15-minute granularity remains out of scope.
- Weekday-average heatmap remains out of scope.
- AI-generated insight text remains out of scope.
