# CPA Usage Analytics Dashboard v1

## Conclusion

CPA Usage v1 will redesign the product around an analytics-first experience: one data analysis page for total usage and breakdowns, plus focused configuration pages for keys, pricing, request events, and settings. The visual direction should reference the Magic dashboard style: airy white surfaces, clear hierarchy, compact operational density, subtle borders, pill controls, and restrained accent colors.

The first version keeps CPA unchanged. CPA remains the source of raw key attribution, while CPA Usage stores local human-readable aliases and calculated cost.

## Goals

- Make total usage understandable at a glance through cost, tokens, requests, and success rate.
- Treat **Cost** and token volume as peer measurement categories.
- Support breakdowns by **Key Alias**, model, and time as the default analytics dimensions.
- Let users assign human-readable **Key Aliases** to raw **CPA Keys**.
- Keep request health available as a secondary stability analysis, not the primary dashboard story.
- Preserve traceability by always keeping a masked **CPA Key** visible when an alias exists.

## Non-Goals

- Do not write aliases back to CPA.
- Do not build bulk alias import or export in v1.
- Do not add AI-generated summaries in v1.
- Do not make the whole product a long single-page scroll.
- Do not make provider the primary analytics axis unless the user filters or drills into it.

## Information Architecture

The product navigation should separate analysis from configuration:

- **Analytics / 数据分析**: total usage, trends, breakdowns, deterministic insights, and stability.
- **Keys / Key 管理**: alias editing, key search, key metadata, usage totals, and last-used state.
- **Pricing / 计价配置**: model unit price configuration used to calculate **Cost**.
- **Events / 请求明细**: raw request event inspection for debugging and traceability.
- **Settings / 系统设置**: sync status, update check, auth, and operational settings.

## Analytics Page

The analytics page should be compact, not a long report. It should have one primary screen with a small number of high-value sections.

### Section Order

1. **Total Usage**
   - Total Cost
   - Total Tokens
   - Requests
   - Success Rate

2. **Cost and Token Trend**
   - Cost and tokens should be switchable or shown side by side.
   - Cost is the default sorting measure when pricing is complete.
   - If pricing is incomplete, token volume remains authoritative and cost should be marked partial or unavailable.

3. **Deterministic Insights**
   - Top cost key
   - Top token key
   - Cost or token spike
   - Pricing missing warning
   - Failure concentration
   - Cache or reasoning token share

4. **Breakdowns**
   - Key Alias
   - Model
   - Time
   - Provider as filter or secondary dimension

5. **Request Health Timeline**
   - Secondary stability strip.
   - Useful for spotting failure clusters, not the headline story.

## Key Alias Model

**Key Alias** is a global user-defined name for a **CPA Key**.

Rules:

- A **CPA Key** may have zero or one alias.
- Multiple **CPA Keys** may share the same alias.
- Alias does not participate in usage attribution.
- Usage attribution remains attached to the raw **CPA Key**.
- Alias is stored by CPA Usage only and is not written back to CPA.
- Alias remains available for historical usage even if the key is no longer active in CPA.
- Search and filters should match both alias and key.
- When alias exists, alias is the primary label and masked key is secondary traceability text.
- Alias edits should take effect immediately in current UI state after save.

## Keys Page

The Keys page is the editing surface for aliases.

Required v1 behavior:

- Search by alias or key.
- Show alias as editable inline text.
- Show masked key as subtitle.
- Show provider/type badges when available.
- Show last used, total cost, and total tokens.
- Do not include bulk import/export.

## Data Capability Check

Current CPA and cpa-usage-keeper data can support most of the target experience.

### Supported by CPA Usage Events

CPA usage queue records include:

- timestamp
- latency_ms
- source
- auth_index
- tokens
- failed
- provider
- model
- model alias
- endpoint
- auth_type
- api_key
- request_id

These fields support totals, trends, request health, model breakdowns, provider filtering, event traceability, and key-based grouping.

### Supported by Keeper Storage

Keeper already persists request events with:

- provider
- endpoint
- auth_type
- request_id
- model
- model_alias
- timestamp
- source
- auth_index
- failed
- latency_ms
- input/output/reasoning/cached/total tokens

Keeper also stores usage identities, pricing settings, and derived overview/analysis aggregates.

### Existing API Coverage

Existing APIs already cover:

- overall request/token totals
- cost summary
- RPM / TPM
- time series
- model series
- request health timeline
- model analysis
- API-like grouping based on `api_group_key`
- request event list and filters

### Required API Additions

The new analytics direction needs more direct key-centric aggregation:

- Key Alias breakdown by cost, tokens, requests, success/failure, and last-used time.
- Key Alias trend series for cost and tokens.
- Key Alias plus model drill-down.
- Deterministic insight payloads.
- Alias update API.

The existing `api_group_key` analysis is not enough because it is not the same as a user-facing **CPA Key** / **Key Alias** dimension.

## Compatibility Decisions

- **CPA compatibility**: keep compatible. CPA is not changed and aliases are not written back.
- **Usage event compatibility**: keep compatible. Existing events remain valid.
- **Alias compatibility**: additive. Missing alias falls back to existing display name or masked key.
- **Cost compatibility**: keep existing local pricing calculation. If model pricing is missing, show partial or unavailable cost instead of inventing values.
- **UI compatibility**: not preserving old information hierarchy. Existing capabilities remain, but navigation and visual hierarchy change.

## Visual Direction

The UI should reference the Magic dashboard style without copying its exact structure:

- white or warm-grey surfaces
- thin grey borders
- subtle shadows
- pill controls
- compact cards with around 8px radius
- restrained green, amber, violet, and blue accents
- dense but readable operational layout
- no decorative blobs or marketing hero sections
- no one-note purple or dark-blue palette

## Plan Review

Review conclusion: actionable after implementation repo is available.

Identified constraints:

- The current local workspace is only a design workspace and does not contain the full implementation repo.
- Key-centric analytics require new backend aggregation rather than front-end-only rearrangement.
- Cost depends on local pricing completeness, so UI must represent partial cost explicitly.

Resolved in this document:

- Alias ownership and persistence boundary.
- Analytics vs configuration IA.
- Default dimensions and first-version insight scope.
- Compatibility decisions.
