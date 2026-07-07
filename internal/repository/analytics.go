package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"time"

	"gorm.io/gorm"
)

func BuildAnalyticsSummaryWithFilter(db *gorm.DB, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		return nil, err
	}
	previousRangeStart, previousRangeEnd, comparison, err := buildAnalyticsSummaryComparison(db, filter, core.Summary)
	if err != nil {
		return nil, err
	}
	heatmap, err := BuildAnalyticsHeatmapWithFilter(db, filter)
	if err != nil {
		return nil, err
	}

	return &dto.AnalyticsSummarySnapshot{
		Summary:            core.Summary,
		Trend:              core.Trend,
		KeyAliasBreakdown:  core.KeyAliasBreakdown,
		APIKeyBreakdown:    core.APIKeyBreakdown,
		ModelBreakdown:     core.ModelBreakdown,
		TimeBreakdown:      core.Trend,
		Insights:           core.Insights,
		ProviderOptions:    core.ProviderOptions,
		PreviousRangeStart: previousRangeStart,
		PreviousRangeEnd:   previousRangeEnd,
		Comparison:         comparison,
		Heatmap:            heatmap,
	}, nil
}

func buildAnalyticsSummaryComparison(db *gorm.DB, filter dto.UsageQueryFilter, current dto.AnalyticsSummary) (*time.Time, *time.Time, dto.AnalyticsComparison, error) {
	previousFilter, ok := analyticsPreviousPeriodFilter(filter)
	if !ok {
		return nil, nil, dto.AnalyticsComparison{}, nil
	}
	previous, err := BuildAnalyticsCoreWithFilter(db, previousFilter)
	if err != nil {
		return nil, nil, dto.AnalyticsComparison{}, err
	}
	comparison := mapAnalyticsComparison(current, previous.Summary)
	return previousFilter.StartTime, previousFilter.EndTime, comparison, nil
}
