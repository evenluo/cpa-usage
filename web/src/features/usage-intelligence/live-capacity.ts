import type { KeyIdentity, QuotaCacheResponse, QuotaRow } from "@/types/api"
import type { LiveCapacityTaskState } from "@/hooks/useQuota"

export type LiveCapacityStatus = "cached" | "no_cache" | "refreshing" | "failed" | "unsupported"
export type ProviderKind = "antigravity" | "claude" | "codex" | "gemini-cli" | "kimi" | "unsupported"

export interface LiveCapacityRow {
  authIndex: string
  provider: string
  providerKind: ProviderKind
  providerLabel: string
  type: string
  name: string
  alias: string
  displayName: string
  status: LiveCapacityStatus
  statusLabel: string
  error?: string
  fiveHour?: LiveCapacityMetric
  weekly?: LiveCapacityMetric
  additionalMetrics: LiveCapacityMetric[]
  resetLabel: string
  planType: string
  priorityLabel?: string
  isPriorityAccount: boolean
  isConstrained: boolean
}

export interface LiveCapacityMetric {
  label: string
  valueLabel: string
  resetLabel: string
  progress: number | null
  tone: "green" | "amber" | "red" | "muted"
}

const PROVIDER_KIND_LABELS: Record<ProviderKind, string> = {
  antigravity: "Antigravity",
  claude: "Claude",
  codex: "Codex",
  "gemini-cli": "Gemini CLI",
  kimi: "Kimi",
  unsupported: "Unsupported",
}

const PROVIDER_KIND_ALIASES: Record<string, ProviderKind> = {
  antigravity: "antigravity",
  claude: "claude",
  codex: "codex",
  "gemini-cli": "gemini-cli",
  kimi: "kimi",
}

export function buildLiveCapacityRows(input: {
  identities: KeyIdentity[]
  cachedQuota?: QuotaCacheResponse
  taskStates?: Record<string, LiveCapacityTaskState>
}): LiveCapacityRow[] {
  const cachedByAuthIndex = new Map((input.cachedQuota?.items ?? []).map((item) => [item.id, item]))
  const taskStates = input.taskStates ?? {}

  return input.identities
    .map((identity) => {
      const taskState = taskStates[identity.identity]
      const providerKind = providerKindFromIdentity(identity)
      const supported = providerKind !== "unsupported"
      const activeQuota = taskState?.status === "completed" ? taskState.quota : cachedByAuthIndex.get(identity.identity)
      const quotaRows = activeQuota?.quota ?? []
      const fiveHour = findQuotaWindow(quotaRows, "5h")
      const weekly = findQuotaWindow(quotaRows, "weekly")
      const additionalMetrics = quotaRows
        .filter((row) => row !== fiveHour && row !== weekly)
        .slice(0, 3)
        .map(metricFromQuotaRow)
      const isConstrained = quotaRows.some(isConstrainedQuotaRow)
      const resolvedPlanType = planType(quotaRows)
      const priorityLabel = priorityLabelFor(providerKind, resolvedPlanType)

      let status: LiveCapacityStatus = activeQuota ? "cached" : "no_cache"
      let statusLabel = activeQuota ? "cached" : "No cached probe"
      let error: string | undefined
      if (!supported) {
        status = "unsupported"
        statusLabel = "Unsupported"
      } else if (taskState?.status === "queued" || taskState?.status === "running") {
        status = "refreshing"
        statusLabel = taskState.status === "queued" ? "Queued" : "Refreshing"
      } else if (taskState?.status === "failed") {
        status = "failed"
        statusLabel = rejectionLabel(taskState.error)
        error = taskState.error
      }

      return {
        authIndex: identity.identity,
        provider: identity.provider,
        providerKind,
        providerLabel: providerLabelFor(providerKind, identity),
        type: identity.type,
        name: identity.name,
        alias: identity.alias,
        displayName: identity.displayName,
        status,
        statusLabel,
        error,
        fiveHour: fiveHour ? metricFromQuotaRow(fiveHour) : undefined,
        weekly: weekly ? metricFromQuotaRow(weekly) : undefined,
        additionalMetrics,
        resetLabel: resetLabel(fiveHour, weekly, ...quotaRows),
        planType: resolvedPlanType,
        priorityLabel,
        isPriorityAccount: Boolean(priorityLabel),
        isConstrained,
      }
    })
    .sort(compareLiveCapacityRows)
}

export function isSupportedQuotaIdentity(identity: KeyIdentity): boolean {
  return providerKindFromIdentity(identity) !== "unsupported"
}

export function providerKindFromIdentity(identity: Pick<KeyIdentity, "provider" | "type">): ProviderKind {
  return [identity.provider, identity.type]
    .map(normalizeProviderValue)
    .map((value) => PROVIDER_KIND_ALIASES[value])
    .find((kind): kind is ProviderKind => Boolean(kind)) ?? "unsupported"
}

function normalizeProviderValue(value: string): string {
  return value.trim().toLowerCase()
}

function providerLabelFor(providerKind: ProviderKind, identity: Pick<KeyIdentity, "provider" | "type">): string {
  if (providerKind !== "unsupported") return PROVIDER_KIND_LABELS[providerKind]
  return identity.provider.trim() || identity.type.trim() || PROVIDER_KIND_LABELS.unsupported
}

function priorityLabelFor(providerKind: ProviderKind, planTypeValue: string): string | undefined {
  if (providerKind === "codex" && hasPlanTypeValue(planTypeValue, "pro")) return "Pro"
  if (providerKind === "claude" && hasPlanTypeValue(planTypeValue, "max")) return "Max"
  return undefined
}

