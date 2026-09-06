import {
  AlertTriangle,
  ArrowRight,
  CalendarDays,
  CalendarRange,
  Clock,
  Eye,
  Gauge,
  Hourglass,
  Loader2,
  ListChecks,
  Power,
  RefreshCw,
  Timer,
  type LucideIcon,
} from "lucide-react"
import { useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  buildLiveCapacityRows,
  FIVE_HOUR_WINDOW_SECONDS,
  mergeLiveCapacityRowOrder,
  orderLiveCapacityRows,
  WEEKLY_WINDOW_SECONDS,
  type LiveCapacityAccountStateTone,
  type LiveCapacityMetric,
  type LiveCapacityPassiveObservation,
  type LiveCapacityPlanTone,
  type LiveCapacityRow,
  type ProviderKind,
} from "@/features/usage-intelligence/live-capacity"
import { useLiveCapacity } from "@/hooks/useQuota"
import { MODEL_SUPPORT_MAX_ACCOUNTS, useModelSupport } from "@/hooks/useModelSupport"
import { useSetIdentityDisabled } from "@/hooks/useKeys"
import { useFlipReorder } from "@/hooks/useFlipReorder"
import { useToast } from "@/components/providers/toast-provider"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { AccountModelSupport, ModelCapability, ModelSupportResponse, RegisteredModelSupport } from "@/types/api"
import { ProviderBrandIcon } from "./provider-brand-icon"

