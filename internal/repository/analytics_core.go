package repository

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"github.com/sirupsen/logrus"
	"gorm.io/gorm"
)

type analyticsCoreWindowPlan struct {
	rawFilters   []dto.UsageQueryFilter
	rollupFilter *dto.UsageQueryFilter
}

func BuildAnalyticsCoreWithFilter(db *gorm.DB, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}

	plan := analyticsCoreRollupWindowPlan(filter)
	if plan.rollupFilter == nil {
		return buildRawAnalyticsCore(db, filter)
	}
	allowed, detail, err := analyticsRollupReadAllowed(db, *plan.rollupFilter)
	if err != nil {
		return nil, err
	}
	if !allowed {
		logAnalyticsCoreRawFallback(filter, detail)
		return buildRawAnalyticsCore(db, filter)
	}

	summary, err := buildAnalyticsCoreSummary(db, plan)
	if err != nil {
		return nil, err
	}
	trend, err := buildAnalyticsCoreTrend(db, plan, filter)
	if err != nil {
		return nil, err
	}
	return &dto.AnalyticsSummarySnapshot{
		Summary:       summary,
		Trend:         trend,
		TimeBreakdown: trend,
	}, nil
}

func buildRawAnalyticsCore(db *gorm.DB, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	summary, err := buildAnalyticsSummary(db, filter)
	if err != nil {
		return nil, err
	}
	trend, err := buildAnalyticsTrend(db, filter)
	if err != nil {
		return nil, err
	}
	return &dto.AnalyticsSummarySnapshot{
		Summary:       summary,
		Trend:         trend,
		TimeBreakdown: trend,
	}, nil
}

func analyticsRollupReadAllowed(db *gorm.DB, filter dto.UsageQueryFilter) (bool, string, error) {
	status, err := GetUsageRollupBackfillStatus(db)
	if err != nil {
		return false, "", err
	}
	if status.Status != dto.RollupBackfillStatusCompleted {
		return false, "status_" + status.Status, nil
	}
	if status.TargetBucketStart == nil {
		return false, "missing_target_bucket", nil
	}
	if status.CoveredBucketStart == nil {
		return false, "missing_covered_bucket", nil
	}
	if filter.EndTime != nil && filter.EndTime.UTC().After(status.CoveredBucketStart.UTC()) {
		return false, "window_after_covered_bucket", nil
	}
	return true, "", nil
}

func logAnalyticsCoreRawFallback(filter dto.UsageQueryFilter, detail string) {
	fields := logrus.Fields{
		"reason":      "backfill_incomplete",
		"detail":      detail,
		"range":       filter.Range,
		"granularity": filter.Granularity,
		"provider":    filter.Provider,
	}
	if filter.StartTime != nil {
		fields["start_time"] = filter.StartTime.UTC()
	}
	if filter.EndTime != nil {
		fields["end_time"] = filter.EndTime.UTC()
	}
	logrus.WithFields(fields).Warn("analytics core raw fallback")
}

