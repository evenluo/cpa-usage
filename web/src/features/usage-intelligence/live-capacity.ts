import type { KeyIdentity, PassiveModelQuotaObservation, PassiveQuotaObservation, QuotaCacheResponse, QuotaRow, QuotaWindow } from "@/types/api"
import type { LiveCapacityTaskState } from "@/hooks/useQuota"

export type LiveCapacityStatus = "cached" | "no_cache" | "refreshing" | "failed" | "unsupported" | "disabled"
export type ProviderKind = "antigravity" | "claude" | "codex" | "gemini-cli" | "kimi" | "unsupported"
export type LiveCapacityPlanTone = "priority" | "ordinary" | "none"
export type LiveCapacityAccountStateTone = "green" | "amber" | "red" | "muted"

export interface LiveCapacityAccountState {
  kind: NonNullable<KeyIdentity["status"]> | "not_reported"
  label: string
  explanation: string
  tone: LiveCapacityAccountStateTone
}

export interface LiveCapacityRow {
  id: number
  authIndex: string
  provider: string
  providerKind: ProviderKind
  providerLabel: string
  type: string
  name: string
  alias: string
  displayName: string
  disabled: boolean
  unavailable: boolean | null
  accountState: LiveCapacityAccountState
  status: LiveCapacityStatus
  /** Humanized refresh-failure label for the attention tooltip; only set for failed rows. */
  errorLabel?: string
  error?: string
  fiveHour?: LiveCapacityMetric
  weekly?: LiveCapacityMetric
  additionalMetrics: LiveCapacityMetric[]
  planType: string
  planLabel?: string
  planTone: LiveCapacityPlanTone
  priorityLabel?: string
  isPriorityAccount: boolean
  isConstrained: boolean
  /** True while cached quota is shown past its expiresAt — the reading is stale. */
  isCacheStale: boolean
  observedAt?: string
  expiresAt?: string
  metadataObservedAt?: string | null
  lastRefresh?: string | null
  nextRetryAfter?: string | null
  passiveQuota?: LiveCapacityPassiveObservation
  passiveModelQuotas: LiveCapacityPassiveModelObservation[]
  /** Subscription start, exposed only while still in the future. */
  activeStart?: string | null
  activeUntil?: string | null
}

export interface LiveCapacityPassiveObservation {
  source: "cpa_passive"
  observedAt: string
  activeLimit?: string
  metrics: LiveCapacityMetric[]
}

export interface LiveCapacityPassiveModelObservation extends LiveCapacityPassiveObservation {
  model: string
}

export interface LiveCapacityMetric {
  label: string
  valueLabel: string
  /** Absolute reset instant reported by the provider, when present. */
  resetAt?: string
  /** Provider-reported seconds until reset, anchored to the observation time. */
  resetAfterSeconds?: number
  progress: number | null
  tone: "green" | "amber" | "red" | "muted"
  /** Window length in seconds; derived from window.seconds (Codex) or duration+unit (Kimi). */
  windowSeconds?: number
}

export type CapacityWindowRole = "short" | "long" | "unknown"

export interface CapacityEntry {
  metric: LiveCapacityMetric
  source: "probe" | "reported"
  observedAt?: string
  /** Window role driving the merge key; null for rows without window semantics (credits, retry hints). */
  windowRole: CapacityWindowRole | null
  isBaseWindow: boolean
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
      const cachedQuota = cachedByAuthIndex.get(identity.identity)
      const activeQuota = taskState?.status === "completed" ? taskState.quota : cachedQuota
      const quotaRows = activeQuota?.quota ?? []
      const fiveHour = findQuotaWindow(quotaRows, "5h")
      const weekly = findQuotaWindow(quotaRows, "weekly")
      const additionalMetrics = quotaRows
        .filter((row) => row !== fiveHour && row !== weekly)
        .map(metricFromQuotaRow)
      const isConstrained = quotaRows.some(isConstrainedQuotaRow)
      const resolvedPlanType = planType(quotaRows, identity.plan_type)
      const planDisplay = planDisplayFor(providerKind, resolvedPlanType)
      const priorityLabel = planDisplay.tone === "priority" ? planDisplay.label : undefined
      const passiveQuota = passiveAccountObservation(providerKind, identity.passive_quota)
      const passiveModelQuotas = passiveModelObservations(providerKind, identity.passive_model_quotas)

