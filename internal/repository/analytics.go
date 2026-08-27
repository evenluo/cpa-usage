package repository

import (
	"context"
	"fmt"
	"time"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func BuildAnalyticsSummaryWithFilter(ctx context.Context, db *gorm.DB, filter dto.AnalyticsFilter) (*dto.AnalyticsSummarySnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	core, err := BuildAnalyticsCoreWithFilter(ctx, db, filter)
	if err != nil {
		return nil, err
	}
	previousRangeStart, previousRangeEnd, comparison, err := buildAnalyticsSummaryComparison(ctx, db, filter, core.Summary)
	if err != nil {
		return nil, err
	}
	heatmap, err := BuildAnalyticsHeatmapWithFilter(ctx, db, filter)
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

func buildAnalyticsSummaryComparison(ctx context.Context, db *gorm.DB, filter dto.AnalyticsFilter, current dto.AnalyticsSummary) (*time.Time, *time.Time, dto.AnalyticsComparison, error) {
	previousFilter, ok := analyticsPreviousPeriodFilter(filter)
	if !ok {
		return nil, nil, dto.AnalyticsComparison{}, nil
	}
	previous, err := BuildAnalyticsCoreWithFilter(ctx, db, previousFilter)
	if err != nil {
		return nil, nil, dto.AnalyticsComparison{}, err
	}
	comparison := mapAnalyticsComparison(current, previous.Summary)
	return previousFilter.StartTime, previousFilter.EndTime, comparison, nil
}
