# Web Capability Coverage

## Conclusion

The current web frontend is reviewed as a product capability coverage surface, not as a one-to-one endpoint index. A backend endpoint can remain uncovered when the product boundary deliberately excludes it from the first web information architecture.

The first web information architecture has three workspaces:

- **Usage Intelligence**: aggregate usage reading, selected-window analytics, fixed-window activity and health evidence.
- **Reference Data**: user-maintained labels and model cost rates that make usage analytics readable and complete.
- **Operations Console**: lightweight sync, runtime, and access state.

## Compatibility Decision

The current web navigation is intentionally incompatible with the old `/keys`, `/pricing`, `/events`, and `/settings` pages. Those old paths are not retained and do not redirect because their old page semantics have been merged into the three workspace model.

## Coverage Matrix

| Workspace | User-facing capability | Frontend owner module | Backend/API capability | Backend owner module | Source data | Coverage status | Decision |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Usage Intelligence | KPI cards, primary trend, Key Alias leaderboard, model/token breakdown, provider options, deterministic insights | `web/src/routes/index.lazy.tsx`, `web/src/hooks/useAnalytics.ts`, `web/src/components/charts/*` | `GET /api/v1/analytics/summary?range=&granularity=&provider=` | `internal/api/analytics.go`, `internal/service/analytics_service.go`, `internal/repository/analytics.go` | SQLite usage events, usage identities, key aliases, cost rates | Covered | Uses the **Selected Analysis Window**. |
| Usage Intelligence | Activity Heatmap | `web/src/routes/index.lazy.tsx`, `web/src/hooks/useAnalytics.ts`, `web/src/components/charts/heatmap.tsx` | `GET /api/v1/analytics/summary?range=30d&granularity=hour&provider=` heatmap payload | `internal/api/analytics.go`, `internal/service/analytics_service.go`, `internal/repository/analytics.go` | SQLite usage events | Covered | Fixed 30-day hourly **Fixed Operational Window**. |
| Usage Intelligence | Request Health | `web/src/routes/index.lazy.tsx`, `web/src/hooks/useUsageOverview.ts`, `web/src/components/charts/health-grid.tsx` | `GET /api/v1/usage/overview?range=24h&provider=` service health payload | `internal/api/usage_overview.go`, `internal/service/usage_service.go`, `internal/repository/usage.go` | SQLite usage events | Covered | Fixed 24-hour **Fixed Operational Window**. |
| Usage Intelligence | Request Evidence | `web/src/components/intelligence/request-evidence.tsx`, `web/src/hooks/useEvents.ts` | `GET /api/v1/usage/events?range=24h&page_size=10` | `internal/api/usage_events.go`, `internal/service/usage_service.go`, `internal/repository/usage.go` | SQLite usage events and usage identity resolution | Covered as evidence strip | Not a full event search or audit log. |
| Reference Data | Key Alias search, edit, and clear | `web/src/routes/reference.lazy.tsx`, `web/src/hooks/useKeys.ts` | `GET /api/v1/usage/identities/page`, `PUT /api/v1/usage/identities/:id/alias`, `DELETE /api/v1/usage/identities/:id/alias` | `internal/api/usage_identities.go`, `internal/service/usage_identities_service.go`, `internal/service/key_alias_service.go`, `internal/repository/usage_identities.go`, `internal/repository/key_alias.go` | Usage identities and key aliases | Covered | First-version alias management is direct editing only. |
| Reference Data | Cost Rate view and save | `web/src/routes/reference.lazy.tsx`, `web/src/hooks/usePricing.ts` | `GET /api/v1/pricing`, `GET /api/v1/models/used`, `PUT /api/v1/pricing` | `internal/api/pricing.go`, `internal/service/pricing_service.go`, `internal/repository/pricing.go` | Used models and model price settings | Covered | Saving/overwriting rates is required for the primary completeness workflow. |
| Reference Data | Cost Rate delete | No first-version frontend owner | `DELETE /api/v1/pricing?model=` | `internal/api/pricing.go`, `internal/service/pricing_service.go`, `internal/repository/pricing.go` | Model price settings | Deliberate gap | Secondary maintenance action, not first-version blocking coverage. |
| Operations Console | Sync state and manual sync | `web/src/routes/operations.lazy.tsx`, `web/src/hooks/useStatus.ts` | `GET /api/v1/status`, `POST /api/v1/sync` | `internal/api/router.go`, `internal/app/manual_sync.go`, `internal/poller/*` | Poller status and sync runner | Covered | First-version operations scope. |
| Operations Console | Runtime state | `web/src/routes/operations.lazy.tsx`, `web/src/hooks/useStatus.ts` | `GET /api/v1/status` | `internal/api/router.go`, `internal/version/version.go`, `internal/config/config.go` | Runtime config and version metadata | Covered | Displays version and timezone only. |
| Operations Console | Access state and logout | `web/src/routes/operations.lazy.tsx`, `web/src/hooks/useAuth.ts` | `GET /api/v1/auth/session`, `POST /api/v1/auth/logout` | `internal/api/auth.go`, `internal/auth/session.go` | In-memory dashboard session | Covered | First-version operations scope. |
| Operations Console | Update check execution and state | No frontend owner | `GET /api/v1/update/check` | `internal/api/update.go`, `internal/updatecheck/checker.go` | GitHub release checker | Explicit non-feature | Current web and `GET /api/v1/status` do not expose update-check actions or update-check state. |
| Operations Console | Quota check, cache, and refresh | No frontend owner | `POST /api/v1/quota/check`, `POST /api/v1/quota/cache`, `POST /api/v1/quota/refresh`, `GET /api/v1/quota/refresh/:task_id` | `internal/api/quota.go`, `internal/quota/*` | CPA management API and provider quota services | Explicit non-feature | CPA Usage is a pure usage product and does not expose account-capacity operations. |
| Operations Console | Backup/log inspection | No frontend owner | Runtime backup and logging behavior | `internal/backup/*`, `internal/logging/*` | SQLite backups and log files | Explicit non-feature | The current web frontend keeps operations simple and does not expose backup or log inspection. |

