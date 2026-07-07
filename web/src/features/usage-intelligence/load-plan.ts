import type { TimeGranularity, TimeRange } from "@/types/api"

export const FIXED_OPERATIONAL_WINDOW = "24h" satisfies TimeRange
export const FIXED_HEATMAP_WINDOW = "30d" satisfies TimeRange
export const FIXED_HEATMAP_GRANULARITY = "day" satisfies TimeGranularity
export const REQUEST_EVIDENCE_PAGE_SIZE = 10

export interface UsageIntelligenceLoadPlan {
  selectedWindow: {
    analytics: {
      range: TimeRange
      granularity: TimeGranularity
      provider: string
    }
  }
  fixedWindow: {
    heatmap: {
      range: TimeRange
      granularity: TimeGranularity
      provider: string
    }
    requestHealth: {
      range: TimeRange
      provider: string
    }
    requestEvidence: {
      range: TimeRange
      pageSize: number
      provider: string
    }
    liveCapacity: {
      provider: string
    }
  }
}

export function buildUsageIntelligenceLoadPlan(input: {
  range: TimeRange
  granularity: TimeGranularity
  provider: string
}): UsageIntelligenceLoadPlan {
  return {
    selectedWindow: {
      analytics: {
        range: input.range,
        granularity: input.granularity,
        provider: input.provider,
      },
    },
    fixedWindow: {
      heatmap: {
        range: FIXED_HEATMAP_WINDOW,
        granularity: FIXED_HEATMAP_GRANULARITY,
        provider: input.provider,
      },
      requestHealth: {
        range: FIXED_OPERATIONAL_WINDOW,
        provider: input.provider,
      },
      requestEvidence: {
        range: FIXED_OPERATIONAL_WINDOW,
        pageSize: REQUEST_EVIDENCE_PAGE_SIZE,
        provider: input.provider,
      },
      liveCapacity: {
        provider: input.provider,
      },
    },
  }
}