function findQuotaWindow(rows: QuotaRow[], kind: "5h" | "weekly"): QuotaRow | undefined {
  return rows.find((row) => {
    const label = (row.label ?? "").toLowerCase()
    const seconds = row.window?.seconds
    if (kind === "5h") return seconds === 18_000 || label === "5h" || label.includes("5h")
    return seconds === 604_800 || label === "weekly" || label.includes("weekly") || label.includes("7d")
  })
}

function metricFromQuotaRow(row: QuotaRow): LiveCapacityMetric {
  const progress = progressFromQuotaRow(row)
  return {
    label: metricLabel(row),
    valueLabel: valueLabel(row),
    resetLabel: resetLabel(row),
    progress,
    tone: toneFromProgress(row, progress),
  }
}

function metricLabel(row: QuotaRow): string {
  return row.label || row.metric || row.scope || row.key || "Capacity"
}

function progressFromQuotaRow(row: QuotaRow): number | null {
  if (typeof row.usedPercent === "number") {
    return clamp(row.usedPercent)
  }
  if (typeof row.remainingFraction === "number") {
    return clamp((1 - row.remainingFraction) * 100)
  }
  if (typeof row.used === "number" && typeof row.limit === "number" && row.limit > 0) {
    return clamp((row.used / row.limit) * 100)
  }
  if (typeof row.remaining === "number" && typeof row.limit === "number" && row.limit > 0) {
    return clamp((1 - row.remaining / row.limit) * 100)
  }
  return null
}

function valueLabel(row: QuotaRow): string {
  if (typeof row.remainingFraction === "number") return `${Math.round(row.remainingFraction * 100)}% left`
  if (typeof row.usedPercent === "number") return `${Math.round(row.usedPercent)}% used`
  if (typeof row.remaining === "number" && typeof row.limit === "number") return `${formatQuotaNumber(row.remaining)} / ${formatQuotaNumber(row.limit)} left`
  if (typeof row.remaining === "number") return `${formatQuotaNumber(row.remaining)} left`
  if (typeof row.used === "number" && typeof row.limit === "number") return `${formatQuotaNumber(row.used)} / ${formatQuotaNumber(row.limit)} used`
  if (typeof row.allowed === "boolean") return row.allowed ? "Allowed" : "Blocked"
  return "Measured"
}

function toneFromProgress(row: QuotaRow, progress: number | null): LiveCapacityMetric["tone"] {
  if (row.limitReached || row.allowed === false) return "red"
  if (isRemainingExhausted(row)) return "red"
  if (progress === null) return "muted"
  if (progress >= 95) return "red"
  if (progress >= 80) return "amber"
  return "green"
}

function isConstrainedQuotaRow(row: QuotaRow | undefined): boolean {
  if (!row) return false
  if (row.limitReached || row.allowed === false) return true
  if (isRemainingExhausted(row)) return true
  const progress = progressFromQuotaRow(row)
  return progress !== null && progress >= 95
}

function isRemainingExhausted(row: QuotaRow): boolean {
  return typeof row.remaining === "number" && row.remaining <= 0
}

function resetLabel(...rows: Array<QuotaRow | undefined>): string {
  const row = rows.find((item) => item?.resetAt || item?.resetAfterSeconds)
  if (!row) return "-"
  if (row.resetAt) return formatResetDate(row.resetAt)
  if (row.resetAfterSeconds) return formatResetDuration(row.resetAfterSeconds)
  return "-"
}

function formatResetDate(value: string): string {
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return value
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date)
}

function formatResetDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.round(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.round(minutes / 60)
  if (hours < 48) return `${hours}h`
  return `${Math.round(hours / 24)}d`
}

function planType(rows: QuotaRow[]): string {
  return rows.find((row) => row.planType)?.planType ?? ""
}

function rejectionLabel(code: string): string {
  switch (code) {
    case "unsupported":
      return "Unsupported"
    case "not_auth_file":
      return "Not auth-file"
    case "not_found":
      return "Not found"
    case "duplicate":
      return "Already refreshing"
    case "invalid":
      return "Invalid"
    default:
      return code || "Failed"
  }
}

function compareLiveCapacityRows(a: LiveCapacityRow, b: LiveCapacityRow): number {
  const priority = rowPriority(a) - rowPriority(b)
  if (priority !== 0) return priority
  return `${accountTitle(a)} ${a.authIndex}`.localeCompare(`${accountTitle(b)} ${b.authIndex}`)
}

function rowPriority(row: LiveCapacityRow): number {
  if (row.status === "unsupported") return 4
  if (row.providerKind === "codex" && row.priorityLabel === "Pro") return 0
  if (row.providerKind === "claude" && row.priorityLabel === "Max") return 1
  if (row.providerKind === "codex" || row.providerKind === "claude") return 2
  return 3
}

function accountTitle(row: LiveCapacityRow): string {
  return row.alias || row.displayName || row.name || row.authIndex
}

function hasPlanTypeValue(planTypeValue: string, keyword: string): boolean {
  return planTypeValue.toLowerCase().includes(keyword)
}

function clamp(value: number): number {
  return Math.max(0, Math.min(100, value))
}

function formatQuotaNumber(value: number): string {
  return Intl.NumberFormat("en", { maximumFractionDigits: value >= 10 ? 0 : 1 }).format(value)
}
