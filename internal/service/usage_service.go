package service

import (
	"context"

	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
)

type UsageProvider interface {
	GetUsageWithFilter(context.Context, servicedto.UsageFilter) (*repodto.StatisticsSnapshot, error)
	GetUsageOverview(context.Context, servicedto.UsageFilter) (*servicedto.UsageOverviewSnapshot, error)
	GetRequestHealth(context.Context, servicedto.UsageFilter) (*servicedto.UsageOverviewHealth, error)
	ListUsageEvents(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventsPage, error)
	ListUsageEventFilterOptions(context.Context, servicedto.UsageFilter) (*servicedto.UsageEventFilterOptions, error)
	GetUsageAnalysis(context.Context, servicedto.UsageFilter) (*servicedto.UsageAnalysisSnapshot, error)
}
