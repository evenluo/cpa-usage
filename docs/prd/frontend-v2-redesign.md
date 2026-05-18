# Frontend v2 Redesign — Usage Intelligence Dashboard

## Problem Statement

The original CPA Usage frontend was a single 1,800-line `App.tsx` file that lacked visual sophistication and polished interaction design. It has been replaced in the active source tree; `web/` is the only frontend implementation. Users frequently interact with the dashboard to monitor AI API consumption, and the interface should feel intentional rather than merely utilitarian. Key pain points addressed by v2 include:

- **Visual flatness**: Generic card layouts, uninspired color palette, and chart styling that does not convey precision or care.
- **Interaction friction**: No loading skeletons, abrupt data changes, hard-coded "Last 7 days" range, and manual routing via `window.location.pathname`.
- **Information hierarchy**: The Dashboard crams all analytics into a single view without clear reading flow. Breakdown sections compete for attention rather than guide the eye.
- **Theme rigidity**: No dark mode support, despite users potentially monitoring usage at varying times and lighting conditions.
- **Error handling**: Failed API calls result in blank areas or generic text, with no retry mechanism or graceful degradation.

## Solution

Use the `web/` frontend with a **Claude-inspired editorial aesthetic** — warm terracotta accents, clean typography, generous whitespace, and a reading-flow information architecture. The frontend will:

1. Present **Usage Intelligence** as a curated data-reading experience rather than a raw metrics dump.
2. Support **light/dark dual themes** with system-level auto-detection and manual override.
3. Use **intelligent polling** for data freshness without unnecessary background requests.
4. Provide **per-module loading and error states** with skeletons, retry buttons, and toast notifications.
5. Offer **flexible time-range selection** (Today / Yesterday / Last 24h / 7d / 30d) with automatic Time Granularity adaptation.
6. Deliver **smooth page transitions** and micro-interactions (count-up numbers, hover lift, staggered reveals).

## User Stories

1. As a platform operator, I want the Dashboard to load with elegant skeleton placeholders, so that the interface feels responsive and intentional even before data arrives.
2. As a platform operator, I want to toggle between Today, Yesterday, Last 24h, 7d, and 30d time ranges, so that I can inspect usage at different time scales.
3. As a platform operator, I want the time range switch to automatically pick the right Time Granularity (hourly for short ranges, daily for 30d), so that I don't have to think about it.
4. As a platform operator, I want the Dashboard to auto-refresh data while the tab is visible, so that I see near-real-time usage without manually reloading.
5. As a platform operator, I want the Dashboard to pause background refreshes when I switch to another tab, so that it doesn't waste resources.
6. As a platform operator, I want a warm, editorial visual design with thoughtful typography and spacing, so that monitoring usage feels like using a premium product rather than a generic admin panel.
7. As a platform operator, I want dark mode support that follows my OS preference, so that I can comfortably view the dashboard at any time of day.
8. As a platform operator, I want KPI cards with embedded sparklines and count-up number animations, so that I can instantly grasp trends and magnitudes.
9. As a platform operator, I want the Cost trend chart to show Cost as a filled area and Token volume as a dotted overlay, so that I can understand the relationship between spend and volume.
10. As a platform operator, I want a Key Alias Leaderboard that shows top contributors ranked by Cost (or Token volume when Cost is unavailable), so that I can identify which keys are driving usage.
11. As a platform operator, I want the Model Mix breakdown to show a donut chart alongside a detailed model list, so that I can see both proportion and absolute numbers.
12. As a platform operator, I want the Token Heatmap to use warm terracotta tones with rounded cells, so that time-pattern visualization feels cohesive with the overall design.
13. As a platform operator, I want the Insight Rail to display deterministic insights in a priority order (metric completeness first, then health risks, then cost/token movements), so that I notice warnings before they become problems.
14. As a platform operator, I want each data module to fail independently with a clear error state and retry button, so that one broken API doesn't blank the entire Dashboard.
15. As a platform operator, I want toast notifications for successful and failed operations (like saving a **Key Alias** or **Cost Rate**), so that I get clear feedback without losing context.
16. As a platform operator, I want toast notifications to stack elegantly in the top-right corner (bottom on mobile) with auto-dismiss and copy-to-clipboard support, so that I can capture error messages if needed.
17. As a platform operator, I want a compact icon-only sidebar on desktop that expands on hover, so that navigation is always accessible without stealing screen real estate.
18. As a platform operator, I want a bottom tab bar on mobile, so that I can navigate between pages with thumb-friendly interactions.
19. As a platform operator, I want page transitions to animate smoothly (fade/slide), so that navigation feels fluid rather than jarring.
20. As a platform operator, I want the Reference page to manage **Key Aliases** and **Cost Rates** in one continuous workspace, so that the supporting data for **Usage Intelligence** does not feel scattered across thin pages.
21. As a platform operator, I want missing **Cost Rates** to be visible next to configured model rates, so that I know which models need rate setup before **Cost** can be read as complete.
22. As a platform operator, I want recent Request Events to appear as a compact evidence strip inside **Usage Intelligence**, so that I can inspect supporting request samples without leaving the primary dashboard.
23. As a platform operator, I want an Operations console for sync state, runtime state, and access state, so that maintenance actions are grouped separately from analytical reading.
24. As a platform operator, I want all numbers to be formatted in English locale with adaptive precision (e.g., 4 decimal places for small Cost, 2 for large), so that readability is maintained across magnitudes.
25. As a platform operator, I want the Provider filter to be a compact, elegant toggle group, so that I can filter by provider without visual clutter.

