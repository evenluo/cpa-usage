package service

import (
	"context"

	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"gorm.io/gorm"
)

type AnalyticsProvider interface {
	GetAnalyticsSummary(context.Context, servicedto.UsageFilter) (*servicedto.AnalyticsSummarySnapshot, error)
}

type analyticsService struct {
	db *gorm.DB
}

func NewAnalyticsService(db *gorm.DB) AnalyticsProvider {
	return &analyticsService{db: db}
}

func (s *analyticsService) GetAnalyticsSummary(_ context.Context, filter servicedto.UsageFilter) (*servicedto.AnalyticsSummarySnapshot, error) {
	snapshot, err := repository.BuildAnalyticsSummaryWithFilter(s.db, repodto.UsageQueryFilter{
		Range:       filter.Range,
		StartTime:   filter.StartTime,
		EndTime:     filter.EndTime,
		Granularity: filter.Granularity,
		Provider:    filter.Provider,
	})
	if err != nil {
		return nil, err
	}
	trend := make([]servicedto.AnalyticsTrendPoint, 0, len(snapshot.Trend))
	for _, point := range snapshot.Trend {
		trend = append(trend, servicedto.AnalyticsTrendPoint{
			Label:         point.Label,
			BucketStart:   point.BucketStart,
			BucketEnd:     point.BucketEnd,
			TotalCost:     point.TotalCost,
			TotalTokens:   point.TotalTokens,
			RequestCount:  point.RequestCount,
			SuccessCount:  point.SuccessCount,
			FailureCount:  point.FailureCount,
			CostAvailable: point.CostAvailable,
			CostStatus:    point.CostStatus,
		})
	}
	keyAliasBreakdown := make([]servicedto.AnalyticsKeyAliasBreakdown, 0, len(snapshot.KeyAliasBreakdown))
	for _, row := range snapshot.KeyAliasBreakdown {
		trend := make([]servicedto.AnalyticsKeyAliasTrendPoint, 0, len(row.Trend))
		for _, point := range row.Trend {
			trend = append(trend, servicedto.AnalyticsKeyAliasTrendPoint{
				Label:         point.Label,
				TotalCost:     point.TotalCost,
				TotalTokens:   point.TotalTokens,
				CostAvailable: point.CostAvailable,
				CostStatus:    point.CostStatus,
			})
		}
		keyAliasBreakdown = append(keyAliasBreakdown, servicedto.AnalyticsKeyAliasBreakdown{
			AuthType:      row.AuthType,
			Identity:      row.Identity,
			Alias:         row.Alias,
			Name:          row.Name,
			AuthTypeName:  row.AuthTypeName,
			Type:          row.Type,
			Provider:      row.Provider,
			Prefix:        row.Prefix,
			BaseURL:       row.BaseURL,
			IsDeleted:     row.IsDeleted,
			TotalCost:     row.TotalCost,
			TotalTokens:   row.TotalTokens,
			RequestCount:  row.RequestCount,
			SuccessCount:  row.SuccessCount,
			FailureCount:  row.FailureCount,
			SuccessRate:   row.SuccessRate,
			LastUsedAt:    row.LastUsedAt,
			CostAvailable: row.CostAvailable,
			CostStatus:    row.CostStatus,
			Trend:         trend,
		})
	}
	modelBreakdown := make([]servicedto.AnalyticsModelBreakdown, 0, len(snapshot.ModelBreakdown))
	for _, row := range snapshot.ModelBreakdown {
		modelBreakdown = append(modelBreakdown, servicedto.AnalyticsModelBreakdown{
			Model:                 row.Model,
			Provider:              row.Provider,
			TotalCost:             row.TotalCost,
			TotalTokens:           row.TotalTokens,
			RequestCount:          row.RequestCount,
			SuccessCount:          row.SuccessCount,
			FailureCount:          row.FailureCount,
			InputTokens:           row.InputTokens,
			CachedTokens:          row.CachedTokens,
			SuccessRate:           row.SuccessRate,
			TotalLatencyMS:        row.TotalLatencyMS,
			LatencySampleCount:    row.LatencySampleCount,
			AverageLatencyMS:      row.AverageLatencyMS,
			CostAvailable:         row.CostAvailable,
			CostStatus:            row.CostStatus,
			CacheReadShare:        row.CacheReadShare,
			CacheReadShareState:   row.CacheReadShareState,
			EstimatedCacheSavings: row.EstimatedCacheSavings,
		})
	}
	timeBreakdown := make([]servicedto.AnalyticsTrendPoint, 0, len(snapshot.TimeBreakdown))
	for _, point := range snapshot.TimeBreakdown {
		timeBreakdown = append(timeBreakdown, servicedto.AnalyticsTrendPoint{
			Label:         point.Label,
			BucketStart:   point.BucketStart,
			BucketEnd:     point.BucketEnd,
			TotalCost:     point.TotalCost,
			TotalTokens:   point.TotalTokens,
			RequestCount:  point.RequestCount,
			SuccessCount:  point.SuccessCount,
			FailureCount:  point.FailureCount,
			CostAvailable: point.CostAvailable,
			CostStatus:    point.CostStatus,
		})
	}
	insights := make([]servicedto.AnalyticsInsight, 0, len(snapshot.Insights))
	for _, insight := range snapshot.Insights {
		insights = append(insights, servicedto.AnalyticsInsight{
			Type:        insight.Type,
			Severity:    insight.Severity,
			Title:       insight.Title,
			Detail:      insight.Detail,
			Subject:     insight.Subject,
			MetricLabel: insight.MetricLabel,
			MetricValue: insight.MetricValue,
			Count:       insight.Count,
			CostStatus:  insight.CostStatus,
		})
	}
	providerOptions := make([]servicedto.AnalyticsProviderOption, 0, len(snapshot.ProviderOptions))
	for _, option := range snapshot.ProviderOptions {
		providerOptions = append(providerOptions, servicedto.AnalyticsProviderOption{
			Provider:      option.Provider,
			RequestCount:  option.RequestCount,
			TotalTokens:   option.TotalTokens,
			TotalCost:     option.TotalCost,
			CostAvailable: option.CostAvailable,
			CostStatus:    option.CostStatus,
		})
	}
	heatmapRows := make([]servicedto.AnalyticsHeatmapRow, 0, len(snapshot.Heatmap.Rows))
	for _, row := range snapshot.Heatmap.Rows {
		cells := make([]servicedto.AnalyticsHeatmapCell, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, servicedto.AnalyticsHeatmapCell{
				Hour:          cell.Hour,
				InRange:       cell.InRange,
				BucketStart:   cell.BucketStart,
				BucketEnd:     cell.BucketEnd,
				TotalTokens:   cell.TotalTokens,
				TotalCost:     cell.TotalCost,
				RequestCount:  cell.RequestCount,
				FailureCount:  cell.FailureCount,
				CostAvailable: cell.CostAvailable,
				CostStatus:    cell.CostStatus,
			})
		}
		heatmapRows = append(heatmapRows, servicedto.AnalyticsHeatmapRow{
			Date:  row.Date,
			Label: row.Label,
			Cells: cells,
		})
	}
	return &servicedto.AnalyticsSummarySnapshot{
		Summary: servicedto.AnalyticsSummary{
			TotalCost:             snapshot.Summary.TotalCost,
			TotalTokens:           snapshot.Summary.TotalTokens,
			RequestCount:          snapshot.Summary.RequestCount,
			SuccessCount:          snapshot.Summary.SuccessCount,
			FailureCount:          snapshot.Summary.FailureCount,
			InputTokens:           snapshot.Summary.InputTokens,
			CachedTokens:          snapshot.Summary.CachedTokens,
			ReasoningTokens:       snapshot.Summary.ReasoningTokens,
			SuccessRate:           snapshot.Summary.SuccessRate,
			CostAvailable:         snapshot.Summary.CostAvailable,
			CostStatus:            snapshot.Summary.CostStatus,
			CacheReadShare:        snapshot.Summary.CacheReadShare,
			CacheReadShareState:   snapshot.Summary.CacheReadShareState,
			EstimatedCacheSavings: snapshot.Summary.EstimatedCacheSavings,
		},
		Trend:              trend,
		KeyAliasBreakdown:  keyAliasBreakdown,
		ModelBreakdown:     modelBreakdown,
		TimeBreakdown:      timeBreakdown,
		Insights:           insights,
		ProviderOptions:    providerOptions,
		PreviousRangeStart: snapshot.PreviousRangeStart,
		PreviousRangeEnd:   snapshot.PreviousRangeEnd,
		Comparison: servicedto.AnalyticsComparison{
			HasPreviousPeriod:     snapshot.Comparison.HasPreviousPeriod,
			TotalCostChangePct:    snapshot.Comparison.TotalCostChangePct,
			TotalTokensChangePct:  snapshot.Comparison.TotalTokensChangePct,
			RequestCountChangePct: snapshot.Comparison.RequestCountChangePct,
			SuccessRateChangePP:   snapshot.Comparison.SuccessRateChangePP,
		},
		Heatmap: servicedto.AnalyticsHeatmap{
			Measure:     snapshot.Heatmap.Measure,
			MaxTokens:   snapshot.Heatmap.MaxTokens,
			MaxCost:     snapshot.Heatmap.MaxCost,
			MaxRequests: snapshot.Heatmap.MaxRequests,
			MaxFailures: snapshot.Heatmap.MaxFailures,
			Rows:        heatmapRows,
		},
	}, nil
}
