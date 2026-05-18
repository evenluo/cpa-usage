package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
)

func buildAnalyticsModelBreakdown(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsModelBreakdown, error) {
	var rows []analyticsModelAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			TRIM(usage_events.model) AS model,
			COALESCE(MIN(NULLIF(TRIM(usage_events.provider), '')), '') AS provider,
			COUNT(DISTINCT NULLIF(TRIM(usage_events.provider), '')) AS provider_count,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.input_tokens") + `), 0) AS input_tokens,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.cached_tokens") + `), 0) AS cached_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpression() + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpression() + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpression() + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(CASE WHEN usage_events.latency_ms > 0 THEN usage_events.latency_ms ELSE 0 END), 0) AS total_latency_ms,
			COALESCE(SUM(CASE WHEN usage_events.latency_ms > 0 THEN 1 ELSE 0 END), 0) AS latency_sample_count,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Where("TRIM(usage_events.model) <> ''").
		Group("TRIM(usage_events.model)").
		Order("total_cost DESC").
		Order("COALESCE(SUM(usage_events.total_tokens), 0) DESC").
		Order("model ASC").
		Limit(20).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics model breakdown: %w", err)
	}

	breakdown := make([]dto.AnalyticsModelBreakdown, 0, len(rows))
	for _, row := range rows {
		breakdown = append(breakdown, mapAnalyticsModelBreakdown(row))
	}
	return breakdown, nil
}

func buildAnalyticsProviderOptions(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsProviderOption, error) {
	var rows []analyticsProviderOptionRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			TRIM(usage_events.provider) AS provider,
			COUNT(*) AS request_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Where("TRIM(usage_events.provider) <> ''").
		Group("TRIM(usage_events.provider)").
		Order("total_cost DESC").
		Order("COALESCE(SUM(usage_events.total_tokens), 0) DESC").
		Order("provider ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics provider options: %w", err)
	}

	options := make([]dto.AnalyticsProviderOption, 0, len(rows))
	for _, row := range rows {
		costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
		options = append(options, dto.AnalyticsProviderOption{
			Provider:      row.Provider,
			RequestCount:  row.RequestCount,
			TotalTokens:   row.TotalTokens,
			TotalCost:     row.TotalCost,
			CostAvailable: costAvailable,
			CostStatus:    costStatus,
		})
	}
	return options, nil
}
func mapAnalyticsModelBreakdown(row analyticsModelAggregateRow) dto.AnalyticsModelBreakdown {
	record := dto.AnalyticsModelBreakdown{
		Model:              row.Model,
		Provider:           row.Provider,
		TotalCost:          row.TotalCost,
		TotalTokens:        row.TotalTokens,
		RequestCount:       row.RequestCount,
		SuccessCount:       row.SuccessCount,
		FailureCount:       row.FailureCount,
		InputTokens:        row.InputTokens,
		CachedTokens:       row.CachedTokens,
		TotalLatencyMS:     row.TotalLatencyMS,
		LatencySampleCount: row.LatencySampleCount,
	}
	if row.ProviderCount > 1 {
		record.Provider = "Multiple providers"
	}
	if row.RequestCount > 0 {
		record.SuccessRate = (float64(row.SuccessCount) / float64(row.RequestCount)) * 100
	}
	if row.LatencySampleCount > 0 {
		record.AverageLatencyMS = float64(row.TotalLatencyMS) / float64(row.LatencySampleCount)
	}
	record.CostAvailable, record.CostStatus = analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	record.CacheReadShare, record.CacheReadShareState, record.EstimatedCacheSavings = analyticsCacheEfficiency(
		row.InputTokens,
		row.CachedTokens,
		row.CacheSavings,
		row.CacheSavingsEligibleRows,
		row.CacheSavingsIneligibleRows,
		record.CostStatus == dto.AnalyticsCostStatusAvailable,
	)
	return record
}
