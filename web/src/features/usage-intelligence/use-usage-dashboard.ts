import { useCallback, useEffect, useMemo, useState } from "react"
import { useAnalyticsCore, useAnalyticsHeatmap } from "@/hooks/useAnalytics"
import { useEvents } from "@/hooks/useEvents"
import { useRequestHealth } from "@/hooks/useRequestHealth"
import { useFailureDistribution } from "@/hooks/useFailureDistribution"
import { useModelMappings, useModelMappingsSummary } from "@/hooks/useModelMappings"
import { useAttemptPerformance } from "@/hooks/useAttemptPerformance"
import { usePerformanceProviders } from "@/hooks/usePerformanceProviders"
import type { AnalyticsCoreResponse, TimeGranularity, TimeRange, UsageAttemptPerformance, UsageEventsPage, UsageFailureDistribution, UsageModelMappingDistribution } from "@/types/api"
import type { UsageModelMappingSummary } from "./model-mapping-summary"
import { buildUsageIntelligenceLoadPlan, type UsageIntelligenceLoadPlan } from "./load-plan"
import { useVisibilityRefresh } from "./refresh"
import { buildUsageDashboardSurfaces, type UsageDashboardSurfaces } from "./surfaces"
import {
  buildUsageDashboardViewModel,
  getEffectiveGranularity,
  resolveStoredTimeRange,
  SELECTED_TIME_RANGE_STORAGE_KEY,
  type LeaderboardScope,
  type TrendView,
  type UsageDashboardViewModel,
} from "./view-model"

export interface UseUsageDashboardResult {
  range: TimeRange
  selectRange: (range: TimeRange) => void
  granularity: TimeGranularity | null
  setGranularity: (granularity: TimeGranularity | null) => void
  provider: string
  setProvider: (provider: string) => void
  trendView: TrendView
  setTrendView: (trendView: TrendView) => void
  leaderboardScope: LeaderboardScope
  setLeaderboardScope: (scope: LeaderboardScope) => void
  effectiveGranularity: TimeGranularity
  loadPlan: UsageIntelligenceLoadPlan
  viewModel: UsageDashboardViewModel
  surfaces: UsageDashboardSurfaces
  coreAnalyticsData?: AnalyticsCoreResponse
  requestEvidenceData?: UsageEventsPage
  isRequestEvidenceLoading: boolean
  isRequestEvidenceRefreshing: boolean
  requestEvidenceError: unknown
  failureDistributionData?: UsageFailureDistribution
  isFailureDistributionLoading: boolean
  failureDistributionError: unknown
  modelMappingsData?: UsageModelMappingDistribution
  modelMappingsSummaryData?: UsageModelMappingSummary
  isModelMappingsSummaryLoading: boolean
  modelMappingsSummaryError: unknown
  isModelMappingDetailsLoading: boolean
  modelMappingDetailsError: unknown
  setModelMappingsExpanded: (expanded: boolean) => void
  attemptPerformanceProvider: string
  attemptPerformanceProviders: string[]
  setAttemptPerformanceProvider: (provider: string) => void
  performanceProvidersError: unknown
  retryPerformanceProviders: () => void
  attemptPerformanceData?: UsageAttemptPerformance
  isAttemptPerformanceLoading: boolean
  attemptPerformanceError: unknown
  retryCore: () => void
  retryHeatmap: () => void
  retryRequestHealth: () => void
  retryRequestEvidence: () => void
  retryFailureDistribution: () => void
  retryModelMappingsSummary: () => void
  retryModelMappingDetails: () => void
  retryAttemptPerformance: () => void
  refreshDashboard: () => void
}

export function readStoredTimeRange(): TimeRange {
  if (typeof window === "undefined") {
    return resolveStoredTimeRange(null)
  }
  try {
    return resolveStoredTimeRange(window.localStorage.getItem(SELECTED_TIME_RANGE_STORAGE_KEY))
  } catch {
    return resolveStoredTimeRange(null)
  }
}

export function writeStoredTimeRange(range: TimeRange) {
  try {
    window.localStorage.setItem(SELECTED_TIME_RANGE_STORAGE_KEY, range)
  } catch {
    // Browser storage can be unavailable in private or restricted contexts.
  }
}

/**
 * Owns the Usage Intelligence dashboard page state: selected window controls,
 * fixed operational window queries, view-model derivation, and refresh policy.
 * Route files consume the returned state and setters for composition only.
 */
