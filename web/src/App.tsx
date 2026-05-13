import { useEffect, useMemo, useState } from 'react'
import {
  Activity,
  BarChart3,
  Bell,
  CalendarRange,
  Check,
  Database,
  KeyRound,
  ListFilter,
  RefreshCw,
  Pencil,
  Search,
  Settings,
  Trash2,
  TableProperties,
  WalletCards,
  X,
} from 'lucide-react'

import { AliasRankingChart, HealthTimeline, MetricTrendChart, ModelDistributionChart, Sparkline, TokenCostCompareChart } from '@/components/charts'
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Tabs, TabsTrigger } from '@/components/ui/tabs'
import { type AliasRow, type ModelRow, type TrendPoint, healthBlocks } from '@/data/analyticsPrototype'

import './index.css'

const navItems = [
  { label: '数据分析 Analytics', href: '/', icon: BarChart3 },
  { label: 'Key 管理 Keys', href: '/keys', icon: KeyRound },
  { label: '计价配置 Pricing', href: '/pricing', icon: WalletCards },
  { label: '请求明细 Events', href: '/events', icon: TableProperties },
  { label: '系统设置 Settings', href: '/settings', icon: Settings },
]

type AppRoute = '/' | '/keys' | '/events' | '/pricing' | '/settings'

function formatCompact(value: number, maximumFractionDigits = 1) {
  return Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits }).format(value)
}

function formatCurrency(value: number) {
  return `$${value.toLocaleString('en', { maximumFractionDigits: 2, minimumFractionDigits: 2 })}`
}

function formatAnalyticsCost(summary: AnalyticsSummaryPayload) {
  return summary.cost_status === 'unavailable' ? 'Unavailable' : formatCurrency(summary.total_cost)
}

function appBasePath() {
  const value = window.__APP_BASE_PATH__
  if (!value || value === '__APP_BASE_PATH__' || value === '/') {
    return ''
  }
  return value.endsWith('/') ? value.slice(0, -1) : value
}

function withBasePath(path: string) {
  return `${appBasePath()}${path}`
}

function currentRoute(): AppRoute {
  const base = appBasePath()
  const pathname = window.location.pathname
  const withoutBase = base && pathname.startsWith(base) ? pathname.slice(base.length) || '/' : pathname
  return withoutBase === '/keys' || withoutBase === '/events' || withoutBase === '/pricing' || withoutBase === '/settings' ? withoutBase : '/'
}

function apiPath(path: string) {
  return withBasePath(`/api/v1${path}`)
}

type AnalyticsSummaryPayload = {
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  success_rate: number
  cost_available: boolean
  cost_status: 'available' | 'partial' | 'unavailable'
}

type AnalyticsTrendPointPayload = {
  label: string
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  cost_available: boolean
  cost_status: 'available' | 'partial' | 'unavailable'
}

type AnalyticsKeyAliasTrendPointPayload = {
  label: string
  total_cost: number
  total_tokens: number
  cost_available: boolean
  cost_status: 'available' | 'partial' | 'unavailable'
}

type AnalyticsKeyAliasPayload = {
  label: string
  alias: string
  traceability: string
  identity: string
  auth_type: number
  auth_type_name: string
  type: string
  provider: string
  is_deleted: boolean
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  success_rate: number
  last_used_at: string | null
  cost_available: boolean
  cost_status: 'available' | 'partial' | 'unavailable'
  trend: AnalyticsKeyAliasTrendPointPayload[]
}

type AnalyticsModelPayload = {
  model: string
  provider: string
  total_cost: number
  total_tokens: number
  request_count: number
  success_count: number
  failure_count: number
  success_rate: number
  total_latency_ms: number
  latency_sample_count: number
  average_latency_ms: number
  cost_available: boolean
  cost_status: 'available' | 'partial' | 'unavailable'
}

type AnalyticsInsightPayload = {
  type: string
  severity: 'green' | 'blue' | 'violet' | 'amber'
  title: string
  detail: string
  subject: string
  metric_label: string
  metric_value: number
  count: number
  cost_status: 'available' | 'partial' | 'unavailable'
}

type AnalyticsSummaryResponse = {
  summary: AnalyticsSummaryPayload
  trend: AnalyticsTrendPointPayload[]
  key_alias_breakdown?: AnalyticsKeyAliasPayload[]
  model_distribution?: AnalyticsModelPayload[]
  time_breakdown?: AnalyticsTrendPointPayload[]
  insights?: AnalyticsInsightPayload[]
}

type AnalyticsState = {
  summary: AnalyticsSummaryPayload
  trend: TrendPoint[]
  keyAliases: AliasRow[]
  models: ModelRow[]
  timeBreakdown: TrendPoint[]
  insights: AnalyticsInsightPayload[]
}

const emptyAnalyticsSummary: AnalyticsSummaryPayload = {
  total_cost: 0,
  total_tokens: 0,
  request_count: 0,
  success_count: 0,
  failure_count: 0,
  success_rate: 0,
  cost_available: true,
  cost_status: 'available',
}

