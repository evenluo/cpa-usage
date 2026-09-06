# Usage diagnostic selection

Status: current

Owner: `internal/api` normalization, `internal/repository` bounded read aggregation

Consumers: failure distribution, attempt performance, observed model mappings, Request Evidence, and CSV export

## Contract

`UsageDiagnosticFilter` is the single repository projection for a diagnostic selection. The API layer owns query-string validation and normalization; consumers must pass the normalized DTO instead of reparsing these values or copying status rules.

The selection is one exact 24-hour snapshot:

- `range` is absent or `24h`.
- `window_end` is an optional RFC3339Nano snapshot anchor. Failure distribution returns its exact `window_end`; drill-down passes it to Request Evidence so aggregate counts and first-page rows use identical inclusive bounds.
- `provider`, `model`, `model_alias`, and `account` are trimmed exact values with a 128-byte maximum. `model_alias` selects the observed CPA alias label and does not mean a client-requested model.
- `endpoint` is a trimmed exact public path with a 256-byte maximum. Query strings, fragments, and control characters are rejected. Stored endpoint queries/fragments are removed before grouping and matching so they cannot enter the response.
- `status` is absent, `unknown`, `other`, one family from `1xx` through `5xx`, or one canonical decimal HTTP code from 100 through 599. `unknown` selects missing status (`status_code = 0`); `other` selects observed nonzero values outside the HTTP range.
- `request_id` is an optional trimmed exact value with a 256-byte maximum and no control characters. Its presence makes the event-list query diagnostic, so both inclusive 24-hour bounds are always concrete. It correlates rows; it never deduplicates them or establishes retry order or a final client outcome.
- `min_latency_ms` is absent or one canonical positive decimal int64. It selects attempts with `latency_ms >= min_latency_ms`; the inclusive edge preserves every attempt tied at a percentile threshold.

Failure distribution always adds `failed = true`. Attempt performance adds its documented result and execution populations. Observed model mappings and Request Evidence apply the same selection; Request Evidence separately carries its existing pagination and `result` selection. A failure breakdown link uses `result=failed`, page one, and the returned `window_end`. A complete mapping row uses its exact `model_alias`, actual `model`, provider, page one, and returned `window_end`. Slow-attempt links add the relevant result and the observed p95 as inclusive `min_latency_ms`.

The event-list response publishes its normalized `window_end` when the query has an upper time bound. The correlated-attempt action constructs a complete new selection from exactly that returned anchor, the current provider scope, and the selected nonempty `request_id`; the API derives and enforces the inclusive 24-hour lower bound. The action explicitly clears model, model alias, account, endpoint, status, minimum latency, result, source, and auth-index restrictions so siblings are not hidden. Consumers adding another event-list filter must add it to the complete frontend search type and explicitly decide whether correlation clears it; spreading the prior selection is not the contract.

## Response and boundedness

`GET /api/v1/usage/failures` returns `total_failures` plus independent breakdowns for status categories, exact statuses, providers, accounts, models, and endpoints. Each breakdown contains at most eight stable rows ordered by count descending and value ascending. `other_count` makes every breakdown sum back to `total_failures`, including blank dimensions, unsupported exact statuses, and rows beyond the limit.

Repository aggregation and request-ID correlation require concrete start and end times and force the existing `idx_usage_events_timestamp_id` search path. Correlated rows retain deterministic `timestamp DESC, id DESC` ordering. The bounded reads do not load lifetime rows, collapse attempts by request ID, infer missing historical attempts, create Cartesian grids, or add caches/workers.

Each failure, mapping, and performance aggregate uses one SQLite read transaction for its complete response. Delayed intake cannot split population counts, grouping, samples, or excluded counts across snapshots; each query still starts with an independent statement.

The reusable cross-stack fixture is `web/src/test/contracts/usage_failure_distribution.json`. It is the response-shape fixture for dependent slices; repository DTOs remain the code-level consumer contract.

## Failure and compatibility policy

Invalid selection returns HTTP 400 without running a repository query. A successful zero count returns HTTP 200 with empty breakdowns. Repository/API failure remains a distinct HTTP 500 and the frontend preserves stale complete data when available.

This is an additive route, Request Evidence filtering and response-metadata contract. Existing event routes, pagination, non-diagnostic model/provider/source/auth-index/result behavior, selected-window analytics, storage, ingestion, endpoint redaction, auth/session, and deployment behavior remain unchanged. Provider scope remains visible during correlation; matching attempts under another provider are omitted when a provider is selected. Historical request-ID-collapsed rows are retained and qualified rather than reconstructed. Status labels describe observations only and never infer a provider root cause or final client-visible outcome. An observed alias-to-model pair does not infer client intent, fallback cause, account selection, or a final request outcome.