## Review Rules

- A missing frontend entry is a **bug** only when the capability is inside the confirmed first-version workspace boundary.
- A missing frontend entry is a **deliberate gap** when the product boundary excludes it from the first version despite backend support.
- A missing frontend entry is an **explicit non-feature** when the product boundary deliberately excludes it beyond the first version.
- A missing frontend entry is **product scope pending** when the backend capability is real but the product has not decided whether to expose it.
- Provider filtering scopes both selected-window analytics and fixed-window health/activity/evidence modules.
- Fixed operational windows must be visually labeled so users do not expect them to follow the selected analysis range.

## Review Sequence

1. Review **Usage Intelligence** data-window consistency: selected-window modules, fixed-window modules, and provider scope.
2. Review **Reference Data** write workflows: alias and rate saves, invalidation, error display, empty values, and **Metric Completeness** effects.
3. Review **Operations Console** action safety: manual sync state, rate limits, and error visibility.
4. Review deliberately excluded capabilities to confirm UI copy does not imply support for Cost Rate deletion, update execution, quota operations, backup inspection, or log inspection.

## Review Findings

### Provider Scope For Fixed Windows

Status: fixed and verified.

The first **Usage Intelligence** review found that provider filtering was not consistently applied to fixed-window modules. Analytics and Activity Heatmap were scoped by provider, but Request Health and **Request Evidence** were effectively reading all providers.

Impact:

- Request Health could show 24-hour health for all providers while the selected provider filter visually suggested a narrower scope.
- **Request Evidence** could show recent samples from providers outside the selected provider.

Fix:

- Parse `provider` in shared usage filter query parsing.
- Pass provider through usage service calls into repository usage queries.
- Apply provider scope to usage overview, usage analysis, event filter options, and event list queries.
- Pass provider from **Usage Intelligence** into **Request Evidence** and include it in the events query key and request URL.

Verification:

- `go test ./internal/api ./internal/repository ./internal/service`
- `npm --prefix ./web run typecheck`

### Cost Rate Empty State

Status: fixed and verified.

An unset **Cost Rate** should display as `-`. Numeric values represent real configured rates, including `0` when the user explicitly chooses a zero rate.

Impact:

- A missing model must not be silently converted into a configured `0/0/0` rate.
- `0` remains a valid configured value, but it is not the default meaning of an unset rate.

Fixed mismatch:

- The Reference Data form initializes missing model rates as `0`.
- Saving an untouched missing model can convert an unset **Cost Rate** into a configured zero rate.

Fix:

- Missing model rate drafts now start as empty strings.
- Rate inputs use `-` as the empty placeholder.
- Saving requires all three rate fields to contain explicit numeric values.

Verification:

- `npm --prefix ./web run typecheck`

### Key Alias Empty Save

Status: fixed and verified.

An empty **Key Alias** is not a saved alias value. Clearing a **Key Alias** must use the distinct Clear action.

Fixed mismatch:

- The Reference Data form can submit an empty alias through Save.
- Save and Clear can therefore express the same outcome through two different user intentions.

Fix:

- Empty alias saves now show an error and do not call the save API.
- Clearing remains a distinct user action.

Verification:

- `npm --prefix ./web run typecheck`

### Operations Logout

Status: fixed and verified.

The first **Operations Console** includes access state and logout.

Fixed mismatch:

