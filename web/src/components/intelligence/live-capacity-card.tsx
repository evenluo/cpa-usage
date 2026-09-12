import {
  AlertTriangle,
  CalendarDays,
  Clock,
  Eye,
  Gauge,
  Loader2,
  ListChecks,
  Power,
  RefreshCw,
  Timer,
} from "lucide-react"
import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  buildLiveCapacityRows,
  capacityLayout,
  FIVE_HOUR_WINDOW_SECONDS,
  mergeCapacityEntries,
  mergeLiveCapacityRowOrder,
  orderLiveCapacityRows,
  resetCountdown,
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
import { formatDate, formatRelativeAge } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { AccountModelSupport, ModelCapability, ModelSupportResponse, RegisteredModelSupport } from "@/types/api"
import { ProviderBrandIcon } from "./provider-brand-icon"

export function LiveCapacityCard({ provider }: { provider: string }) {
  const cardRef = useRef<HTMLDivElement>(null)
  const [isActivated, setIsActivated] = useState(() => typeof IntersectionObserver === "undefined")
  useEffect(() => {
    if (isActivated || !cardRef.current || typeof IntersectionObserver === "undefined") return
    const observer = new IntersectionObserver((entries) => {
      if (!entries.some((entry) => entry.isIntersecting)) return
      setIsActivated(true)
      observer.disconnect()
    }, { rootMargin: "240px 0px" })
    observer.observe(cardRef.current)
    return () => observer.disconnect()
  }, [isActivated])

  const { identities, observations, taskStates, refresh, refreshLimit, isLoading, isRefreshing, error } = useLiveCapacity(provider, isActivated)
  const modelSupport = useModelSupport()
  const derivedRows = useMemo(
    () => buildLiveCapacityRows({ identities, observations, taskStates }),
    [identities, observations, taskStates],
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
  const [supportOpen, setSupportOpen] = useState(false)
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
  const previousSupportProvider = useRef(provider)

  const resetSupportSelection = useCallback(() => {
    setSelectedSupportIDs(new Set())
    setLoadedSupportScopeKey("")
    setRequestedSupportScopeKey("")
    modelSupport.reset()
  }, [modelSupport])
  useEffect(() => {
    if (previousSupportProvider.current === provider) return
    previousSupportProvider.current = provider
    resetSupportSelection()
  }, [provider, resetSupportSelection])
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
    <Card ref={cardRef}>
      <CardHeader className="flex flex-col items-start justify-between gap-3 pb-3 sm:flex-row sm:items-center">
        <div>
          <CardTitle className="flex items-center gap-2">
            Live Capacity
            <Gauge className="h-3.5 w-3.5 text-muted-foreground/40" aria-label="Live quotas" />
          </CardTitle>
          <CardDescription>Current quotas</CardDescription>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          <Badge variant="blue">Live</Badge>
          {displayedCount > refreshLimit ? <Badge variant="amber">max {refreshLimit}</Badge> : null}
          <Button type="button" variant="outline" size="sm" onClick={refreshDisplayed} disabled={displayedCount === 0}>
            <RefreshCw className={cn("mr-1.5 h-3.5 w-3.5", isRefreshing && "animate-spin")} />
            {refreshLabel}
          </Button>
        </div>
      </CardHeader>
      <CardContent>
        {!isActivated || isLoading ? (
          <div className="space-y-2">
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
            <Skeleton className="h-14 w-full" />
          </div>
        ) : error ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-red-500">
            Couldn't load live capacity
          </div>
        ) : identities.length === 0 ? (
          <div className="flex h-[140px] items-center justify-center rounded-lg border border-dashed border-border text-sm text-muted-foreground">
            No accounts
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
            <details
              className="rounded-md border border-border/70 bg-muted/[0.12]"
              onToggle={(event) => setSupportOpen(event.currentTarget.open)}
            >
              <summary className="flex cursor-pointer list-none flex-wrap items-center justify-between gap-2 p-2.5 text-xs [&::-webkit-details-marker]:hidden">
                <span className="flex items-center gap-1.5 font-medium text-foreground/70">
                  <ListChecks className="h-3.5 w-3.5" aria-hidden="true" />
                  Model support
                </span>
                <span className={cn("text-muted-foreground", selectionTooLarge && "text-amber-700 dark:text-amber-300")}>
                  {selectedSupportIdentityIDs.length}/{MODEL_SUPPORT_MAX_ACCOUNTS} accounts selected
                </span>
              </summary>
              <div className="space-y-2 border-t border-border/60 p-2.5">
                <div className="flex flex-wrap items-center justify-end gap-2 text-xs">
                  <div className="flex items-center gap-2">
                    <Button
                      type="button"
                      variant="ghost"
                      size="sm"
                      onClick={toggleDisplayedSelection}
                      disabled={modelSupport.isPending || (!displayedSelected && displayedIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS)}
                    >
                      {displayedSelected ? "Clear selection" : displayedIdentityIDs.length > MODEL_SUPPORT_MAX_ACCOUNTS ? `Choose up to ${MODEL_SUPPORT_MAX_ACCOUNTS}` : "Select displayed"}
                    </Button>
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
                  </div>
                </div>
                {selectionTooLarge ? (
                  <div className="rounded-md border border-amber-500/30 bg-amber-500/[0.05] p-2.5 text-xs text-amber-700 dark:text-amber-300" role="alert">
                    Select {MODEL_SUPPORT_MAX_ACCOUNTS} accounts or fewer.
                  </div>
                ) : null}
                {modelSupport.isPending ? (
                  <Skeleton className="h-20 w-full" />
                ) : modelSupport.isError && requestedSupportScopeKey === selectedSupportScopeKey ? (
                  <div className="rounded-md border border-red-500/25 bg-red-500/[0.025] p-3 text-sm text-red-600" role="alert">
                    Couldn't load model support
                  </div>
                ) : loadedModelSupport ? (
                  <ModelSupportCoveragePanel result={loadedModelSupport} />
                ) : null}
              </div>
            </details>
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
                      supportSelectionVisible={supportOpen}
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
                        supportSelectionVisible={supportOpen}
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
  supportSelectionVisible,
  modelSupport,
  supportSelectionDisabled,
}: {
  row: LiveCapacityRow
  onRefresh: () => void
  selectedForSupport: boolean
  onToggleSupportSelection: () => void
  supportSelectionVisible: boolean
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
          onError: (error) => toast.error(error instanceof Error ? error.message : "Failed to update account"),
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
        onError: (error) => toast.error(error instanceof Error ? error.message : "Failed to update account"),
      },
    )
  }

  const copyAuthIndex = async () => {
    try {
      await navigator.clipboard.writeText(row.authIndex)
      toast.success("Copied")
    } catch {
      toast.error("Failed to copy auth index")
    }
  }

  const isRowRefreshing = row.status === "refreshing"
  const hasAttention = row.isConstrained || row.status === "failed" || row.unavailable === true || row.accountState.kind === "error"
  const accountTitle = row.alias || row.displayName || row.name || row.authIndex
  const attentionLabel = row.status === "failed"
    ? `Refresh failed: ${row.errorLabel ?? "Failed"}`
    : row.unavailable === true
      ? "Unavailable in CPA"
      : row.accountState.kind === "error"
        ? "CPA error"
        : row.isConstrained
          ? "Capacity constrained"
          : undefined

  const capacityEntries = mergeCapacityEntries(row)
  const layout = capacityLayout(capacityEntries, row.providerKind)
  const visibleModelSnapshots = row.providerKind === "codex" ? [] : row.passiveModelQuotas
  const sharedObservation = Boolean(row.metadataObservedAt && row.observedAt && row.metadataObservedAt === row.observedAt)
  const timingLineCount =
    (sharedObservation ? 1 : (row.metadataObservedAt ? 1 : 0) + (row.observedAt ? 1 : 0)) +
    (row.lastRefresh ? 1 : 0) +
    (row.nextRetryAfter ? 1 : 0) +
    (row.activeStart ? 1 : 0) +
    (row.activeUntil ? 1 : 0)
  const foldedCount =
    layout.extras.length +
    visibleModelSnapshots.length +
    timingLineCount +
    (row.passiveQuota?.activeLimit ? 1 : 0)
  const observationSources: Array<[string, string]> = [
    ...(row.observedAt ? [["Manual probe", row.observedAt] as [string, string]] : []),
    ...(row.passiveQuota?.observedAt ? [["Reported by CPA", row.passiveQuota.observedAt] as [string, string]] : []),
    ...visibleModelSnapshots.map((observation) => [
      `Reported by CPA (${observation.model})`,
      observation.observedAt,
    ] as [string, string]),
  ]
  const latestObservedAt = observationSources.reduce<string | undefined>(
    (latest, [, at]) => (latest === undefined || Date.parse(at) > Date.parse(latest) ? at : latest),
    undefined,
  )

  return (
    <div
      className={cn(
        "group flex min-h-[190px] min-w-0 flex-col rounded-lg border border-border bg-background/70 p-3 text-sm transition-[background-color,border-color,box-shadow] duration-300 hover:border-terracotta-500/25 hover:shadow-xs",
        row.disabled && "opacity-60",
        row.status === "failed" && "border-red-500/25 bg-red-500/[0.025]",
        row.status !== "failed" && row.isConstrained && "border-amber-500/30 bg-amber-500/[0.03]",
        row.status !== "failed" && !row.isConstrained && row.isPriorityAccount && "border-terracotta-500/30 shadow-[inset_2px_0_0_rgba(192,80,62,0.45)]",
        isRowRefreshing && "border-amber-500/25 shadow-[0_0_0_1px_rgba(245,158,11,0.08)]",
      )}
    >
      {/* 不再渲染状态 chip：额度观测由读数本身表达，refreshing 由刷新按钮动画表达，
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
            title={`Copy ${row.authIndex}`}
            aria-label={`Copy auth index ${row.authIndex}`}
            data-auth-index={row.authIndex}
            onClick={copyAuthIndex}
          >
            {formatAuthIndex(row.authIndex)}
          </button>
          {supportSelectionVisible ? (
            <label className="mt-1 flex w-fit cursor-pointer items-center gap-1.5 text-[10px] text-muted-foreground">
              <input
                type="checkbox"
                className="h-3.5 w-3.5 accent-terracotta-600"
                checked={selectedForSupport}
                onChange={onToggleSupportSelection}
                disabled={supportSelectionDisabled}
                aria-label={`Include ${accountTitle} in model support coverage`}
              />
              Include
            </label>
          ) : null}
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
            Disable?
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
          >
            {setIdentityDisabled.isPending ? (
              <Loader2 className="h-3.5 w-3.5 animate-spin" />
            ) : (
              <Power className="h-3.5 w-3.5" />
            )}
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          size="icon"
          className="h-7 w-7 opacity-70 transition-opacity group-hover:opacity-100"
          onClick={onRefresh}
          disabled={isRowRefreshing || setIdentityDisabled.isPending}
          aria-label={`Refresh ${accountTitle}`}
        >
          <RefreshCw className={cn("h-3.5 w-3.5", isRowRefreshing && "animate-spin")} />
        </Button>
        </div>
      </div>

      {layout.hasWindowSkeleton ? (
        <div className="mt-3 grid gap-2">
          <MetricMeter
            title="5h"
            metric={layout.baseShort?.metric}
            source={layout.baseShort?.source}
            observedAt={layout.baseShort?.observedAt}
            windowSeconds={FIVE_HOUR_WINDOW_SECONDS}
          />
          <MetricMeter
            title="Weekly"
            metric={layout.baseLong?.metric}
            source={layout.baseLong?.source}
            observedAt={layout.baseLong?.observedAt}
            windowSeconds={WEEKLY_WINDOW_SECONDS}
          />
        </div>
      ) : layout.main.length > 0 ? (
        <div className="mt-3 grid gap-2">
          {layout.main.map((entry, index) => (
            <MetricMeter
              key={`${index}:${entry.metric.label}`}
              title={entry.metric.label}
              metric={entry.metric}
              source={entry.source}
              observedAt={entry.observedAt}
            />
          ))}
        </div>
      ) : row.providerKind === "codex" && capacityEntries.length === 0 ? (
        <div className="mt-3 rounded-md border border-border/70 bg-muted/20 p-2 text-xs text-muted-foreground">No account quota readings</div>
      ) : null}

      {row.providerKind === "codex" && layout.reserve.length > 0 ? (
        <section className="mt-2" aria-label="Luna Reserve">
          {layout.reserve.map((entry, index) => (
            <MetricMeter
              key={`${index}:${entry.metric.label}`}
              title="Luna Reserve"
              metric={entry.metric}
              source={entry.source}
              observedAt={entry.observedAt}
            />
          ))}
        </section>
      ) : null}

      {foldedCount > 0 ? (
        <details className="mt-2 rounded-md border border-border/70 bg-muted/[0.12] px-2.5 py-1.5">
          <summary className="cursor-pointer text-[10px] font-medium text-muted-foreground">··· {foldedCount} more</summary>
          <div className="mt-2 space-y-3">
            {row.passiveQuota?.activeLimit ? (
              <p
                className="text-[10px] text-muted-foreground"
                title={`Reported by CPA · observed ${formatDate(row.passiveQuota.observedAt)}`}
              >
                Active limit {row.passiveQuota.activeLimit}
              </p>
            ) : null}
            {layout.extras.length > 0 ? (
              <section aria-label="More limits">
                <p className="text-[10px] font-medium text-foreground/70">More limits ({layout.extras.length})</p>
                <div className="mt-1.5 grid gap-1.5">
                  {layout.extras.map((entry, index) => (
                    <MetricMeter
                      key={`${index}:${entry.metric.label}`}
                      title={entry.metric.label}
                      metric={entry.metric}
                      source={entry.source}
                      observedAt={entry.observedAt}
                    />
                  ))}
                </div>
              </section>
            ) : null}
            {visibleModelSnapshots.length > 0 ? (
              <section aria-label="Model request observations">
                <p className="text-[10px] font-medium text-foreground/70">Model request observations ({visibleModelSnapshots.length})</p>
                <p className="mt-1 text-[10px] leading-4 text-muted-foreground">
                  From requests to this model, not a separate quota.
                </p>
                <div className="mt-1.5 space-y-3">
                  {visibleModelSnapshots.map((observation) => (
                    <PassiveQuotaObservationSection key={`${observation.model}:${observation.observedAt}`} label={observation.model} observation={observation} />
                  ))}
                </div>
              </section>
            ) : null}
            {timingLineCount > 0 ? (
              <AccountTiming
                metadataObservedAt={row.metadataObservedAt}
                lastRefresh={row.lastRefresh}
                nextRetryAfter={row.nextRetryAfter}
                observedAt={row.observedAt}
                activeStart={row.activeStart}
                activeUntil={row.activeUntil}
              />
            ) : null}
          </div>
        </details>
      ) : null}

      <AccountAvailabilitySummary row={row} />

      {modelSupport ? <AccountModelSupportDetails account={modelSupport} /> : null}

      {latestObservedAt ? (
        <div
          className="mt-2 flex items-center justify-end gap-1 text-[10px] text-muted-foreground"
          title={observationSources.map(([label, at]) => `${label} · ${formatDate(at)}`).join("\n")}
        >
          <span>Last updated {formatRelativeAge(latestObservedAt)}</span>
        </div>
      ) : null}
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
          {result.scope_complete ? "Complete" : "Partial"}
        </Badge>
        <span className="text-xs text-muted-foreground">{result.loaded_count}/{result.selected_count} accounts loaded</span>
      </div>
      {result.models.length > 0 ? (
        <div className="mt-3 flex flex-wrap gap-2">
          {result.models.map((model) => (
            <span key={model.model_id} className="rounded-md border border-border/70 bg-muted/20 px-2 py-1 text-xs" title={model.model_id}>
              <span className="font-medium">{model.display_name || model.model_id}</span>
              <span className="ml-1.5 text-muted-foreground">
                {result.scope_complete
                  ? model.single_registered_account_in_scope
                    ? "1 account"
                    : `${model.observed_supporting_accounts}/${model.selected_accounts} accounts`
                  : `${model.observed_supporting_accounts}/${result.loaded_count} loaded`}
              </span>
            </span>
          ))}
        </div>
      ) : (
        <p className="mt-3 text-xs text-muted-foreground">
          {result.scope_complete ? "No models registered." : "No models in the loaded accounts."}
        </p>
      )}
      {failures.length > 0 ? (
        <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/[0.04] p-2.5 text-xs text-amber-700 dark:text-amber-300">
          <p className="font-medium">Load failed — support unknown</p>
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
    <section aria-label={`${label} request observation`}>
      <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1 text-[10px]">
        <span className="truncate font-medium text-foreground/90" title={label}>{label}</span>
        {observation.activeLimit ? (
          <span className="truncate rounded-full bg-muted px-1.5 py-0.5 text-[9px] text-muted-foreground" title={observation.activeLimit}>
            Active limit {observation.activeLimit}
          </span>
        ) : null}
        <time className="ml-auto text-muted-foreground" dateTime={observation.observedAt} title={observation.observedAt}>
          Observed {formatDate(observation.observedAt)}
        </time>
      </div>
      <div className="mt-1.5 grid gap-1.5">
        {observation.metrics.map((metric, index) => (
          <MetricMeter key={`${index}:${metric.label}`} title={metric.label} metric={metric} source="reported" observedAt={observation.observedAt} />
        ))}
      </div>
    </section>
  )
}

function AccountModelSupportDetails({ account }: { account: AccountModelSupport }) {
  if (account.status === "failed") {
    return (
      <div className="mt-3 rounded-md border border-amber-500/25 bg-amber-500/[0.04] p-2.5 text-[11px] text-amber-700 dark:text-amber-300">
        Couldn't load models: {modelSupportErrorLabel(account.error_code)}
      </div>
    )
  }
  return (
    <details className="mt-3 rounded-md border border-border/70 bg-muted/[0.12] p-2.5">
      <summary
        className="cursor-pointer text-[11px] font-medium"
        title="Registered with CPA, not live routing."
      >
        Registered models ({account.registered_models.length})
      </summary>
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
    facts.push(details.length > 0 ? `Thinking: ${details.join("; ")}` : "Thinking")
  }
  return facts.length > 0 ? <p className="mt-1 text-[10px] leading-4 text-muted-foreground">{facts.join(" · ")}</p> : null
}

function modelSupportErrorLabel(code: AccountModelSupport["error_code"]): string {
  if (code === "auth_file_missing") return "Auth file missing in CPA"
  if (code === "invalid_upstream_response") return "Invalid CPA response"
  return "CPA timed out"
}

function definitionStatusLabel(status: RegisteredModelSupport["definition_status"]): string {
  if (status === "available") return "No extra capability data."
  if (status === "absent") return "No catalog entry for this model."
  if (status === "unknown_channel") return "No catalog for this account type."
  return "Couldn't load capability data."
}

function AccountAvailabilitySummary({ row }: { row: LiveCapacityRow }) {
  // Omit a matching disabled lifecycle state already covered by the header badge.
  const showAccountState = row.accountState.kind !== "active"
    && row.accountState.kind !== "not_reported"
    && !(row.disabled && row.accountState.kind === "disabled")
  if (row.unavailable !== true && !showAccountState) return null
  return (
    <div className="mt-3 flex flex-wrap items-center gap-1.5" role="group" aria-label="Account availability">
      {row.unavailable === true ? <Badge variant="red" className="px-1.5 py-0 text-[10px] leading-4">Unavailable</Badge> : null}
      {showAccountState ? (
        <Badge
          variant={accountStateBadgeVariant(row.accountState.tone)}
          className="px-1.5 py-0 text-[10px] leading-4"
          title={row.accountState.explanation}
        >
          CPA: {row.accountState.label}
        </Badge>
      ) : null}
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
  activeStart,
  activeUntil,
}: {
  metadataObservedAt?: string | null
  lastRefresh?: string | null
  nextRetryAfter?: string | null
  observedAt?: string | null
  activeStart?: string | null
  activeUntil?: string | null
}) {
  // activeStart arrives pre-filtered by buildLiveCapacityRows: it is only set
  // while the subscription start is still in the future.
  const sharedObservation = metadataObservedAt && observedAt && metadataObservedAt === observedAt ? observedAt : null

  return (
    <div className="grid gap-1.5" role="group" aria-label="Account and observation timing">
      {sharedObservation ? (
        <TimingLine label="Observed" value={sharedObservation} title="Same time as the quota reading." />
      ) : (
        <>
          {metadataObservedAt ? <TimingLine label="Metadata observed" value={metadataObservedAt} /> : null}
          {observedAt ? <TimingLine label="Observed" value={observedAt} /> : null}
        </>
      )}
      {lastRefresh ? <TimingLine label="Token refreshed" value={lastRefresh} /> : null}
      {nextRetryAfter ? (
        <TimingLine label="Retry eligible" value={nextRetryAfter} title="Eligibility time only, not a recovery guarantee." />
      ) : null}
      {activeStart ? <TimingLine label="Starts" value={activeStart} /> : null}
      {activeUntil ? <TimingLine label="Ends" value={activeUntil} /> : null}
    </div>
  )
}

function TimingLine({
  label,
  value,
  title,
}: {
  label: string
  value: string
  title?: string
}) {
  return (
    <div className="flex min-w-0 items-center gap-2 text-[10px] text-foreground/70" title={title}>
      <span className="font-medium">{label}</span>
      <time
        className="ml-auto truncate font-medium text-foreground/90"
        dateTime={value}
        title={value}
      >
        {formatDate(value)}
      </time>
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
  source = "probe",
  observedAt,
  windowSeconds,
}: {
  title: string
  metric?: LiveCapacityMetric
  /** "reported" marks CPA passive observations, as opposed to manual probe readings. */
  source?: "probe" | "reported"
  /** Observation time anchoring relative reset hints (resetAfterSeconds). */
  observedAt?: string
  /** Window length supplying the slot icon when no reading exists. */
  windowSeconds?: number
}) {
  const WindowIcon = (metric?.windowSeconds ?? windowSeconds) === FIVE_HOUR_WINDOW_SECONDS
    ? Timer
    : (metric?.windowSeconds ?? windowSeconds) === WEEKLY_WINDOW_SECONDS
      ? CalendarDays
      : null

  if (!metric) {
    return (
      <div className="flex min-w-0 items-center gap-1.5 rounded-md border border-border/70 bg-muted/20 p-2 text-xs text-muted-foreground">
        {WindowIcon ? <WindowIcon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" /> : null}
        <span className="truncate">{title}</span>
        <span className="ml-auto shrink-0">No reading</span>
      </div>
    )
  }

  const countdown = resetCountdown(metric, observedAt)
  const resetText = countdown
    ? countdown.isDue
      ? "reset due"
      : `in ${countdown.relativeLabel}`
    : null
  const resetTitle = countdown
    ? [
        resetText,
        countdown.isDue ? "Quota window reset has passed; waiting for the provider's next report" : null,
        countdown.resetAt ? formatDate(countdown.resetAt) : null,
      ].filter((part): part is string => Boolean(part)).join(" · ")
    : undefined
  const sourceLabel = source === "reported" ? "Reported by CPA" : "Manual probe"
  const displayTitle = metric.displayLabel ?? title

  return (
    <div
      className="min-w-0 rounded-md border border-border/70 bg-muted/20 p-2"
      title={observedAt ? `${sourceLabel} · observed ${formatDate(observedAt)}` : sourceLabel}
    >
      <div className="flex items-center justify-between gap-2 text-xs">
        <span className="flex min-w-0 items-center gap-1.5 text-muted-foreground">
          {WindowIcon ? <WindowIcon className="h-3.5 w-3.5 shrink-0" aria-hidden="true" /> : null}
          {source === "reported" ? <Eye className="h-3 w-3 shrink-0" aria-hidden="true" /> : null}
          <span className="truncate" title={metric.displayLabel ? metric.label : undefined}>{displayTitle}</span>
        </span>
        <span className="truncate font-medium">{metric.valueLabel}</span>
      </div>
      <div
        className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
        aria-label={`${displayTitle}: ${metric.valueLabel}`}
      >
        {metric.progress !== null ? (
          <div
            className={cn("h-full rounded-full transition-[width,background-color] duration-500", metricToneClasses(metric.tone).bar)}
            style={{ width: `${metric.progress}%` }}
          />
        ) : null}
      </div>
      {resetText ? (
        <div
          className={cn("mt-1.5 flex min-w-0 items-center gap-1 text-xs font-semibold", metricToneClasses(metric.tone).reset)}
          title={resetTitle}
        >
          <Clock className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
          <span className="truncate">{resetText}</span>
          {countdown?.resetAt ? (
            <time className="ml-auto shrink-0 text-[10px] font-normal text-muted-foreground" dateTime={countdown.resetAt}>
              {formatDate(countdown.resetAt)}
            </time>
          ) : null}
        </div>
      ) : null}
      {metric.description ? <p className="mt-1.5 text-[10px] leading-4 text-muted-foreground">{metric.description}</p> : null}
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