      let status: LiveCapacityStatus = activeQuota ? "cached" : "no_cache"
      let error: string | undefined
      let errorLabel: string | undefined
      if (identity.disabled) {
        status = "disabled"
      } else if (!supported) {
        status = "unsupported"
      } else if (taskState?.status === "starting" || taskState?.status === "queued" || taskState?.status === "running") {
        status = "refreshing"
      } else if (taskState?.status === "failed") {
        status = "failed"
        errorLabel = rejectionLabel(taskState.error)
        error = taskState.error
      }

      const observedAt = taskState?.status === "completed" ? taskState.cachedAt : cachedQuota?.cachedAt
      const expiresAt = taskState?.status === "completed" ? taskState.expiresAt : cachedQuota?.expiresAt

      return {
        id: identity.id,
        authIndex: identity.identity,
        provider: identity.provider,
        providerKind,
        providerLabel: providerLabelFor(providerKind, identity),
        type: identity.type,
        name: identity.name,
        alias: identity.alias,
        displayName: identity.displayName,
        disabled: identity.disabled === true,
        unavailable: identity.unavailable ?? null,
        accountState: accountStateFromIdentity(identity.status),
        status,
        errorLabel,
        error,
        fiveHour: fiveHour ? metricFromQuotaRow(fiveHour) : undefined,
        weekly: weekly ? metricFromQuotaRow(weekly) : undefined,
        additionalMetrics,
        planType: resolvedPlanType,
        planLabel: planDisplay.label,
        planTone: planDisplay.tone,
        priorityLabel,
        isPriorityAccount: Boolean(priorityLabel),
        isConstrained,
        observedAt,
        expiresAt,
        metadataObservedAt: identity.metadata_observed_at,
        lastRefresh: identity.last_refresh,
        nextRetryAfter: identity.next_retry_after,
        passiveQuota,
        passiveModelQuotas,
        isCacheStale: status === "cached" && isPastTimestamp(expiresAt),
        // active_start only carries signal while still in the future (the
        // subscription is not yet effective); past starts are display noise.
        activeStart: isFutureTimestamp(identity.active_start) ? identity.active_start : null,
        activeUntil: identity.active_until,
      }
    })
    .sort(compareLiveCapacityRows)
}

function passiveAccountObservation(providerKind: ProviderKind, observation: PassiveQuotaObservation | null | undefined): LiveCapacityPassiveObservation | undefined {
  if (!supportsPassiveQuota(providerKind) || observation?.source !== "cpa_passive" || observation.scope !== "account" || !validObservationTime(observation.observed_at)) {
    return undefined
  }
  const metrics = observation.quota.map(metricFromQuotaRow)
  if (metrics.length === 0 && !observation.active_limit) return undefined
  return { source: observation.source, observedAt: observation.observed_at, activeLimit: observation.active_limit, metrics }
}

function passiveModelObservations(providerKind: ProviderKind, observations: PassiveModelQuotaObservation[] | null | undefined): LiveCapacityPassiveModelObservation[] {
  if (!supportsPassiveQuota(providerKind)) return []
  return (observations ?? []).flatMap((observation) => {
    const model = observation.model.trim()
    if (observation.source !== "cpa_passive" || observation.scope !== "model" || !model || !validObservationTime(observation.observed_at)) return []
    const metrics = observation.quota.map(metricFromQuotaRow)
    if (metrics.length === 0 && !observation.active_limit) return []
    return [{ source: observation.source, model, observedAt: observation.observed_at, activeLimit: observation.active_limit, metrics }]
  })
}

function supportsPassiveQuota(providerKind: ProviderKind): boolean {
  return providerKind === "claude" || providerKind === "codex"
}

function validObservationTime(value: string): boolean {
  return value.trim() !== "" && parseTime(value) !== undefined
}

