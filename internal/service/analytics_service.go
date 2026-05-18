package service

import (
	"context"

	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"gorm.io/gorm"
)

type AnalyticsProvider interface {
	GetAnalyticsSummary(context.Context, servicedto.UsageFilter) (*repodto.AnalyticsSummarySnapshot, error)
}

type analyticsService struct {
	db *gorm.DB
}

func NewAnalyticsService(db *gorm.DB) AnalyticsProvider {
	return &analyticsService{db: db}
}

func (s *analyticsService) GetAnalyticsSummary(_ context.Context, filter servicedto.UsageFilter) (*repodto.AnalyticsSummarySnapshot, error) {
	return repository.BuildAnalyticsSummaryWithFilter(s.db, repodto.UsageQueryFilter{
		Range:          filter.Range,
		StartTime:      filter.StartTime,
		EndTime:        filter.EndTime,
		FixedWindowEnd: filter.FixedWindowEnd,
		Granularity:    filter.Granularity,
		Provider:       filter.Provider,
	})
}
