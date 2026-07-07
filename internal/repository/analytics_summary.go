package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
	"time"
)

func buildAnalyticsSummary(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsSummary, error) {
	var row analyticsAggregateRow
	row, err := buildAnalyticsSummaryAggregateRow(db, filter)
	if err != nil {
		return dto.AnalyticsSummary{}, err
	}

	return mapAnalyticsSummary(row), nil
}

func buildAnalyticsSummaryAggregateRow(db *gorm.DB, filter dto.UsageQueryFilter) (analyticsAggregateRow, error) {
	var row analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.input_tokens") + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.output_tokens") + `), 0) AS output_tokens,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.cached_tokens") + `), 0) AS cached_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.reasoning_tokens") + `), 0) AS reasoning_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpression() + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpression() + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpression() + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Scan(&row).Error; err != nil {
		return analyticsAggregateRow{}, fmt.Errorf("build analytics summary: %w", err)
	}
	return row, nil
}

func buildAnalyticsComparison(db *gorm.DB, filter dto.UsageQueryFilter, current dto.AnalyticsSummary) (*time.Time, *time.Time, dto.AnalyticsComparison, error) {
	previousFilter, ok := analyticsPreviousPeriodFilter(filter)
	if !ok {
		return nil, nil, dto.AnalyticsComparison{}, nil
	}
	previous, err := buildAnalyticsSummary(db, previousFilter)
	if err != nil {
		return nil, nil, dto.AnalyticsComparison{}, err
	}
	comparison := mapAnalyticsComparison(current, previous)
	return previousFilter.StartTime, previousFilter.EndTime, comparison, nil
}

func analyticsPreviousPeriodFilter(filter dto.UsageQueryFilter) (dto.UsageQueryFilter, bool) {
	if filter.StartTime == nil || filter.EndTime == nil {
		return dto.UsageQueryFilter{}, false
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	if !end.After(start) {
		return dto.UsageQueryFilter{}, false
	}
	duration := end.Sub(start) + time.Nanosecond
	previousStart := start.Add(-duration)
	previousEnd := start.Add(-time.Nanosecond)
	previousFilter := filter
	previousFilter.StartTime = &previousStart
	previousFilter.EndTime = &previousEnd
	return previousFilter, true
}

func mapAnalyticsComparison(current dto.AnalyticsSummary, previous dto.AnalyticsSummary) dto.AnalyticsComparison {
	if previous.RequestCount <= 0 {
		return dto.AnalyticsComparison{HasPreviousPeriod: false}
	}
	return dto.AnalyticsComparison{
		HasPreviousPeriod:     true,
		TotalCostChangePct:    analyticsCostPercentChange(current, previous),
		TotalTokensChangePct:  analyticsPercentChange(float64(current.TotalTokens), float64(previous.TotalTokens)),
		RequestCountChangePct: analyticsPercentChange(float64(current.RequestCount), float64(previous.RequestCount)),
		SuccessRateChangePP:   analyticsPointChange(current.SuccessRate, previous.SuccessRate),
	}
}

func analyticsCostPercentChange(current dto.AnalyticsSummary, previous dto.AnalyticsSummary) *float64 {
	if current.CostStatus != dto.AnalyticsCostStatusAvailable || previous.CostStatus != dto.AnalyticsCostStatusAvailable {
		return nil
	}
	return analyticsPercentChange(current.TotalCost, previous.TotalCost)
}

func analyticsPercentChange(current float64, previous float64) *float64 {
	if previous == 0 {
		return nil
	}
	change := ((current - previous) / previous) * 100
	return &change
}

func analyticsPointChange(current float64, previous float64) *float64 {
	change := current - previous
	return &change
}
func mapAnalyticsSummary(row analyticsAggregateRow) dto.AnalyticsSummary {
	summary := dto.AnalyticsSummary{
		TotalCost:       row.TotalCost,
		TotalTokens:     row.TotalTokens,
		RequestCount:    row.RequestCount,
		SuccessCount:    row.SuccessCount,
		FailureCount:    row.FailureCount,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		CachedTokens:    row.CachedTokens,
		ReasoningTokens: row.ReasoningTokens,
	}
	if row.RequestCount > 0 {
		summary.SuccessRate = (float64(row.SuccessCount) / float64(row.RequestCount)) * 100
	}
	summary.CostAvailable, summary.CostStatus = analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	summary.CacheReadShare, summary.CacheReadShareState, summary.EstimatedCacheSavings = analyticsCacheEfficiency(
		row.InputTokens,
		row.CachedTokens,
		row.CacheSavings,
		row.CacheSavingsEligibleRows,
		row.CacheSavingsIneligibleRows,
		summary.CostStatus == dto.AnalyticsCostStatusAvailable,
	)
	return summary
}
func analyticsCacheEfficiency(inputTokens int64, cachedTokens int64, cacheSavings float64, eligibleRows int64, ineligibleRows int64, pricingComplete bool) (float64, string, *float64) {
	if inputTokens <= 0 {
		return 0, dto.AnalyticsCacheReadShareStateNoPromptInput, nil
	}
	if cachedTokens <= 0 {
		return 0, dto.AnalyticsCacheReadShareStateNoCacheData, nil
	}
	share := (float64(cachedTokens) / float64(inputTokens)) * 100
	if !pricingComplete || eligibleRows == 0 || ineligibleRows > 0 {
		return share, dto.AnalyticsCacheReadShareStateAvailable, nil
	}
	return share, dto.AnalyticsCacheReadShareStateAvailable, &cacheSavings
}
