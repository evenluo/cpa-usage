import {
  Area,
  AreaChart,
  Bar,
  BarChart,
  CartesianGrid,
  Cell,
  Line,
  LineChart,
  Pie,
  PieChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'

import { type AliasRow, type ModelRow, type TrendPoint } from '@/data/analyticsPrototype'

const axisStyle = { fontSize: 11, fill: '#71717a' }
const gridStroke = '#e4e4e7'
const defaultInitialDimension = { width: 360, height: 220 }
const compactInitialDimension = { width: 120, height: 36 }

function compactNumber(value: number) {
  return Intl.NumberFormat('en', { notation: 'compact', maximumFractionDigits: 1 }).format(value)
}

function formatCurrency(value: number) {
  return `$${value.toLocaleString('en', { maximumFractionDigits: 2, minimumFractionDigits: 2 })}`
}

function formatTrendCost(point: TrendPoint) {
  if (point.costStatus === 'unavailable' || (point.costAvailable === false && point.costStatus !== 'partial')) {
    return 'Cost unavailable'
  }
  if (point.costStatus === 'partial') {
    return `${formatCurrency(point.cost)} partial`
  }
  return formatCurrency(point.cost)
}

type TrendTooltipPayload = {
  payload?: TrendPoint
}

export function TrendBucketTooltip({
  active,
  label,
  payload,
}: {
  active?: boolean
  label?: string | number
  payload?: TrendTooltipPayload[]
}) {
  const point = payload?.[0]?.payload
  if (!active || !point) {
    return null
  }
  return (
    <div
      aria-label="Trend bucket detail"
      className="rounded-md border border-border bg-background px-3 py-2 text-xs shadow-lg"
      role="tooltip"
    >
      <p className="font-semibold text-foreground">{label ?? point.label}</p>
      <div className="mt-2 grid gap-1 text-muted-foreground">
        <p>Cost <span className="font-semibold text-foreground">{formatTrendCost(point)}</span></p>
        <p>{compactNumber(point.tokens)} tokens</p>
        <p>{point.requests.toLocaleString('en')} requests</p>
        <p>{point.failures.toLocaleString('en')} failures</p>
      </div>
    </div>
  )
}

export function MetricTrendChart({ data }: { data: TrendPoint[] }) {
  const chartData = data.map((point) => ({
    ...point,
    chartCost: point.costStatus === 'unavailable' ? null : point.cost,
  }))
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={defaultInitialDimension}>
      <AreaChart data={chartData} margin={{ left: 0, right: 4, top: 8, bottom: 0 }}>
        <defs>
          <linearGradient id="costFill" x1="0" x2="0" y1="0" y2="1">
            <stop offset="5%" stopColor="#059669" stopOpacity={0.24} />
            <stop offset="95%" stopColor="#059669" stopOpacity={0} />
          </linearGradient>
        </defs>
        <CartesianGrid stroke={gridStroke} strokeDasharray="3 3" vertical={false} />
        <XAxis axisLine={false} dataKey="label" tick={axisStyle} tickLine={false} />
        <YAxis axisLine={false} tick={axisStyle} tickFormatter={formatCurrency} tickLine={false} width={42} />
        <Tooltip
          contentStyle={{ borderColor: '#e4e4e7', borderRadius: 8, boxShadow: '0 8px 24px rgba(24,24,27,0.08)' }}
          formatter={(value) => [`$${Number(value).toFixed(2)}`, 'Cost']}
        />
        <Area dataKey="chartCost" fill="url(#costFill)" stroke="#059669" strokeWidth={2.5} type="monotone" />
      </AreaChart>
    </ResponsiveContainer>
  )
}

export function TokenCostCompareChart({ data }: { data: TrendPoint[] }) {
  const chartData = data.map((point) => ({
    ...point,
    chartCost: point.costStatus === 'unavailable' ? null : point.cost,
  }))
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={defaultInitialDimension}>
      <LineChart data={chartData} margin={{ left: 0, right: 6, top: 8, bottom: 0 }}>
        <CartesianGrid stroke={gridStroke} strokeDasharray="3 3" vertical={false} />
        <XAxis axisLine={false} dataKey="label" tick={axisStyle} tickLine={false} />
        <YAxis
          yAxisId="tokens"
          axisLine={false}
          tick={axisStyle}
          tickFormatter={(value) => compactNumber(Number(value))}
          tickLine={false}
          width={48}
        />
        <YAxis
          yAxisId="cost"
          axisLine={false}
          orientation="right"
          tick={axisStyle}
          tickFormatter={formatCurrency}
          tickLine={false}
          width={42}
        />
        <Tooltip
          content={<TrendBucketTooltip />}
          contentStyle={{ borderColor: '#e4e4e7', borderRadius: 8, boxShadow: '0 8px 24px rgba(24,24,27,0.08)' }}
          formatter={(value, name) => [name === 'tokens' ? compactNumber(Number(value)) : `$${Number(value).toFixed(2)}`, name]}
        />
        <Line yAxisId="tokens" dataKey="tokens" dot={false} stroke="#2563eb" strokeWidth={2.5} type="monotone" />
        <Line yAxisId="cost" dataKey="chartCost" dot={false} stroke="#059669" strokeWidth={2.5} type="monotone" />
      </LineChart>
    </ResponsiveContainer>
  )
}