function useAnalyticsSummary(enabled: boolean) {
  const [analytics, setAnalytics] = useState<AnalyticsState>({ summary: emptyAnalyticsSummary, trend: [], keyAliases: [], models: [], timeBreakdown: [], insights: [] })

  useEffect(() => {
    if (!enabled) {
      return
    }
    let active = true
    fetch(apiPath('/analytics/summary?range=7d'))
      .then((response) => {
        if (!response.ok) {
          throw new Error('load analytics failed')
        }
        return response.json() as Promise<AnalyticsSummaryResponse>
      })
      .then((payload) => {
        if (!active) {
          return
        }
        setAnalytics({
          summary: payload.summary ?? emptyAnalyticsSummary,
          trend: (payload.trend ?? []).map((point) => ({
            label: point.label,
            cost: point.total_cost,
            tokens: point.total_tokens,
            requests: point.request_count,
            failures: point.failure_count,
            costAvailable: point.cost_available,
            costStatus: point.cost_status,
          })),
          keyAliases: (payload.key_alias_breakdown ?? []).map((row) => ({
            alias: row.label,
            key: row.traceability || row.identity,
            provider: row.provider || row.type || row.auth_type_name,
            cost: row.total_cost,
            tokens: row.total_tokens,
            requests: row.request_count,
            successRate: row.success_rate,
            failures: row.failure_count,
            isDeleted: row.is_deleted,
            costAvailable: row.cost_available,
            costStatus: row.cost_status,
            trend: row.trend.map((point) => (point.cost_status === 'unavailable' ? point.total_tokens : point.total_cost)),
          })),
          models: (payload.model_distribution ?? []).map((row, index) => ({
            model: row.model,
            provider: row.provider,
            cost: row.total_cost,
            tokens: row.total_tokens,
            requests: row.request_count,
            successRate: row.success_rate,
            averageLatencyMS: row.average_latency_ms,
            costAvailable: row.cost_available,
            costStatus: row.cost_status,
            color: modelPalette[index % modelPalette.length],
          })),
          timeBreakdown: (payload.time_breakdown ?? payload.trend ?? []).map((point) => ({
            label: point.label,
            cost: point.total_cost,
            tokens: point.total_tokens,
            requests: point.request_count,
            failures: point.failure_count,
            costAvailable: point.cost_available,
            costStatus: point.cost_status,
          })),
          insights: payload.insights ?? [],
        })
      })
      .catch(() => {
        if (active) {
          setAnalytics({ summary: emptyAnalyticsSummary, trend: [], keyAliases: [], models: [], timeBreakdown: [], insights: [] })
        }
      })
    return () => {
      active = false
    }
  }, [enabled])

  return analytics
}

const modelPalette = ['#2563eb', '#7c3aed', '#059669', '#d97706', '#0891b2', '#be123c']
type BreakdownMode = 'key_alias' | 'model' | 'time'

