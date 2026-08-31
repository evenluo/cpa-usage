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

**Cost Rate**:
A user-maintained model unit rate used to calculate **Cost** from token usage.
_Avoid_: Price, pricing entry

**Cache Read Share**:
The share of observed prompt input served from explicit provider cache reads. It is exact at complete prompt-input token coverage and otherwise carries an explicit partial coverage percentage; legacy generic `cached_tokens` is not used for this metric.
_Avoid_: Cache hit rate, cache/reasoning share

**Metric Completeness**:
The degree to which a dashboard metric has the required supporting data to be read as a complete conclusion.
_Avoid_: Data trust, data truth

**Usage Intelligence**:
The primary analytics workspace for understanding aggregate usage, cost, attempt health, and time patterns before drilling into individual events.
_Avoid_: Request event log, raw events page

**Time Granularity**:
The selected aggregation level for time-series analytics, such as hourly or daily buckets, applied to the selected analysis window.
_Avoid_: Fixed by-day chart, chart-only grouping

**Selected Analysis Window**:
The user-selected range and granularity used to read current aggregate usage, trends, and ranked contributors.
_Avoid_: Global dashboard range, page range

**Fixed Operational Window**:
A fixed recent window used for stable activity, health, or evidence readings that should not change with the selected analysis range.
_Avoid_: Ignored filter, stale range

**Live Capacity**:
A restricted operational reading that uses CPA generic `api-call` support to probe cached or refreshed auth-file account capacity.
_Avoid_: Usage quota analytics, CPA native quota source, billing quota

**Reference Data**:
Supporting user-maintained labels and rates that make **Usage Intelligence** readable and complete.
_Avoid_: Credentials, setup data, generic data

**Operations Console**:
The workspace for maintaining ingestion, runtime, and access state.
_Avoid_: Settings, admin panel, analytics controls

**Request Evidence**:
Recent **Usage Attempt** samples shown inside **Usage Intelligence** to support aggregate health and usage readings.
_Avoid_: Request event workbench, full event search, audit log

**Usage Attempt**:
One CPA usage record for one upstream provider call. Retries, failovers, and additional-model calls can be separate attempts that share one request ID.
_Avoid_: Final request outcome, request-ID event

**Output TPS**:
The per-attempt provider-normalized output tokens generated per second after the first token; it excludes input and cached tokens.
_Avoid_: Total token TPS, Effective TPS, Visible TPS

## Relationships

