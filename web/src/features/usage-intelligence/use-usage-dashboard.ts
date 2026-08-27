import { useCallback, useEffect, useMemo, useState } from "react"
import { useAnalyticsCore, useAnalyticsHeatmap } from "@/hooks/useAnalytics"
import { useEvents } from "@/hooks/useEvents"
import { useRequestHealth } from "@/hooks/useRequestHealth"
import type { AnalyticsCoreResponse, TimeGranularity, TimeRange, UsageEventsPage } from "@/types/api"
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
  retryCore: () => void
  retryHeatmap: () => void
  retryRequestHealth: () => void
  retryRequestEvidence: () => void
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
  const [provider, setProvider] = useState("")
  const [trendView, setTrendView] = useState<TrendView>("cost-token")
  const [leaderboardScope, setLeaderboardScope] = useState<LeaderboardScope>("api-key")

  useEffect(() => {
    writeStoredTimeRange(range)
  }, [range])

  const selectRange = useCallback((next: TimeRange) => {
    setRange(next)
    setGranularity(null)
  }, [])

  const effectiveGranularity = getEffectiveGranularity(range, granularity)
  const loadPlan = useMemo(
    () => buildUsageIntelligenceLoadPlan({ range, granularity: effectiveGranularity, provider }),
    [range, effectiveGranularity, provider],
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

  const refreshDashboard = useCallback(() => {
    void Promise.allSettled([refetchCoreAnalytics(), refetchRequestEvidence()])
  }, [refetchCoreAnalytics, refetchRequestEvidence])
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
    refreshDashboard,
  }
}