export function useUsageDashboard(): UseUsageDashboardResult {
  const [range, setRange] = useState<TimeRange>(readStoredTimeRange)
  const [granularity, setGranularity] = useState<TimeGranularity | null>(null)
  const [provider, setProviderState] = useState("")
  const [selectedPerformanceProvider, setAttemptPerformanceProvider] = useState<string | null>(null)
  const [modelMappingsExpanded, setModelMappingsExpandedState] = useState(false)
  const [modelMappingDetailScope, setModelMappingDetailScope] = useState<{
    provider: string
    windowEnd: string
    summary: UsageModelMappingSummary
  } | null>(null)
  const [trendView, setTrendView] = useState<TrendView>("cost-token")
  const [leaderboardScope, setLeaderboardScope] = useState<LeaderboardScope>("api-key")

  useEffect(() => {
    writeStoredTimeRange(range)
  }, [range])

  const selectRange = useCallback((next: TimeRange) => {
    setRange(next)
    setGranularity(null)
  }, [])

  const setProvider = useCallback((nextProvider: string) => {
    setProviderState(nextProvider)
    setModelMappingsExpandedState(false)
    setModelMappingDetailScope(null)
  }, [])

  const performanceOptions = usePerformanceProviders()
  const attemptPerformanceProviders = [...(performanceOptions.data?.provider_options ?? [])]
    .filter((option) => option.provider !== "")
    .sort((a, b) => b.request_count - a.request_count || a.provider.localeCompare(b.provider))
    .map((option) => option.provider)
  const attemptPerformanceProvider = selectedPerformanceProvider ?? attemptPerformanceProviders[0] ?? ""
  if (selectedPerformanceProvider === null && attemptPerformanceProvider) {
    setAttemptPerformanceProvider(attemptPerformanceProvider)
  }
  // Keep an explicit choice available even when its last-24h activity disappears.
  if (attemptPerformanceProvider && !attemptPerformanceProviders.includes(attemptPerformanceProvider)) {
    attemptPerformanceProviders.push(attemptPerformanceProvider)
  }

  const effectiveGranularity = getEffectiveGranularity(range, granularity)
  const loadPlan = useMemo(
    () => buildUsageIntelligenceLoadPlan({ range, granularity: effectiveGranularity, provider, attemptPerformanceProvider }),
    [range, effectiveGranularity, provider, attemptPerformanceProvider],
  )
  const selectedAnalytics = loadPlan.selectedWindow.analytics
  const fixedWindow = loadPlan.fixedWindow

  const {
    data: heatmapData,
    isLoading: isHeatmapLoading,
    refetch: refetchHeatmap,
    error: heatmapError,
  } = useAnalyticsHeatmap(fixedWindow.heatmap.range, fixedWindow.heatmap.granularity, fixedWindow.heatmap.provider)
  const {
    data: coreAnalyticsData,
    isLoading: isCoreAnalyticsLoading,
    refetch: refetchCoreAnalytics,
    error: coreAnalyticsError,
  } = useAnalyticsCore(selectedAnalytics.range, selectedAnalytics.granularity, selectedAnalytics.provider, false)
  const {
    data: requestEvidenceData,
    isLoading: isRequestEvidenceLoading,
    isFetching: isRequestEvidenceFetching,
    refetch: refetchRequestEvidence,
    error: requestEvidenceError,
  } = useEvents(
    fixedWindow.requestEvidence.range,
    fixedWindow.requestEvidence.pageSize,
    fixedWindow.requestEvidence.provider,
    1,
    false,
  )
  const {
    data: requestHealthData,
    isLoading: isRequestHealthLoading,
    refetch: refetchRequestHealth,
    error: requestHealthError,
  } = useRequestHealth(fixedWindow.requestHealth.range, fixedWindow.requestHealth.provider)
  const {
    data: failureDistributionData,
    isLoading: isFailureDistributionLoading,
    refetch: refetchFailureDistribution,
    error: failureDistributionError,
  } = useFailureDistribution(fixedWindow.failureDistribution.provider)
  const {
    data: latestModelMappingsSummaryData,
    isLoading: isModelMappingsSummaryLoading,
    refetch: refetchModelMappingsSummary,
    error: modelMappingsSummaryError,
  } = useModelMappingsSummary(fixedWindow.modelMappings.provider)
  const modelMappingDetailScopeMatches = modelMappingDetailScope?.provider === fixedWindow.modelMappings.provider
  const {
    data: scopedModelMappingsData,
    isLoading: isModelMappingDetailsLoading,
    refetch: refetchModelMappings,
    error: modelMappingDetailsError,
  } = useModelMappings(
    fixedWindow.modelMappings.provider,
    modelMappingsExpanded && Boolean(modelMappingDetailScopeMatches),
    modelMappingDetailScopeMatches ? modelMappingDetailScope.windowEnd : "",
  )
  const modelMappingsData = modelMappingDetailScopeMatches ? scopedModelMappingsData : undefined
  const setModelMappingsExpanded = useCallback((expanded: boolean) => {
    setModelMappingsExpandedState(expanded)
    if (!expanded || !latestModelMappingsSummaryData?.window_end) return
    setModelMappingDetailScope({
      provider: fixedWindow.modelMappings.provider,
      windowEnd: latestModelMappingsSummaryData.window_end,
      summary: latestModelMappingsSummaryData,
    })
  }, [fixedWindow.modelMappings.provider, latestModelMappingsSummaryData])
  const modelMappingsSummaryData = modelMappingsExpanded && modelMappingDetailScopeMatches
    ? modelMappingDetailScope.summary
    : latestModelMappingsSummaryData
  const {
    data: attemptPerformanceData,
    isLoading: isScopedPerformanceLoading,
    refetch: refetchAttemptPerformance,
    error: scopedPerformanceError,
  } = useAttemptPerformance(fixedWindow.attemptPerformance.provider, Boolean(attemptPerformanceProvider))

  const isAttemptPerformanceLoading = attemptPerformanceProvider ? isScopedPerformanceLoading : performanceOptions.isLoading
  const attemptPerformanceError = attemptPerformanceProvider ? scopedPerformanceError : performanceOptions.error
  const refetchPerformanceOptions = performanceOptions.refetch

  const refreshDashboard = useCallback(() => {
    const queries: Promise<unknown>[] = [
      refetchCoreAnalytics({ cancelRefetch: false }),
      refetchRequestEvidence({ cancelRefetch: false }),
      refetchFailureDistribution({ cancelRefetch: false }),
      refetchModelMappingsSummary({ cancelRefetch: false }),
      refetchPerformanceOptions({ cancelRefetch: false }),
    ]
    if (attemptPerformanceProvider) queries.push(refetchAttemptPerformance({ cancelRefetch: false }))
    void Promise.allSettled(queries)
  }, [refetchCoreAnalytics, refetchRequestEvidence, refetchFailureDistribution, refetchModelMappingsSummary, refetchAttemptPerformance, refetchPerformanceOptions, attemptPerformanceProvider])
  useVisibilityRefresh(refreshDashboard)

  const viewModel = useMemo(
    () =>
      buildUsageDashboardViewModel({
        analytics: coreAnalyticsData,
        fixedHeatmap: heatmapData?.heatmap,
        requestHealth: requestHealthData,
        leaderboardScope,
      }),
    [coreAnalyticsData, heatmapData, requestHealthData, leaderboardScope],
  )
  const surfaces = useMemo(
    () =>
      buildUsageDashboardSurfaces({
        viewModel,
        core: { data: coreAnalyticsData, isLoading: isCoreAnalyticsLoading, error: coreAnalyticsError },
        heatmap: { isLoading: isHeatmapLoading, error: heatmapError },
        requestHealth: { isLoading: isRequestHealthLoading, error: requestHealthError },
      }),
    [
      viewModel,
      coreAnalyticsData,
      isCoreAnalyticsLoading,
      coreAnalyticsError,
      isHeatmapLoading,
      heatmapError,
      isRequestHealthLoading,
      requestHealthError,
    ],
  )

  return {
    range,
    selectRange,
    granularity,
    setGranularity,
    provider,
    setProvider,
    trendView,
    setTrendView,
    leaderboardScope,
    setLeaderboardScope,
    effectiveGranularity,
    loadPlan,
    viewModel,
    surfaces,
    coreAnalyticsData,
    requestEvidenceData,
    isRequestEvidenceLoading,
    isRequestEvidenceRefreshing: isRequestEvidenceFetching && Boolean(requestEvidenceData),
    requestEvidenceError,
    failureDistributionData,
    isFailureDistributionLoading,
    failureDistributionError,
    modelMappingsData,
    modelMappingsSummaryData,
    isModelMappingsSummaryLoading,
    modelMappingsSummaryError,
    isModelMappingDetailsLoading,
    modelMappingDetailsError,
    setModelMappingsExpanded,
    attemptPerformanceProvider,
    attemptPerformanceProviders,
    setAttemptPerformanceProvider,
    performanceProvidersError: performanceOptions.error,
    retryPerformanceProviders: () => { void refetchPerformanceOptions() },
    attemptPerformanceData: attemptPerformanceProvider ? attemptPerformanceData : undefined,
    isAttemptPerformanceLoading,
    attemptPerformanceError,
    retryCore: () => {
      void refetchCoreAnalytics()
    },
    retryHeatmap: () => {
      void refetchHeatmap()
    },
    retryRequestHealth: () => {
      void refetchRequestHealth()
    },
    retryRequestEvidence: () => {
      void refetchRequestEvidence()
    },
    retryFailureDistribution: () => {
      void refetchFailureDistribution()
    },
    retryModelMappingsSummary: () => {
      void refetchModelMappingsSummary({ cancelRefetch: false })
    },
    retryModelMappingDetails: () => {
      if (modelMappingsExpanded && modelMappingDetailScopeMatches) void refetchModelMappings({ cancelRefetch: false })
    },
    retryAttemptPerformance: () => {
      if (attemptPerformanceProvider) void refetchAttemptPerformance()
      else void refetchPerformanceOptions()
    },
    refreshDashboard,
  }
}
