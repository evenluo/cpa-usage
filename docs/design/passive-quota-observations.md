# Passive quota observation contract

CPA Usage reads passive quota watermarks only from the existing successful `/management/auth-files` metadata snapshot. It does not call a provider while serving the page, add a worker, or merge these facts into the retained manual capacity-probe observations. [Quota observation retention](quota-observation-retention.md) defines their shared user-facing lifetime and **Last updated** semantics.

## Producer and shape

The pinned producer is CLIProxyAPI commit `c76dfd4e0edabab9000628b1560ab8ab379eadb8`. Supported auth-file entries expose:

- `quota`: the latest account observation as `observed_at` plus bounded `signals`;
- `model_quotas`: zero or more snapshots captured during requests to each model, with the same shape. The model key identifies the request that produced the snapshot; it does not identify an independent quota pool.

The pinned producer [records the same response headers in account and request-model state](https://github.com/router-for-me/CLIProxyAPI/blob/c76dfd4e0edabab9000628b1560ab8ab379eadb8/sdk/cliproxy/auth/conductor_cooldown.go#L927-L931). In particular, Codex model records can contain the regular account limit and additional limits such as Spark together. Different model records can show different percentages solely because their latest quota-bearing responses occurred at different times.

Only Claude and Codex are supported. A response with no new provider signal may retain an older observation in CPA, so CPA Usage persists its original `observed_at`; metadata sync time is not substituted. Missing, empty, malformed and unsupported observations produce no new passive fact and do not clear a previously stored successful observation. Each account and model retains its latest successful observation; older or equal-time reports do not replace it. A successful complete auth-file snapshot still owns identity absence/deletion as documented by the account lifecycle contract.

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

The identity projection emits only normalized `passive_quota` and `passive_model_quotas`, each with `source=cpa_passive`, explicit scope and original observation time. Passive data does not alter identity status, unavailable/disabled state, plan ordering, manual probe status, `next_retry_after`, or refresh eligibility. It has no inferred expiry or observation-history collection; only the latest successful observation per scope is retained.

The dashboard display layer may union passive and manual-probe readings per window (newer observation wins) as long as each meter keeps its own source and observation time in the tooltip; this merging never writes back or changes the backend semantics above. Claude retains fixed 5h and Weekly slots with a "No reading" placeholder when absent. Codex renders only observed ordinary windows, ordered 5h then Weekly; a Weekly-only account has no 5h placeholder. Reported-only rows without a known window (unknown-window "Window" rows, credit balances) remain in the folded section. A Codex account with no account quota entries shows one neutral **No account quota readings** message, without naming unobserved allowances.

Codex cards do not render model snapshots as additional quota meters. Those repeated account readings remain in the API and database, with their request-model names, original observation times, active limits and readings. They do not contribute to the card's folded item count or **Last updated** time. A model snapshot cannot supply a missing account allowance, including Reserve. An older Terra reading must never be presented as Terra's independent allowance or relabelled as Luna Reserve. Claude retains the separate **Model request observations** section.

## Codex additional allowances

Manual probes preserve every parsed additional rate-limit window, including `gpt-reserve` and entries whose `metered_feature` is `base_model_inference`. Neither field justifies dropping a reading or treating its reset time as synthetic. Passive normalization continues to preserve supported named limits.

The UI displays explicitly named `gpt-reserve` account limits as **Luna Reserve** in the card's main area. The raw identifier remains visible, and the original label remains the key for observation matching. The explanation is **Extra Luna usage after regular usage is exhausted.** If no account reading exists, the card omits the Reserve section entirely. It does not invent an allowance slot or infer eligibility from absence. Absence in the stored observation does not establish whether OpenAI currently reports Reserve; an explicit refresh may collect a reading previously discarded by an older version. A successful refresh without Reserve must not create a placeholder for it.

[OpenAI's Luna Reserve documentation](https://help.openai.com/en/articles/20001499-luna-reserve-in-codex-and-chatgpt-work) defines the separate allowance, available only to selected accounts; the `gpt-reserve` identifier is corroborated by the [CPA reserve-routing report](https://github.com/router-for-me/CLIProxyAPI/issues/5568), not an official wire-format contract. An unfamiliar limit with the same metered feature retains its original name. This display neither routes requests nor establishes that CPA can consume the reserve. Missing data does not imply zero allowance or account eligibility.

Compatibility (2026-09-11): existing API fields, raw normalized labels, SQLite records, observation scope and retention remain compatible. Codex presentation intentionally removes duplicated model meters and unobserved window/Reserve placeholders, while making actual Reserve readings visible on the card surface. Manual refreshes retain additional limits that the earlier implementation discarded. Already discarded readings cannot be reconstructed; a later successful explicit refresh can supply them. No data migration, automatic probe or fallback routing is introduced.
