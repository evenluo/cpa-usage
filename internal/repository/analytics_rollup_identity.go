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

func buildAnalyticsCoreKeyAliasBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.AnalyticsFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	combined := map[analyticsIdentityKey]analyticsIdentityAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsKeyAliasSegmentRows(db, rawFilter, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsKeyAliasSegmentRows(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	return mapAnalyticsCoreIdentityBreakdown(db, plan, filter, combined, false)
}

func buildAnalyticsCoreAPIKeyBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.AnalyticsFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	combined := map[analyticsIdentityKey]analyticsIdentityAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsAPIKeySegmentRows(db, rawFilter, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsAPIKeySegmentRows(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityRows(combined, rows)
	}
	return mapAnalyticsCoreIdentityBreakdown(db, plan, filter, combined, true)
}

func mapAnalyticsCoreIdentityBreakdown(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.AnalyticsFilter, combined map[analyticsIdentityKey]analyticsIdentityAggregateRow, apiKeys bool) ([]dto.AnalyticsKeyAliasBreakdown, error) {
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

// buildAnalyticsKeyAliasSegmentRows 按聚合源渲染 Key Alias 段查询，raw 与 rollup 共用同一份列定义。
func buildAnalyticsKeyAliasSegmentRows(db *gorm.DB, filter dto.AnalyticsFilter, source analyticsAggregateSource) ([]analyticsIdentityAggregateRow, error) {
	var rows []analyticsIdentityAggregateRow
	if err := source.identityQuery(db, filter).
		Select(analyticsIdentityAggregateSelect(source, source.identityAuthTypeExpr, source.identityExpr)).
		Group(source.identityAuthTypeExpr + ", " + source.identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s key alias segment rows: %w", source.name, err)
	}
	return rows, nil
}

// buildAnalyticsAPIKeySegmentRows 按聚合源渲染 API Key 段查询，raw 与 rollup 共用同一份列定义。
func buildAnalyticsAPIKeySegmentRows(db *gorm.DB, filter dto.AnalyticsFilter, source analyticsAggregateSource) ([]analyticsIdentityAggregateRow, error) {
	identityExpr := source.apiKeyIdentityExpr
	var rows []analyticsIdentityAggregateRow
	if err := source.apiKeyQuery(db, filter).
		Select(analyticsAPIKeyAggregateSelect(source, analyticsAPIKeyAuthTypeSQLExpression(), identityExpr)).
		Group(identityExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s api key segment rows: %w", source.name, err)
	}
	return rows, nil
}

func buildAnalyticsCoreIdentityTrends(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.AnalyticsFilter, keys []analyticsIdentityKey, apiKeys bool) (map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint, error) {
	combined := map[analyticsIdentityTrendKey]analyticsIdentityTrendRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsIdentityTrendSegmentRows(db, rawFilter, keys, apiKeys, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsIdentityTrendRows(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsIdentityTrendSegmentRows(db, *plan.rollupFilter, keys, apiKeys, analyticsRollupsAggregateSource())
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

func buildAnalyticsIdentityTrendSegmentRows(db *gorm.DB, filter dto.AnalyticsFilter, keys []analyticsIdentityKey, apiKeys bool, source analyticsAggregateSource) ([]analyticsIdentityTrendRow, error) {
	if len(keys) == 0 {
		return []analyticsIdentityTrendRow{}, nil
	}
	bucketExpr := source.bucketExpr(analyticsTrendBucketsByDay(filter))
	var query *gorm.DB
	var authTypeExpr string
	var identityExpr string
	var groupExpr string
	if apiKeys {
		authTypeExpr = analyticsAPIKeyAuthTypeSQLExpression()
		identityExpr = source.apiKeyIdentityExpr
		query = source.apiKeyQuery(db, filter)
		groupExpr = identityExpr + ", bucket"
	} else {
		authTypeExpr = source.identityAuthTypeExpr
		identityExpr = source.identityExpr
		query = source.identityQuery(db, filter)
		groupExpr = authTypeExpr + ", " + identityExpr + ", bucket"
	}

	var rows []analyticsIdentityTrendRow
	if err := applyAnalyticsIdentityKeyFilter(query, keys, authTypeExpr, identityExpr).
		Select(`
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			` + bucketExpr + ` AS bucket,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
		Group(groupExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s identity trend segment rows: %w", source.name, err)
	}
	return rows, nil
}

func analyticsRollupIdentityWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	authTypeExpr := analyticsRollupUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsRollupUsageIdentitySQLExpression()
	return analyticsRollupsWithPricingQuery(db, filter).
		Joins("LEFT JOIN usage_identities ON usage_identities.auth_type = " + authTypeExpr + " AND usage_identities.identity = " + identityExpr).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = " + authTypeExpr + " AND key_aliases.identity = " + identityExpr).
		Where(authTypeExpr + " <> 0").
		Where(identityExpr + " <> ''")
}

func analyticsRollupAPIKeyWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	identityExpr := analyticsRollupAPIKeyIdentitySQLExpression()
	return analyticsRollupsWithPricingQuery(db, filter).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = ? AND key_aliases.identity = "+identityExpr, entities.UsageIdentityAuthTypeAIProvider).
		Where(identityExpr + " <> ''")
}

// analyticsIdentityAggregateSelect 按聚合源渲染 Key Alias 维度的身份聚合列。
func analyticsIdentityAggregateSelect(source analyticsAggregateSource, authTypeExpr string, identityExpr string) string {
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
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', ` + source.lastUsedAtExpr + `)) AS last_used_at`
}

// analyticsAPIKeyAggregateSelect 按聚合源渲染 API Key 维度的身份聚合列。
func analyticsAPIKeyAggregateSelect(source analyticsAggregateSource, authTypeExpr string, identityExpr string) string {
	return `
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			COALESCE(MAX(key_aliases.alias), '') AS alias,
			'' AS name,
			'apikey' AS auth_type_name,
			'' AS type,
			COALESCE(MIN(NULLIF(` + source.providerExpr + `, '')), '') AS provider,
			'' AS prefix,
			'' AS base_url,
			0 AS is_deleted,
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', ` + source.lastUsedAtExpr + `)) AS last_used_at`
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
