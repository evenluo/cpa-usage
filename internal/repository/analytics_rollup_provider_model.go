package repository

import (
	"fmt"
	"sort"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func buildAnalyticsCoreProviderOptions(db *gorm.DB, plan analyticsCoreWindowPlan) ([]dto.AnalyticsProviderOption, error) {
	combined := map[string]analyticsProviderOptionRow{}
	for _, filter := range plan.rawFilters {
		rows, err := buildAnalyticsProviderOptionSegmentRows(db, filter, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsProviderOptionRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsProviderOptionSegmentRows(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsProviderOptionRows(combined, rows)
	}

	rows := make([]analyticsProviderOptionRow, 0, len(combined))
	for _, row := range combined {
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCost != rows[j].TotalCost {
			return rows[i].TotalCost > rows[j].TotalCost
		}
		if rows[i].TotalTokens != rows[j].TotalTokens {
			return rows[i].TotalTokens > rows[j].TotalTokens
		}
		return rows[i].Provider < rows[j].Provider
	})

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

func buildAnalyticsCoreModelBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan) ([]dto.AnalyticsModelBreakdown, error) {
	combined := map[string]analyticsModelAggregateRow{}
	providersByModel := map[string]map[string]struct{}{}
	for _, filter := range plan.rawFilters {
		rows, err := buildAnalyticsModelSegmentRows(db, filter, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsModelRows(combined, providersByModel, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsModelSegmentRows(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsModelRows(combined, providersByModel, rows)
	}

	rows := make([]analyticsModelAggregateRow, 0, len(combined))
	for model, row := range combined {
		providers := providersByModel[model]
		row.ProviderCount = int64(len(providers))
		row.Provider = ""
		if len(providers) == 1 {
			for provider := range providers {
				row.Provider = provider
			}
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].TotalCost != rows[j].TotalCost {
			return rows[i].TotalCost > rows[j].TotalCost
		}
		if rows[i].TotalTokens != rows[j].TotalTokens {
			return rows[i].TotalTokens > rows[j].TotalTokens
		}
		return rows[i].Model < rows[j].Model
	})
	if len(rows) > 20 {
		rows = rows[:20]
	}

	breakdown := make([]dto.AnalyticsModelBreakdown, 0, len(rows))
	for _, row := range rows {
		breakdown = append(breakdown, mapAnalyticsModelBreakdown(row))
	}
	return breakdown, nil
}

// buildAnalyticsProviderOptionSegmentRows 按聚合源渲染 provider 选项段查询，raw 与 rollup 共用同一份列定义。
func buildAnalyticsProviderOptionSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter, source analyticsAggregateSource) ([]analyticsProviderOptionRow, error) {
	var rows []analyticsProviderOptionRow
	if err := source.query(db, filter).
		Select(`
			` + source.providerExpr + ` AS provider,
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
		Where(source.providerExpr + " <> ''").
		Group(source.providerExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s provider option segment rows: %w", source.name, err)
	}
	return rows, nil
}

// buildAnalyticsModelSegmentRows 按聚合源渲染 model 段查询，raw 与 rollup 共用同一份列定义。
func buildAnalyticsModelSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter, source analyticsAggregateSource) ([]analyticsModelAggregateRow, error) {
	var rows []analyticsModelAggregateRow
	if err := source.query(db, filter).
		Select(`
			` + source.modelExpr + ` AS model,
			` + source.providerExpr + ` AS provider,
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.inputTokensExpr) + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.outputTokensExpr) + `), 0) AS output_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.reasoningTokensExpr) + `), 0) AS reasoning_tokens,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.cachedTokensExpr) + `), 0) AS cached_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpressionFor(source.cachedTokensExpr) + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpressionFor(source.cachedTokensExpr, source.requestCountExpr) + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpressionFor(source.cachedTokensExpr, source.requestCountExpr) + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + source.latencySumExpr + `), 0) AS total_latency_ms,
			COALESCE(SUM(` + source.latencyCountExpr + `), 0) AS latency_sample_count,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
		Where(source.modelExpr + " <> ''").
		Group(source.modelExpr + ", " + source.providerExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s model segment rows: %w", source.name, err)
	}
	return rows, nil
}

func addAnalyticsProviderOptionRows(dst map[string]analyticsProviderOptionRow, rows []analyticsProviderOptionRow) {
	for _, row := range rows {
		combined := dst[row.Provider]
		combined.Provider = row.Provider
		combined.RequestCount += row.RequestCount
		combined.TotalTokens += row.TotalTokens
		combined.TotalCost += row.TotalCost
		combined.MissingPricingEvents += row.MissingPricingEvents
		combined.PricedBillableEvents += row.PricedBillableEvents
		dst[row.Provider] = combined
	}
}

func addAnalyticsModelRows(dst map[string]analyticsModelAggregateRow, providersByModel map[string]map[string]struct{}, rows []analyticsModelAggregateRow) {
	for _, row := range rows {
		combined := dst[row.Model]
		combined.Model = row.Model
		combined.RequestCount += row.RequestCount
		combined.SuccessCount += row.SuccessCount
		combined.FailureCount += row.FailureCount
		combined.InputTokens += row.InputTokens
		combined.OutputTokens += row.OutputTokens
		combined.ReasoningTokens += row.ReasoningTokens
		combined.TotalTokens += row.TotalTokens
		combined.TotalCost += row.TotalCost
		combined.CachedTokens += row.CachedTokens
		combined.CacheSavings += row.CacheSavings
		combined.CacheSavingsEligibleRows += row.CacheSavingsEligibleRows
		combined.CacheSavingsIneligibleRows += row.CacheSavingsIneligibleRows
		combined.TotalLatencyMS += row.TotalLatencyMS
		combined.LatencySampleCount += row.LatencySampleCount
		combined.MissingPricingEvents += row.MissingPricingEvents
		combined.PricedBillableEvents += row.PricedBillableEvents
		dst[row.Model] = combined
		if row.Provider != "" {
			if providersByModel[row.Model] == nil {
				providersByModel[row.Model] = map[string]struct{}{}
			}
			providersByModel[row.Model][row.Provider] = struct{}{}
		}
	}
}
