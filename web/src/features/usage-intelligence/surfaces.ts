import type {
  AnalyticsCoreResponse,
  HeatmapData,
  KeyAliasBreakdown,
  ServiceHealth,
  TrendPoint,
} from "@/types/api"
import type { UsageDashboardViewModel } from "./view-model"

export type UsageSurfaceStatus = "loading" | "error" | "ready" | "empty"

export type UsageSurface<TData> =
  | { status: "loading"; data: undefined }
  | { status: "error"; data: undefined }
  | { status: "ready"; data: TData }
  | { status: "empty"; data: undefined }

export interface UsageDashboardSurfaces {
  kpis: { status: UsageSurfaceStatus }
  trend: { status: UsageSurfaceStatus; data: TrendPoint[] }
  leaderboard: { status: UsageSurfaceStatus; data: KeyAliasBreakdown[] }
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
export function buildSurface<TData>(query: UsageSurfaceQuery<TData>): UsageSurface<TData> {
  if (query.data === undefined) {
    if (query.isLoading) return { status: "loading", data: undefined }
    if (query.error) return { status: "error", data: undefined }
    return { status: "empty", data: undefined }
  }
  return { status: "ready", data: query.data }
}

export function buildUsageDashboardSurfaces(input: {
  viewModel: UsageDashboardViewModel
  core: UsageSurfaceQuery<AnalyticsCoreResponse>
  heatmap: Omit<UsageSurfaceQuery<unknown>, "data">
  requestHealth: Omit<UsageSurfaceQuery<unknown>, "data">
}): UsageDashboardSurfaces {
  const { viewModel } = input
  const coreSurface = buildSurface(input.core)
  // The leaderboard has no error branch: it only waits while the core query
  // has not produced its breakdown yet, otherwise it renders whatever rows
  // the view model derived (possibly none).
  const leaderboardStatus: UsageSurfaceStatus =
    !viewModel.hasLeaderboardBreakdown && coreSurface.status === "loading"
      ? "loading"
      : viewModel.hasLeaderboardBreakdown
        ? "ready"
        : "empty"
  return {
    kpis: { status: coreSurface.status },
    trend: { status: coreSurface.status, data: viewModel.trend },
    leaderboard: { status: leaderboardStatus, data: viewModel.leaderboardRows },
    heatmap: buildSurface({
      data: viewModel.fixedHeatmap,
      isLoading: input.heatmap.isLoading,
      error: input.heatmap.error,
    }),
    requestHealth: buildSurface({
      data: viewModel.serviceHealth,
      isLoading: input.requestHealth.isLoading,
      error: input.requestHealth.error,
    }),
  }
}