export function accountStateFromIdentity(status: KeyIdentity["status"]): LiveCapacityAccountState {
  switch (status) {
    case "active":
      return { kind: status, label: "Active", explanation: "CPA observed this auth file as active.", tone: "green" }
    case "pending":
      return { kind: status, label: "Pending", explanation: "CPA observed this auth file waiting for an external action.", tone: "amber" }
    case "refreshing":
      return { kind: status, label: "Refreshing", explanation: "CPA observed this auth file refreshing its authentication state.", tone: "amber" }
    case "error":
      return { kind: status, label: "Error", explanation: "CPA observed an error state for this auth file.", tone: "red" }
    case "disabled":
      return { kind: status, label: "Disabled state", explanation: "CPA reported a disabled lifecycle state for this auth file.", tone: "amber" }
    case "unknown":
      return { kind: status, label: "Unknown", explanation: "CPA reported that this auth-file state is unknown.", tone: "muted" }
    case "other":
      return { kind: status, label: "Other state", explanation: "CPA reported another bounded auth-file state.", tone: "muted" }
    default:
      return { kind: "not_reported", label: "State not reported", explanation: "CPA did not report an auth-file state in this observation.", tone: "muted" }
  }
}

function isFutureTimestamp(value: string | null | undefined): boolean {
  const time = parseTime(value)
  return time !== undefined && time > Date.now()
}

function isPastTimestamp(value: string | null | undefined): boolean {
  const time = parseTime(value)
  return time !== undefined && time <= Date.now()
}

export function mergeCapacityEntries(row: LiveCapacityRow, options?: { includeManualProbe?: boolean }): CapacityEntry[] {
  const candidates: CapacityEntry[] = []
  // Disabled accounts never surface manual-probe readings: merging must happen
  // after that exclusion so a winning probe entry cannot swallow the reported one.
  if (options?.includeManualProbe ?? true) {
    for (const metric of [row.fiveHour, row.weekly, ...row.additionalMetrics]) {
      if (metric) candidates.push(capacityEntry(metric, "probe", row.observedAt))
    }
  }
  for (const metric of row.passiveQuota?.metrics ?? []) {
    candidates.push(capacityEntry(metric, "reported", row.passiveQuota?.observedAt))
  }

  const entries: CapacityEntry[] = []
  const mergeIndexByKey = new Map<string, number>()
  for (const entry of candidates) {
    const key = capacityMergeKey(entry)
    if (key === null) {
      entries.push(entry)
      continue
    }
    const existingIndex = mergeIndexByKey.get(key)
    if (existingIndex === undefined) {
      mergeIndexByKey.set(key, entries.length)
      entries.push(entry)
      continue
    }
    // Probe candidates iterate first, so keeping the incumbent on equal
    // observation times is what prefers probe over reported.
    const existingTime = parseTime(entries[existingIndex].observedAt) ?? Number.NEGATIVE_INFINITY
    const candidateTime = parseTime(entry.observedAt) ?? Number.NEGATIVE_INFINITY
    if (candidateTime > existingTime) entries[existingIndex] = entry
  }
  return entries
}

function capacityEntry(metric: LiveCapacityMetric, source: CapacityEntry["source"], observedAt?: string): CapacityEntry {
  const windowRole = capacityWindowRole(metric)
  return {
    metric,
    source,
    observedAt,
    windowRole,
    isBaseWindow: capacityLabelStem(metric.label) === "base",
  }
}

function capacityMergeKey(entry: CapacityEntry): string | null {
  if (entry.windowRole === null) return null
  return `${capacityLabelStem(entry.metric.label)}|${entry.windowRole}`
}

const WINDOW_LABEL_SUFFIX = /\s+(5h|weekly|window)$/i

function capacityLabelStem(label: string): string {
  const normalized = label.trim().toLowerCase()
  if (normalized === "5h" || normalized === "weekly" || normalized === "window") return "base"
  return normalized.replace(WINDOW_LABEL_SUFFIX, "")
}

function capacityWindowRole(metric: LiveCapacityMetric): CapacityWindowRole | null {
  if (metric.windowSeconds === FIVE_HOUR_WINDOW_SECONDS) return "short"
  if (metric.windowSeconds === WEEKLY_WINDOW_SECONDS) return "long"
  const label = metric.label.trim().toLowerCase()
  if (label.includes("5h")) return "short"
  if (label.includes("weekly")) return "long"
  if (metric.windowSeconds !== undefined) return "unknown"
  if (label === "window" || label.endsWith(" window")) return "unknown"
  return null
}

