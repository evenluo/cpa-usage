package repository

import (
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"

	"gorm.io/gorm"
)

type analyticsAggregateRow struct {
	Bucket               string
	RequestCount         int64
	SuccessCount         int64
	FailureCount         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
}

type analyticsIdentityAggregateRow struct {
	AuthType             int
	Identity             string
	Alias                string
	Name                 string
	AuthTypeName         string
	Type                 string
	Provider             string
	Prefix               string
	BaseURL              string
	IsDeleted            bool
	RequestCount         int64
	SuccessCount         int64
	FailureCount         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
	LastUsedAt           string
}

type analyticsIdentityTrendRow struct {
	AuthType             int
	Identity             string
	Bucket               string
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
}

type analyticsIdentityKey struct {
	AuthType int
	Identity string
}

const analyticsKeyAliasBreakdownLimit = 20

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

	return &dto.AnalyticsSummarySnapshot{Summary: summary, Trend: trend, KeyAliasBreakdown: keyAliasBreakdown}, nil
}

func buildAnalyticsSummary(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsSummaryRecord, error) {
	var row analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Scan(&row).Error; err != nil {
		return dto.AnalyticsSummaryRecord{}, fmt.Errorf("build analytics summary: %w", err)
	}

	return mapAnalyticsSummary(row), nil
}

func buildAnalyticsTrend(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsTrendPointRecord, error) {
	bucketByDay := shouldBucketUsageOverviewByDay(filter, computeWindowMinutes(filter))
	bucketExpr := analyticsBucketSQLExpression(bucketByDay)
	var rows []analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			` + bucketExpr + ` AS bucket,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics trend: %w", err)
	}

	trend := make([]dto.AnalyticsTrendPointRecord, 0, len(rows))
	for _, row := range rows {
		point, err := mapAnalyticsTrendPoint(row, bucketByDay)
		if err != nil {
			return nil, err
		}
		trend = append(trend, point)
	}
	return trend, nil
}

func analyticsEventsWithPricingQuery(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	return applyUsageOverviewQuery(db.Model(&entities.UsageEvent{}), filter).
		Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_events.model)")
}

func analyticsIdentityEventsWithPricingQuery(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	return analyticsEventsWithPricingQuery(db, filter).
		Joins("LEFT JOIN usage_identities ON usage_identities.auth_type = " + authTypeExpr + " AND usage_identities.identity = " + identityExpr).
		Joins("LEFT JOIN key_aliases ON key_aliases.auth_type = " + authTypeExpr + " AND key_aliases.identity = " + identityExpr).
		Where(authTypeExpr + " <> 0").
		Where(identityExpr + " <> ''")
}

func buildAnalyticsKeyAliasBreakdown(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsKeyAliasBreakdownRecord, error) {
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
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', usage_events.timestamp)) AS last_used_at`).
		Group(authTypeExpr + ", " + identityExpr).
		Order("total_cost DESC").
		Order("COALESCE(SUM(usage_events.total_tokens), 0) DESC").
		Order("last_used_at DESC").
		Limit(analyticsKeyAliasBreakdownLimit).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics key alias breakdown: %w", err)
	}

	breakdown := make([]dto.AnalyticsKeyAliasBreakdownRecord, 0, len(rows))
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