- The Operations Access card shows whether the dashboard session is authenticated.
- It does not provide a logout action despite backend support for `POST /api/v1/auth/logout`.

Fix:

- The Operations Access card now includes a logout action.
- Successful logout updates the auth session query to unauthenticated and refreshes session state.
- Logout should navigate to the login surface rather than leaving the user inside a protected workspace.
- Unauthenticated access to protected workspaces redirects to the login surface.

Verification:

- `npm --prefix ./web run typecheck`

### Manual Sync Read-Model Refresh

Status: fixed and verified.

A successful manual sync should refresh frontend read models that depend on newly ingested usage and identity data.
Manual sync completion should also refresh status after failures or warnings so backend `last_error` and `last_warning` become visible.

Fixed mismatch:

- The Operations sync action refreshes status only.
- **Usage Intelligence**, **Request Evidence**, **Reference Data**, and used-model rate completeness can remain stale until their normal query refresh.

Required refreshes:

- `["status"]`
- `["analytics", "summary"]`
- `["usage", "overview"]`
- `["events"]`
- `["keys", "identities"]`
- `["pricing"]`

Additional status refresh:

- Sync success, warning, and error paths invalidate `["status"]`.

Verification:

- `npm --prefix ./web run typecheck`

### Key Leaderboard Sort Label

Status: fixed and verified.

Leaderboards default to **Cost** when cost metrics are complete; token volume becomes the ordering measure when cost is unavailable.

Fixed mismatch:

- The **Usage Intelligence** Key Leaderboard label always says `Sort: Cost`.
- When **Cost** is unavailable or partial, that label can misrepresent the actual reading basis.

Fix:

- The label now follows summary cost status:
  - `available` -> `Sort: Cost`
  - `partial` -> `Sort: Cost partial`
  - `unavailable` -> `Sort: Tokens`

Verification:

- `npm --prefix ./web run typecheck`

### Cache KPI Vocabulary

Status: fixed and verified.

The cache KPI represents **Cache Read Share**, not request-level cache hit rate.

Fixed mismatch:

- The **Usage Intelligence** Cache KPI caption uses `Cache hit rate`.
- That term conflicts with the project glossary and can be read as request hit rate.

Fix:

- The caption now uses `Cache Read Share`.

Verification:

- `npm --prefix ./web run typecheck`

### Request Evidence Identity Display

Status: fixed and verified.

**Request Evidence** should read as supporting evidence inside **Usage Intelligence**, not as raw request log output.

Fixed mismatch:

- The evidence card prefers `auth_index` over the resolved `source` display value.
- This makes recent evidence harder to read and over-emphasizes raw traceability.

Fix:

- **Request Evidence** now prefers resolved `source` display text and uses `auth_index` only as fallback traceability.

Verification:

- `npm --prefix ./web run typecheck`

## Follow-Up Decisions

- If `Request Evidence` exposes a drill-down later, it should live inside **Usage Intelligence** as a secondary explanation path. It should not restore a top-level Events page and should not move event analysis into **Operations Console**.
- An empty **Key Alias** is not a saved alias value. Users should clear an alias through a distinct Clear action rather than by saving an empty edit field.
- Provider filter options are derived from the **Selected Analysis Window**, not from fixed windows or a global provider catalog.
- Key Leaderboard continues to sort by priced **Cost** when **Cost** is partial, but must label the sort as partial. It falls back to token volume only when **Cost** is unavailable.
- The **Reference Data** Key Alias list may show `identity` as secondary traceability because the primary readable label remains alias or display name. This is not a first-version coverage bug.
- Saving a **Cost Rate** must refresh **Reference Data** rates and **Usage Intelligence** analytics. It does not need to immediately refresh Reference key usage totals in the first version.
- Saving or clearing a **Key Alias** must refresh **Reference Data** identities and **Usage Intelligence** analytics. The current first-version invalidation boundary is sufficient.
- Deleting a **Cost Rate** remains a possible future secondary maintenance action inside **Reference Data**; it is not a first-version blocking capability and is not an explicit non-feature.
- Update-check actions and update-check state are explicit non-features for the current web frontend because there is no user-facing update-management workflow.
- Keeping the backend `/api/v1/update/check` endpoint does not create a web product commitment because the current web frontend does not expose, imply, or depend on update-check behavior.
- Quota operations are an explicit non-feature for the current web frontend because CPA Usage remains a pure usage product. Account-capacity operations should not enter **Usage Intelligence**, **Reference Data**, or **Operations Console**.
- Keeping backend quota endpoints does not create a web product commitment because the current web frontend does not expose, imply, or depend on account-capacity behavior.
- Backup and log inspection are explicit non-features for the current web frontend because **Operations Console** should stay simple and lightweight.

## Open Review Questions

None.
