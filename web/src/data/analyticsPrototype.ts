export type TrendPoint = {
  label: string
  cost: number
  tokens: number
  requests: number
  failures: number
  costAvailable?: boolean
  costStatus?: 'available' | 'partial' | 'unavailable'
}

export type AliasRow = {
  alias: string
  key: string
  provider: string
  cost: number
  tokens: number
  requests: number
  successRate: number
  trend: number[]
  costAvailable?: boolean
  costStatus?: 'available' | 'partial' | 'unavailable'
  failures?: number
  isDeleted?: boolean
}

export type ModelRow = {
  model: string
  provider: string
  cost: number
  tokens: number
  inputTokens: number
  cachedTokens: number
  cacheReadShare: number
  cacheReadShareState: 'available' | 'no_cache_data' | 'no_prompt_input'
  estimatedCacheSavings?: number
  requests: number
  successRate: number
  averageLatencyMS: number
  costAvailable?: boolean
  costStatus?: 'available' | 'partial' | 'unavailable'
  color: string
}

export const trendData: TrendPoint[] = [
  { label: '05-06', cost: 92, tokens: 1180000, requests: 1240, failures: 8 },
  { label: '05-07', cost: 118, tokens: 1420000, requests: 1518, failures: 12 },
  { label: '05-08', cost: 104, tokens: 1330000, requests: 1392, failures: 7 },
  { label: '05-09', cost: 156, tokens: 1890000, requests: 1774, failures: 24 },
  { label: '05-10', cost: 141, tokens: 1710000, requests: 1690, failures: 10 },
  { label: '05-11', cost: 184, tokens: 2230000, requests: 2108, failures: 28 },
  { label: '05-12', cost: 211, tokens: 2510000, requests: 2364, failures: 18 },
]

export const aliasRows: AliasRow[] = [
  {
    alias: 'Agent Research',
    key: 'sk-cpa...7A91',
    provider: 'codex',
    cost: 388.42,
    tokens: 4920000,
    requests: 3814,
    successRate: 99.1,
    trend: [24, 28, 26, 33, 36, 39, 44],
  },
  {
    alias: '产品分析组',
    key: 'sk-cpa...29CD',
    provider: 'claude',
    cost: 236.18,
    tokens: 2780000,
    requests: 1906,
    successRate: 97.8,
    trend: [18, 16, 19, 20, 24, 22, 25],
  },
  {
    alias: 'Ops Automation',
    key: 'sk-cpa...B441',
    provider: 'gemini',
    cost: 141.76,
    tokens: 3440000,
    requests: 2610,
    successRate: 98.6,
    trend: [10, 13, 12, 18, 16, 21, 19],
  },
  {
    alias: '未命名 Key',
    key: 'sk-cpa...0F18',
    provider: 'kimi',
    cost: 72.09,
    tokens: 860000,
    requests: 698,
    successRate: 95.9,
    trend: [8, 9, 7, 10, 9, 12, 11],
  },
]

export const modelRows: ModelRow[] = [
  { model: 'gpt-5.5', provider: 'openai', cost: 296.3, tokens: 3210000, inputTokens: 2400000, cachedTokens: 480000, cacheReadShare: 20, cacheReadShareState: 'available', estimatedCacheSavings: 1.2, requests: 1400, successRate: 99.2, averageLatencyMS: 380, color: '#2563eb' },
  { model: 'claude-sonnet-4.5', provider: 'anthropic', cost: 221.8, tokens: 2420000, inputTokens: 1800000, cachedTokens: 0, cacheReadShare: 0, cacheReadShareState: 'no_cache_data', requests: 1180, successRate: 98.6, averageLatencyMS: 420, color: '#7c3aed' },
  { model: 'gemini-3-pro', provider: 'gemini', cost: 144.6, tokens: 3020000, inputTokens: 0, cachedTokens: 0, cacheReadShare: 0, cacheReadShareState: 'no_prompt_input', requests: 980, successRate: 97.9, averageLatencyMS: 360, color: '#059669' },
  { model: 'kimi-k2', provider: 'kimi', cost: 86.1, tokens: 1340000, inputTokens: 1100000, cachedTokens: 110000, cacheReadShare: 10, cacheReadShareState: 'available', requests: 510, successRate: 96.4, averageLatencyMS: 520, color: '#d97706' },
]

export const healthBlocks = trendData.map((point) => ({
  label: point.label,
  success: point.requests - point.failures,
  failure: point.failures,
  rate: Number((((point.requests - point.failures) / point.requests) * 100).toFixed(1)),
}))

export const insightChips = [
  { label: 'Top Cost Key', value: 'Agent Research', tone: 'green' },
  { label: 'Token Spike', value: '+31% on 05-12', tone: 'blue' },
  { label: 'Pricing Missing', value: '2 models partial', tone: 'amber' },
  { label: 'Failure Cluster', value: 'Claude 05-11', tone: 'violet' },
] as const