## Implementation Decisions

### Technology Stack
- **Framework**: React 19 + Vite + TypeScript
- **Styling**: Tailwind CSS v3 with custom design tokens (warm terracotta palette, cream backgrounds)
- **Components**: shadcn/ui primitives (Button, Card, Tabs, Badge, Input, Dialog, Tooltip, etc.)
- **Routing**: TanStack Router for type-safe, animated route transitions
- **Data Fetching**: TanStack Query with intelligent polling (visibility-aware)
- **Charts**: Recharts with heavy customization to match the editorial aesthetic
- **Fonts**: Newsreader (serif, display headings), DM Sans (sans-serif, body), JetBrains Mono (data/monospace)

### Theme Architecture
- CSS variables drive the color system. Two complete token sets: light (cream background, terracotta accent) and dark (deep charcoal background, more saturated terracotta).
- Theme preference stored in `localStorage` with three states: `system` (default), `light`, `dark`.
- `matchMedia('(prefers-color-scheme: dark)')` listener for system-level changes when preference is `system`.
- Tailwind `darkMode: ['class']` strategy with `.dark` class on `<html>`.

### Navigation Architecture
- **Desktop**: Left sidebar, ~56px wide, icon-only. Hover reveals label tooltip. Active state uses terracotta accent.
- **Mobile**: Bottom tab bar with 3 primary workspaces: Intelligence, Reference, Operations.
- **Information architecture**: The app uses 3 primary workspaces:
  - **Intelligence** (`/`) is the default workspace for **Usage Intelligence** and includes a compact Request Evidence strip.
  - **Reference** (`/reference`) is a single-page **Reference Data** workspace for **Key Aliases** and **Cost Rates**.
  - **Operations** (`/operations`) is a lightweight operations console for sync state, runtime state, and access state.
- **Compatibility**: This is a new frontend project. Old route paths such as `/keys`, `/pricing`, `/events`, and `/settings` are not retained and do not redirect.
- **Page transitions**: Framer Motion or CSS transitions for `fade-in` + `slide-up` on route change.

### Data Fetching Strategy
- All API calls go through TanStack Query hooks with `staleTime: 30s` and `refetchInterval: 60s` when document is visible.
- `refetchOnWindowFocus: 'always'` to refresh when user returns to the tab.
- Each data module (KPIs, Trend, Key Leaderboard, Model Mix, Heatmap, Insights, Health) fetches independently via its own query key, enabling independent loading/error states.
- Analytics summary query invalidated when time range, granularity, or provider filter changes.

### Time Range & Granularity
- UI: Pill/toggle group with 5 options: Today, Yesterday, Last 24h, 7d (default), 30d.
- Granularity mapping:
  - Today / Yesterday / Last 24h → `hour`
  - 7d → `hour` (could also offer `day` toggle)
  - 30d → `day`
- Range parameter mapping to API:
  - Today → `range=today`
  - Yesterday → `range=yesterday`
  - Last 24h → `range=24h`
  - 7d → `range=7d`
  - 30d → `range=30d`
- URL query params synced via TanStack Router for shareable/bookmarkable state.

### Chart Styling
- **Trend Chart**: Recharts `<AreaChart>` for Cost with a warm terracotta gradient fill. `<Line>` with dotted stroke for Token volume. Custom tooltip with refined styling.
- **Model Mix**: Recharts `<PieChart>` as a donut (innerRadius ~60%) with a legend to the right showing model details.
- **Heatmap**: Custom CSS Grid (not Recharts) for full control over cell shapes. Terracotta gradient from cream to terracotta-500. Rounded corners (`rounded-md`).
- **Sparklines**: Recharts `<AreaChart>` with no axes, no grid, no labels — just a smooth area fill with gradient.

### Loading & Error States
- **Loading**: Per-module skeletons using shadcn/ui Skeleton component. KPI cards show pulse rectangles. Charts show skeleton boxes with subtle shimmer.
- **Error**: Each module shows an inline error card with an icon, brief message, and "Retry" button. Does not block other modules.
- **Empty**: Custom empty states with illustration/icon and contextual copy (e.g., "No model usage in this range").

