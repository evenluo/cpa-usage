# Passive quota observation contract

CPA Usage reads passive quota watermarks only from the existing successful `/management/auth-files` metadata snapshot. It does not call a provider while serving the page, add a worker, or merge these facts into the manual capacity-probe cache.

## Producer and shape

The pinned producer is CLIProxyAPI commit `c76dfd4e0edabab9000628b1560ab8ab379eadb8`. Supported auth-file entries expose:

- `quota`: the latest account observation as `observed_at` plus bounded `signals`;
- `model_quotas`: zero or more model observations with the same shape.

Only Claude and Codex are supported. Missing observations on these supported providers remain unavailable. A response with no new provider signal may retain an older observation in CPA, so CPA Usage persists its original `observed_at`; metadata sync time is not substituted. Missing, empty, malformed and unsupported observations produce no passive fact. A successful complete auth-file snapshot still owns identity absence/deletion as documented by the account lifecycle contract.

## Allowlisted interpretation

Claude:

- named 5-hour and 7-day `Utilization` values are fractions with denominator `1`; valid `[0,1]` values become `[0,100]` percent;
- named, generic unified and Fable-specific `7d_oi` `Status` values recognize `allowed`, `allowed_warning` and `rejected` only;
- reset values may be Unix seconds, RFC3339 or HTTP-date;
- numeric `Retry-After` is delta seconds; absolute RFC3339 or HTTP-date is a timestamp.

The Fable-specific row is model-scoped and does not invent utilization. Claude observations do not establish token counts, absolute request limits, dollars or billing renewal.

Codex:

- base, code-review and named additional primary/secondary `Used-Percent` values use denominator `100` and must be within `[0,100]`;
- `Window-Minutes` is converted to seconds; `Reset-After-Seconds` remains seconds; `Reset-At` is Unix seconds;
- `Allowed` and `Limit-Reached` are boolean limit state. Contradictory pairs stay unknown while independent valid window facts remain readable;
- bounded `Limit-Name`, `Plan-Type` and `Active-Limit` remain labels/identifiers, not routing authority;
- credit balance is measured in provider credits with no known denominator or dollar conversion. `Has-Credits` and `Unlimited` remain separate booleans.

Unknown signal names, non-string values, control characters, oversized values, invalid times and invalid numeric ranges are discarded. Raw signal maps, arbitrary headers and bodies never enter the database or HTTP response. `X-Ratelimit-*` and `Over-Secondary-Limit-Percent` are not interpreted because the pinned source does not establish stable product semantics for this surface.

## Read-model semantics

The identity projection emits only normalized `passive_quota` and `passive_model_quotas`, each with `source=cpa_passive`, explicit scope and original observation time. Passive data does not alter identity status, unavailable/disabled state, plan ordering, manual probe status, `next_retry_after`, or refresh eligibility. It has no inferred expiry or local history.