function App() {
  const route = currentRoute()
  const [breakdownMode, setBreakdownMode] = useState<BreakdownMode>('key_alias')
  const analytics = useAnalyticsSummary(route === '/')
  const analyticsSummary = analytics.summary
  const analyticsTrend = analytics.trend
  const analyticsAliases = analytics.keyAliases
  const analyticsModels = analytics.models
  const analyticsTimeBreakdown = analytics.timeBreakdown
  const analyticsInsights = analytics.insights

  return (
    <main className="min-h-screen bg-background text-foreground">
      <div className="grid min-h-screen grid-cols-[240px_minmax(0,1fr)] max-lg:grid-cols-1">
        <aside className="border-r border-border bg-card px-5 py-5 max-lg:border-b max-lg:border-r-0">
          <div className="flex items-center gap-3">
            <div className="grid size-9 place-items-center rounded-lg bg-foreground text-background">
              <Database className="size-4" aria-hidden="true" />
            </div>
            <div>
              <p className="text-xs font-semibold uppercase text-muted-foreground">CPA Usage</p>
              <h1 className="text-lg font-semibold tracking-normal">CPA Usage</h1>
            </div>
          </div>

          <nav className="mt-7 grid gap-1 max-lg:grid-cols-5 max-md:grid-cols-2" aria-label="Primary navigation">
            {navItems.map((item, index) => {
              const Icon = item.icon
              const active = item.href === route || (route === '/' && index === 0)
              return (
                <a
                  className={`flex min-h-10 items-center gap-2 rounded-md px-3 text-sm font-semibold ${
                    active ? 'bg-emerald-50 text-emerald-700' : 'text-muted-foreground hover:bg-muted hover:text-foreground'
                  }`}
                  href={withBasePath(item.href)}
                  key={item.href}
                >
                  <Icon className="size-4 shrink-0" aria-hidden="true" />
                  <span>{item.label}</span>
                </a>
              )
            })}
          </nav>
        </aside>

        <section className="min-w-0 px-6 py-5 max-md:px-4" aria-labelledby={routeTitleID(route)}>
          {route === '/keys' ? (
            <KeysWorkspace />
          ) : route === '/events' ? (
            <EventsWorkspace />
          ) : route === '/pricing' ? (
            <PricingWorkspace />
          ) : route === '/settings' ? (
            <SettingsWorkspace />
          ) : (
            <>
          <header className="flex items-start justify-between gap-4 max-md:grid">
            <div>
              <p className="text-xs font-semibold uppercase text-muted-foreground">Analytics / 数据分析</p>
              <h2 id="analytics-title" className="mt-1 text-2xl font-semibold tracking-normal">
                Usage and Cost workspace
              </h2>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              <Button variant="outline" size="sm">
                <CalendarRange className="size-4" aria-hidden="true" />
                Last 7 days
              </Button>
              <Button variant="outline" size="icon" aria-label="Notifications">
                <Bell className="size-4" aria-hidden="true" />
              </Button>
            </div>
          </header>

          <div className="mt-5 grid gap-4 xl:grid-cols-[minmax(0,1.55fr)_minmax(320px,0.85fr)]">
            <section className="grid gap-4">
              <div className="grid gap-3 md:grid-cols-4">
                <MetricCard label="Total Cost" value={formatAnalyticsCost(analyticsSummary)} caption={`Cost ${analyticsSummary.cost_status}`} tone="green" />
                <MetricCard label="Total Tokens" value={formatCompact(analyticsSummary.total_tokens, 2)} caption="Cost peer measure" tone="blue" />
                <MetricCard label="Requests" value={analyticsSummary.request_count.toLocaleString('en')} caption="Selected range total" tone="violet" />
                <MetricCard label="Success Rate" value={`${analyticsSummary.success_rate.toFixed(1)}%`} caption={`${analyticsSummary.failure_count} failures in range`} tone="amber" />
              </div>

              <Card>
                <CardHeader className="flex flex-row items-start justify-between gap-4">
                  <div>
                    <CardTitle>Cost and Token Trend</CardTitle>
                    <CardDescription>Cost and tokens stay visible as peer measures across the primary trend.</CardDescription>
                  </div>
                  <Tabs aria-label="Trend measure">
                    <TabsTrigger aria-selected>Cost</TabsTrigger>
                    <TabsTrigger>Tokens</TabsTrigger>
                    <TabsTrigger>Both</TabsTrigger>
                  </Tabs>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_280px]">
                    <div className="h-[270px] min-w-0">
                      <MetricTrendChart data={analyticsTrend} />
                    </div>
                    <div className="h-[270px] min-w-0 rounded-lg border border-border bg-muted/40 p-3">
                      <TokenCostCompareChart data={analyticsTrend} />
                    </div>
                  </div>
                </CardContent>
              </Card>

              <div className="grid gap-3 md:grid-cols-4">
                {analyticsInsights.length === 0 ? (
                  <Card className="shadow-none">
                    <CardContent className="p-4">
                      <Badge variant="outline">Insights</Badge>
                      <p className="mt-3 text-sm font-semibold">No deterministic insights</p>
                    </CardContent>
                  </Card>
                ) : analyticsInsights.map((insight) => (
                  <Card className="shadow-none" key={insight.type}>
                    <CardContent className="p-4">
                      <Badge variant={insight.severity}>{insight.title}</Badge>
                      <p className="mt-3 text-sm font-semibold">{insight.subject}</p>
                      <p className="mt-1 text-xs text-muted-foreground">{formatInsightMetric(insight)}</p>
                    </CardContent>
                  </Card>
                ))}
              </div>
            </section>

            <Card>
              <CardHeader>
                <CardTitle>Breakdown Controls</CardTitle>
                <CardDescription>Key Alias, model, and time are the default analysis dimensions.</CardDescription>
              </CardHeader>
              <CardContent className="space-y-4">
                <div className="flex items-center gap-2 rounded-md border border-border bg-background px-3 py-2">
                  <Search className="size-4 text-muted-foreground" aria-hidden="true" />
                  <span className="text-sm text-muted-foreground">Search alias or masked CPA Key</span>
                </div>
                <div className="grid grid-cols-3 gap-2">
                  <Button variant={breakdownMode === 'key_alias' ? 'subtle' : 'outline'} size="sm" onClick={() => setBreakdownMode('key_alias')}>Key Alias</Button>
                  <Button variant={breakdownMode === 'model' ? 'subtle' : 'outline'} size="sm" onClick={() => setBreakdownMode('model')}>Model</Button>
                  <Button variant={breakdownMode === 'time' ? 'subtle' : 'outline'} size="sm" onClick={() => setBreakdownMode('time')}>Time</Button>
                </div>
                <div className="flex flex-wrap gap-2">
                  <Badge variant="outline">Provider: All</Badge>
                  <Badge variant="outline">Cost: Partial</Badge>
                  <Badge variant="outline">Health: Visible</Badge>
                </div>
                <div className="rounded-lg border border-border bg-muted/40 p-4">
                  <div className="mb-3 flex items-center gap-2 text-sm font-semibold">
                    <ListFilter className="size-4 text-muted-foreground" aria-hidden="true" />
                    Active workspace
                  </div>
                  <p className="text-sm leading-6 text-muted-foreground">
                    Repeated analysis stays dense: ranking, model mix, time trend, and request health are available without turning the page into a long report.
                  </p>
                </div>
              </CardContent>
            </Card>
          </div>

          <section className="mt-4">
            {breakdownMode === 'key_alias' ? (
            <Card>
              <CardHeader className="flex flex-row items-start justify-between gap-3">
                <div>
                  <CardTitle>Key Alias Ranking</CardTitle>
                  <CardDescription>Alias is primary; masked CPA Key remains secondary for traceability.</CardDescription>
                </div>
                <Badge variant="green">Cost sort</Badge>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 lg:grid-cols-[minmax(0,0.8fr)_minmax(0,1.2fr)]">
                  <div className="h-[250px] min-w-0">
                    <AliasRankingChart rows={analyticsAliases} />
                  </div>
                  <div className="grid gap-2">
                    {analyticsAliases.length === 0 ? (
                      <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
                        No key alias usage in this range
                      </div>
                    ) : analyticsAliases.map((row) => (
                      <div className="grid grid-cols-[minmax(0,1fr)_86px_96px] items-center gap-3 rounded-md border border-border p-3" key={row.key}>
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{row.alias}</p>
                          <p className="truncate text-xs text-muted-foreground">
                            {row.key}
                          </p>
                          {row.isDeleted ? <Badge variant="outline">Deleted</Badge> : null}
                        </div>
                        <div className="h-9">
                          <Sparkline values={row.trend} />
                        </div>
                        <div className="text-right">
                          <p className="text-sm font-semibold">{row.costAvailable === false ? 'Cost unavailable' : formatCost(row.cost)}</p>
                          <p className="text-xs text-muted-foreground">{formatCompact(row.tokens, 2)} tokens</p>
                          <p className="text-xs text-muted-foreground">{row.successRate}% success</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
            ) : null}
            {breakdownMode === 'model' ? (
            <Card>
              <CardHeader>
                <CardTitle>Model Distribution</CardTitle>
                <CardDescription>Provider remains a secondary dimension while model impact is shown by Cost and tokens.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-[180px_minmax(0,1fr)] xl:grid-cols-1 2xl:grid-cols-[180px_minmax(0,1fr)]">
                  <div className="h-[190px] min-w-0">
                    <ModelDistributionChart rows={analyticsModels} />
                  </div>
                  <div className="grid gap-2">
                    {analyticsModels.length === 0 ? (
                      <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
                        No model usage in this range
                      </div>
                    ) : analyticsModels.map((row) => (
                      <div className="grid grid-cols-[10px_minmax(0,1fr)_auto] items-center gap-3" key={row.model}>
                        <span className="h-10 rounded-full" style={{ backgroundColor: row.color }} />
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{row.model}</p>
                          <p className="text-xs text-muted-foreground">
                            {row.provider || 'Provider unknown'} · {Intl.NumberFormat('en', { notation: 'compact' }).format(row.tokens)} tokens · {formatCompact(row.requests, 1)} requests
                          </p>
                        </div>
                        <div className="text-right">
                          <p className="text-sm font-semibold">{row.costAvailable === false ? 'Cost unavailable' : formatCost(row.cost)}</p>
                          <p className="text-xs text-muted-foreground">{row.averageLatencyMS ? `${row.averageLatencyMS.toFixed(0)}ms avg` : 'No latency samples'}</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
            ) : null}
            {breakdownMode === 'time' ? (
            <Card>
              <CardHeader>
                <CardTitle>Time Breakdown</CardTitle>
                <CardDescription>Time buckets keep Cost and tokens visible over the selected range.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 lg:grid-cols-[minmax(0,1fr)_minmax(280px,0.8fr)]">
                  <div className="h-[250px] min-w-0">
                    <TokenCostCompareChart data={analyticsTimeBreakdown} />
                  </div>
                  <div className="grid gap-2">
                    {analyticsTimeBreakdown.length === 0 ? (
                      <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">
                        No time bucket usage in this range
                      </div>
                    ) : analyticsTimeBreakdown.map((row) => (
                      <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3 rounded-md border border-border p-3" key={row.label}>
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{row.label}</p>
                          <p className="text-xs text-muted-foreground">{formatCompact(row.requests, 1)} requests · {row.failures} failures</p>
                        </div>
                        <div className="text-right">
                          <p className="text-sm font-semibold">{formatBreakdownCost(row)}</p>
                          <p className="text-xs text-muted-foreground">{formatCompact(row.tokens, 2)} tokens</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
            ) : null}
          </section>

          <Card className="mt-4">
            <CardHeader className="flex flex-row items-start justify-between gap-4">
              <div>
                <CardTitle>Request Health Timeline</CardTitle>
                <CardDescription>Secondary stability strip for connecting usage spikes with failures.</CardDescription>
              </div>
              <Badge variant="amber">
                <Activity className="mr-1 size-3" aria-hidden="true" />
                Secondary
              </Badge>
            </CardHeader>
            <CardContent>
              <div className="h-[170px] min-w-0">
                <HealthTimeline data={healthBlocks} />
              </div>
            </CardContent>
          </Card>
            </>
          )}
        </section>
      </div>
    </main>
  )
}

type KeyIdentity = {
  id: number
  name: string
  displayName: string
  alias: string
  auth_type: number
  auth_type_name: string
  identity: string
  type: string
  provider: string
  total_tokens: number
  total_cost: number
  cost_available: boolean
  last_used_at: string | null
}

type KeyIdentityPage = {
  identities: KeyIdentity[]
  total_pages?: number
}

type UsageEvent = {
  id?: number
  timestamp: string
  model: string
  source: string
  auth_index?: string
  failed: boolean
  latency_ms: number
  tokens: {
    total_tokens: number
  }
}

type UsageEventsPage = {
  events: UsageEvent[]
}

type PricingEntry = {
  model: string
  prompt_price_per_1m: number
  completion_price_per_1m: number
  cache_price_per_1m: number
}

type PricingPayload = {
  pricing: PricingEntry[]
}

type UsedModelsPayload = {
  models: string[]
}

type PricingDraft = {
  prompt: string
  completion: string
  cache: string
}

type StatusPayload = {
  running?: boolean
  sync_running?: boolean
  last_status?: string
  last_run_at?: string
  last_error?: string
  last_warning?: string
  timezone?: string
  version?: string
  updateCheckEnabled?: boolean
}

type AuthSessionPayload = {
  authenticated?: boolean
}

const keyIdentityPageSize = 100

async function fetchKeyIdentityPage(page: number) {
  const response = await fetch(apiPath(`/usage/identities/page?page=${page}&page_size=${keyIdentityPageSize}`))
  if (!response.ok) {
    throw new Error('load keys failed')
  }
  return response.json() as Promise<KeyIdentityPage>
}

async function fetchAllKeyIdentities() {
  const firstPage = await fetchKeyIdentityPage(1)
  const totalPages = Math.max(1, Math.trunc(firstPage.total_pages ?? 1))
  const remainingPages = await Promise.all(
    Array.from({ length: totalPages - 1 }, (_, index) => fetchKeyIdentityPage(index + 2)),
  )
  return [firstPage, ...remainingPages].flatMap((page) => page.identities ?? [])
}

function KeysWorkspace() {
  const [keys, setKeys] = useState<KeyIdentity[]>([])
  const [query, setQuery] = useState('')
  const [editingID, setEditingID] = useState<number | null>(null)
  const [draftAlias, setDraftAlias] = useState('')
  const [savingID, setSavingID] = useState<number | null>(null)

  useEffect(() => {
    let active = true
    fetchAllKeyIdentities()
      .then((identities) => {
        if (active) {
          setKeys(identities)
        }
      })
      .catch(() => {
        if (active) {
          setKeys([])
        }
      })
    return () => {
      active = false
    }
  }, [])

  const filteredKeys = useMemo(() => {
    const normalized = query.trim().toLowerCase()
    if (!normalized) {
      return keys
    }
    return keys.filter((key) => {
      const values = [key.alias, key.displayName, key.name, key.identity, key.provider, key.type, key.auth_type_name]
      return values.some((value) => value.toLowerCase().includes(normalized))
    })
  }, [keys, query])

  function startEditing(key: KeyIdentity) {
    setEditingID(key.id)
    setDraftAlias(key.alias)
  }

  async function saveAlias(key: KeyIdentity) {
    setSavingID(key.id)
    try {
      const response = await fetch(apiPath(`/usage/identities/${key.id}/alias`), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ alias: draftAlias }),
      })
      if (!response.ok) {
        return
      }
      const payload = (await response.json()) as { alias: string }
      setKeys((current) => current.map((item) => (item.id === key.id ? { ...item, alias: payload.alias } : item)))
      setEditingID(null)
    } finally {
      setSavingID(null)
    }
  }

  async function clearAlias(key: KeyIdentity) {
    setSavingID(key.id)
    try {
      const response = await fetch(apiPath(`/usage/identities/${key.id}/alias`), { method: 'DELETE' })
      if (!response.ok) {
        return
      }
      setKeys((current) => current.map((item) => (item.id === key.id ? { ...item, alias: '' } : item)))
      if (editingID === key.id) {
        setEditingID(null)
      }
    } finally {
      setSavingID(null)
    }
  }

  return (
    <>
      <header className="flex items-start justify-between gap-4 max-md:grid">
        <div>
          <p className="text-xs font-semibold uppercase text-muted-foreground">Keys / Key 管理</p>
          <h2 id="keys-title" className="mt-1 text-2xl font-semibold tracking-normal">
            Key Management
          </h2>
        </div>
        <Badge variant="green">{keys.length} active</Badge>
      </header>

      <Card className="mt-5">
        <CardHeader className="flex flex-row items-start justify-between gap-4">
          <div>
            <CardTitle>Key Alias Directory</CardTitle>
            <CardDescription>Alias labels are stored locally and do not change CPA source data.</CardDescription>
          </div>
          <div className="flex min-w-[260px] items-center gap-2 rounded-md border border-border bg-background px-3 py-2 max-md:min-w-0">
            <Search className="size-4 text-muted-foreground" aria-hidden="true" />
            <input
              className="min-w-0 flex-1 bg-transparent text-sm outline-none placeholder:text-muted-foreground"
              id="key-search"
              name="key-search"
              onChange={(event) => setQuery(event.target.value)}
              placeholder="Search alias or key"
              value={query}
            />
          </div>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2">
            {filteredKeys.map((key) => {
              const label = keyLabel(key)
              const editing = editingID === key.id
              return (
                <div className="grid grid-cols-[minmax(0,1fr)_120px_130px_112px] items-center gap-3 rounded-md border border-border p-3 max-lg:grid-cols-1" key={key.id}>
                  <div className="min-w-0">
                    {editing ? (
                      <input
                        aria-label={`Alias for ${label}`}
                        className="h-9 w-full rounded-md border border-border bg-background px-3 text-sm font-semibold outline-none"
                        maxLength={80}
                        onChange={(event) => setDraftAlias(event.target.value)}
                        value={draftAlias}
                      />
                    ) : (
                      <p className="truncate text-sm font-semibold">{label}</p>
                    )}
                    <p className="mt-1 truncate text-xs text-muted-foreground">{key.identity}</p>
                    <div className="mt-2 flex flex-wrap gap-2">
                      <Badge variant="outline">{key.provider}</Badge>
                      <Badge variant="outline">{key.type}</Badge>
                      <Badge variant="outline">{key.auth_type_name}</Badge>
                    </div>
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase text-muted-foreground">Last used</p>
                    <p className="mt-1 text-sm font-semibold">{formatLastUsed(key.last_used_at)}</p>
                  </div>
                  <div>
                    <p className="text-xs font-semibold uppercase text-muted-foreground">Usage</p>
                    <p className="mt-1 text-sm font-semibold">{formatCompact(key.total_tokens, 2)} tokens</p>
                    <p className="text-xs text-muted-foreground">{key.cost_available ? formatCost(key.total_cost) : 'Cost unavailable'}</p>
                  </div>
                  <div className="flex justify-end gap-2 max-lg:justify-start">
                    {editing ? (
                      <>
                        <Button aria-label={`Save alias for ${label}`} disabled={savingID === key.id} onClick={() => void saveAlias(key)} size="icon" variant="outline">
                          <Check className="size-4" aria-hidden="true" />
                        </Button>
                        <Button aria-label={`Cancel alias edit for ${label}`} onClick={() => setEditingID(null)} size="icon" variant="outline">
                          <X className="size-4" aria-hidden="true" />
                        </Button>
                      </>
                    ) : (
                      <>
                        <Button aria-label={`Edit alias for ${label}`} onClick={() => startEditing(key)} size="icon" variant="outline">
                          <Pencil className="size-4" aria-hidden="true" />
                        </Button>
                        <Button aria-label={`Clear alias for ${label}`} disabled={!key.alias || savingID === key.id} onClick={() => void clearAlias(key)} size="icon" variant="outline">
                          <Trash2 className="size-4" aria-hidden="true" />
                        </Button>
                      </>
                    )}
                  </div>
                </div>
              )
            })}
          </div>
        </CardContent>
      </Card>
    </>
  )
}

function EventsWorkspace() {
  const [events, setEvents] = useState<UsageEvent[]>([])

  useEffect(() => {
    let active = true
    fetch(apiPath('/usage/events?range=24h&page_size=20'))
      .then((response) => {
        if (!response.ok) {
          throw new Error('load events failed')
        }
        return response.json() as Promise<UsageEventsPage>
      })
      .then((payload) => {
        if (active) {
          setEvents(payload.events ?? [])
        }
      })
      .catch(() => {
        if (active) {
          setEvents([])
        }
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <>
      <header className="flex items-start justify-between gap-4 max-md:grid">
        <div>
          <p className="text-xs font-semibold uppercase text-muted-foreground">Events / 请求明细</p>
          <h2 id="events-title" className="mt-1 text-2xl font-semibold tracking-normal">
            Request Events
          </h2>
        </div>
        <Badge variant="outline">Traceability</Badge>
      </header>

      <Card className="mt-5">
        <CardHeader>
          <CardTitle>Request Event Inspection</CardTitle>
          <CardDescription>Resolved source display remains primary, with auth index kept as secondary traceability.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2">
            {events.length === 0 ? (
              <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">No request events in this range</div>
            ) : events.map((event) => (
              <div className="grid grid-cols-[minmax(0,1.2fr)_minmax(0,1fr)_100px_120px] items-center gap-3 rounded-md border border-border p-3 max-lg:grid-cols-1" key={`${event.id ?? event.timestamp}-${event.auth_index ?? event.source}`}>
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold">{event.source || 'Unknown source'}</p>
                  <p className="truncate text-xs text-muted-foreground">{event.auth_index || 'No auth index'}</p>
                </div>
                <div className="min-w-0">
                  <p className="truncate text-sm font-semibold">{event.model || 'Unknown model'}</p>
                  <p className="text-xs text-muted-foreground">{formatLastUsed(event.timestamp)}</p>
                </div>
                <Badge variant={event.failed ? 'amber' : 'green'}>{event.failed ? 'Failed' : 'Success'}</Badge>
                <div className="text-right max-lg:text-left">
                  <p className="text-sm font-semibold">{formatCompact(event.tokens?.total_tokens ?? 0, 2)} tokens</p>
                  <p className="text-xs text-muted-foreground">{event.latency_ms > 0 ? `${event.latency_ms}ms` : 'No latency'}</p>
                </div>
              </div>
            ))}
          </div>
        </CardContent>
      </Card>
    </>
  )
}

function PricingWorkspace() {
  const [pricing, setPricing] = useState<PricingEntry[]>([])
  const [usedModels, setUsedModels] = useState<string[]>([])
  const [pricingDrafts, setPricingDrafts] = useState<Record<string, PricingDraft>>({})
  const [savingModel, setSavingModel] = useState<string | null>(null)
  const [pricingStatus, setPricingStatus] = useState<string | null>(null)

  useEffect(() => {
    let active = true
    Promise.all([
      fetch(apiPath('/pricing')).then((response) => response.ok ? response.json() as Promise<PricingPayload> : { pricing: [] }),
      fetch(apiPath('/models/used')).then((response) => response.ok ? response.json() as Promise<UsedModelsPayload> : { models: [] }),
    ])
      .then(([pricingPayload, usedModelsPayload]) => {
        if (active) {
          setPricing(pricingPayload.pricing ?? [])
          setUsedModels(usedModelsPayload.models ?? [])
        }
      })
      .catch(() => {
        if (active) {
          setPricing([])
          setUsedModels([])
        }
      })
    return () => {
      active = false
    }
  }, [])

  const pricingByModel = new Map(pricing.map((entry) => [entry.model, entry]))
  const modelRows = Array.from(new Set([...usedModels, ...pricing.map((entry) => entry.model)])).sort()
  const missingModels = modelRows.filter((model) => !pricingByModel.has(model))

  function pricingDraftFromEntry(entry?: PricingEntry): PricingDraft {
    return {
      prompt: String(entry?.prompt_price_per_1m ?? 0),
      completion: String(entry?.completion_price_per_1m ?? 0),
      cache: String(entry?.cache_price_per_1m ?? 0),
    }
  }

  function updatePricingDraft(model: string, field: keyof PricingDraft, value: string) {
    setPricingDrafts((current) => ({
      ...current,
      [model]: {
        ...pricingDraftFromEntry(pricingByModel.get(model)),
        ...current[model],
        [field]: value,
      },
    }))
  }

  async function savePricing(model: string) {
    const draft = pricingDrafts[model] ?? pricingDraftFromEntry(pricingByModel.get(model))
    const promptPrice = Number(draft.prompt)
    const completionPrice = Number(draft.completion)
    const cachePrice = Number(draft.cache)
    if (![promptPrice, completionPrice, cachePrice].every((value) => Number.isFinite(value) && value >= 0)) {
      setPricingStatus('Prices must be non-negative numbers')
      return
    }

    setSavingModel(model)
    setPricingStatus(null)
    try {
      const response = await fetch(apiPath(`/pricing/${encodeURIComponent(model)}`), {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model,
          prompt_price_per_1m: promptPrice,
          completion_price_per_1m: completionPrice,
          cache_price_per_1m: cachePrice,
        }),
      })
      if (!response.ok) {
        throw new Error('save pricing failed')
      }
      const saved = await response.json() as PricingEntry
      setPricing((current) => {
        const withoutSaved = current.filter((entry) => entry.model !== saved.model)
        return [...withoutSaved, saved].sort((a, b) => a.model.localeCompare(b.model))
      })
      setPricingDrafts((current) => ({ ...current, [saved.model]: pricingDraftFromEntry(saved) }))
      setUsedModels((current) => current.includes(saved.model) ? current : [...current, saved.model])
      setPricingStatus(`${saved.model} pricing saved`)
    } catch {
      setPricingStatus('Pricing save failed')
    } finally {
      setSavingModel(null)
    }
  }

  return (
    <>
      <header className="flex items-start justify-between gap-4 max-md:grid">
        <div>
          <p className="text-xs font-semibold uppercase text-muted-foreground">Pricing / 计价配置</p>
          <h2 id="pricing-title" className="mt-1 text-2xl font-semibold tracking-normal">
            Model Unit Pricing
          </h2>
        </div>
        <Badge variant={missingModels.length > 0 ? 'amber' : 'green'}>{missingModels.length} missing</Badge>
      </header>

      <Card className="mt-5">
        <CardHeader>
          <CardTitle>Configured Cost Rates</CardTitle>
          <CardDescription>These model unit prices are used by analytics Cost calculations.</CardDescription>
        </CardHeader>
        <CardContent>
          <div className="grid gap-2">
            {modelRows.length === 0 ? (
              <div className="rounded-md border border-dashed border-border p-4 text-sm text-muted-foreground">No models available for pricing</div>
            ) : modelRows.map((model) => {
              const draft = pricingDrafts[model] ?? pricingDraftFromEntry(pricingByModel.get(model))
              const configured = pricingByModel.has(model)
              return (
                <div className="grid grid-cols-[minmax(0,1fr)_140px_140px_140px_96px] items-end gap-3 rounded-md border border-border p-3 max-xl:grid-cols-1" key={model}>
                  <div className="min-w-0">
                    <p className="truncate text-sm font-semibold">{model}</p>
                    <Badge variant={configured ? 'green' : 'amber'}>{configured ? 'Configured' : 'Missing price'}</Badge>
                  </div>
                  <label className="grid gap-1 text-xs font-medium text-muted-foreground">
                    Prompt
                    <input
                      aria-label={`${model} prompt price per 1M`}
                      className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
                      min="0"
                      onChange={(event) => updatePricingDraft(model, 'prompt', event.target.value)}
                      step="0.000001"
                      type="number"
                      value={draft.prompt}
                    />
                  </label>
                  <label className="grid gap-1 text-xs font-medium text-muted-foreground">
                    Completion
                    <input
                      aria-label={`${model} completion price per 1M`}
                      className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
                      min="0"
                      onChange={(event) => updatePricingDraft(model, 'completion', event.target.value)}
                      step="0.000001"
                      type="number"
                      value={draft.completion}
                    />
                  </label>
                  <label className="grid gap-1 text-xs font-medium text-muted-foreground">
                    Cache
                    <input
                      aria-label={`${model} cache price per 1M`}
                      className="h-9 rounded-md border border-input bg-background px-3 text-sm text-foreground"
                      min="0"
                      onChange={(event) => updatePricingDraft(model, 'cache', event.target.value)}
                      step="0.000001"
                      type="number"
                      value={draft.cache}
                    />
                  </label>
                  <Button aria-label={`Save pricing for ${model}`} disabled={savingModel === model} onClick={() => void savePricing(model)} size="sm" variant="outline">
                    Save
                  </Button>
                </div>
              )
            })}
          </div>
          {pricingStatus ? <p className="mt-3 text-sm text-muted-foreground">{pricingStatus}</p> : null}
        </CardContent>
      </Card>
    </>
  )
}

function SettingsWorkspace() {
  const [status, setStatus] = useState<StatusPayload>({})
  const [session, setSession] = useState<AuthSessionPayload>({})

  useEffect(() => {
    let active = true
    Promise.all([
      fetch(apiPath('/status')).then((response) => response.ok ? response.json() as Promise<StatusPayload> : {}),
      fetch(apiPath('/auth/session')).then((response) => response.ok ? response.json() as Promise<AuthSessionPayload> : {}),
    ])
      .then(([statusPayload, sessionPayload]) => {
        if (active) {
          setStatus(statusPayload)
          setSession(sessionPayload)
        }
      })
      .catch(() => {
        if (active) {
          setStatus({})
          setSession({})
        }
      })
    return () => {
      active = false
    }
  }, [])

  return (
    <>
      <header className="flex items-start justify-between gap-4 max-md:grid">
        <div>
          <p className="text-xs font-semibold uppercase text-muted-foreground">Settings / 系统设置</p>
          <h2 id="settings-title" className="mt-1 text-2xl font-semibold tracking-normal">
            Operational Settings
          </h2>
        </div>
        <Badge variant={status.sync_running ? 'amber' : 'outline'}>{status.sync_running ? 'Syncing' : 'Idle'}</Badge>
      </header>

      <div className="mt-5 grid gap-4 md:grid-cols-3">
        <Card>
          <CardHeader>
            <CardTitle>Sync Status</CardTitle>
            <CardDescription>Current ingestion and manual sync state.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="flex items-center gap-2">
              <RefreshCw className="size-4 text-muted-foreground" aria-hidden="true" />
              <p className="text-sm font-semibold">{status.last_status || 'No sync status'}</p>
            </div>
            <p className="mt-2 text-xs text-muted-foreground">{status.last_run_at ? formatLastUsed(status.last_run_at) : 'Never run'}</p>
            {status.last_error ? <p className="mt-2 text-xs text-red-600">{status.last_error}</p> : null}
            {status.last_warning ? <p className="mt-2 text-xs text-amber-700">{status.last_warning}</p> : null}
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Update Check</CardTitle>
            <CardDescription>Version and update-check capability inherited from the backend.</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm font-semibold">{status.version || 'dev'}</p>
            <p className="mt-2 text-xs text-muted-foreground">{status.updateCheckEnabled ? 'Update check enabled' : 'Update check disabled'}</p>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle>Auth Session</CardTitle>
            <CardDescription>Current UI authentication state.</CardDescription>
          </CardHeader>
          <CardContent>
            <p className="text-sm font-semibold">{session.authenticated ? 'Authenticated' : 'Not authenticated'}</p>
            <p className="mt-2 text-xs text-muted-foreground">{status.timezone || 'Local timezone'}</p>
          </CardContent>
        </Card>
      </div>
    </>
  )
}

function keyLabel(key: KeyIdentity) {
  return key.alias.trim() || key.displayName || key.name || key.identity
}

function routeTitleID(route: AppRoute) {
  switch (route) {
    case '/keys':
      return 'keys-title'
    case '/events':
      return 'events-title'
    case '/pricing':
      return 'pricing-title'
    case '/settings':
      return 'settings-title'
    default:
      return 'analytics-title'
  }
}

function formatCost(value: number) {
  return `$${value.toLocaleString('en', { maximumFractionDigits: 2, minimumFractionDigits: 2 })}`
}

function formatBreakdownCost(row: Pick<TrendPoint, 'cost' | 'costAvailable' | 'costStatus'>) {
  if (row.costAvailable === false) {
    return row.costStatus === 'partial' ? 'Cost partial' : 'Cost unavailable'
  }
  return formatCost(row.cost)
}

function formatInsightMetric(insight: AnalyticsInsightPayload) {
  switch (insight.metric_label) {
    case 'Cost':
      return formatCost(insight.metric_value)
    case 'Tokens':
      return `${formatCompact(insight.metric_value, 2)} tokens`
    case 'Failures':
      return `${insight.count.toLocaleString('en')} failures`
    case 'Share':
      return `${insight.metric_value.toFixed(1)}% token share`
    case 'Cost status':
      return `Cost ${insight.cost_status}`
    default:
      return `${insight.metric_label}: ${formatCompact(insight.metric_value, 2)}`
  }
}

function formatLastUsed(value: string | null) {
  if (!value) {
    return 'Never'
  }
  return new Intl.DateTimeFormat('en', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  }).format(new Date(value))
}

function MetricCard({ label, value, caption, tone }: { label: string; value: string; caption: string; tone: 'green' | 'blue' | 'violet' | 'amber' }) {
  const toneClass = {
    green: 'text-emerald-700 bg-emerald-50 border-emerald-200',
    blue: 'text-blue-700 bg-blue-50 border-blue-200',
    violet: 'text-violet-700 bg-violet-50 border-violet-200',
    amber: 'text-amber-700 bg-amber-50 border-amber-200',
  }[tone]

  return (
    <Card>
      <CardContent className="p-4">
        <div className={`mb-3 inline-flex rounded-full border px-2 py-1 text-xs font-semibold ${toneClass}`}>{label}</div>
        <p className="text-2xl font-semibold tracking-normal">{value}</p>
        <p className="mt-1 text-xs text-muted-foreground">{caption}</p>
      </CardContent>
    </Card>
  )
}

export default App
