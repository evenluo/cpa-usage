package repository

import (
	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

func analyticsIdentityEventsWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	return analyticsEventsWithPricingQuery(db, filter).
		Joins("LEFT JOIN usage_identities ON usage_identities.auth_type = " + authTypeExpr + " AND usage_identities.identity = " + identityExpr).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = " + authTypeExpr + " AND key_aliases.identity = " + identityExpr).
		Where(authTypeExpr + " <> 0").
		Where(identityExpr + " <> ''")
}

func buildAnalyticsKeyAliasBreakdown(db *gorm.DB, filter dto.AnalyticsFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	source := analyticsEventsAggregateSource()
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	var rows []analyticsIdentityAggregateRow
	if err := analyticsIdentityEventsWithPricingQuery(db, filter).
		Select(`
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
			COUNT(*) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', usage_events.timestamp)) AS last_used_at`).
		Group(authTypeExpr + ", " + identityExpr).
		Order("total_cost DESC").
		Order(analyticsTotalTokensDescOrder(source)).
		Order("last_used_at DESC").
		Limit(analyticsKeyAliasBreakdownLimit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics key alias breakdown: %w", err)
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

	trends, err := buildAnalyticsKeyAliasTrends(db, filter, breakdownKeys)
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

func buildAnalyticsKeyAliasTrends(db *gorm.DB, filter dto.AnalyticsFilter, keys []analyticsIdentityKey) (map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint, error) {
	if len(keys) == 0 {
		return map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint{}, nil
	}
	source := analyticsEventsAggregateSource()
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	bucketByDay := analyticsTrendBucketsByDay(filter)
	bucketExpr := analyticsBucketSQLExpression(bucketByDay)
	var rows []analyticsIdentityTrendRow
	if err := applyAnalyticsIdentityKeyFilter(analyticsIdentityEventsWithPricingQuery(db, filter), keys, authTypeExpr, identityExpr).
		Select(`
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			` + bucketExpr + ` AS bucket,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
		Group(authTypeExpr + ", " + identityExpr + ", bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics key alias trends: %w", err)
	}

	trends := make(map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint)
	for _, row := range rows {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		cost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
		trends[key] = append(trends[key], dto.AnalyticsKeyAliasTrendPoint{
			Label:         row.Bucket,
			TotalCost:     row.TotalCost,
			TotalTokens:   row.TotalTokens,
			CostAvailable: cost.Available,
			CostStatus:    cost.Status,
		})
	}
	return trends, nil
}

func analyticsAPIKeyEventsWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	identityExpr := analyticsAPIKeyIdentitySQLExpression()
	return analyticsEventsWithPricingQuery(db, filter).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = ? AND key_aliases.identity = "+identityExpr, entities.UsageIdentityAuthTypeAIProvider).
		Where(identityExpr + " <> ''")
}

func buildAnalyticsAPIKeyBreakdown(db *gorm.DB, filter dto.AnalyticsFilter) ([]dto.AnalyticsKeyAliasBreakdown, error) {
	source := analyticsEventsAggregateSource()
	var factRows []apiKeyAggregateFactRow
	if err := apiKeyAggregateFactsQuery(db, filter, source).
		Order("total_cost DESC").
		Order(analyticsTotalTokensDescOrder(source)).
		Order("last_used_at DESC").
		Limit(analyticsKeyAliasBreakdownLimit).
		Scan(&factRows).Error; err != nil {
		return nil, fmt.Errorf("build analytics api key breakdown: %w", err)
	}
	rows := make([]analyticsIdentityAggregateRow, 0, len(factRows))
	for _, row := range factRows {
		rows = append(rows, analyticsIdentityAggregateRowFromAPIKeyFact(row))
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

	trends, err := buildAnalyticsAPIKeyTrends(db, filter, breakdownKeys)
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

func buildAnalyticsAPIKeyTrends(db *gorm.DB, filter dto.AnalyticsFilter, keys []analyticsIdentityKey) (map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint, error) {
	if len(keys) == 0 {
		return map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint{}, nil
	}
	source := analyticsEventsAggregateSource()
	authTypeExpr := analyticsAPIKeyAuthTypeSQLExpression()
	identityExpr := analyticsAPIKeyIdentitySQLExpression()
	bucketByDay := analyticsTrendBucketsByDay(filter)
	bucketExpr := analyticsBucketSQLExpression(bucketByDay)
	var rows []analyticsIdentityTrendRow
	if err := applyAnalyticsIdentityKeyFilter(analyticsAPIKeyEventsWithPricingQuery(db, filter), keys, authTypeExpr, identityExpr).
		Select(`
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			` + bucketExpr + ` AS bucket,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`).
		Group(identityExpr + ", bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics api key trends: %w", err)
	}

	trends := make(map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPoint)
	for _, row := range rows {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		cost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
		trends[key] = append(trends[key], dto.AnalyticsKeyAliasTrendPoint{
			Label:         row.Bucket,
			TotalCost:     row.TotalCost,
			TotalTokens:   row.TotalTokens,
			CostAvailable: cost.Available,
			CostStatus:    cost.Status,
		})
	}
	return trends, nil
}
func applyAnalyticsIdentityKeyFilter(query *gorm.DB, keys []analyticsIdentityKey, authTypeExpr string, identityExpr string) *gorm.DB {
	conditions := make([]string, 0, len(keys))
	args := make([]any, 0, len(keys)*2)
	for _, key := range keys {
		conditions = append(conditions, "("+authTypeExpr+" = ? AND "+identityExpr+" = ?)")
		args = append(args, key.AuthType, key.Identity)
	}
	return query.Where(strings.Join(conditions, " OR "), args...)
}
func mapAnalyticsKeyAliasBreakdown(row analyticsIdentityAggregateRow) dto.AnalyticsKeyAliasBreakdown {
	authType := entities.UsageIdentityAuthType(row.AuthType)
	maskedIdentity := analyticsMaskedIdentity(authType, row.Identity)
	label := analyticsKeyAliasLabel(authType, row, maskedIdentity)
	record := dto.AnalyticsKeyAliasBreakdown{
		Label:          label,
		Traceability:   analyticsKeyAliasTraceability(maskedIdentity, row.Provider),
		MaskedIdentity: maskedIdentity,
		AuthType:       row.AuthType,
		Identity:       row.Identity,
		Alias:          row.Alias,
		Name:           row.Name,
		AuthTypeName:   row.AuthTypeName,
		Type:           row.Type,
		Provider:       row.Provider,
		Prefix:         row.Prefix,
		BaseURL:        row.BaseURL,
		IsDeleted:      row.IsDeleted,
		TotalCost:      row.TotalCost,
		TotalTokens:    row.TotalTokens,
		RequestCount:   row.RequestCount,
		SuccessCount:   row.SuccessCount,
		FailureCount:   row.FailureCount,
		LastUsedAt:     parseAnalyticsTimestamp(row.LastUsedAt),
		Trend:          []dto.AnalyticsKeyAliasTrendPoint{},
	}
	if row.RequestCount > 0 {
		record.SuccessRate = (float64(row.SuccessCount) / float64(row.RequestCount)) * 100
	}
	cost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
	record.CostAvailable, record.CostStatus = cost.Available, cost.Status
	return record
}

func analyticsMaskedIdentity(authType entities.UsageIdentityAuthType, identity string) string {
	if authType == entities.UsageIdentityAuthTypeAIProvider {
		return redact.APIKeyDisplayName(identity)
	}
	return strings.TrimSpace(identity)
}

func analyticsKeyAliasLabel(authType entities.UsageIdentityAuthType, row analyticsIdentityAggregateRow, maskedIdentity string) string {
	if alias := strings.TrimSpace(row.Alias); alias != "" {
		return alias
	}
	displayName := entities.UsageIdentity{
		Name:     row.Name,
		AuthType: authType,
		Type:     row.Type,
		Provider: row.Provider,
		Prefix:   row.Prefix,
		BaseURL:  row.BaseURL,
	}.DisplayName()
	if strings.TrimSpace(displayName) != "" {
		return displayName
	}
	return maskedIdentity
}

func analyticsKeyAliasTraceability(maskedIdentity string, provider string) string {
	traceability := strings.TrimSpace(maskedIdentity)
	if provider := strings.TrimSpace(provider); provider != "" {
		traceability += " · " + provider
	}
	return traceability
}

func parseAnalyticsTimestamp(value string) *time.Time {
	if value == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil
	}
	return &parsed
}
