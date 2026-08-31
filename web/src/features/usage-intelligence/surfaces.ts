import type {
  AnalyticsCoreResponse,
  HeatmapData,
  Insight,
  KeyAliasBreakdown,
  ModelDistribution,
  ServiceHealth,
  TrendPoint,
} from "@/types/api"
import type { UsageDashboardViewModel } from "./view-model"

export type UsageSurfaceStatus = "loading" | "error" | "ready" | "empty"

export type UsageSurface<TData> =
  | { status: "loading"; data: undefined; refreshError?: undefined }
  | { status: "error"; data: undefined; error: unknown; refreshError?: undefined }
  | { status: "ready"; data: TData; refreshError?: unknown }
  | { status: "empty"; data: undefined; refreshError?: unknown }

export interface UsageDashboardSurfaces {
  core: UsageSurface<AnalyticsCoreResponse>
  kpis: { status: UsageSurfaceStatus }
  trend: { status: UsageSurfaceStatus; data: TrendPoint[] }
  leaderboard: { status: UsageSurfaceStatus; data: KeyAliasBreakdown[] }
  modelMix: { status: UsageSurfaceStatus; data: ModelDistribution[] }
  insights: { status: UsageSurfaceStatus; data: Insight[] }
  heatmap: UsageSurface<HeatmapData>
  requestHealth: UsageSurface<ServiceHealth>
}

export interface UsageSurfaceQuery<TData> {
  data: TData | undefined
  isLoading: boolean
  error: unknown
}

/**
 * Shared per-surface invariant: a surface only reports loading or error when
 * there is no cached data to show; stale data keeps the surface ready while a
 * refresh is in flight or has failed.
 */
export function buildSurface<TData>(
  query: UsageSurfaceQuery<TData>,
  isEmpty: (data: TData) => boolean = () => false,
): UsageSurface<TData> {
  if (query.data === undefined) {
    if (query.isLoading) return { status: "loading", data: undefined }
    if (query.error) return { status: "error", data: undefined, error: query.error }
    return { status: "empty", data: undefined }
  }
  if (isEmpty(query.data)) {
    return query.error
      ? { status: "empty", data: undefined, refreshError: query.error }
      : { status: "empty", data: undefined }
  }
  return query.error
    ? { status: "ready", data: query.data, refreshError: query.error }
    : { status: "ready", data: query.data }
}

export function buildUsageDashboardSurfaces(input: {
  viewModel: UsageDashboardViewModel
  core: UsageSurfaceQuery<AnalyticsCoreResponse>
  heatmap: Omit<UsageSurfaceQuery<unknown>, "data">
  requestHealth: Omit<UsageSurfaceQuery<unknown>, "data">
}): UsageDashboardSurfaces {
  const { viewModel } = input
  const coreSurface = buildSurface(input.core, (data) => data.summary.request_count === 0)
  const leaderboardStatus: UsageSurfaceStatus =
    coreSurface.status === "loading" || coreSurface.status === "error" || coreSurface.status === "empty"
      ? coreSurface.status
      : viewModel.hasLeaderboardBreakdown && viewModel.leaderboardRows.length > 0
        ? "ready"
        : "empty"
  const modelMixStatus = buildCoreCollectionStatus(
    coreSurface.status,
    viewModel.hasModelDistribution,
    viewModel.modelDistribution.length,
  )
  const insightsStatus = buildCoreCollectionStatus(
    coreSurface.status,
    viewModel.hasInsights,
    viewModel.insights.length,
  )
  return {
    core: coreSurface,
    kpis: { status: coreSurface.status },
    trend: { status: coreSurface.status, data: viewModel.trend },
    leaderboard: { status: leaderboardStatus, data: viewModel.leaderboardRows },
    modelMix: { status: modelMixStatus, data: viewModel.modelDistribution },
    insights: { status: insightsStatus, data: viewModel.insights },
    heatmap: buildSurface(
      {
        data: viewModel.fixedHeatmap,
        isLoading: input.heatmap.isLoading,
        error: input.heatmap.error,
      },
      (data) => data.rows.length === 0 || data.rows.every((row) => row.cells.every((cell) => cell.request_count === 0)),
    ),
    requestHealth: buildSurface(
      {
        data: viewModel.serviceHealth,
        isLoading: input.requestHealth.isLoading,
        error: input.requestHealth.error,
      },
      (data) => data.total_success + data.total_failure === 0,
    ),
  }
}

function buildCoreCollectionStatus(
  coreStatus: UsageSurfaceStatus,
  fieldIsPresent: boolean,
  itemCount: number,
): UsageSurfaceStatus {
  if (coreStatus !== "ready") return coreStatus
  return fieldIsPresent && itemCount > 0 ? "ready" : "empty"
}