func buildAnalyticsKeyAliasTrends(db *gorm.DB, filter dto.UsageQueryFilter, keys []analyticsIdentityKey) (map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPointRecord, error) {
	if len(keys) == 0 {
		return map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPointRecord{}, nil
	}
	authTypeExpr := analyticsUsageIdentityAuthTypeSQLExpression()
	identityExpr := analyticsUsageIdentitySQLExpression()
	bucketByDay := shouldBucketUsageOverviewByDay(filter, computeWindowMinutes(filter))
	bucketExpr := analyticsBucketSQLExpression(bucketByDay)
	var rows []analyticsIdentityTrendRow
	if err := applyAnalyticsIdentityKeyFilter(analyticsIdentityEventsWithPricingQuery(db, filter), keys, authTypeExpr, identityExpr).
		Select(`
			` + authTypeExpr + ` AS auth_type,
			` + identityExpr + ` AS identity,
			` + bucketExpr + ` AS bucket,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Group(authTypeExpr + ", " + identityExpr + ", bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics key alias trends: %w", err)
	}

	trends := make(map[analyticsIdentityKey][]dto.AnalyticsKeyAliasTrendPointRecord)
	for _, row := range rows {
		key := analyticsIdentityKey{AuthType: row.AuthType, Identity: row.Identity}
		costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
		trends[key] = append(trends[key], dto.AnalyticsKeyAliasTrendPointRecord{
			Label:         row.Bucket,
			TotalCost:     row.TotalCost,
			TotalTokens:   row.TotalTokens,
			CostAvailable: costAvailable,
			CostStatus:    costStatus,
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

func analyticsCostSQLExpression() string {
	inputTokens := analyticsPositiveTokenSQLExpression("usage_events.input_tokens")
	outputTokens := analyticsPositiveTokenSQLExpression("usage_events.output_tokens")
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	promptTokens := "(CASE WHEN " + inputTokens + " - " + cachedTokens + " > 0 THEN " + inputTokens + " - " + cachedTokens + " ELSE 0 END)"
	return `CASE
		WHEN model_price_settings.id IS NULL THEN 0
		ELSE
			(` + promptTokens + ` / 1000000.0) * model_price_settings.prompt_price_per1_m +
			(` + outputTokens + ` / 1000000.0) * model_price_settings.completion_price_per1_m +
			(` + cachedTokens + ` / 1000000.0) * model_price_settings.cache_price_per1_m
	END`
}

func analyticsPositiveTokenSQLExpression(column string) string {
	return "(CASE WHEN " + column + " > 0 THEN " + column + " ELSE 0 END)"
}

func analyticsMissingPricingSQLExpression() string {
	return `CASE
		WHEN model_price_settings.id IS NULL
			AND (usage_events.input_tokens > 0 OR usage_events.output_tokens > 0 OR usage_events.cached_tokens > 0)
		THEN 1
		ELSE 0
	END`
}

func analyticsPricedBillableSQLExpression() string {
	return `CASE
		WHEN model_price_settings.id IS NOT NULL
			AND (usage_events.input_tokens > 0 OR usage_events.output_tokens > 0 OR usage_events.cached_tokens > 0)
		THEN 1
		ELSE 0
	END`
}

func analyticsBucketSQLExpression(bucketByDay bool) string {
	if bucketByDay {
		return "strftime('%Y-%m-%d', usage_events.timestamp, 'localtime')"
	}
	return "strftime('%Y-%m-%dT%H:00:00Z', usage_events.timestamp)"
}

func analyticsUsageIdentityAuthTypeSQLExpression() string {
	return `(CASE
		WHEN TRIM(usage_events.auth_type) = 'oauth' THEN 1
		WHEN TRIM(usage_events.auth_type) = 'apikey' THEN 2
		ELSE 0
	END)`
}

func analyticsUsageIdentitySQLExpression() string {
	return "TRIM(usage_events.auth_index)"
}

func mapAnalyticsSummary(row analyticsAggregateRow) dto.AnalyticsSummaryRecord {
	summary := dto.AnalyticsSummaryRecord{
		TotalCost:    row.TotalCost,
		TotalTokens:  row.TotalTokens,
		RequestCount: row.RequestCount,
		SuccessCount: row.SuccessCount,
		FailureCount: row.FailureCount,
	}
	if row.RequestCount > 0 {
		summary.SuccessRate = (float64(row.SuccessCount) / float64(row.RequestCount)) * 100
	}
	summary.CostAvailable, summary.CostStatus = analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	return summary
}

func mapAnalyticsTrendPoint(row analyticsAggregateRow, bucketByDay bool) (dto.AnalyticsTrendPointRecord, error) {
	var bucketStart time.Time
	var err error
	var bucketEnd time.Time
	if bucketByDay {
		bucketStart, err = time.ParseInLocation(time.DateOnly, row.Bucket, time.Local)
		bucketEnd = bucketStart.AddDate(0, 0, 1)
	} else {
		bucketStart, err = time.Parse(time.RFC3339, row.Bucket)
		bucketEnd = bucketStart.Add(time.Hour)
	}
	if err != nil {
		return dto.AnalyticsTrendPointRecord{}, fmt.Errorf("parse analytics trend bucket %q: %w", row.Bucket, err)
	}
	costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	return dto.AnalyticsTrendPointRecord{
		Label:         row.Bucket,
		BucketStart:   bucketStart.UTC(),
		BucketEnd:     bucketEnd.UTC(),
		TotalCost:     row.TotalCost,
		TotalTokens:   row.TotalTokens,
		RequestCount:  row.RequestCount,
		SuccessCount:  row.SuccessCount,
		FailureCount:  row.FailureCount,
		CostAvailable: costAvailable,
		CostStatus:    costStatus,
	}, nil
}

func mapAnalyticsKeyAliasBreakdown(row analyticsIdentityAggregateRow) dto.AnalyticsKeyAliasBreakdownRecord {
	record := dto.AnalyticsKeyAliasBreakdownRecord{
		AuthType:     row.AuthType,
		Identity:     row.Identity,
		Alias:        row.Alias,
		Name:         row.Name,
		AuthTypeName: row.AuthTypeName,
		Type:         row.Type,
		Provider:     row.Provider,
		Prefix:       row.Prefix,
		BaseURL:      row.BaseURL,
		IsDeleted:    row.IsDeleted,
		TotalCost:    row.TotalCost,
		TotalTokens:  row.TotalTokens,
		RequestCount: row.RequestCount,
		SuccessCount: row.SuccessCount,
		FailureCount: row.FailureCount,
		LastUsedAt:   parseAnalyticsTimestamp(row.LastUsedAt),
		Trend:        []dto.AnalyticsKeyAliasTrendPointRecord{},
	}
	if row.RequestCount > 0 {
		record.SuccessRate = (float64(row.SuccessCount) / float64(row.RequestCount)) * 100
	}
	record.CostAvailable, record.CostStatus = analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	return record
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

func analyticsCostAvailability(missingPricingEvents int64, pricedBillableEvents int64) (bool, string) {
	if missingPricingEvents == 0 {
		return true, dto.AnalyticsCostStatusAvailable
	}
	if pricedBillableEvents > 0 {
		return false, dto.AnalyticsCostStatusPartial
	}
	return false, dto.AnalyticsCostStatusUnavailable
}