### Toast System
- Custom toast hook using React context.
- Position: top-right on desktop (`top-4 right-4`), bottom-center on mobile.
- Max 3 visible toasts. New toasts push older ones out.
- Auto-dismiss after 4 seconds with a progress bar. Pause on hover.
- Types: `success` (terracotta), `error` (rose), `warning` (amber), `info` (slate).
- Copy button on each toast to copy message to clipboard.

### Number Formatting
- All numbers formatted with `Intl.NumberFormat('en', ...)`.
- Cost: `$0.00` for large values, `$0.0000` for values < 1.
- Tokens: compact notation (`1.2M`, `845K`) with 2 significant digits.
- Percentages: `1` decimal place.
- Counts: locale string (`12,345`).

### API Integration
All existing `/api/v1/*` endpoints remain unchanged. New frontend consumes:
- `GET /api/v1/auth/session` — auth check
- `POST /api/v1/auth/login` — login
- `GET /api/v1/analytics/summary?range=&granularity=&provider=` — Dashboard data
- `GET /api/v1/usage/identities/page?page=&page_size=` — Key list
- `PUT/DELETE /api/v1/usage/identities/:id/alias` — Alias CRUD
- `GET /api/v1/usage/events?range=&page_size=` — Events
- `GET/PUT /api/v1/pricing` — Pricing
- `GET /api/v1/models/used` — Used models
- `GET /api/v1/status` — System status
- `POST /api/v1/sync` — Manual sync

### Module Boundaries (Deep Modules)
1. **ThemeProvider** — Encapsulates all theme logic (system detection, persistence, CSS variable injection). Simple interface: `<ThemeProvider>{children}</ThemeProvider>`.
2. **ApiClient** — Thin fetch wrapper with base path awareness (`window.__APP_BASE_PATH__`), auth headers, and JSON parsing. All data fetching goes through here.
3. **useAnalytics** TanStack Query hook — Encapsulates analytics data fetching, caching, polling, and parameter serialization. Returns `{ data, isLoading, isError, refetch }`.
4. **ChartComponents** — Recharts wrappers with design-system styling baked in. Props are data-only; no styling concerns leak to consumers.
5. **ToastProvider** — Global toast state management. Interface: `toast.success(message)`, `toast.error(message)`, etc.
6. **Sidebar / MobileNav** — Responsive navigation component. Desktop renders icon rail; mobile renders bottom tabs. Route definitions centralized.

## Testing Decisions

- **What makes a good test**: Tests verify external behavior ("given this API response, the KPI card shows $1,234.56") rather than implementation details ("the component calls `useQuery` with these exact options").
- **Modules to test**:
  - `lib/format.ts` — Number/date formatting functions (pure, easy to test).
  - `hooks/useAnalytics.ts` — Mock TanStack Query's `useQuery` and verify correct query keys and parameter transformation.
  - `components/ui/chart-components` — Verify Recharts receives correctly transformed data.
  - `lib/api-client.ts` — Verify base path injection, auth header attachment, and error handling.
- **Test tooling**: `web/` now includes Vitest + React Testing Library for feature-level regression coverage alongside lint, TypeScript typecheck, and Vite build in the frontend verification gate.

## Out of Scope

1. **Data export** (CSV/JSON) — Not requested; can be added later.
2. **Keyboard shortcuts** — Not requested; focus on visual polish first.
3. **PWA / offline support** — Not requested; standard web app is sufficient.
4. **Customizable Dashboard layout** (drag-to-rearrange cards) — Would add significant complexity; static curated layout is preferred for editorial aesthetic.
5. **Notifications / alerting** — No alerting mechanism exists in the backend; out of scope.
6. **Real-time WebSocket updates** — Polling is sufficient for this use case.
7. **Modifications to backend API** — All existing endpoints used as-is.
8. **Compatibility with the removed old frontend implementation** — `web/` is the production frontend source and does not keep component, route, or test compatibility with the removed implementation.

## Further Notes

- The design direction is intentionally opinionated. The "Claude aesthetic" — warm, editorial, restrained — is chosen to differentiate CPA Usage from generic SaaS dashboards. Avoid the temptation to add features that would dilute this vision (e.g., excessive color accents, busy animations).
- Performance target: First Contentful Paint < 1.5s on a 4G connection. Code-split routes with lazy loading.
- Accessibility: All charts need aria-labels and keyboard-navigable controls. Toast notifications should be announced via `aria-live`.
- The build output (`web/dist/`) is embedded in the Go static file serving pipeline. The Go backend's `router.go` serves `index.html` with base path injection — this mechanism remains unchanged.