func analyticsCoreRollupWindowPlan(filter dto.UsageQueryFilter) analyticsCoreWindowPlan {
	if filter.StartTime == nil || filter.EndTime == nil {
		return analyticsCoreWindowPlan{rawFilters: []dto.UsageQueryFilter{filter}}
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	if end.Before(start) {
		return analyticsCoreWindowPlan{rawFilters: []dto.UsageQueryFilter{filter}}
	}

	firstFullBucket := start.Truncate(time.Hour)
	if firstFullBucket.Before(start) {
		firstFullBucket = firstFullBucket.Add(time.Hour)
	}
	endExclusive := end.Add(time.Nanosecond)
	lastFullBucket := end.Truncate(time.Hour)
	if lastFullBucket.Add(time.Hour).After(endExclusive) {
		lastFullBucket = lastFullBucket.Add(-time.Hour)
	}
	if lastFullBucket.Before(firstFullBucket) {
		return analyticsCoreWindowPlan{rawFilters: []dto.UsageQueryFilter{filter}}
	}

	plan := analyticsCoreWindowPlan{}
	if start.Before(firstFullBucket) {
		rawEnd := firstFullBucket.Add(-time.Nanosecond)
		plan.rawFilters = append(plan.rawFilters, analyticsFilterWithWindow(filter, start, minTime(rawEnd, end)))
	}
	rollupFilter := analyticsFilterWithWindow(filter, firstFullBucket, lastFullBucket)
	plan.rollupFilter = &rollupFilter
	tailStart := lastFullBucket.Add(time.Hour)
	if !tailStart.After(end) {
		plan.rawFilters = append(plan.rawFilters, analyticsFilterWithWindow(filter, tailStart, end))
	}
	return plan
}

func analyticsFilterWithWindow(filter dto.UsageQueryFilter, start time.Time, end time.Time) dto.UsageQueryFilter {
	filter.StartTime = &start
	filter.EndTime = &end
	return filter
}

func minTime(a time.Time, b time.Time) time.Time {
	if a.Before(b) {
		return a
	}
	return b
}

func buildAnalyticsCoreSummary(db *gorm.DB, plan analyticsCoreWindowPlan) (dto.AnalyticsSummary, error) {
	var combined analyticsAggregateRow
	for _, rawFilter := range plan.rawFilters {
		row, err := buildAnalyticsSummaryAggregateRow(db, rawFilter)
		if err != nil {
			return dto.AnalyticsSummary{}, err
		}
		addAnalyticsAggregateRow(&combined, row)
	}
	if plan.rollupFilter != nil {
		row, err := buildAnalyticsRollupSummaryAggregateRow(db, *plan.rollupFilter)
		if err != nil {
			return dto.AnalyticsSummary{}, err
		}
		addAnalyticsAggregateRow(&combined, row)
	}
	return mapAnalyticsSummary(combined), nil
}

func buildAnalyticsCoreTrend(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.UsageQueryFilter) ([]dto.AnalyticsTrendPoint, error) {
	bucketByDay := analyticsTrendBucketsByDay(filter)
	combined := map[string]analyticsAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsTrendAggregateRows(db, rawFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsAggregateRowsByBucket(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsRollupTrendAggregateRows(db, *plan.rollupFilter)
		if err != nil {
			return nil, err
		}
		addAnalyticsAggregateRowsByBucket(combined, rows)
	}

	buckets := make([]string, 0, len(combined))
	for bucket := range combined {
		buckets = append(buckets, bucket)
	}
	sort.Strings(buckets)
	trend := make([]dto.AnalyticsTrendPoint, 0, len(buckets))
	for _, bucket := range buckets {
		point, err := mapAnalyticsTrendPoint(combined[bucket], bucketByDay)
		if err != nil {
			return nil, err
		}
		trend = append(trend, point)
	}
	return trend, nil
}

func addAnalyticsAggregateRowsByBucket(dst map[string]analyticsAggregateRow, rows []analyticsAggregateRow) {
	for _, row := range rows {
		combined := dst[row.Bucket]
		addAnalyticsAggregateRow(&combined, row)
		combined.Bucket = row.Bucket
		dst[row.Bucket] = combined
	}
}

func addAnalyticsAggregateRow(dst *analyticsAggregateRow, src analyticsAggregateRow) {
	dst.RequestCount += src.RequestCount
	dst.SuccessCount += src.SuccessCount
	dst.FailureCount += src.FailureCount
	dst.InputTokens += src.InputTokens
	dst.OutputTokens += src.OutputTokens
	dst.TotalTokens += src.TotalTokens
	dst.TotalCost += src.TotalCost
	dst.CachedTokens += src.CachedTokens
	dst.ReasoningTokens += src.ReasoningTokens
	dst.CacheSavings += src.CacheSavings
	dst.CacheSavingsEligibleRows += src.CacheSavingsEligibleRows
	dst.CacheSavingsIneligibleRows += src.CacheSavingsIneligibleRows
	dst.MissingPricingEvents += src.MissingPricingEvents
	dst.PricedBillableEvents += src.PricedBillableEvents
}

func buildAnalyticsRollupSummaryAggregateRow(db *gorm.DB, filter dto.UsageQueryFilter) (analyticsAggregateRow, error) {
	var row analyticsAggregateRow
	if err := analyticsRollupsWithPricingQuery(db, filter).
		Select(analyticsRollupAggregateSelect()).
		Scan(&row).Error; err != nil {
		return analyticsAggregateRow{}, fmt.Errorf("build analytics rollup summary: %w", err)
	}
	return row, nil
}

func buildAnalyticsRollupTrendAggregateRows(db *gorm.DB, filter dto.UsageQueryFilter) ([]analyticsAggregateRow, error) {
	bucketExpr := analyticsRollupBucketSQLExpression(analyticsTrendBucketsByDay(filter))
	var rows []analyticsAggregateRow
	if err := analyticsRollupsWithPricingQuery(db, filter).
		Select(bucketExpr + " AS bucket,\n" + analyticsRollupAggregateSelect()).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics rollup trend: %w", err)
	}
	return rows, nil
}

func analyticsRollupsWithPricingQuery(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	return applyAnalyticsRollupQueryFilter(db.Model(&entities.UsageRollupHourly{}), filter).
		Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_rollups_hourly.model)")
}

func applyAnalyticsRollupQueryFilter(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	if filter.StartTime != nil {
		query = query.Where("usage_rollups_hourly.bucket_start >= ?", filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		query = query.Where("usage_rollups_hourly.bucket_start <= ?", filter.EndTime.UTC())
	}
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("TRIM(usage_rollups_hourly.provider) = ?", provider)
	}
	return query
}

func analyticsRollupAggregateSelect() string {
	requestCount := "usage_rollups_hourly.request_count"
	inputTokens := "usage_rollups_hourly.input_tokens"
	billablePromptTokens := "usage_rollups_hourly.billable_prompt_tokens"
	outputTokens := "usage_rollups_hourly.output_tokens"
	reasoningTokens := "usage_rollups_hourly.reasoning_tokens"
	cachedTokens := "usage_rollups_hourly.cached_tokens"
	return `
			COALESCE(SUM(` + requestCount + `), 0) AS request_count,
			COALESCE(SUM(usage_rollups_hourly.success_count), 0) AS success_count,
			COALESCE(SUM(usage_rollups_hourly.failure_count), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(inputTokens) + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(outputTokens) + `), 0) AS output_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(reasoningTokens) + `), 0) AS reasoning_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(cachedTokens) + `), 0) AS cached_tokens,
			COALESCE(SUM(usage_rollups_hourly.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpressionFor(cachedTokens) + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpressionFor(cachedTokens, requestCount) + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpressionFor(cachedTokens, requestCount) + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsCostSQLExpressionWithPromptTokens(billablePromptTokens, analyticsPositiveTokenSQLExpression(outputTokens), analyticsPositiveTokenSQLExpression(cachedTokens)) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpressionFor(inputTokens, outputTokens, cachedTokens, requestCount) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpressionFor(inputTokens, outputTokens, cachedTokens, requestCount) + `), 0) AS priced_billable_events`
}
