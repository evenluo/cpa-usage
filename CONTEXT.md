# CPA Usage

CPA Usage provides a human-readable usage dashboard on top of CPA usage data.

## Language

**CPA Key**:
A raw key known by CPA and used to attribute usage events.
_Avoid_: Credential, account

**Key Alias**:
A global user-defined human-readable name for a **CPA Key**.
_Avoid_: Credential alias, model alias, redacted alias

**Cost**:
The calculated spend derived from token usage and local model pricing.
_Avoid_: Price

**Cache Read Share**:
The share of provider-normalized prompt input tokens served from cache reads, calculated as cached tokens divided by input tokens.
_Avoid_: Cache hit rate, cache/reasoning share

**Metric Completeness**:
The degree to which a dashboard metric has the required supporting data to be read as a complete conclusion.
_Avoid_: Data trust, data truth

**Usage Intelligence**:
The primary analytics workspace for understanding aggregate usage, cost, request health, and time patterns before drilling into individual events.
_Avoid_: Request event log, raw events page

**Time Granularity**:
The selected aggregation level for time-series analytics, such as hourly or daily buckets, applied consistently across trend charts and time-pattern views.
_Avoid_: Fixed by-day chart, chart-only grouping

## Relationships

- A **CPA Key** may have zero or one global **Key Alias**
- A **Key Alias** belongs to exactly one **CPA Key**
- Multiple **CPA Keys** may share the same **Key Alias**.
- Usage attribution remains attached to the **CPA Key**, not to the **Key Alias**
- **Key Aliases** are stored by CPA Usage and are not written back to CPA.
- A **Key Alias** remains available for historical usage even if the **CPA Key** is no longer active in CPA.
- The Credentials view is where users manage **Key Aliases**.
- The first alias management version supports direct editing only, not bulk import or export.
- Saved **Key Alias** edits should immediately affect current dashboard labels without waiting for the next CPA sync.
- Search and filters can match both **Key Alias** and **CPA Key**.
- When a **Key Alias** exists, it is the primary display label and the masked **CPA Key** is secondary traceability text.
- The analytics page is organized around total usage first, then breakdowns by dimensions such as **Key Alias**, model, provider, and health.
- **Usage Intelligence** is the aggregate dashboard surface; request events are supporting drill-down evidence, not the primary dashboard structure.
- Token volume and **Cost** are peer measurement categories in the dashboard.
- The primary trend is controlled by **Time Granularity** and must not be limited to a fixed by-day aggregation.
- The default **Time Granularity** for the primary trend is hourly; daily is an explicit roll-up mode.
- The default heatmap measure is token volume because it represents usage intensity without depending on pricing completeness.
- The first heatmap view uses date-by-hour buckets for the selected range, not weekday averages.
- KPI comparison uses the immediately previous period for the same selected range; missing previous-period data is shown explicitly instead of inferred.
- The primary trend includes bucket-derived summary stats for average and peak **Cost** and token volume, plus total **Cost** and tokens.
- The first **Usage Intelligence** refinement keeps the selected range fixed to Last 7 days while adding **Time Granularity** support.
- **Cache Read Share** is an efficiency metric for prompt input tokens, not a replacement for **Cost**, token volume, requests, or success rate.
- **Metric Completeness** warnings explain incomplete interpretation, not false or invalid usage events.
- Leaderboards default to **Cost** ordering when cost metrics are complete; token volume becomes the ordering measure when cost is unavailable.
- The default analytics breakdown dimensions are **Key Alias**, model, and time.
- Request health appears as a stability breakdown within analytics, not as the primary dashboard story.
- First-version insights are deterministic metrics and warnings, not AI-generated summaries.
- **Usage Intelligence** insights prioritize metric completeness and health risks before cost, token, and contributor movements.
- Production rollout for **Usage Intelligence** refinements updates the `/cpa-usage` service only and must leave the existing `/usage` service and CPA root service intact.

## Example dialogue

> **Dev:** "When usage events show a raw **CPA Key**, should the dashboard display the **Key Alias** instead?"
> **Domain expert:** "Yes, the key is still the source of truth, but humans should read the alias."
>
> **Dev:** "Should cached output tokens count toward **Cache Read Share**?"
> **Domain expert:** "No — cache reads are measured against provider-normalized prompt input tokens."

## Flagged ambiguities

- "key alias" means a human-readable label for a **CPA Key**, not a generated redaction token or a model alias.
- "cache hit rate" means **Cache Read Share** in CPA Usage, not cached tokens divided by total tokens and not the combined cache/reasoning token share.
- "data trust" should be expressed as **Metric Completeness** when the issue is missing pricing, missing cache-token support, or a zero denominator for a derived metric.
