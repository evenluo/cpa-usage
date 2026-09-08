# Observed CPA model mappings

Status: current, slice D of Parent #140 / Issue #147 r1

## Contract

`GET /api/v1/usage/model-mappings` is a fixed 24-hour diagnostic read. It uses the API-owned selection in [usage-diagnostic-selection.md](usage-diagnostic-selection.md), including the exact RFC3339Nano `window_end`, and returns observed CPA alias labels grouped by actual model and provider. One CPA usage row remains one upstream **Usage Attempt**; request IDs are not collapsed.

The response reports the selected attempt population, attempts with a nonblank observed alias, missing-alias attempts, and alias coverage. Mapping rows are ordered by attempts descending and alias/model/provider ascending, limited to 20 rows, and `other_attempts` preserves the excluded observed population. Missing aliases are a coverage gap and are never assigned to an actual model. Blank actual model or provider facts stay blank in the payload and are presented as unavailable.

CPA v7.2.152 serializes an upstream blank alias as the actual model before emitting usage. Therefore `model_alias == model` means only **No distinct alias observed / direct or canonicalized**. A distinct alias supports only an observed remap. Neither case proves the client-requested name, fallback cause, chosen account, or final client-visible outcome.

Each row carries attempt count, failure count/share, positive-latency sample count and mean, plus the existing operator-maintained three-rate Cost calculation and completeness status. The response-level observed Cost can be partial across model mappings; row and response Cost never add tier multipliers or reinterpret accounting-v2 facts.

Complete rows link to first-page Request Evidence with exact provider, actual model, observed `model_alias`, and snapshot `window_end`. Rows missing model or provider remain visible without a broader link that could falsely claim aggregate/evidence parity. Model Mix remains the selected-window actual-model view and is not replaced or duplicated by this explanatory fixed-window Module.

The reusable response fixture is `web/src/test/contracts/usage_model_mappings.json`. It covers alias splits, an observed remap, alias-equals-model canonicalization, missing alias/provider, failures, latency coverage, and mixed Cost availability using synthetic data only.

## Dashboard presentation

The default surface is a collapsed 24-hour summary showing displayed mapping count, alias coverage and missing aliases. Expanded rows connect the observed alias to actual model/provider and show attempt-share bars using all observed-alias attempts as the denominator, including omitted rows. Same-name rows are labelled `Same observed name`, with no routing inference. The attempt-share denominator stays beside the chart; the routing explanation is available through the clickable About these mappings details. Costs and latency remain supporting values inside the details. Refresh failures remain visible and retryable while the details are collapsed.
