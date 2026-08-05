# Store UTC, aggregate day buckets in the business timezone

Status: current

CPA Usage stores timestamps and rollup buckets in UTC and applies the business timezone (`TZ`, default `Asia/Shanghai`) only at the boundaries where a calendar day meaning matters: day-granularity aggregation, Today/Yesterday windows, daily maintenance triggers, and log-file rotation. Mixing UTC and localtime at the wrong layer produces silently shifted analytics, so this decision fixes the contract for every layer.

## Decision

- Store event timestamps in UTC semantics: `usage_events.timestamp` is a `time.Time` written and read as UTC; any provider timestamp is normalized to UTC before persistence.
- Key hourly rollups by UTC-aligned hour buckets: `usage_rollups_hourly.bucket_start` is `event.Timestamp.UTC().Truncate(time.Hour)` and never a business-timezone hour.
- Apply `strftime(..., 'localtime')` only for day-granularity aggregation, on both the raw events source and the rollup source. `'localtime'` resolves against the process `time.Local`, which is set from the `TZ` configuration (embedded `time/tzdata`, default `Asia/Shanghai`).
- Hour-granularity buckets stay UTC-aligned (`%Y-%m-%dT%H:00:00Z` for the raw fallback path; rollup buckets are already UTC).
- Compute Today/Yesterday analysis windows as business-timezone calendar-day boundaries (`time.Local` day start/end) and convert the resulting boundaries to UTC before querying. Rolling windows (`24h`, `7d`, `30d`) anchor on `time.Now().UTC()`.
- Keep all SQL filter boundaries in UTC (`StartTime.UTC()` / `EndTime.UTC()`); never compare localtime strings against stored timestamps.
- Use the business timezone for operational scheduling that is calendar-relative: daily storage cleanup and database backup at fixed local times, and daily log-file rotation.
- Backfill and rollup maintenance progress in UTC hour buckets: `target_bucket_start` and `covered_bucket_start` are UTC-aligned.
- Do not convert stored UTC values into a different timezone at the presentation boundary; the frontend formats UTC instants into the operator's locale without shifting the underlying instant.

## Consequences

- Day-granularity analytics follow the configured business timezone while hour-granularity analytics and rollups remain UTC-aligned, giving operators stable daily boundaries without drifting hour buckets across deployments with different `TZ` values.
- A deployment that changes `TZ` re-reads historical daily aggregation under the new calendar-day boundaries while hourly rollup coverage remains unchanged; operators must treat day-level history as timezone-dependent.
- Raw fallback day buckets and rollup day buckets use the same localtime expression, so `backfill_incomplete` fallback results remain consistent with rollup results.
- New analytics SQL must route through the shared bucket expression helpers instead of writing ad-hoc `strftime` calls, so the UTC/localtime split stays in one place.
- The `TZ` environment variable now has a documented dual role: calendar-day analytics boundaries and calendar-relative operational scheduling.

## Compatibility

This decision does not change external behavior for existing deployments: the current implementation already stores UTC and aggregates day buckets with `'localtime'` under the `TZ` configuration. The ADR fixes the contract so future changes (new breakdowns, new windows, or read-model refactors) preserve the split instead of regressing to mixed timezone handling.
