package repository

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type analyticsCoreWindowPlan struct {
	rawFilters   []dto.AnalyticsFilter
	rollupFilter *dto.AnalyticsFilter
}

func BuildAnalyticsCoreWithFilter(ctx context.Context, db *gorm.DB, filter dto.AnalyticsFilter) (*dto.AnalyticsSummarySnapshot, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	plan := analyticsCoreRollupWindowPlan(filter)
	if plan.rollupFilter != nil {
		allowed, detail, err := analyticsRollupReadAllowed(ctx, db, *plan.rollupFilter)
		if err != nil {
			return nil, err
		}
		if !allowed {
			logAnalyticsCoreRawFallback(filter, detail)
			plan = analyticsCoreRawWindowPlan(filter)
		}
	}
	return buildAnalyticsCoreSnapshot(db, filter, plan)
}

func buildAnalyticsCoreSnapshot(db *gorm.DB, filter dto.AnalyticsFilter, plan analyticsCoreWindowPlan) (*dto.AnalyticsSummarySnapshot, error) {
	summary, err := buildAnalyticsCoreSummary(db, plan)
	if err != nil {
		return nil, err
	}
	trend, err := buildAnalyticsCoreTrend(db, plan, filter)
	if err != nil {
		return nil, err
	}
	var keyAliasBreakdown []dto.AnalyticsKeyAliasBreakdown
	var apiKeyBreakdown []dto.AnalyticsKeyAliasBreakdown
	var modelBreakdown []dto.AnalyticsModelBreakdown
	if rawFilter, ok := plan.rawOnlyFilter(); ok {
		// The raw-only Adapter keeps SQL-side LIMIT 20 for high-cardinality identity and model
		// dimensions. Mixed and rollup plans must merge source segments before applying top-N.
		keyAliasBreakdown, err = buildAnalyticsKeyAliasBreakdown(db, rawFilter)
		if err != nil {
			return nil, err
		}
		apiKeyBreakdown, err = buildAnalyticsAPIKeyBreakdown(db, rawFilter)
		if err != nil {
			return nil, err
		}
		modelBreakdown, err = buildAnalyticsModelBreakdown(db, rawFilter)
		if err != nil {
			return nil, err
		}
	} else {
		keyAliasBreakdown, err = buildAnalyticsCoreKeyAliasBreakdown(db, plan, filter)
		if err != nil {
			return nil, err
		}
		apiKeyBreakdown, err = buildAnalyticsCoreAPIKeyBreakdown(db, plan, filter)
		if err != nil {
			return nil, err
		}
		modelBreakdown, err = buildAnalyticsCoreModelBreakdown(db, plan)
		if err != nil {
			return nil, err
		}
	}
	providerOptions, err := buildAnalyticsCoreProviderOptions(db, plan)
	if err != nil {
		return nil, err
	}
	return &dto.AnalyticsSummarySnapshot{
		Summary:           summary,
		Trend:             trend,
		KeyAliasBreakdown: keyAliasBreakdown,
		APIKeyBreakdown:   apiKeyBreakdown,
		ModelBreakdown:    modelBreakdown,
		TimeBreakdown:     trend,
		Insights:          buildAnalyticsInsights(summary, trend, keyAliasBreakdown, modelBreakdown),
		ProviderOptions:   providerOptions,
	}, nil
}

func analyticsCoreRawWindowPlan(filter dto.AnalyticsFilter) analyticsCoreWindowPlan {
	return analyticsCoreWindowPlan{rawFilters: []dto.AnalyticsFilter{filter}}
}

func (plan analyticsCoreWindowPlan) rawOnlyFilter() (dto.AnalyticsFilter, bool) {
	if plan.rollupFilter != nil || len(plan.rawFilters) != 1 {
		return dto.AnalyticsFilter{}, false
	}
	return plan.rawFilters[0], true
}

func analyticsRollupReadAllowed(ctx context.Context, db *gorm.DB, filter dto.AnalyticsFilter) (bool, string, error) {
	status, err := GetUsageRollupBackfillStatus(ctx, db)
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
	if status.CoveredBucketStart.UTC().Truncate(time.Hour).Before(status.TargetBucketStart.UTC().Truncate(time.Hour)) {
		return false, "covered_before_target", nil
	}
	return true, "", nil
}