export function AliasRankingChart({ rows }: { rows: AliasRow[] }) {
  const data = rows
    .filter((row) => row.costAvailable !== false && row.costStatus !== 'unavailable')
    .map((row) => ({ name: row.alias, cost: row.cost }))
  if (data.length === 0) {
    return (
      <div className="grid h-full place-items-center rounded-md border border-dashed border-border text-sm text-muted-foreground">
        No available alias cost data
      </div>
    )
  }
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={defaultInitialDimension}>
      <BarChart data={data} layout="vertical" margin={{ left: 8, right: 12, top: 0, bottom: 0 }}>
        <CartesianGrid horizontal={false} stroke={gridStroke} strokeDasharray="3 3" />
        <XAxis axisLine={false} tick={axisStyle} tickFormatter={formatCurrency} tickLine={false} type="number" />
        <YAxis axisLine={false} dataKey="name" tick={axisStyle} tickLine={false} type="category" width={108} />
        <Tooltip
          contentStyle={{ borderColor: '#e4e4e7', borderRadius: 8, boxShadow: '0 8px 24px rgba(24,24,27,0.08)' }}
          formatter={(value) => [`$${Number(value).toFixed(2)}`, 'Cost']}
        />
        <Bar dataKey="cost" fill="#059669" radius={[0, 6, 6, 0]} />
      </BarChart>
    </ResponsiveContainer>
  )
}

export function ModelDistributionChart({ measure = 'cost', rows }: { measure?: 'cost' | 'tokens'; rows: ModelRow[] }) {
  const data = rows.map((row) => ({
    ...row,
    measureValue: measure === 'cost' ? row.cost : row.tokens,
  }))
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={defaultInitialDimension}>
      <PieChart>
        <Pie data={data} dataKey="measureValue" innerRadius="58%" outerRadius="82%" paddingAngle={3}>
          {data.map((row) => (
            <Cell fill={row.color} key={row.model} />
          ))}
        </Pie>
        <Tooltip
          contentStyle={{ borderColor: '#e4e4e7', borderRadius: 8, boxShadow: '0 8px 24px rgba(24,24,27,0.08)' }}
          formatter={(value) => [measure === 'cost' ? `$${Number(value).toFixed(2)}` : compactNumber(Number(value)), measure === 'cost' ? 'Cost' : 'Tokens']}
        />
      </PieChart>
    </ResponsiveContainer>
  )
}

export function HealthTimeline({ data }: { data: Array<{ label: string; success: number; failure: number; rate: number }> }) {
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={defaultInitialDimension}>
      <BarChart data={data} margin={{ left: 0, right: 4, top: 4, bottom: 0 }}>
        <CartesianGrid stroke={gridStroke} strokeDasharray="3 3" vertical={false} />
        <XAxis axisLine={false} dataKey="label" tick={axisStyle} tickLine={false} />
        <YAxis axisLine={false} tick={axisStyle} tickFormatter={(value) => compactNumber(Number(value))} tickLine={false} width={44} />
        <Tooltip
          contentStyle={{ borderColor: '#e4e4e7', borderRadius: 8, boxShadow: '0 8px 24px rgba(24,24,27,0.08)' }}
          formatter={(value, name) => [compactNumber(Number(value)), name]}
        />
        <Bar dataKey="success" fill="#d4d4d8" stackId="requests" />
        <Bar dataKey="failure" fill="#f59e0b" radius={[4, 4, 0, 0]} stackId="requests" />
      </BarChart>
    </ResponsiveContainer>
  )
}

export function Sparkline({ values }: { values: Array<number | null> }) {
  const data = values.map((value, index) => ({ index, value }))
  return (
    <ResponsiveContainer width="100%" height="100%" minWidth={0} initialDimension={compactInitialDimension}>
      <LineChart data={data} margin={{ left: 0, right: 0, top: 4, bottom: 4 }}>
        <Line dataKey="value" dot={false} stroke="#059669" strokeWidth={2} type="monotone" />
      </LineChart>
    </ResponsiveContainer>
  )
}
