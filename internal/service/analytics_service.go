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
		Trend: trend,
	}, nil
}