func logAnalyticsCoreRawFallback(filter dto.AnalyticsFilter, detail string) {
	logAnalyticsRawFallback("analytics core raw fallback", filter, detail)
}

func logAnalyticsRawFallback(message string, filter dto.AnalyticsFilter, detail string) {
	attrs := []any{
		"reason", "backfill_incomplete",
		"detail", detail,
		"range", filter.Range,
		"granularity", filter.Granularity,
		"provider", filter.Provider,
	}
	if filter.StartTime != nil {
		attrs = append(attrs, "start_time", filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		attrs = append(attrs, "end_time", filter.EndTime.UTC())
	}
	slog.Warn(message, attrs...)
}

func analyticsCoreRollupWindowPlan(filter dto.AnalyticsFilter) analyticsCoreWindowPlan {
	if filter.StartTime == nil || filter.EndTime == nil {
		return analyticsCoreRawWindowPlan(filter)
	}
	start := filter.StartTime.UTC()
	end := filter.EndTime.UTC()
	if end.Before(start) {
		return analyticsCoreRawWindowPlan(filter)
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
		return analyticsCoreRawWindowPlan(filter)
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

func analyticsFilterWithWindow(filter dto.AnalyticsFilter, start time.Time, end time.Time) dto.AnalyticsFilter {
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
		row, err := buildAnalyticsAggregateRow(db, rawFilter, analyticsEventsAggregateSource())
		if err != nil {
			return dto.AnalyticsSummary{}, err
		}
		addAnalyticsAggregateRow(&combined, row)
	}
	if plan.rollupFilter != nil {
		row, err := buildAnalyticsAggregateRow(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
		if err != nil {
			return dto.AnalyticsSummary{}, err
		}
		addAnalyticsAggregateRow(&combined, row)
	}
	return mapAnalyticsSummary(combined), nil
}

func buildAnalyticsCoreTrend(db *gorm.DB, plan analyticsCoreWindowPlan, filter dto.AnalyticsFilter) ([]dto.AnalyticsTrendPoint, error) {
	bucketByDay := analyticsTrendBucketsByDay(filter)
	combined := map[string]analyticsAggregateRow{}
	for _, rawFilter := range plan.rawFilters {
		rows, err := buildAnalyticsAggregateRowsByBucket(db, rawFilter, analyticsEventsAggregateSource())
		if err != nil {
			return nil, err
		}
		addAnalyticsAggregateRowsByBucket(combined, rows)
	}
	if plan.rollupFilter != nil {
		rows, err := buildAnalyticsAggregateRowsByBucket(db, *plan.rollupFilter, analyticsRollupsAggregateSource())
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
	dst.CacheReadTokens += src.CacheReadTokens
	dst.CacheReadObservedInputTokens += src.CacheReadObservedInputTokens
	dst.ReasoningTokens += src.ReasoningTokens
	dst.CacheSavings += src.CacheSavings
	dst.CacheSavingsEligibleRows += src.CacheSavingsEligibleRows
	dst.CacheSavingsIneligibleRows += src.CacheSavingsIneligibleRows
	dst.MissingPricingEvents += src.MissingPricingEvents
	dst.PricedBillableEvents += src.PricedBillableEvents
}

func analyticsRollupsWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	return applyAnalyticsRollupQueryFilter(db.Model(&entities.UsageRollupHourly{}), filter).
		Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_rollups_hourly.model)")
}

func applyAnalyticsRollupQueryFilter(query *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	return applyAnalyticsRollupScopeFilter(query, filter.UsageTimeScope)
}

func applyAnalyticsRollupScopeFilter(query *gorm.DB, scope dto.UsageTimeScope) *gorm.DB {
	if scope.StartTime != nil {
		query = query.Where("usage_rollups_hourly.bucket_start >= ?", scope.StartTime.UTC())
	}
	if scope.EndTime != nil {
		query = query.Where("usage_rollups_hourly.bucket_start <= ?", scope.EndTime.UTC())
	}
	if provider := strings.TrimSpace(scope.Provider); provider != "" {
		query = query.Where("TRIM(usage_rollups_hourly.provider) = ?", provider)
	}
	return query
}
