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
		Range:     filter.Range,
		StartTime: filter.StartTime,
		EndTime:   filter.EndTime,
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
	return &servicedto.AnalyticsSummarySnapshot{
		Summary: servicedto.AnalyticsSummary{
			TotalCost:     snapshot.Summary.TotalCost,
			TotalTokens:   snapshot.Summary.TotalTokens,
			RequestCount:  snapshot.Summary.RequestCount,
			SuccessCount:  snapshot.Summary.SuccessCount,
			FailureCount:  snapshot.Summary.FailureCount,
			SuccessRate:   snapshot.Summary.SuccessRate,
			CostAvailable: snapshot.Summary.CostAvailable,
			CostStatus:    snapshot.Summary.CostStatus,
		},
		Trend:             trend,
		KeyAliasBreakdown: keyAliasBreakdown,
	}, nil
}
