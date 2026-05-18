package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
)

func BuildAnalyticsSummaryWithFilter(db *gorm.DB, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	summary, err := buildAnalyticsSummary(db, filter)
	if err != nil {
		return nil, err
	}
	trend, err := buildAnalyticsTrend(db, filter)
	if err != nil {
		return nil, err
	}
	keyAliasBreakdown, err := buildAnalyticsKeyAliasBreakdown(db, filter)
	if err != nil {
		return nil, err
	}
	apiKeyBreakdown, err := buildAnalyticsAPIKeyBreakdown(db, filter)
	if err != nil {
		return nil, err
	}
	modelBreakdown, err := buildAnalyticsModelBreakdown(db, filter)
	if err != nil {
		return nil, err
	}
	providerOptions, err := buildAnalyticsProviderOptions(db, filter)
	if err != nil {
		return nil, err
	}
	previousRangeStart, previousRangeEnd, comparison, err := buildAnalyticsComparison(db, filter, summary)
	if err != nil {
		return nil, err
	}
	heatmap, err := buildAnalyticsHeatmap(db, filter)
	if err != nil {
		return nil, err
	}

	return &dto.AnalyticsSummarySnapshot{
		Summary:            summary,
		Trend:              trend,
		KeyAliasBreakdown:  keyAliasBreakdown,
		APIKeyBreakdown:    apiKeyBreakdown,
		ModelBreakdown:     modelBreakdown,
		TimeBreakdown:      trend,
		Insights:           buildAnalyticsInsights(summary, trend, keyAliasBreakdown, modelBreakdown),
		ProviderOptions:    providerOptions,
		PreviousRangeStart: previousRangeStart,
		PreviousRangeEnd:   previousRangeEnd,
		Comparison:         comparison,
		Heatmap:            heatmap,
	}, nil
}
