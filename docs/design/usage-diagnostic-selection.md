# Usage diagnostic selection

Status: current

Owner: `internal/api` normalization, `internal/repository` bounded read aggregation

Consumers: failure distribution, Request Evidence, and parent slices C/D/J/K

## Contract

`UsageDiagnosticFilter` is the single repository projection for a diagnostic selection. The API layer owns query-string validation and normalization; consumers must pass the normalized DTO instead of reparsing these values or copying status rules.

The selection is one exact 24-hour snapshot:

- `range` is absent or `24h`.
- `window_end` is an optional RFC3339Nano snapshot anchor. Failure distribution returns its exact `window_end`; drill-down passes it to Request Evidence so aggregate counts and first-page rows use identical inclusive bounds.
- `provider`, `model`, and `account` are trimmed exact values with a 128-byte maximum.
- `endpoint` is a trimmed exact public path with a 256-byte maximum. Query strings, fragments, and control characters are rejected. Stored endpoint queries/fragments are removed before grouping and matching so they cannot enter the response.
- `status` is absent, `unknown`, `other`, one family from `1xx` through `5xx`, or one canonical decimal HTTP code from 100 through 599. `unknown` selects missing status (`status_code = 0`); `other` selects observed nonzero values outside the HTTP range.

Failure distribution always adds `failed = true`. Request Evidence applies the same selection and separately carries its existing pagination and `result` selection. A breakdown link uses `result=failed`, page one, and the returned `window_end`.

## Response and boundedness

`GET /api/v1/usage/failures` returns `total_failures` plus independent breakdowns for status categories, exact statuses, providers, accounts, models, and endpoints. Each breakdown contains at most eight stable rows ordered by count descending and value ascending. `other_count` makes every breakdown sum back to `total_failures`, including blank dimensions, unsupported exact statuses, and rows beyond the limit.

Repository aggregation requires concrete start and end times and forces the existing `idx_usage_events_timestamp_id` search path before grouping. It performs SQL aggregation over the selected 24-hour attempts; it does not load raw lifetime rows, collapse attempts by request ID, create Cartesian grids, or add caches/workers.

The reusable cross-stack fixture is `web/src/test/contracts/usage_failure_distribution.json`. It is the response-shape fixture for dependent slices; repository DTOs remain the code-level consumer contract.

## Failure and compatibility policy

Invalid selection returns HTTP 400 without running a repository query. A successful zero count returns HTTP 200 with empty breakdowns. Repository/API failure remains a distinct HTTP 500 and the frontend preserves stale complete data when available.

This is a compatible additive route and additive Request Evidence filtering contract. Existing event routes, pagination, selected-window analytics, storage, ingestion, auth/session, and deployment behavior remain unchanged. Status labels describe observations only and never infer a provider root cause or final client-visible outcome.