export function LiveCapacityCard({ provider }: { provider: string }) {
  const { identities, cachedQuota, taskStates, refresh, refreshLimit, isLoading, isRefreshing, error } = useLiveCapacity(provider)
  const modelSupport = useModelSupport()
  const derivedRows = useMemo(
    () => buildLiveCapacityRows({ identities, cachedQuota, taskStates }),
    [identities, cachedQuota, taskStates],
  )

  const providerGroups = useMemo(() => {
    const groups = new Map<ProviderKind, { kind: ProviderKind; label: string; count: number }>()
    for (const row of derivedRows) {
      const group = groups.get(row.providerKind)
      if (group) group.count += 1
      else groups.set(row.providerKind, { kind: row.providerKind, label: row.providerLabel, count: 1 })
    }
    return [...groups.values()].sort(
      (a, b) => b.count - a.count || a.label.localeCompare(b.label),
    )
  }, [derivedRows])

  const [selectedKind, setSelectedKind] = useState<ProviderKind | "all">("all")
  const effectiveKind = selectedKind !== "all" && providerGroups.some((group) => group.kind === selectedKind)
    ? selectedKind
    : "all"

  // Split and ordering run on the unfiltered set so that switching provider
  // chips never rewrites the remembered regular-section order (rowOrder).
  const [allPriorityRows, allRegularRows] = useMemo(() => {
    const priority: typeof derivedRows = []
    const regular: typeof derivedRows = []
    for (const row of derivedRows) {
      if (row.isPriorityAccount) priority.push(row)
      else regular.push(row)
    }
    return [priority, regular] as const
  }, [derivedRows])

  const [rowOrder, setRowOrder] = useState<string[]>([])
  useLayoutEffect(() => {
    if (isLoading || error) return
    // eslint-disable-next-line react-hooks/set-state-in-effect -- useLayoutEffect blocks paint, so no flash
    setRowOrder((currentOrder) => mergeLiveCapacityRowOrder(currentOrder, allRegularRows))
  }, [allRegularRows, error, isLoading])
  const orderedRegularRows = useMemo(
    () => orderLiveCapacityRows(allRegularRows, rowOrder),
    [allRegularRows, rowOrder],
  )

  const priorityRows = useMemo(
    () => effectiveKind === "all" ? allPriorityRows : allPriorityRows.filter((row) => row.providerKind === effectiveKind),
    [allPriorityRows, effectiveKind],
  )
  const regularRows = useMemo(() => {
    const visible = effectiveKind === "all" ? orderedRegularRows : orderedRegularRows.filter((row) => row.providerKind === effectiveKind)
    // Disabled accounts sink to the end of the regular section; the relative
    // order of enabled rows (and the remembered rowOrder) is left untouched.
    return [...visible].sort((a, b) => Number(a.disabled) - Number(b.disabled))
  }, [orderedRegularRows, effectiveKind])
  const displayedRows = useMemo(() => [...priorityRows, ...regularRows], [priorityRows, regularRows])

  const [selectedSupportIDs, setSelectedSupportIDs] = useState<Set<number>>(() => new Set())
  const [loadedSupportScopeKey, setLoadedSupportScopeKey] = useState("")
  const [requestedSupportScopeKey, setRequestedSupportScopeKey] = useState("")
  const visibleIdentityIDs = useMemo(() => new Set(identities.map((identity) => identity.id)), [identities])
  const selectedSupportIdentityIDs = useMemo(
    () => [...selectedSupportIDs].filter((id) => visibleIdentityIDs.has(id)).sort((a, b) => a - b),
    [selectedSupportIDs, visibleIdentityIDs],
  )
  const selectedSupportScopeKey = selectedSupportIdentityIDs.join(",")
  const loadedModelSupport = modelSupport.data && loadedSupportScopeKey === selectedSupportScopeKey
    ? modelSupport.data
    : undefined
  const supportByIdentityID = useMemo(
    () => new Map((loadedModelSupport?.accounts ?? []).map((account) => [account.identity_id, account])),
    [loadedModelSupport],
  )
  const displayedIdentityIDs = useMemo(() => displayedRows.map((row) => row.id), [displayedRows])
  const displayedSelected = displayedIdentityIDs.length > 0 && displayedIdentityIDs.every((id) => selectedSupportIDs.has(id))
  const selectionTooLarge = selectedSupportIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS

  const resetSupportSelection = () => {
    setSelectedSupportIDs(new Set())
    setLoadedSupportScopeKey("")
    setRequestedSupportScopeKey("")
    modelSupport.reset()
  }
  const selectProviderKind = (kind: ProviderKind | "all") => {
    setSelectedKind(kind)
    resetSupportSelection()
  }
  const toggleSupportSelection = (identityID: number) => {
    setLoadedSupportScopeKey("")
    setRequestedSupportScopeKey("")
    modelSupport.reset()
    setSelectedSupportIDs((current) => {
      const next = new Set(current)
      if (next.has(identityID)) next.delete(identityID)
      else next.add(identityID)
      return next
    })
  }
  const toggleDisplayedSelection = () => {
    if (displayedSelected) {
      resetSupportSelection()
      return
    }
    if (displayedIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS) return
    setLoadedSupportScopeKey("")
    setRequestedSupportScopeKey("")
    modelSupport.reset()
    setSelectedSupportIDs(new Set(displayedIdentityIDs))
  }
  const loadSelectedSupport = () => {
    const scopeKey = selectedSupportScopeKey
    setRequestedSupportScopeKey(scopeKey)
    modelSupport.mutate(selectedSupportIdentityIDs, {
      onSuccess: () => setLoadedSupportScopeKey(scopeKey),
    })
  }

  const regularRowKeys = useMemo(() => regularRows.map((r) => r.authIndex), [regularRows])
  const flipEnabled = !isLoading && !error && regularRows.length > 0
  const { containerRef, registerItem } = useFlipReorder(regularRowKeys, { enabled: flipEnabled })

  const displayedCount = displayedRows.length
  const refreshLabel = displayedCount > refreshLimit ? `Refresh first ${refreshLimit}` : "Refresh"
  const refreshDisplayed = () => {
    if (effectiveKind === "all") refresh()
    else refresh(displayedRows.map((row) => row.authIndex))
  }

  return (
    <Card>
      <CardHeader className="flex flex-col items-start justify-between gap-3 pb-3 sm:flex-row sm:items-center">
        <div>
          <CardTitle className="flex items-center gap-2">
            Live Capacity
            <Gauge className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Fixed live capacity probe" />
          </CardTitle>
          <CardDescription>Manual probes and CPA passive observations</CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="blue">live probe</Badge>
          <Badge variant="outline">fixed</Badge>
          {displayedCount > refreshLimit ? <Badge variant="amber">max {refreshLimit}</Badge> : null}
          <Badge variant={selectionTooLarge ? "amber" : "outline"}>support {selectedSupportIdentityIDs.length}/{MODEL_SUPPORT_MAX_ACCOUNTS}</Badge>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={loadSelectedSupport}
            disabled={selectedSupportIdentityIDs.length === 0 || selectionTooLarge || modelSupport.isPending}
          >
            {modelSupport.isPending ? <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" /> : <ListChecks className="mr-1.5 h-3.5 w-3.5" />}
            Load model support
          </Button>
          <Button type="button" variant="outline" size="sm" onClick={refreshDisplayed} disabled={displayedCount === 0}>
            <RefreshCw className={cn("mr-1.5 h-3.5 w-3.5", isRefreshing && "animate-spin")} />
            {refreshLabel}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : error ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Failed to load live capacity
          </div>
        ) : identities.length === 0 ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No auth-file accounts
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            {providerGroups.length > 1 ? (
              <div className="flex flex-wrap items-center gap-1.5" role="group" aria-label="Filter accounts by provider">
                <ProviderFilterChip
                  label="All"
                  count={derivedRows.length}
                  active={effectiveKind === "all"}
                  onClick={() => selectProviderKind("all")}
                />
                {providerGroups.map((group) => (
                  <ProviderFilterChip
                    key={group.kind}
                    label={group.label}
                    count={group.count}
                    providerKind={group.kind}
                    active={effectiveKind === group.kind}
                    onClick={() => selectProviderKind(group.kind)}
                  />
                ))}
              </div>
            ) : null}
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-md border border-border/70 bg-muted/[0.12] p-2.5 text-xs">
              <span className="text-muted-foreground">
                Select an account set, then load registered support. This does not test current routing availability.
              </span>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={toggleDisplayedSelection}
                disabled={modelSupport.isPending || (!displayedSelected && displayedIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS)}
              >
                {displayedSelected ? "Clear selection" : displayedIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS ? `Choose up to ${MODEL_SUPPORT_MAX_ACCOUNTS}` : "Select displayed"}
              </Button>
            </div>
            {selectionTooLarge ? (
              <div className="rounded-md border border-amber-500/30 bg-amber-500/[0.05] p-2.5 text-xs text-amber-700 dark:text-amber-300" role="alert">
                Narrow selection to {MODEL_SUPPORT_MAX_ACCOUNTS} accounts or fewer. Nothing will be silently omitted.
              </div>
            ) : null}
            {modelSupport.isPending ? (
              <Skeleton className="h-20 w-full" />
            ) : modelSupport.isError && requestedSupportScopeKey === selectedSupportScopeKey ? (
              <div className="rounded-md border border-red-500/25 bg-red-500/[0.025] p-3 text-sm text-red-600" role="alert">
                Failed to load registered model support. {modelSupport.error instanceof Error ? modelSupport.error.message : "Try again."}
              </div>
            ) : loadedModelSupport ? (
              <ModelSupportCoveragePanel result={loadedModelSupport} />
            ) : null}
            <div className="flex max-h-[560px] flex-col gap-3 overflow-y-auto pr-1">
              {priorityRows.length > 0 ? (
                <div className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3">
                  {priorityRows.map((row) => (
                    <LiveCapacityAccountTile
                      key={row.authIndex}
                      row={row}
                      onRefresh={() => refresh(row.authIndex)}
                      selectedForSupport={selectedSupportIDs.has(row.id)}
                      onToggleSupportSelection={() => toggleSupportSelection(row.id)}
                      modelSupport={supportByIdentityID.get(row.id)}
                      supportSelectionDisabled={modelSupport.isPending}
                    />
                  ))}
                </div>
              ) : null}

              {priorityRows.length > 0 && regularRows.length > 0 ? (
                <div className="border-t border-border/50" role="separator" />
              ) : null}

              {regularRows.length > 0 ? (
                <div
                  ref={containerRef}
                  className="grid grid-cols-1 gap-3 md:grid-cols-2 xl:grid-cols-3"
                >
                  {regularRows.map((row) => (
                    <div key={row.authIndex} ref={registerItem(row.authIndex)}>
                      <LiveCapacityAccountTile
                        row={row}
                        onRefresh={() => refresh(row.authIndex)}
                        selectedForSupport={selectedSupportIDs.has(row.id)}
                        onToggleSupportSelection={() => toggleSupportSelection(row.id)}
                        modelSupport={supportByIdentityID.get(row.id)}
                        supportSelectionDisabled={modelSupport.isPending}
                      />
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ProviderFilterChip({
  label,
  count,
  providerKind,
  active,
  onClick,
}: {
  label: string
  count: number
  providerKind?: ProviderKind
  active: boolean
  onClick: () => void
}) {
  return (
    <button
      type="button"
      aria-pressed={active}
      onClick={onClick}
      className={cn(
        "flex items-center gap-1.5 rounded-full border px-2.5 py-1 text-xs font-medium transition-[background-color,border-color,color] duration-200",
        active
          ? "border-terracotta-500/40 bg-terracotta-500/10 text-terracotta-700 dark:text-terracotta-300"
          : "border-border bg-background/60 text-muted-foreground hover:border-terracotta-500/25 hover:text-foreground",
      )}
    >
      {providerKind ? (
        <ProviderBrandIcon providerKind={providerKind} label={label} className="h-3.5 w-3.5" />
      ) : null}
      <span>{label}</span>
      <span
        className={cn(
          "rounded-full px-1.5 text-[10px] leading-4 tabular-nums",
          active ? "bg-terracotta-500/15 text-terracotta-700 dark:text-terracotta-300" : "bg-muted text-muted-foreground",
        )}
      >
        {count}
      </span>
    </button>
  )
}

function LiveCapacityAccountTile({
  row,
  onRefresh,
  selectedForSupport,
  onToggleSupportSelection,
  modelSupport,
  supportSelectionDisabled,
}: {
  row: LiveCapacityRow
  onRefresh: () => void
  selectedForSupport: boolean
  onToggleSupportSelection: () => void
  modelSupport?: AccountModelSupport
  supportSelectionDisabled: boolean
}) {
  const toast = useToast()
  const setIdentityDisabled = useSetIdentityDisabled()
  const [confirmingDisable, setConfirmingDisable] = useState(false)
  const confirmTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  useEffect(() => () => {
    if (confirmTimerRef.current) clearTimeout(confirmTimerRef.current)
  }, [])

  const clearConfirmTimer = () => {
    if (confirmTimerRef.current) {
      clearTimeout(confirmTimerRef.current)
      confirmTimerRef.current = null
    }
  }
  const handlePowerClick = () => {
    if (setIdentityDisabled.isPending) return
    if (row.disabled) {
      setIdentityDisabled.mutate(
        { id: row.id, disabled: false },
        {
          onSuccess: () => toast.success("Account enabled"),
          onError: () => toast.error("Failed to update account"),
        },
      )
      return
    }
    if (!confirmingDisable) {
      setConfirmingDisable(true)
      confirmTimerRef.current = setTimeout(() => {
        confirmTimerRef.current = null
        setConfirmingDisable(false)
      }, 3_000)
      return
    }
    clearConfirmTimer()
    setConfirmingDisable(false)
    setIdentityDisabled.mutate(
      { id: row.id, disabled: true },
      {
        onSuccess: () => toast.success("Account disabled"),
        onError: () => toast.error("Failed to update account"),
      },
    )
  }

  const copyAuthIndex = async () => {
    try {
      await navigator.clipboard.writeText(row.authIndex)
      toast.success("Auth index copied")
    } catch {
      toast.error("Failed to copy auth index")
    }
  }

  const primaryMetric = row.fiveHour ?? row.additionalMetrics[0]
  const secondaryMetric = row.weekly ?? row.additionalMetrics.find((metric) => metric !== primaryMetric)
  const remainingMetrics = row.additionalMetrics.filter((metric) => metric !== primaryMetric && metric !== secondaryMetric)
  const isRowRefreshing = row.status === "refreshing"
  const hasAttention = row.isConstrained || row.status === "failed" || row.unavailable === true || row.accountState.kind === "error"
  const accountTitle = row.alias || row.displayName || row.name || row.authIndex
  const attentionLabel = row.status === "failed"
    ? `Refresh failed: ${row.errorLabel ?? "Failed"}`
    : row.unavailable === true
      ? "Temporarily unavailable in CPA"
      : row.accountState.kind === "error"
        ? "CPA observed an account error state"
        : row.isConstrained
          ? "Capacity constrained"
          : undefined

  return (
    <div
      className={cn(
        "group flex min-h-[190px] min-w-0 flex-col rounded-lg border border-border bg-background/70 p-3 text-sm transition-[background-color,border-color,box-shadow] duration-300 hover:border-terracotta-500/25 hover:shadow-sm",
        row.disabled && "opacity-60",
        row.status === "failed" && "border-red-500/25 bg-red-500/[0.025]",
        row.status !== "failed" && row.isConstrained && "border-amber-500/30 bg-amber-500/[0.03]",
        row.status !== "failed" && !row.isConstrained && row.isPriorityAccount && "border-terracotta-500/30 shadow-[inset_2px_0_0_rgba(192,80,62,0.45)]",
        isRowRefreshing && "border-amber-500/25 shadow-[0_0_0_1px_rgba(245,158,11,0.08)]",
      )}
    >
      {/* 不再渲染状态 chip：cached 由配额数据和 Cache expires 表达，refreshing 由刷新按钮动画表达，
          failed 由 ⚠️ 与红色边框表达，disabled 由标题旁徽章表达。 */}
      <div className="flex items-center gap-2">
        <div
          className="flex h-[42px] w-9 shrink-0 items-center justify-center rounded-md border border-border/70 bg-muted/20 dark:border-white/10 dark:bg-white/90"
          title={row.providerLabel}
          aria-label={row.providerLabel}
        >
          <ProviderBrandIcon providerKind={row.providerKind} label={row.providerLabel} className="h-5 w-5" />
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex min-w-0 items-center gap-1.5">
            <p className="truncate font-medium leading-5" title={accountTitle}>{accountTitle}</p>
            {row.planLabel ? (
              <PlanBadge label={row.planLabel} tone={row.planTone} rawPlanType={row.planType} />
            ) : null}
            {row.disabled ? (
              <Badge variant="amber" className="shrink-0 px-1.5 py-0 text-[10px] leading-4">Disabled</Badge>
            ) : null}
          </div>
          <button
            type="button"
            className="mt-0.5 block max-w-full truncate text-left text-xs text-muted-foreground transition-colors hover:text-foreground"
            title={`${row.authIndex} — click to copy`}
            aria-label={`Copy auth index ${row.authIndex}`}
            data-auth-index={row.authIndex}
            onClick={copyAuthIndex}
          >
            {formatAuthIndex(row.authIndex)}
          </button>
          <label className="mt-1 flex w-fit cursor-pointer items-center gap-1.5 text-[10px] text-muted-foreground">
            <input
              type="checkbox"
              className="h-3.5 w-3.5 accent-terracotta-600"
              checked={selectedForSupport}
              onChange={onToggleSupportSelection}
              disabled={supportSelectionDisabled}
              aria-label={`Include ${accountTitle} in model support coverage`}
            />
            Support scope
          </label>
        </div>
        <div className="flex shrink-0 items-center gap-1.5">
        {hasAttention ? (
          <span title={attentionLabel}>
            <AlertTriangle
              className={cn("h-4 w-4", row.status === "failed" ? "text-red-600" : "text-amber-600")}
              aria-label={attentionLabel}
            />
          </span>
        ) : null}
        {confirmingDisable && !row.disabled ? (
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="h-7 px-2 text-xs font-medium text-amber-700 hover:text-amber-800 dark:text-amber-400 dark:hover:text-amber-300"
            onClick={handlePowerClick}
            disabled={setIdentityDisabled.isPending}
            aria-label={`Confirm disabling ${accountTitle}`}
          >
            Confirm?
          </Button>
        ) : (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className={cn(
              "h-7 w-7 transition-opacity",
              row.disabled ? "text-red-600 hover:text-red-700" : "opacity-70 group-hover:opacity-100",
            )}
            onClick={handlePowerClick}
            disabled={setIdentityDisabled.isPending}
            aria-label={row.disabled ? `Enable ${accountTitle}` : `Disable ${accountTitle}`}
            title={row.disabled ? "Enable this account" : "Disable this account"}
          >
            {setIdentityDisabled.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Power className="h-3.5 w-3.5" />
            )}
          </Button>
        )}
        {!row.disabled ? (
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-7 w-7 opacity-70 transition-opacity group-hover:opacity-100"
            onClick={onRefresh}
            disabled={isRowRefreshing || setIdentityDisabled.isPending}
            aria-label={`Refresh ${accountTitle}`}
            title="Refresh this account"
          >
            <RefreshCw className={cn("h-3.5 w-3.5", isRowRefreshing && "animate-spin")} />
          </Button>
        ) : null}
        </div>
      </div>

      <AccountAvailabilitySummary row={row} />

      <PassiveQuotaEvidence
        supported={row.providerKind === "claude" || row.providerKind === "codex"}
        account={row.passiveQuota}
        models={row.passiveModelQuotas}
      />
      {modelSupport ? <AccountModelSupportDetails account={modelSupport} /> : null}

      {!row.disabled ? (
        <div className="mt-3 grid gap-2">
          <p className="text-[10px] font-medium text-foreground/70">Manual capacity probe</p>
          <MetricMeter title={primaryMetric?.label ?? "5h"} metric={primaryMetric} />
          <MetricMeter title={secondaryMetric?.label ?? "Weekly"} metric={secondaryMetric} />
          {remainingMetrics.map((metric, index) => (
            <MetricMeter key={`${index}:${metric.label}`} title={metric.label} metric={metric} />
          ))}
        </div>
      ) : null}
      {row.metadataObservedAt || row.lastRefresh || row.nextRetryAfter || row.observedAt || row.expiresAt || row.activeStart || row.activeUntil ? (
        <AccountTiming
          metadataObservedAt={row.metadataObservedAt}
          lastRefresh={row.lastRefresh}
          nextRetryAfter={row.nextRetryAfter}
          observedAt={row.observedAt}
          expiresAt={row.expiresAt}
          activeStart={row.activeStart}
          activeUntil={row.activeUntil}
          cacheStale={row.isCacheStale}
        />
      ) : null}
    </div>
  )
}

function PassiveQuotaEvidence({
  supported,
  account,
  models,
}: {
  supported: boolean
  account?: LiveCapacityPassiveObservation
  models: LiveCapacityRow["passiveModelQuotas"]
}) {
  if (!supported) return null
  return (
    <div
      className="mt-3 rounded-md border border-terracotta-500/20 bg-terracotta-500/[0.025] p-2.5"
      role="group"
      aria-label="CPA passive quota observation"
    >
      <div className="flex items-center gap-1.5 text-[10px] font-medium text-foreground/70">
        <Eye className="h-3.5 w-3.5 text-terracotta-600 dark:text-terracotta-300" aria-hidden="true" />
        <span>CPA passive quota observation</span>
      </div>
      <div className="mt-2 space-y-3">
        {!account && models.length === 0 ? (
          <p className="text-[10px] text-muted-foreground">No readable passive quota observation.</p>
        ) : null}
        {account ? <PassiveQuotaObservationSection label="Account" observation={account} /> : null}
        {models.map((observation) => (
          <PassiveQuotaObservationSection key={`${observation.model}:${observation.observedAt}`} label={observation.model} observation={observation} />
        ))}
      </div>
      <p className="mt-2 text-[9px] leading-3 text-muted-foreground">
        Latest provider watermark observed by CPA. No expiry or history is inferred; relative reset hints are anchored to the observation time.
      </p>
    </div>
  )
}

function ModelSupportCoveragePanel({ result }: { result: ModelSupportResponse }) {
  const failures = result.accounts.filter((account) => account.status === "failed")
  return (
    <div className="rounded-lg border border-border bg-background/70 p-3 text-sm" aria-live="polite">
      <div className="flex flex-wrap items-center gap-2">
        <span className="font-medium">Registered model support</span>
        <Badge variant={result.scope_complete ? "green" : "amber"}>
          {result.scope_complete ? "Complete selected scope" : "Partial selected scope"}
        </Badge>
        <span className="text-xs text-muted-foreground">{result.loaded_count}/{result.selected_count} accounts loaded</span>
      </div>
      <p className="mt-1 text-xs leading-5 text-muted-foreground">
        Counts describe registry membership in this selected scope, not current routing availability or provider health.
      </p>
      {result.models.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-2">
          {result.models.map((model) => (
            <span key={model.model_id} className="rounded-md border border-border/70 bg-muted/20 px-2 py-1 text-xs" title={model.model_id}>
              <span className="font-medium">{model.display_name || model.model_id}</span>
              <span className="ml-1.5 text-muted-foreground">
                {result.scope_complete
                  ? model.single_registered_account_in_scope
                    ? "1 registered account in this scope"
                    : `${model.observed_supporting_accounts}/${model.selected_accounts} accounts`
                  : `observed in ${model.observed_supporting_accounts}/${result.loaded_count} loaded accounts`}
              </span>
            </span>
          ))}
        </div>
      ) : (
        <p className="mt-3 text-xs text-muted-foreground">
          {result.scope_complete ? "CPA returned no registered models for this selected scope." : "No registered models were observed in the successfully loaded accounts."}
        </p>
      )}
      {failures.length > 0 ? (
        <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/[0.04] p-2.5 text-xs text-amber-700 dark:text-amber-300">
          <p className="font-medium">Failed accounts are unknown, not unsupported</p>
          <ul className="mt-1 list-disc space-y-0.5 pl-4">
            {failures.map((account) => (
              <li key={account.identity_id}>{account.display_name || account.auth_index}: {modelSupportErrorLabel(account.error_code)}</li>
            ))}
          </ul>
        </div>
      ) : null}
    </div>
  )
}

function PassiveQuotaObservationSection({
  label,
  observation,
}: {
  label: string
  observation: LiveCapacityPassiveObservation
}) {
  return (
    <section aria-label={`${label} passive quota`}>
      <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[10px]">
        <span className="truncate font-medium text-foreground/90" title={label}>{label}</span>
        {observation.activeLimit ? (
          <span className="truncate rounded-full bg-muted px-1.5 py-0.5 text-[9px] text-muted-foreground" title={observation.activeLimit}>
            Active limit {observation.activeLimit}
          </span>
        ) : null}
        <time className="ml-auto text-muted-foreground" dateTime={observation.observedAt} title={observation.observedAt}>
          {formatDate(observation.observedAt)}
        </time>
      </div>
      <div className="mt-1.5 grid gap-1.5">
        {observation.metrics.map((metric, index) => (
          <MetricMeter key={`${index}:${metric.label}`} title={metric.label} metric={metric} resetPrefix="reported reset" />
        ))}
      </div>
    </section>
  )
}

function AccountModelSupportDetails({ account }: { account: AccountModelSupport }) {
  if (account.status === "failed") {
    return (
      <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/[0.04] p-2.5 text-[11px] text-amber-700 dark:text-amber-300">
        Registered support unknown: {modelSupportErrorLabel(account.error_code)}. This is not an unsupported result.
      </div>
    )
  }
  return (
    <details className="mt-3 rounded-md border border-border/70 bg-muted/[0.12] p-2.5">
      <summary className="cursor-pointer text-[11px] font-medium">
        Registered models ({account.registered_models.length})
      </summary>
      <p className="mt-1 text-[10px] leading-4 text-muted-foreground">Registered capability only; not current routing availability.</p>
      {account.registered_models.length === 0 ? (
        <p className="mt-2 text-[11px] text-muted-foreground">CPA returned no registered models for this account.</p>
      ) : (
        <ul className="mt-2 space-y-2">
          {account.registered_models.map((model) => <RegisteredModelDetails key={model.id} model={model} />)}
        </ul>
      )}
    </details>
  )
}

function RegisteredModelDetails({ model }: { model: RegisteredModelSupport }) {
  return (
    <li className="rounded border border-border/60 bg-background/50 p-2 text-[11px]">
      <div className="flex flex-wrap items-center gap-1.5">
        <span className="break-all font-medium">{model.display_name || model.id}</span>
        {model.display_name ? <span className="break-all text-[10px] text-muted-foreground">{model.id}</span> : null}
        {model.type ? <Badge variant="outline" className="px-1.5 py-0 text-[9px]">{model.type}</Badge> : null}
      </div>
      {model.capability ? <ModelCapabilityDetails capability={model.capability} /> : (
        <p className="mt-1 text-[10px] text-muted-foreground">{definitionStatusLabel(model.definition_status)}</p>
      )}
    </li>
  )
}

function ModelCapabilityDetails({ capability }: { capability: ModelCapability }) {
  const facts: string[] = []
  if (capability.context_length !== undefined) facts.push(`Context ${capability.context_length.toLocaleString()}`)
  if (capability.input_token_limit !== undefined) facts.push(`Input limit ${capability.input_token_limit.toLocaleString()}`)
  if (capability.max_completion_tokens !== undefined) facts.push(`Max completion ${capability.max_completion_tokens.toLocaleString()}`)
  if (capability.output_token_limit !== undefined) facts.push(`Output limit ${capability.output_token_limit.toLocaleString()}`)
  if (capability.supported_input_modalities?.length) facts.push(`Input ${capability.supported_input_modalities.join(", ")}`)
  if (capability.supported_output_modalities?.length) facts.push(`Output ${capability.supported_output_modalities.join(", ")}`)
  const thinking = capability.thinking
  if (thinking) {
    const details: string[] = []
    if (thinking.min !== undefined || thinking.max !== undefined) details.push(`budget ${thinking.min ?? "?"}–${thinking.max ?? "?"}`)
    if (thinking.levels?.length) details.push(`levels ${thinking.levels.join(", ")}`)
    if (thinking.zero_allowed !== undefined) details.push(`zero ${thinking.zero_allowed ? "allowed" : "not allowed"}`)
    if (thinking.dynamic_allowed !== undefined) details.push(`dynamic ${thinking.dynamic_allowed ? "allowed" : "not allowed"}`)
    facts.push(details.length > 0 ? `Thinking: ${details.join("; ")}` : "Thinking metadata returned")
  }
  return facts.length > 0 ? <p className="mt-1 text-[10px] leading-4 text-muted-foreground">{facts.join(" · ")}</p> : null
}

function modelSupportErrorLabel(code: AccountModelSupport["error_code"]): string {
  if (code === "auth_file_missing") return "auth file is no longer present in the current CPA lookup"
  if (code === "invalid_upstream_response") return "CPA returned an invalid model-support response"
  return "CPA model-support request failed or timed out"
}

function definitionStatusLabel(status: RegisteredModelSupport["definition_status"]): string {
  if (status === "available") return "Static definition returned without supported capability fields."
  if (status === "absent") return "No exact static definition was returned for this model ID."
  if (status === "unknown_channel") return "No supported static catalog channel is known for this account type."
  return "Static capability metadata could not be loaded."
}

function AccountAvailabilitySummary({ row }: { row: LiveCapacityRow }) {
  return (
    <div className="mt-3 rounded-md border border-border/70 bg-muted/[0.12] p-2.5" role="group" aria-label="Account availability">
      <div className="flex flex-wrap items-center gap-1.5">
        {row.disabled ? <Badge variant="amber" className="px-1.5 py-0 text-[10px] leading-4">Operator disabled</Badge> : null}
        {row.unavailable === true ? <Badge variant="red" className="px-1.5 py-0 text-[10px] leading-4">Temporarily unavailable</Badge> : null}
        <Badge variant={accountStateBadgeVariant(row.accountState.tone)} className="px-1.5 py-0 text-[10px] leading-4">
          CPA status: {row.accountState.label}
        </Badge>
      </div>
      <div className="mt-1.5 space-y-1 text-[11px] leading-4 text-muted-foreground">
        {row.disabled ? <p>Operator disabled in CPA; capacity probes stay excluded until re-enabled.</p> : null}
        {row.unavailable === true ? <p>CPA marked this account temporarily unavailable. Retry eligibility is not a recovery guarantee.</p> : null}
        <p>{row.accountState.explanation}</p>
      </div>
    </div>
  )
}

function accountStateBadgeVariant(tone: LiveCapacityAccountStateTone): "green" | "amber" | "red" | "secondary" {
  return tone === "muted" ? "secondary" : tone
}

function AccountTiming({
  metadataObservedAt,
  lastRefresh,
  nextRetryAfter,
  observedAt,
  expiresAt,
  activeStart,
  activeUntil,
  cacheStale = false,
}: {
  metadataObservedAt?: string | null
  lastRefresh?: string | null
  nextRetryAfter?: string | null
  observedAt?: string | null
  expiresAt?: string | null
  activeStart?: string | null
  activeUntil?: string | null
  cacheStale?: boolean
}) {
  const hasAuthFileEvidence = Boolean(metadataObservedAt || lastRefresh || nextRetryAfter)
  const hasProbeWindow = Boolean(observedAt || expiresAt)
  const hasBothProbeEndpoints = Boolean(observedAt && expiresAt)
  // activeStart arrives pre-filtered by buildLiveCapacityRows: it is only set
  // while the subscription start is still in the future.
  const activeRange = activeStart && activeUntil ? { start: activeStart, until: activeUntil } : null
  const singleActiveEndpoint = activeRange ? null : (activeUntil ?? activeStart ?? null)
  const hasActiveWindow = Boolean(activeRange || singleActiveEndpoint)

  return (
    <div
      className="mt-3 rounded-md border border-border/70 bg-muted/[0.12] p-2.5"
      role="group"
      aria-label="Account and cache timing"
    >
      {hasAuthFileEvidence ? (
        <div>
          <div className="flex items-center gap-1.5 text-[10px] font-medium text-foreground/70">
            <Eye className="h-3.5 w-3.5 text-terracotta-600 dark:text-terracotta-300" aria-hidden="true" />
            <span>CPA auth-file evidence</span>
          </div>
          <div className="mt-1.5 grid gap-1.5">
            {metadataObservedAt ? <TimingLine label="Metadata observed" value={metadataObservedAt} /> : null}
            {lastRefresh ? <TimingLine label="Token refreshed" value={lastRefresh} /> : null}
            {nextRetryAfter ? <TimingLine label="Retry eligible" value={nextRetryAfter} /> : null}
          </div>
          {nextRetryAfter ? <p className="mt-1.5 text-[9px] leading-3 text-muted-foreground">Eligibility time only, not a recovery guarantee.</p> : null}
        </div>
      ) : null}

      {hasProbeWindow ? (
        <div className={cn(hasAuthFileEvidence && "mt-2 border-t border-border/60 pt-2")}>
          <div className="mb-1.5 flex items-center gap-1.5 text-[10px] font-medium text-foreground/70">
            <Gauge className="h-3.5 w-3.5 text-terracotta-600 dark:text-terracotta-300" aria-hidden="true" />
            <span>Capacity probe evidence</span>
          </div>
          <div className={cn("grid items-center gap-2", hasBothProbeEndpoints ? "grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)]" : "grid-cols-1")}>
            {observedAt ? <TimingEndpoint icon={Eye} label="Observed" value={observedAt} /> : null}
            {hasBothProbeEndpoints ? <TimingConnector /> : null}
            {expiresAt ? <TimingEndpoint icon={Hourglass} label="Cache expires" value={expiresAt} align={observedAt ? "end" : "start"} stale={cacheStale} /> : null}
          </div>
        </div>
      ) : null}

      {hasActiveWindow ? (
        <div className={cn((hasAuthFileEvidence || hasProbeWindow) && "mt-2 border-t border-border/60 pt-2")}>
          {activeRange ? (
            <>
              <div className="flex items-center gap-1.5 text-[10px] font-medium text-foreground/70">
                <CalendarRange className="h-3.5 w-3.5 text-terracotta-600 dark:text-terracotta-300" aria-hidden="true" />
                <span>Account active</span>
              </div>
              <div className="mt-1.5 grid grid-cols-[minmax(0,1fr)_auto_minmax(0,1fr)] items-center gap-2">
                <TimingEndpoint label="Starts" value={activeRange.start} compact />
                <TimingConnector />
                <TimingEndpoint label="Ends" value={activeRange.until} align="end" compact />
              </div>
            </>
          ) : singleActiveEndpoint ? (
            <div className="flex items-center gap-1.5 text-[10px] text-foreground/70">
              <CalendarRange className="h-3.5 w-3.5 shrink-0 text-terracotta-600 dark:text-terracotta-300" aria-hidden="true" />
              <span className="font-medium">{activeUntil ? "Active until" : "Starts"}</span>
              <time
                className="ml-auto truncate font-medium text-foreground/90"
                dateTime={singleActiveEndpoint}
                title={singleActiveEndpoint}
              >
                {formatDate(singleActiveEndpoint)}
              </time>
            </div>
          ) : null}
        </div>
      ) : null}
    </div>
  )
}

function TimingLine({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex min-w-0 items-center gap-2 text-[10px] text-foreground/70">
      <span className="font-medium">{label}</span>
      <time className="ml-auto truncate font-medium text-foreground/90" dateTime={value} title={value}>{formatDate(value)}</time>
    </div>
  )
}

function TimingEndpoint({
  icon: Icon,
  label,
  value,
  align = "start",
  compact = false,
  stale = false,
}: {
  icon?: LucideIcon
  label: string
  value: string
  align?: "start" | "end"
  compact?: boolean
  stale?: boolean
}) {
  return (
    <div className={cn("min-w-0", align === "end" && "text-right")}>
      <div
        className={cn(
          "flex items-center gap-1.5 text-[10px] text-foreground/65",
          align === "end" && "justify-end",
        )}
      >
        {Icon ? (
          <Icon
            className={cn("h-3.5 w-3.5", stale ? "text-amber-600 dark:text-amber-400" : "text-terracotta-600 dark:text-terracotta-300")}
            aria-hidden="true"
          />
        ) : null}
        <span>{label}</span>
        {stale ? (
          <span className="rounded-full bg-amber-500/15 px-1.5 text-[9px] font-semibold uppercase tracking-wide text-amber-700 dark:text-amber-400">
            Stale
          </span>
        ) : null}
      </div>
      <div
        className={cn(
          "mt-0.5 truncate font-medium",
          compact ? "text-[10px]" : "text-[11px]",
          stale ? "text-amber-700 dark:text-amber-300" : "text-foreground/90",
        )}
      >
        <time dateTime={value} title={value}>{formatDate(value)}</time>
      </div>
    </div>
  )
}

function TimingConnector() {
  return (
    <div className="flex w-7 items-center text-muted-foreground/55" aria-hidden="true">
      <span className="h-px min-w-0 flex-1 bg-muted-foreground/35" />
      <ArrowRight className="h-3 w-3 shrink-0 -ml-px" />
    </div>
  )
}

/** Long auth indexes truncate to an 8-char prefix; the full value stays on title/copy. */
function formatAuthIndex(authIndex: string): string {
  return authIndex.length > 12 ? `${authIndex.slice(0, 8)}…` : authIndex
}

function PlanBadge({
  label,
  tone,
  rawPlanType,
}: {
  label: string
  tone: LiveCapacityPlanTone
  rawPlanType: string
}) {
  return (
    <Badge
      variant={tone === "priority" ? "terracotta" : "secondary"}
      className={cn(
        "shrink-0 px-1.5 py-0 text-[10px] leading-4",
        tone === "ordinary" && "border-border/70 bg-muted text-muted-foreground",
      )}
      title={rawPlanType}
    >
      {label}
    </Badge>
  )
}

function MetricMeter({
  title,
  metric,
  resetPrefix = "reset",
}: {
  title: string
  metric?: LiveCapacityMetric
  resetPrefix?: string
}) {
  const progress = metric?.progress ?? null
  const resetLabel = metric?.resetLabel ?? "-"
  const resetText = resetLabel === "-" ? "-" : `${resetPrefix} ${resetLabel}`
  const WindowIcon = metric?.windowSeconds === FIVE_HOUR_WINDOW_SECONDS
    ? Timer
    : metric?.windowSeconds === WEEKLY_WINDOW_SECONDS
      ? CalendarDays
      : null

  return (
    <div className="min-w-0 rounded-md border border-border/70 bg-muted/20 p-2">
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
          {WindowIcon ? <WindowIcon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" /> : null}
          <span className="truncate">{title}</span>
        </span>
        <span className="truncate font-medium">{metric?.valueLabel ?? "-"}</span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
        aria-label={metric ? `${title}: ${metric.valueLabel}` : `${title}: no capacity reading`}
      >
        {progress !== null ? (
          <div
            className={cn("h-full rounded-full transition-[width,background-color] duration-500", metricToneClasses(metric?.tone).bar)}
            style={{ width: `${progress}%` }}
          />
        ) : null}
      </div>
      <div
        className={cn("mt-1.5 flex min-w-0 items-center gap-1 text-[11px]", metricToneClasses(metric?.tone).reset)}
        title={resetText}
      >
        <Clock className="h-3 w-3 shrink-0" aria-hidden="true" />
        <span className="truncate">{resetText}</span>
      </div>
    </div>
  )
}

const METRIC_TONE_CLASSES: Record<LiveCapacityMetric["tone"], { bar: string; reset: string }> = {
  red: { bar: "bg-red-500", reset: "text-red-600 dark:text-red-400" },
  amber: { bar: "bg-amber-500", reset: "text-amber-600 dark:text-amber-400" },
  green: { bar: "bg-emerald-500", reset: "text-muted-foreground" },
  muted: { bar: "bg-muted-foreground/40", reset: "text-muted-foreground" },
}

function metricToneClasses(tone: LiveCapacityMetric["tone"] | undefined): { bar: string; reset: string } {
  return (tone ? METRIC_TONE_CLASSES[tone] : undefined) ?? { bar: "bg-muted-foreground/30", reset: "text-muted-foreground" }
}