export function mergeLiveCapacityRowOrder(currentOrder: string[], rows: LiveCapacityRow[]): string[] {
  const rowAuthIndexes = new Set(rows.map((row) => row.authIndex))
  const nextOrder = currentOrder.filter((authIndex) => rowAuthIndexes.has(authIndex))
  const orderedAuthIndexes = new Set(nextOrder)
  for (const row of rows) {
    if (!orderedAuthIndexes.has(row.authIndex)) {
      nextOrder.push(row.authIndex)
    }
  }
  if (nextOrder.length === currentOrder.length && nextOrder.every((authIndex, index) => authIndex === currentOrder[index])) {
    return currentOrder
  }
  return nextOrder
}

export function orderLiveCapacityRows(rows: LiveCapacityRow[], rowOrder: string[]): LiveCapacityRow[] {
  if (rowOrder.length === 0) return rows
  const orderByAuthIndex = new Map(rowOrder.map((authIndex, index) => [authIndex, index]))
  return [...rows].sort((a, b) => {
    const aOrder = orderByAuthIndex.get(a.authIndex)
    const bOrder = orderByAuthIndex.get(b.authIndex)
    if (aOrder !== undefined && bOrder !== undefined) return aOrder - bOrder
    if (aOrder !== undefined) return -1
    if (bOrder !== undefined) return 1
    return 0
  })
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

function planDisplayFor(providerKind: ProviderKind, planTypeValue: string): { label?: string; tone: LiveCapacityPlanTone } {
  const normalizedPlanType = planTypeValue.trim()
  if (!normalizedPlanType) return { tone: "none" }
  const priorityLabel = priorityLabelFor(providerKind, normalizedPlanType)
  if (priorityLabel) return { label: priorityLabel, tone: "priority" }
  return { label: formatPlanType(normalizedPlanType), tone: "ordinary" }
}

function formatPlanType(planTypeValue: string): string {
  return planTypeValue
    .split(/[\s_-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ")
}

export const FIVE_HOUR_WINDOW_SECONDS = 18_000
export const WEEKLY_WINDOW_SECONDS = 604_800

function findQuotaWindow(rows: QuotaRow[], kind: "5h" | "weekly"): QuotaRow | undefined {
  const seconds = kind === "5h" ? FIVE_HOUR_WINDOW_SECONDS : WEEKLY_WINDOW_SECONDS
  // window.seconds is authoritative (the backend derives labels from it), so
  // rows that carry it win over label-only rows regardless of array position;
  // label matching is the fallback for providers that omit window entirely.
  return (
    rows.find((row) => row.window?.seconds === seconds) ??
    rows.find((row) => {
      const label = (row.label ?? "").toLowerCase()
      if (kind === "5h") return label === "5h" || label.includes("5h")
      return label === "weekly" || label.includes("weekly") || label.includes("7d")
    })
  )
}

const WINDOW_UNIT_SECONDS: Record<string, number> = {
  second: 1,
  minute: 60,
  hour: 3_600,
  day: 86_400,
  week: 604_800,
}

/** Codex reports window.seconds directly; Kimi reports duration+unit. */
function quotaWindowSeconds(window: QuotaWindow | undefined): number | undefined {
  if (!window) return undefined
  if (typeof window.seconds === "number" && window.seconds > 0) return window.seconds
  if (typeof window.duration === "number" && window.duration > 0 && window.unit) {
    const unit = window.unit.trim().toLowerCase().replace(/s$/, "")
    const unitSeconds = WINDOW_UNIT_SECONDS[unit]
    if (unitSeconds) return window.duration * unitSeconds
  }
  return undefined
}

function metricFromQuotaRow(row: QuotaRow): LiveCapacityMetric {
  const progress = progressFromQuotaRow(row)
  return {
    label: metricLabel(row),
    valueLabel: valueLabel(row),
    resetAt: row.resetAt,
    resetAfterSeconds: typeof row.resetAfterSeconds === "number" ? row.resetAfterSeconds : undefined,
    progress,
    tone: toneFromProgress(row, progress),
    windowSeconds: quotaWindowSeconds(row.window),
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
  // Credit state is a boolean provider fact; pairing it with a raw remaining
  // count ("0 credits left · No credits") reads as a duplicate statement.
  if (typeof row.hasCredits === "boolean") return row.hasCredits ? "Credits available" : "No credits"

  let measurement = ""
  if (row.unlimited === true) measurement = "Unlimited"
  else if (typeof row.usedPercent === "number") measurement = `${Math.round(row.usedPercent)}% used`
  else if (typeof row.remainingFraction === "number") measurement = `${Math.round((1 - row.remainingFraction) * 100)}% used`
  if (typeof row.remaining === "number" && typeof row.limit === "number" && row.limit > 0) {
    measurement ||= `${formatQuotaNumber(Math.max(0, row.limit - row.remaining))} / ${formatQuotaNumber(row.limit)} used`
  }
  if (!measurement && typeof row.remaining === "number") measurement = `${formatQuotaNumber(row.remaining)}${row.unit ? ` ${row.unit}` : ""} left`
  if (!measurement && typeof row.used === "number" && typeof row.limit === "number") measurement = `${formatQuotaNumber(row.used)} / ${formatQuotaNumber(row.limit)} used`

  let state = ""
  if (typeof row.allowed === "boolean") state = row.allowed ? "Allowed" : "Blocked"
  else if (typeof row.limitReached === "boolean") state = row.limitReached ? "Limit reached" : "Limit not reached"

  if (measurement && state) return `${measurement} · ${state}`
  return measurement || state || "Measured"
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
  return row.unlimited !== true && typeof row.remaining === "number" && row.remaining <= 0
}

export interface LiveCapacityResetCountdown {
  /** Compact relative label, e.g. "2h 13m". */
  relativeLabel: string
  /** True once the reset instant has passed. */
  isDue: boolean
  /** Absolute reset instant (ISO), when it could be derived. */
  resetAt?: string
}

/**
 * Resolves a metric's reset hint to a countdown. resetAt is absolute;
 * resetAfterSeconds is anchored to the observation time, never to now.
 * Returns undefined when no usable reset hint exists (rendered as "-").
 */
export function resetCountdown(
  metric: Pick<LiveCapacityMetric, "resetAt" | "resetAfterSeconds">,
  observedAt?: string,
  now: number = Date.now(),
): LiveCapacityResetCountdown | undefined {
  let instant = parseTime(metric.resetAt)
  if (instant === undefined && typeof metric.resetAfterSeconds === "number") {
    const anchor = parseTime(observedAt)
    if (anchor !== undefined) instant = anchor + metric.resetAfterSeconds * 1000
  }
  if (instant === undefined) return undefined
  const seconds = Math.round((instant - now) / 1000)
  return {
    relativeLabel: seconds <= 0 ? "due" : formatCountdownDuration(seconds),
    isDue: seconds <= 0,
    resetAt: new Date(instant).toISOString(),
  }
}

function parseTime(value: string | null | undefined): number | undefined {
  if (!value) return undefined
  const time = new Date(value).getTime()
  return Number.isFinite(time) ? time : undefined
}

function formatCountdownDuration(seconds: number): string {
  if (seconds < 60) return `${seconds}s`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${hours}h ${minutes % 60}m`
  const days = Math.floor(hours / 24)
  return `${days}d ${hours % 24}h`
}

function planType(rows: QuotaRow[], identityPlanType?: string | null): string {
  const quotaPlanType = rows.find((row) => row.planType)?.planType?.trim()
  if (quotaPlanType) return quotaPlanType
  return identityPlanType?.trim() ?? ""
}

function rejectionLabel(code: string): string {
  switch (code) {
    case "refresh_unavailable":
      return "Refresh unavailable"
    case "unsupported":
      return "Unsupported"
    case "not_auth_file":
      return "Not auth-file"
    case "not_found":
      return "Not found"
    case "disabled":
      return "Disabled"
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
