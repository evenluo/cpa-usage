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
import { aliasRows, healthBlocks, insightChips, modelRows, trendData } from '@/data/analyticsPrototype'

import './index.css'

const navItems = [
  { label: '数据分析 Analytics', href: '/', icon: BarChart3 },
  { label: 'Key 管理 Keys', href: '/keys', icon: KeyRound },
  { label: '计价配置 Pricing', href: '/pricing', icon: WalletCards },
  { label: '请求明细 Events', href: '/events', icon: TableProperties },
  { label: '系统设置 Settings', href: '/settings', icon: Settings },
]

function formatCompact(value: number, maximumFractionDigits = 1) {
  return Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits }).format(value)
}

function formatCurrency(value: number) {
  return `$${value.toLocaleString('en', { maximumFractionDigits: 0 })}`
}

const totals = trendData.reduce(
  (result, point) => ({
    cost: result.cost + point.cost,
    tokens: result.tokens + point.tokens,
    requests: result.requests + point.requests,
    failures: result.failures + point.failures,
  }),
  { cost: 0, tokens: 0, requests: 0, failures: 0 },
)

const kpis = {
  cost: formatCurrency(totals.cost),
  tokens: formatCompact(totals.tokens, 2),
  requests: totals.requests.toLocaleString('en'),
  successRate: `${(((totals.requests - totals.failures) / totals.requests) * 100).toFixed(1)}%`,
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

function currentRoute() {
  const base = appBasePath()
  const pathname = window.location.pathname
  const withoutBase = base && pathname.startsWith(base) ? pathname.slice(base.length) || '/' : pathname
  return withoutBase === '/keys' ? '/keys' : '/'
}

function apiPath(path: string) {
  return withBasePath(`/api/v1${path}`)
}

function App() {
  const route = currentRoute()

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

        <section className="min-w-0 px-6 py-5 max-md:px-4" aria-labelledby={route === '/keys' ? 'keys-title' : 'analytics-title'}>
          {route === '/keys' ? (
            <KeysWorkspace />
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
                <MetricCard label="Total Cost" value={kpis.cost} caption="Partial: 2 unpriced models" tone="green" />
                <MetricCard label="Total Tokens" value={kpis.tokens} caption="Cost peer measure" tone="blue" />
                <MetricCard label="Requests" value={kpis.requests} caption="+14.8% vs previous" tone="violet" />
                <MetricCard label="Success Rate" value={kpis.successRate} caption={`${totals.failures} failures in range`} tone="amber" />
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
                      <MetricTrendChart data={trendData} />
                    </div>
                    <div className="h-[270px] min-w-0 rounded-lg border border-border bg-muted/40 p-3">
                      <TokenCostCompareChart data={trendData} />
                    </div>
                  </div>
                </CardContent>
              </Card>

              <div className="grid gap-3 md:grid-cols-4">
                {insightChips.map((chip) => (
                  <Card className="shadow-none" key={chip.label}>
                    <CardContent className="p-4">
                      <Badge variant={chip.tone}>{chip.label}</Badge>
                      <p className="mt-3 text-sm font-semibold">{chip.value}</p>
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
                  <Button variant="subtle" size="sm">Key Alias</Button>
                  <Button variant="outline" size="sm">Model</Button>
                  <Button variant="outline" size="sm">Time</Button>
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

          <section className="mt-4 grid gap-4 xl:grid-cols-[minmax(0,1.1fr)_minmax(320px,0.9fr)]">
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
                    <AliasRankingChart rows={aliasRows} />
                  </div>
                  <div className="grid gap-2">
                    {aliasRows.map((row) => (
                      <div className="grid grid-cols-[minmax(0,1fr)_86px_96px] items-center gap-3 rounded-md border border-border p-3" key={row.key}>
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{row.alias}</p>
                          <p className="truncate text-xs text-muted-foreground">
                            {row.key} · {row.provider}
                          </p>
                        </div>
                        <div className="h-9">
                          <Sparkline values={row.trend} />
                        </div>
                        <div className="text-right">
                          <p className="text-sm font-semibold">${row.cost.toFixed(0)}</p>
                          <p className="text-xs text-muted-foreground">{formatCompact(row.tokens, 2)} tokens</p>
                          <p className="text-xs text-muted-foreground">{row.successRate}% success</p>
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>

            <Card>
              <CardHeader>
                <CardTitle>Model Distribution</CardTitle>
                <CardDescription>Model impact is shown by Cost with token volume preserved beside it.</CardDescription>
              </CardHeader>
              <CardContent>
                <div className="grid gap-4 md:grid-cols-[180px_minmax(0,1fr)] xl:grid-cols-1 2xl:grid-cols-[180px_minmax(0,1fr)]">
                  <div className="h-[190px] min-w-0">
                    <ModelDistributionChart rows={modelRows} />
                  </div>
                  <div className="grid gap-2">
                    {modelRows.map((row) => (
                      <div className="grid grid-cols-[10px_minmax(0,1fr)_auto] items-center gap-3" key={row.model}>
                        <span className="h-10 rounded-full" style={{ backgroundColor: row.color }} />
                        <div className="min-w-0">
                          <p className="truncate text-sm font-semibold">{row.model}</p>
                          <p className="text-xs text-muted-foreground">{Intl.NumberFormat('en', { notation: 'compact' }).format(row.tokens)} tokens</p>
                        </div>
                        <p className="text-sm font-semibold">${row.cost.toFixed(0)}</p>
                      </div>
                    ))}
                  </div>
                </div>
              </CardContent>
            </Card>
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

function keyLabel(key: KeyIdentity) {
  return key.alias.trim() || key.displayName || key.name || key.identity
}

function formatCost(value: number) {
  return `$${value.toLocaleString('en', { maximumFractionDigits: 2, minimumFractionDigits: 2 })}`
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