- A **CPA Key** may have zero or one global **Key Alias**
- A **Key Alias** belongs to exactly one **CPA Key**
- Multiple **CPA Keys** may share the same **Key Alias**.
- Usage attribution remains attached to the **CPA Key**, not to the **Key Alias**
- **Key Aliases** are stored by CPA Usage and are not written back to CPA.
- A **Key Alias** remains available for historical usage even if the **CPA Key** is no longer active in CPA.
- The Reference Data view is where users manage **Key Aliases** and **Cost Rates**.
- The current web navigation is an incompatible information-architecture change from the old Keys, Pricing, Events, and Settings pages to **Usage Intelligence**, **Reference Data**, and **Operations Console**.
- The first alias management version supports direct editing only, not bulk import or export.
- The first **Reference Data** version supports viewing and saving **Cost Rates**; deleting a **Cost Rate** is a secondary maintenance action, not required for the primary completeness workflow.
- An unset **Cost Rate** is displayed as `-`; only an explicitly entered numeric value represents a real configured **Cost Rate**.
- An empty **Key Alias** is not a saved alias value; clearing a **Key Alias** is a distinct user action from saving one.
- Saved **Key Alias** edits should immediately affect current dashboard labels without waiting for the next CPA sync.
- Search and filters can match both **Key Alias** and **CPA Key**.
- When a **Key Alias** exists, it is the primary display label and the masked **CPA Key** is secondary traceability text.
- The analytics page is organized around total usage first, then breakdowns by dimensions such as **Key Alias**, model, provider, and health.
- **Usage Intelligence** is the aggregate dashboard surface; request events are supporting drill-down evidence, not the primary dashboard structure.
- Token volume and **Cost** are peer measurement categories in the dashboard.
- The primary trend is controlled by **Time Granularity** and must not be limited to a fixed by-day aggregation.
- The frontend defaults the 30-day **Selected Analysis Window** to daily granularity and all other selectable windows to hourly granularity; an explicit user selection overrides that default.
- **Usage Intelligence** uses the **Selected Analysis Window** for KPIs, primary trends, and ranked contributors.
- **Usage Intelligence** may also include **Fixed Operational Windows** for activity density, attempt health, recent request evidence, and **Live Capacity**.
- **Live Capacity** is a restricted **Fixed Operational Window** reading for operator visibility; it is powered by CPA generic `api-call` quota probes and cached refresh tasks, not by a CPA native quota datasource.
- **Live Capacity** displays active auth-file accounts that can be probed for capacity. Unsupported auth-file accounts are shown explicitly instead of blocking supported accounts.
- **Live Capacity** is cache-first. Loading **Usage Intelligence** reads cached quota probe results only; manual refresh is the user action that may trigger provider calls.
- **Live Capacity** shows the probe observation and cache-expiry times, the auth-file active window, and every provider quota row returned by the existing probe contract. These timestamps and rows are operational evidence, not billing renewal or account-history claims.
- A manual **Live Capacity** refresh is rejected as unavailable once its worker lifecycle starts shutting down; it must not return a task that cannot run.
- **Live Capacity** follows provider filtering, but the **Selected Analysis Window** and **Time Granularity** do not change its query key or probe window.
- **Activity Heatmap** uses a fixed 30-day **Fixed Operational Window** with date-by-hour cells to show recent usage rhythm. Its dedicated frontend load uses day granularity and remains independent of the **Selected Analysis Window**.
- Attempt health and **Request Evidence** use fixed 24-hour **Fixed Operational Windows** to show recent stability and supporting samples, independent of the **Selected Analysis Window**.
- Usage event counts, success rates, and failure rates describe **Usage Attempts**. A request ID is correlation detail only and does not collapse retries or imply the final client-visible outcome.
- New Redis-ingested attempts use their persisted inbox row as stable identity. Historical request-ID-collapsed rows remain unchanged because missing attempts cannot be reconstructed locally.
- Newly popped Redis records are projected to replay-required fields before inbox persistence. Provider failure bodies, response headers, and unknown fields do not enter SQLite or its backups; malformed records retain only a digest and byte length for decode-failure observability.
- Provider filtering scopes both **Selected Analysis Window** modules and **Fixed Operational Window** modules.
- **Request Evidence** drill-down preserves the current provider scope and begins that scope on its first result page.
- **Request Evidence** drill-down may further filter its fixed 24-hour attempt set by actual model and attempt result.
- Provider filter options are derived from the **Selected Analysis Window**, not from fixed windows or a global provider catalog.
- The default heatmap measure is token volume because it represents usage intensity without depending on pricing completeness.
- The first heatmap view uses date-by-hour buckets for the fixed 30-day **Fixed Operational Window**, not weekday averages and not the **Selected Analysis Window**.
- KPI comparison uses the immediately previous period for the same selected range; missing previous-period data is shown explicitly instead of inferred.
- **Cache Read Share** is calculated only from internally consistent explicit provider cache-read observations: provider-normalized prompt input must be positive and cache-read tokens must be between zero and that input total. Invalid provider facts contribute to neither the read numerator nor the observed-input denominator, while Request Evidence preserves and shows their raw generic, cache-read, and cache-creation values separately. Coverage is the valid observed prompt-input token volume divided by total prompt-input token volume; complete coverage is `Exact`, incomplete nonzero coverage is `Partial`, and no valid observed coverage is unavailable.
- **Metric Completeness** warnings explain incomplete interpretation, not false or invalid usage events.
- Leaderboards default to **Cost** ordering when cost metrics are complete; partial **Cost** may still order by the priced cost portion when labeled as partial; token volume becomes the ordering measure when cost is unavailable.
- The default analytics breakdown dimensions are **Key Alias**, model, and time.
- **Model Mix** is a current visible **Selected Analysis Window** reading of the returned model breakdown; rankings and shares are scoped to that returned set. Deterministic **Insights** appear only when a warning affects how the selected window should be interpreted; informational maxima remain in their owning trend, ranking, or metric surface.
- Attempt health appears as a stability breakdown within analytics, not as the primary dashboard story.
- **Request Evidence** supports **Usage Intelligence** with recent attempt samples; it is not the complete request event inspection surface.
- **Request Evidence** status and failure metadata describe the selected upstream attempt, not the final client request outcome.
- **Request Evidence** displays **Output TPS** only when output tokens, total latency, and time to first token are available and internally consistent; otherwise it displays `-` instead of estimating a fallback value.
- **Request Evidence** drill-down lives inside **Usage Intelligence** as a secondary explanation path, not as a top-level Events page and not inside the **Operations Console**.
- First-version insights are conditional deterministic warnings, not AI-generated summaries and not a duplicate summary of visible metrics.
- **Usage Intelligence** insights prioritize metric completeness and health risks; cost, token, and contributor movements remain in their owning analysis surfaces.
- CPA native quota administration remains out of scope; the supported capacity surface is the restricted **Live Capacity** probe inside **Usage Intelligence**.
- The first **Operations Console** covers manual sync state, rollup backfill coverage from the existing status contract, runtime state, access state, and logout. It does not claim background-ingestion freshness.
- Update-check actions and update-check state are explicit non-features for the current web frontend because there is no user-facing update-management workflow.
- Backup inspection and log inspection are explicit non-features for the current web frontend because **Operations Console** should stay simple and lightweight.
- Logout should leave the user at the login surface rather than keeping them inside a protected workspace.
- A successful manual sync should refresh usage, evidence, identity, and reference-data read models in the frontend.
- Production rollout for **Usage Intelligence** refinements updates the `cpa-usage` service on `/usage` and must leave the CPA root service intact.

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
