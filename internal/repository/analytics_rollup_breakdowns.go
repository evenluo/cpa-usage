package repository

import (
	"fmt"
	"sort"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type analyticsIdentityTrendKey struct {
	AuthType int
	Identity string
	Bucket   string
}

func buildAnalyticsCoreProviderOptions(db *gorm.DB, plan analyticsCoreWindowPlan) ([]dto.AnalyticsProviderOption, error) {
	combined := map[string]analyticsProviderOptionRow{}
	for _, filter := range plan.rawFilters {
		rows, err := buildAnalyticsProviderOptionSegmentRows(db, filter)
		if err != nil {
			return nil, err
		}
		addAnalyticsProviderOptionRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsRollupProviderOptionSegmentRows(db, *plan.rollupFilter)
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
		rows, err := buildAnalyticsModelSegmentRows(db, filter)
		if err != nil {
			return nil, err
		}
		addAnalyticsModelRows(combined, providersByModel, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsRollupModelSegmentRows(db, *plan.rollupFilter)
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

func buildAnalyticsCoreKeyAliasBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.UsageQueryFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	combined := map[analyticsIdentityKey]analyticsIdentityAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsKeyAliasSegmentRows(db, rawFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsRollupKeyAliasSegmentRows(db, *plan.rollupFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	return mapAnalyticsCoreIdentityBreakdown(db, plan, filter, combined, false)
}

func buildAnalyticsCoreAPIKeyBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.UsageQueryFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	combined := map[analyticsIdentityKey]analyticsIdentityAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsAPIKeySegmentRows(db, rawFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsRollupAPIKeySegmentRows(db, *plan.rollupFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	return mapAnalyticsCoreIdentityBreakdown(db, plan, filter, combined, true)
}

func mapAnalyticsCoreIdentityBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.UsageQueryFilter, combined map[analyticsIdentityKey]analyticsIdentityAggregateRow, apiKeys bool) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	rows := make([]analyticsIdentityAggregateRow, 0, len(combined))
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
		if rows[i].LastUsedAt != rows[j].LastUsedAt {
			return rows[i].LastUsedAt > rows[j].LastUsedAt
		}
		if rows[i].AuthType != rows[j].AuthType {
			return rows[i].AuthType < rows[j].AuthType
		}
		return rows[i].Identity < rows[j].Identity
	})
	if len(rows) > analyticsKeyAliasBreakdownLimit {
		rows = rows[:analyticsKeyAliasBreakdownLimit]
	}

	breakdown := make([]dto.AnalyticsKeyAliasBreakdown, 0, len(rows))
	breakdownIndexes := make(map[analyticsIdentityKey]int, len(rows))
	breakdownKeys := make([]analyticsIdentityKey, 0, len(rows))
	for _, row := range rows {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		breakdownIndexes[key] = len(breakdown)
		breakdownKeys = append(breakdownKeys, key)
		breakdown = append(breakdown, mapAnalyticsKeyAliasBreakdown(row))
	}
	if len(breakdown) == 0 {
		return breakdown, nil
	}

	trends, err := buildAnalyticsCoreIdentityTrends(db, plan, filter, breakdownKeys, apiKeys)
	if err != nil {
		return nil, err
	}
	for key, points := range trends {
		index, ok := breakdownIndexes[key]
		if !ok {
			continue
		}
		breakdown[index].Trend = points
	}
	return breakdown, nil
}

func buildAnalyticsProviderOptionSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsProviderOptionRow, error) {
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
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics provider option segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsRollupProviderOptionSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsProviderOptionRow, error) {
	var rows []analyticsProviderOptionRow
	if err := analyticsRollupsWithPricingQuery(db, filter).
		Select(`
			TRIM(usage_rollups_hourly.provider) AS provider,
			COALESCE(SUM(usage_rollups_hourly.request_count), 0) AS request_count,
			COALESCE(SUM(usage_rollups_hourly.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsRollupCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsRollupMissingPricingSQLExpression("usage_rollups_hourly.request_count") + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsRollupPricedBillableSQLExpression("usage_rollups_hourly.request_count") + `), 0) AS priced_billable_events`).
		Where("TRIM(usage_rollups_hourly.provider) <> ''").
		Group("TRIM(usage_rollups_hourly.provider)").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics rollup provider option segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsModelSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsModelAggregateRow, error) {
	var rows []analyticsModelAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			TRIM(usage_events.model) AS model,
			TRIM(usage_events.provider) AS provider,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.input_tokens") + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.output_tokens") + `), 0) AS output_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.reasoning_tokens") + `), 0) AS reasoning_tokens,
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
		Group("TRIM(usage_events.model), TRIM(usage_events.provider)").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics model segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsRollupModelSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsModelAggregateRow, error) {
	var rows []analyticsModelAggregateRow
	if err := analyticsRollupsWithPricingQuery(db, filter).
		Select(`
			TRIM(usage_rollups_hourly.model) AS model,
			TRIM(usage_rollups_hourly.provider) AS provider,
			COALESCE(SUM(usage_rollups_hourly.request_count), 0) AS request_count,
			COALESCE(SUM(usage_rollups_hourly.success_count), 0) AS success_count,
			COALESCE(SUM(usage_rollups_hourly.failure_count), 0) AS failure_count,
			COALESCE(SUM(usage_rollups_hourly.input_tokens), 0) AS input_tokens,
			COALESCE(SUM(usage_rollups_hourly.output_tokens), 0) AS output_tokens,
			COALESCE(SUM(usage_rollups_hourly.reasoning_tokens), 0) AS reasoning_tokens,
			COALESCE(SUM(usage_rollups_hourly.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(usage_rollups_hourly.cached_tokens), 0) AS cached_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpressionFor("usage_rollups_hourly.cached_tokens") + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpressionFor("usage_rollups_hourly.cached_tokens", "usage_rollups_hourly.request_count") + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpressionFor("usage_rollups_hourly.cached_tokens", "usage_rollups_hourly.request_count") + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsRollupCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(usage_rollups_hourly.total_latency_ms), 0) AS total_latency_ms,
			COALESCE(SUM(usage_rollups_hourly.latency_sample_count), 0) AS latency_sample_count,
			COALESCE(SUM(` + analyticsRollupMissingPricingSQLExpression("usage_rollups_hourly.request_count") + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsRollupPricedBillableSQLExpression("usage_rollups_hourly.request_count") + `), 0) AS priced_billable_events`).
		Where("TRIM(usage_rollups_hourly.model) <> ''").
		Group("TRIM(usage_rollups_hourly.model), TRIM(usage_rollups_hourly.provider)").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics rollup model segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsKeyAliasSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsIdentityAggregateRow, error) {
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	var rows []analyticsIdentityAggregateRow
	if err := analyticsIdentityEventsWithPricingQuery(db, filter).
		Select(analyticsIdentityAggregateSelect(authTypeExpr, identityExpr, "usage_events", analyticsCostSQLExpression(), analyticsMissingPricingSQLExpression(), analyticsPricedBillableSQLExpression())).
		Group(authTypeExpr + ", " + identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics key alias segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsRollupKeyAliasSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsIdentityAggregateRow, error) {
	authTypeExpr := analyticsRollupUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsRollupUsageIdentitySQLExpression()
	var rows []analyticsIdentityAggregateRow
	if err := analyticsRollupIdentityWithPricingQuery(db, filter).
		Select(analyticsIdentityAggregateSelect(authTypeExpr, identityExpr, "usage_rollups_hourly", analyticsRollupCostSQLExpression(), analyticsRollupMissingPricingSQLExpression("usage_rollups_hourly.request_count"), analyticsRollupPricedBillableSQLExpression("usage_rollups_hourly.request_count"))).
		Group(authTypeExpr + ", " + identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics rollup key alias segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsAPIKeySegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsIdentityAggregateRow, error) {
	authTypeExpr := analyticsAPIKeyAuthTypeSQLExpression()
	identityExpr := analyticsAPIKeyIdentitySQLExpression()
	var rows []analyticsIdentityAggregateRow
	if err := analyticsAPIKeyEventsWithPricingQuery(db, filter).
		Select(analyticsAPIKeyAggregateSelect(authTypeExpr, identityExpr, "usage_events", analyticsCostSQLExpression(), analyticsMissingPricingSQLExpression(), analyticsPricedBillableSQLExpression())).
		Group(identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics api key segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsRollupAPIKeySegmentRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsIdentityAggregateRow, error) {
	authTypeExpr := analyticsAPIKeyAuthTypeSQLExpression()
	identityExpr := analyticsRollupAPIKeyIdentitySQLExpression()
	var rows []analyticsIdentityAggregateRow
	if err := analyticsRollupAPIKeyWithPricingQuery(db, filter).
		Select(analyticsAPIKeyAggregateSelect(authTypeExpr, identityExpr, "usage_rollups_hourly", analyticsRollupCostSQLExpression(), analyticsRollupMissingPricingSQLExpression("usage_rollups_hourly.request_count"), analyticsRollupPricedBillableSQLExpression("usage_rollups_hourly.request_count"))).
		Group(identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics rollup api key segment rows: %w", err)
	}
	return rows, nil
}

func buildAnalyticsCoreIdentityTrends(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.UsageQueryFilter, keys []analyticsIdentityKey, apiKeys bool) (map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint, error) {
	combined := map[analyticsIdentityTrendKey]analyticsIdentityTrendRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsIdentityTrendSegmentRows(db, rawFilter, keys, apiKeys, false)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityTrendRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsIdentityTrendSegmentRows(db, *plan.rollupFilter, keys, apiKeys, true)
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityTrendRows(combined, rows)
	}

	rowsByKey := map[analyticsIdentityKey][]analyticsIdentityTrendRow{}
	for _, row := range combined {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		rowsByKey[key] = append(rowsByKey[key], row)
	}
	trends := make(map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint, len(rowsByKey))
	for key, rows := range rowsByKey {
		sort.Slice(rows, func(i, j int) bool {
			return rows[i].Bucket < rows[j].Bucket
		})
		for _, row := range rows {
			costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
			trends[key] = append(trends[key], dto.AnalyticsKeyAliasTrendPoint{
				Label:         row.Bucket,
				TotalCost:     row.TotalCost,
				TotalTokens:   row.TotalTokens,
				CostAvailable: costAvailable,
				CostStatus:    costStatus,
			})
		}
	}
	return trends, nil
}

func buildAnalyticsIdentityTrendSegmentRows(db *gorm.DB, filter dto.UsageQueryFilter, keys []analyticsIdentityKey, apiKeys bool, rollup bool) ([]analyticsIdentityTrendRow, error) {
	if len(keys) == 0 {
		return []analyticsIdentityTrendRow{}, nil
	}
	bucketByDay := analyticsTrendBucketsByDay(filter)
	var query *gorm.DB
	var authTypeExpr string
	var identityExpr string
	var bucketExpr string
	var totalTokensExpr string
	var costExpr string
	var missingPricingExpr string
	var pricedBillableExpr string
	var groupExpr string
	if rollup {
		bucketExpr = analyticsRollupBucketSQLExpression(bucketByDay)
		totalTokensExpr = "usage_rollups_hourly.total_tokens"
		costExpr = analyticsRollupCostSQLExpression()
		missingPricingExpr = analyticsRollupMissingPricingSQLExpression("usage_rollups_hourly.request_count")
		pricedBillableExpr = analyticsRollupPricedBillableSQLExpression("usage_rollups_hourly.request_count")
		if apiKeys {
			authTypeExpr = analyticsAPIKeyAuthTypeSQLExpression()
			identityExpr = analyticsRollupAPIKeyIdentitySQLExpression()
			query = analyticsRollupAPIKeyWithPricingQuery(db, filter)
			groupExpr = identityExpr + ", bucket"
		} else {
			authTypeExpr = analyticsRollupUsageIdentityAuthTypeSQLExpression()
			identityExpr = analyticsRollupUsageIdentitySQLExpression()
			query = analyticsRollupIdentityWithPricingQuery(db, filter)
			groupExpr = authTypeExpr + ", " + identityExpr + ", bucket"
		}
	} else {
		bucketExpr = analyticsBucketSQLExpression(bucketByDay)
		totalTokensExpr = "usage_events.total_tokens"
		costExpr = analyticsCostSQLExpression()
		missingPricingExpr = analyticsMissingPricingSQLExpression()
		pricedBillableExpr = analyticsPricedBillableSQLExpression()
		if apiKeys {
			authTypeExpr = analyticsAPIKeyAuthTypeSQLExpression()
			identityExpr = analyticsAPIKeyIdentitySQLExpression()
			query = analyticsAPIKeyEventsWithPricingQuery(db, filter)
			groupExpr = identityExpr + ", bucket"
		} else {
			authTypeExpr = analyticsUsageIdentityAuthTypeSQLExpression()
			identityExpr = analyticsUsageIdentitySQLExpression()
			query = analyticsIdentityEventsWithPricingQuery(db, filter)
			groupExpr = authTypeExpr + ", " + identityExpr + ", bucket"
		}
	}

	var rows []analyticsIdentityTrendRow
	if err := applyAnalyticsIdentityKeyFilter(query, keys, authTypeExpr, identityExpr).
		Select(`
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			` + bucketExpr + ` AS bucket,
			COALESCE(SUM(` + totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + costExpr + `), 0) AS total_cost,
			COALESCE(SUM(` + missingPricingExpr + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + pricedBillableExpr + `), 0) AS priced_billable_events`).
		Group(groupExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics identity trend segment rows: %w", err)
	}
	return rows, nil
}

func analyticsRollupIdentityWithPricingQuery(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	authTypeExpr := analyticsRollupUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsRollupUsageIdentitySQLExpression()
	return analyticsRollupsWithPricingQuery(db, filter).
		Joins("LEFT JOIN usage_identities ON usage_identities.auth_type = " + authTypeExpr + " AND usage_identities.identity = " + identityExpr).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = " + authTypeExpr + " AND key_aliases.identity = " + identityExpr).
		Where(authTypeExpr + " <> 0").
		Where(identityExpr + " <> ''")
}

func analyticsRollupAPIKeyWithPricingQuery(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	identityExpr := analyticsRollupAPIKeyIdentitySQLExpression()
	return analyticsRollupsWithPricingQuery(db, filter).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = ? AND key_aliases.identity = "+identityExpr, entities.UsageIdentityAuthTypeAIProvider).
		Where(identityExpr + " <> ''")
}

func analyticsIdentityAggregateSelect(authTypeExpr string, identityExpr string, table string, costExpr string, missingPricingExpr string, pricedBillableExpr string) string {
	return `
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			COALESCE(MAX(key_aliases.alias), '') AS alias,
			COALESCE(MAX(usage_identities.name), '') AS name,
			COALESCE(MAX(usage_identities.auth_type_name), '') AS auth_type_name,
			COALESCE(MAX(usage_identities.type), '') AS type,
			COALESCE(MAX(usage_identities.provider), '') AS provider,
			COALESCE(MAX(usage_identities.prefix), '') AS prefix,
			COALESCE(MAX(usage_identities.base_url), '') AS base_url,
			COALESCE(MAX(CASE WHEN usage_identities.is_deleted THEN 1 ELSE 0 END), 0) AS is_deleted,
			` + analyticsRequestCountSQLExpression(table) + ` AS request_count,
			` + analyticsSuccessCountSQLExpression(table) + ` AS success_count,
			` + analyticsFailureCountSQLExpression(table) + ` AS failure_count,
			COALESCE(SUM(` + table + `.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + costExpr + `), 0) AS total_cost,
			COALESCE(SUM(` + missingPricingExpr + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + pricedBillableExpr + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', ` + analyticsLastUsedAtColumn(table) + `)) AS last_used_at`
}

func analyticsAPIKeyAggregateSelect(authTypeExpr string, identityExpr string, table string, costExpr string, missingPricingExpr string, pricedBillableExpr string) string {
	return `
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			COALESCE(MAX(key_aliases.alias), '') AS alias,
			'' AS name,
			'apikey' AS auth_type_name,
			'' AS type,
			COALESCE(MIN(NULLIF(TRIM(` + table + `.provider), '')), '') AS provider,
			'' AS prefix,
			'' AS base_url,
			0 AS is_deleted,
			` + analyticsRequestCountSQLExpression(table) + ` AS request_count,
			` + analyticsSuccessCountSQLExpression(table) + ` AS success_count,
			` + analyticsFailureCountSQLExpression(table) + ` AS failure_count,
			COALESCE(SUM(` + table + `.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + costExpr + `), 0) AS total_cost,
			COALESCE(SUM(` + missingPricingExpr + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + pricedBillableExpr + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', ` + analyticsLastUsedAtColumn(table) + `)) AS last_used_at`
}

func analyticsRequestCountSQLExpression(table string) string {
	if table == "usage_rollups_hourly" {
		return "COALESCE(SUM(usage_rollups_hourly.request_count), 0)"
	}
	return "COUNT(*)"
}

func analyticsSuccessCountSQLExpression(table string) string {
	if table == "usage_rollups_hourly" {
		return "COALESCE(SUM(usage_rollups_hourly.success_count), 0)"
	}
	return "COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0)"
}

func analyticsFailureCountSQLExpression(table string) string {
	if table == "usage_rollups_hourly" {
		return "COALESCE(SUM(usage_rollups_hourly.failure_count), 0)"
	}
	return "COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0)"
}

func analyticsLastUsedAtColumn(table string) string {
	if table == "usage_rollups_hourly" {
		return "usage_rollups_hourly.last_event_at"
	}
	return "usage_events.timestamp"
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

func addAnalyticsIdentityRows(dst map[analyticsIdentityKey]analyticsIdentityAggregateRow, rows []analyticsIdentityAggregateRow) {
	for _, row := range rows {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		combined := dst[key]
		combined.AuthType = row.AuthType
		combined.Identity = row.Identity
		combined.Alias = firstNonEmpty(combined.Alias, row.Alias)
		combined.Name = firstNonEmpty(combined.Name, row.Name)
		combined.AuthTypeName = firstNonEmpty(combined.AuthTypeName, row.AuthTypeName)
		combined.Type = firstNonEmpty(combined.Type, row.Type)
		combined.Provider = minNonEmpty(combined.Provider, row.Provider)
		combined.Prefix = firstNonEmpty(combined.Prefix, row.Prefix)
		combined.BaseURL = firstNonEmpty(combined.BaseURL, row.BaseURL)
		combined.IsDeleted = combined.IsDeleted || row.IsDeleted
		combined.RequestCount += row.RequestCount
		combined.SuccessCount += row.SuccessCount
		combined.FailureCount += row.FailureCount
		combined.TotalTokens += row.TotalTokens
		combined.TotalCost += row.TotalCost
		combined.MissingPricingEvents += row.MissingPricingEvents
		combined.PricedBillableEvents += row.PricedBillableEvents
		if row.LastUsedAt > combined.LastUsedAt {
			combined.LastUsedAt = row.LastUsedAt
		}
		dst[key] = combined
	}
}

func addAnalyticsIdentityTrendRows(dst map[analyticsIdentityTrendKey]analyticsIdentityTrendRow, rows []analyticsIdentityTrendRow) {
	for _, row := range rows {
		key := analyticsIdentityTrendKey{AuthType: row.AuthType, Identity: row.Identity, Bucket: row.Bucket}
		combined := dst[key]
		combined.AuthType = row.AuthType
		combined.Identity = row.Identity
		combined.Bucket = row.Bucket
		combined.TotalTokens += row.TotalTokens
		combined.TotalCost += row.TotalCost
		combined.MissingPricingEvents += row.MissingPricingEvents
		combined.PricedBillableEvents += row.PricedBillableEvents
		dst[key] = combined
	}
}

func firstNonEmpty(current string, next string) string {
	if current != "" {
		return current
	}
	return next
}

func minNonEmpty(current string, next string) string {
	if current == "" {
		return next
	}
	if next == "" || current <= next {
		return current
	}
	return next
}

func analyticsRollupCostSQLExpression() string {
	return analyticsCostSQLExpressionWithPromptTokens(
		"usage_rollups_hourly.billable_prompt_tokens",
		analyticsPositiveTokenSQLExpression("usage_rollups_hourly.output_tokens"),
		analyticsPositiveTokenSQLExpression("usage_rollups_hourly.cached_tokens"),
	)
}

func analyticsRollupMissingPricingSQLExpression(countExpression string) string {
	return analyticsMissingPricingSQLExpressionFor("usage_rollups_hourly.input_tokens", "usage_rollups_hourly.output_tokens", "usage_rollups_hourly.cached_tokens", countExpression)
}

func analyticsRollupPricedBillableSQLExpression(countExpression string) string {
	return analyticsPricedBillableSQLExpressionFor("usage_rollups_hourly.input_tokens", "usage_rollups_hourly.output_tokens", "usage_rollups_hourly.cached_tokens", countExpression)
}
