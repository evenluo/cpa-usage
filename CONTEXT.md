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
A restricted operational reading that keeps user-triggered CPA generic `api-call` probes separate from the latest supported passive quota watermarks already present in CPA auth-file metadata.
_Avoid_: Usage quota analytics, billing quota, quota history, inferred expiry or automatic provider calls

**Last updated**:
The latest successful quota observation time for an account, whether obtained by a manual refresh or reported by CPA. Elapsed time alone does not invalidate a reading; a failed refresh or missing report does not erase the previous successful observation.
_Avoid_: Metadata sync time, token refresh time, cache expiry, proof of present capacity

**Registered Model Support**:
An explicitly loaded, selected-scope reading of models currently registered to auth-file accounts in CPA, optionally enriched by exact-ID static capability metadata.
_Avoid_: Live model availability, routable model catalog, provider health

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

**Observed Model Alias**:
The alias label present on a CPA **Usage Attempt**, shown beside its actual model and provider. Equality with the actual model means only that no distinct alias was observed; it may be direct or producer-canonicalized.
_Avoid_: Requested model, client intent, configured route

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
- **Live Capacity** is a restricted **Fixed Operational Window** reading for operator visibility. It presents the latest successful manual CPA generic `api-call` probes and supported passive CPA auth-file observations with their original source times.
- **Live Capacity** displays active auth-file accounts that can be probed for capacity. Unsupported auth-file accounts are shown explicitly instead of blocking supported accounts.
- **Live Capacity** can disable or re-enable an auth-file account in CPA via the account power action; disabling requires an inline confirmation, enabling applies immediately. Disabled accounts stay visible with a Disabled badge, sink to the end of the account grid, and are excluded from capacity probes until re-enabled.
- Successful account toggles re-read CPA account state before reporting success; lifecycle status and availability are never inferred from the requested disabled flag. If CPA accepted the change but readback or persistence fails, the UI reloads identities and reports the unconfirmed result; Trigger Sync is the explicit recovery path.
- Disabled auth-file accounts remain active identities in the local read model with a disabled marker instead of being dropped during metadata sync, so their historical usage and re-enable action stay available.
- Temporarily unavailable auth-file accounts also remain the same active local identities. Operator-disabled, temporarily unavailable, and CPA lifecycle status are independent observations; absence of a reported status or availability flag does not imply an active account.
- **Live Capacity** keeps CPA auth-file metadata observation time separate from upstream token refresh time and quota observation time. A reported next-retry time is only the earliest retry eligibility, never a recovery guarantee.
- Claude and Codex passive quota observations keep their original CPA observation time, account/model scope and supported quota readings. Missing, empty, unsupported or malformed reports provide no new observation; they do not erase previous successful readings or imply zero usage or health.
- Manual probes and CPA reports retain their source identity. When both describe the same quota window, the newer observation supplies the displayed reading. Passive retry hints never replace the account's reported retry eligibility.
- Availability metadata compatibility: **Compatible**. Availability columns are nullable and their API fields are additive and optional; existing rows without these observations remain unknown.
- Loading **Usage Intelligence** reads retained quota observations; manual refresh is the user action that may trigger provider calls.
- **Live Capacity** retains successful readings and **Last updated** across page reloads, service restarts, failed refreshes and account disabling. A newer successful observation replaces an earlier one; elapsed time alone does not clear readings or mark them expired. Accounts that have never supplied quota observations show **No reading**.
- **Last updated** includes account and model quota observations, but excludes metadata sync and token refresh times. Per-reading source times remain available so the account timestamp does not imply that every window was observed together.
- **Live Capacity** shows the auth-file active window and provider quota reset times as reported. These timestamps are operational evidence, not inferred cache expiry, billing renewal or account-history claims.
- A manual **Live Capacity** refresh is rejected as unavailable once its worker lifecycle starts shutting down; it must not return a task that cannot run.
- **Live Capacity** follows provider filtering, but the **Selected Analysis Window** and **Time Granularity** do not change its query key or probe window.
- **Registered Model Support** is loaded only by an explicit user action for a selected auth-file account set; loading **Usage Intelligence** never fans out model-support requests.
- **Registered Model Support** remains separate from **Live Capacity**: registry membership and static capability metadata do not prove current routing availability, capacity, or health.
- Selected-scope model coverage is complete only when every selected account model lookup succeeds. A failed account is unknown rather than unsupported, and a one-supporting-account conclusion is available only for a complete selected scope.
- Static model capability metadata joins registered models by exact model ID only. Missing definitions, unknown channels, and catalog errors stay explicit; no family fallback is inferred.
- **Activity Heatmap** uses a fixed 30-day **Fixed Operational Window** with date-by-hour cells to show recent usage rhythm. Its dedicated frontend load uses day granularity and remains independent of the **Selected Analysis Window**.
- Attempt health and **Request Evidence** use fixed 24-hour **Fixed Operational Windows** to show recent stability and supporting samples, independent of the **Selected Analysis Window**.
- Usage event counts, success rates, and failure rates describe **Usage Attempts**. A request ID is correlation detail only and does not collapse retries or imply the final client-visible outcome.
- New Redis-ingested attempts use their persisted inbox row as stable identity. Historical request-ID-collapsed rows remain unchanged because missing attempts cannot be reconstructed locally.
- Newly popped Redis records are projected to replay-required fields before inbox persistence. Provider failure bodies, response headers, and unknown fields do not enter SQLite or its backups; malformed records retain only a digest and byte length for decode-failure observability.
- Accounting v2 is the sole token metric source under the [accepted direct cut](docs/design/accounting-v2-direct-cut.md). New intake requires the supported schema and explicit generate/stream. The repository owns canonical validation, quality and metric interpretation; persisted availability is only valid/absent and accepted wire versions are not repeated in storage or reads; malformed/unsupported new messages use the existing observable inbox failure lifecycle.
- KPI, trend, contributors, Request Evidence and CSV use canonical buckets. Historical raw attempts retain known counts/status/timing but their absent canonical facts remain unavailable. Coverage counts valid canonical attempts; complete/inconsistent/unclassified quality remains separately visible. Output TPS includes canonical reasoning output and requires complete generating-streaming evidence. Local three-rate Cost uses complete canonical observations, preserving operator configuration without implying upstream billed amounts.
- Global Provider filtering scopes **Selected Analysis Window** modules and **Fixed Operational Window** modules except **Attempt performance**, which owns its Provider selection.
- **Request Evidence** drill-down preserves the current provider scope and begins that scope on its first result page.
- **Request Evidence** drill-down may further filter its fixed 24-hour attempt set by actual model, **Observed Model Alias**, and attempt result.
- **Failure concentration** appears as a compact failed-attempt summary inside Attempt Health, with its six-dimensional breakdown collapsed by default. It is a fixed 24-hour diagnostic reading of failed **Usage Attempts**, grouped independently by observed status family, exact status, provider, account, actual model, and public endpoint path. Missing status stays in an Unknown bucket; an observed HTTP status is evidence, not a proven root cause.
- **Attempt performance** is a fixed 24-hour diagnostic reading of nearest-rank p50/p95 latency, TTFT, and **Output TPS**. Successful and failed latency, known generating/streaming execution, and historical unknown execution stay separately qualified with sample coverage. Provider, actual-model, and account comparisons retain excluded lower-rank attempts explicitly, and slow evidence uses an inclusive observed latency threshold.
- Selecting a **Failure concentration** breakdown opens first-page **Request Evidence** with the same provider/model/account/endpoint/status selection and the distribution's exact 24-hour snapshot window. Each breakdown is bounded to ranked rows and preserves omitted or unavailable attempts in an explicit other count.
- **Observed model mappings** is a collapsed, on-demand detail with alias coverage visible in its summary. Its expanded rows connect observed aliases to actual models/providers and compare attempt shares within the observed-alias population, retaining omitted counts. It is a distinct fixed 24-hour explanation of CPA alias labels split by actual model/provider. It reports missing-alias coverage and bounded omitted attempts; alias equality is direct-or-canonicalized, not proof of a requested model. Complete mapping rows open first-page **Request Evidence** with the exact alias/model/provider and snapshot window, while Model Mix remains the selected-window actual-model view.
- Global Provider filter options are derived from the **Selected Analysis Window**, not from fixed windows or a global provider catalog.
- The default heatmap measure is token volume because it represents usage intensity without depending on pricing completeness.
- **Activity Heatmap** can switch locally between Tokens, Attempts, and Failures without changing its fixed 30-day query or triggering another read. Its legend and cells distinguish no activity, observed zero, missing token facts, partial token coverage, and out-of-window hours. Failure shading describes counts, not failure rates.
- **Token breakdown** stays folded in its own section below the KPI grid so expanding it does not stretch the KPI cards. Input and output bars each show their own internal composition; unclassified tokens remain a separate amount. Per-side zero totals have no percentage, and missing token facts never become zero. Attempt coverage and producer quality remain separate observations.
- **Failure concentration** bars use each group's share of all failed attempts in the snapshot, never a group's failure rate. Six independent dimensions retain the backend rank order, distinguish expandable returned rows from unavailable/lower-ranked groups, and preserve exact evidence links.
- **Attempt performance** shows its comparison chart by default, initially grouped by actual model; provider and account dimensions are local selections. Its primary plots select successful latency, generating-streaming TTFT, or Output TPS locally; failed latency remains a separate collapsed comparison. Each dimension uses its own labelled linear axis for p50/p95, keeps all returned groups ordered by attempt volume, and keeps sample counts and full coverage in clickable details while partial or unavailable coverage remains visible. These are frequent groups, not a slowest ranking; absent percentiles have no plotted point. The card has its own Provider selector, initially set to the most-used provider in the last 24 hours (provider name breaks request-count ties). Its complete option list is independent of the global range and provider; explicit selection remains stable across refreshes. All card metrics and evidence links use that provider, so Output TPS is immediately scoped for comparison.
- The first heatmap view uses date-by-hour buckets for the fixed 30-day **Fixed Operational Window**, not weekday averages and not the **Selected Analysis Window**.
- KPI comparison uses the immediately previous period for the same selected range; missing previous-period data is shown explicitly instead of inferred.
- **Cache Read Share** is canonical cache-read input divided by canonical total input. Missing historical canonical facts remain unavailable; accounting attempt coverage separately reports valid canonical attempts against all attempts. Generic cached scalars never supply a fallback numerator or denominator.
- **Metric Completeness** warnings explain incomplete interpretation, not false or invalid usage events.
- Leaderboards use **Cost** ordering only when Cost is complete; otherwise they order by canonical token volume and qualify incomplete Cost.
- The default analytics breakdown dimensions are **Key Alias**, model, and time.
- **Model Mix** is a current visible **Selected Analysis Window** reading of the returned model breakdown; rankings and shares are scoped to that returned set. Deterministic **Insights** appear only when a warning affects how the selected window should be interpreted; informational maxima remain in their owning trend, ranking, or metric surface.
- The Dashboard reads from selected-window totals, trends and consumption composition into fixed 24-hour performance and health, followed by current Live Capacity and the fixed 30-day Activity Heatmap. Attempt performance is the primary comparison in the diagnostic section; Attempt Health owns compact failure analysis beside Request Evidence, followed by on-demand model mappings. Time windows remain fixed; the performance card owns a separate Provider selection while the other diagnostic cards follow the global Provider filter.
- **Request Evidence** supports **Usage Intelligence** with recent attempt samples; it is not the complete request event inspection surface.
- **Request Evidence** status and failure metadata describe the selected upstream attempt, not the final client request outcome.
- **Correlated attempts** are distinct Request Evidence rows sharing one nonempty request ID inside the same fixed 24-hour window and visible provider scope. Entering correlation clears filters that could hide sibling attempts; it does not infer retries, ordering, missing historical attempts, or a final client outcome.
- **Request Evidence** reads tokens, execution, **Output TPS**, and requested/response service tiers from the single `attempt_facts` projection. TPS requires complete canonical accounting, generating/streaming execution and valid timing; unavailable values display `-`.
- **Request Evidence** displays nullable generate/stream execution facts as Yes, No, or Unknown and can preserve an inclusive minimum-latency diagnostic selection.
- **Request Evidence** drill-down lives inside **Usage Intelligence** as a secondary explanation path, not as a top-level Events page and not inside the **Operations Console**.
- First-version insights are conditional deterministic warnings, not AI-generated summaries and not a duplicate summary of visible metrics.
- **Usage Intelligence** insights prioritize metric completeness and health risks; cost, token, and contributor movements remain in their owning analysis surfaces.
- CPA native quota administration remains out of scope; **Live Capacity** is read-only for passive quota metadata and retains the existing explicit manual probe action.
- The first **Operations Console** covers manual sync state, rollup backfill coverage from the existing status contract, runtime state, access state, and logout. It does not claim background-ingestion freshness.
- Update-check actions and update-check state are explicit non-features for the current web frontend because there is no user-facing update-management workflow.
- Backup inspection and log inspection are explicit non-features for the current web frontend because **Operations Console** should stay simple and lightweight.
- Logout should leave the user at the login surface rather than keeping them inside a protected workspace.
- A successful manual sync should refresh usage, evidence, identity, and reference-data read models in the frontend.
- Operations keeps manual sync state separate from local ingestion observations. Pending inbox rows, the last observed nonempty processing batch, scrape-derived processing rate, and runner state do not establish upstream freshness or end-to-end ingestion health; a missing observation is not a zero or a health verdict.
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
