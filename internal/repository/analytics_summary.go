package repository

import (
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"time"
)

func buildAnalyticsSummary(db *gorm.DB, filter dto.AnalyticsFilter) (dto.AnalyticsSummary, error) {
	row, err := buildAnalyticsAggregateRow(db, filter, analyticsEventsAggregateSource())
	if err != nil {
		return dto.AnalyticsSummary{}, err
	}

	return mapAnalyticsSummary(row), nil
}

func analyticsPreviousPeriodFilter(filter dto.AnalyticsFilter) (dto.AnalyticsFilter, bool) {
	if filter.StartTime == nil || filter.EndTime == nil {
		return dto.AnalyticsFilter{}, false
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	if !end.After(start) {
		return dto.AnalyticsFilter{}, false
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
	if current.CostStatus != dto.CostStatusAvailable || previous.CostStatus != dto.CostStatusAvailable {
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
	cost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
	summary.CostAvailable, summary.CostStatus = cost.Available, cost.Status
	summary.CacheReadShare, summary.CacheReadShareState, summary.EstimatedCacheSavings = analyticsCacheEfficiency(
		row.InputTokens,
		row.CachedTokens,
		row.CacheSavings,
		row.CacheSavingsEligibleRows,
		row.CacheSavingsIneligibleRows,
		summary.CostStatus == dto.CostStatusAvailable,
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
