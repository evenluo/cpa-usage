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
	Bucket                     string
	RequestCount               int64
	SuccessCount               int64
	FailureCount               int64
	InputTokens                int64
	TotalTokens                int64
	TotalCost                  float64
	CachedTokens               int64
	ReasoningTokens            int64
	CacheSavings               float64
	CacheSavingsEligibleRows   int64
	CacheSavingsIneligibleRows int64
	MissingPricingEvents       int64
	PricedBillableEvents       int64
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

type analyticsModelAggregateRow struct {
	Model                      string
	Provider                   string
	ProviderCount              int64
	RequestCount               int64
	SuccessCount               int64
	FailureCount               int64
	InputTokens                int64
	TotalTokens                int64
	TotalCost                  float64
	CachedTokens               int64
	CacheSavings               float64
	CacheSavingsEligibleRows   int64
	CacheSavingsIneligibleRows int64
	TotalLatencyMS             int64
	LatencySampleCount         int64
	MissingPricingEvents       int64
	PricedBillableEvents       int64
}

type analyticsProviderOptionRow struct {
	Provider             string
	RequestCount         int64
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
	modelBreakdown, err := buildAnalyticsModelBreakdown(db, filter)
	if err != nil {
		return nil, err
	}
	providerOptions, err := buildAnalyticsProviderOptions(db, filter)
	if err != nil {
		return nil, err
	}
	previousRangeStart, previousRangeEnd, comparison, err := buildAnalyticsComparison(db, filter, summary)
	if err != nil {
		return nil, err
	}
	heatmap, err := buildAnalyticsHeatmap(db, filter)
	if err != nil {
		return nil, err
	}

	return &dto.AnalyticsSummarySnapshot{
		Summary:            summary,
		Trend:              trend,
		KeyAliasBreakdown:  keyAliasBreakdown,
		ModelBreakdown:     modelBreakdown,
		TimeBreakdown:      trend,
		Insights:           buildAnalyticsInsights(summary, trend, keyAliasBreakdown, modelBreakdown),
		ProviderOptions:    providerOptions,
		PreviousRangeStart: previousRangeStart,
		PreviousRangeEnd:   previousRangeEnd,
		Comparison:         comparison,
		Heatmap:            heatmap,
	}, nil
}

func buildAnalyticsSummary(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsSummaryRecord, error) {
	var row analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
				COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
				COALESCE(SUM(` + analyticsPositiveTokenSQLExpression("usage_events.input_tokens") + `), 0) AS input_tokens,
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
		return dto.AnalyticsSummaryRecord{}, fmt.Errorf("build analytics summary: %w", err)
	}

	return mapAnalyticsSummary(row), nil
}

func buildAnalyticsComparison(db *gorm.DB, filter dto.UsageQueryFilter, current dto.AnalyticsSummaryRecord) (*time.Time, *time.Time, dto.AnalyticsComparisonRecord, error) {
	previousFilter, ok := analyticsPreviousPeriodFilter(filter)
	if !ok {
		return nil, nil, dto.AnalyticsComparisonRecord{}, nil
	}
	previous, err := buildAnalyticsSummary(db, previousFilter)
	if err != nil {
		return nil, nil, dto.AnalyticsComparisonRecord{}, err
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

func mapAnalyticsComparison(current dto.AnalyticsSummaryRecord, previous dto.AnalyticsSummaryRecord) dto.AnalyticsComparisonRecord {
	if previous.RequestCount <= 0 {
		return dto.AnalyticsComparisonRecord{HasPreviousPeriod: false}
	}
	return dto.AnalyticsComparisonRecord{
		HasPreviousPeriod:     true,
		TotalCostChangePct:    analyticsCostPercentChange(current, previous),
		TotalTokensChangePct:  analyticsPercentChange(float64(current.TotalTokens), float64(previous.TotalTokens)),
		RequestCountChangePct: analyticsPercentChange(float64(current.RequestCount), float64(previous.RequestCount)),
		SuccessRateChangePP:   analyticsPointChange(current.SuccessRate, previous.SuccessRate),
	}
}

func analyticsCostPercentChange(current dto.AnalyticsSummaryRecord, previous dto.AnalyticsSummaryRecord) *float64 {
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

func buildAnalyticsHeatmap(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsHeatmapRecord, error) {
	heatmap := dto.AnalyticsHeatmapRecord{Measure: "tokens", Rows: []dto.AnalyticsHeatmapRowRecord{}}
	if filter.StartTime == nil || filter.EndTime == nil {
		return heatmap, nil
	}

	windowStart := filter.StartTime.UTC()
	windowEnd := filter.EndTime.UTC()
	startDay := localDateStart(filter.StartTime.In(time.Local))
	endDay := localDateStart(filter.EndTime.In(time.Local))
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		row := dto.AnalyticsHeatmapRowRecord{
			Date:  day.Format(time.DateOnly),
			Label: day.Format("Mon 01/02"),
			Cells: make([]dto.AnalyticsHeatmapCellRecord, 0, 24),
		}
		for hour := 0; hour < 24; hour++ {
			bucketStart := time.Date(day.Year(), day.Month(), day.Day(), hour, 0, 0, 0, time.Local)
			bucketEnd := bucketStart.Add(time.Hour)
			cell := dto.AnalyticsHeatmapCellRecord{
				Hour:          hour,
				InRange:       analyticsHeatmapCellInRange(bucketStart.UTC(), bucketEnd.UTC(), windowStart, windowEnd),
				BucketStart:   bucketStart.UTC(),
				BucketEnd:     bucketEnd.UTC(),
				CostAvailable: true,
				CostStatus:    dto.AnalyticsCostStatusAvailable,
			}
			if cell.InRange {
				if err := fillAnalyticsHeatmapCell(db, filter, &cell); err != nil {
					return dto.AnalyticsHeatmapRecord{}, err
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		heatmap.Rows = append(heatmap.Rows, row)
	}

	for _, row := range heatmap.Rows {
		for _, cell := range row.Cells {
			if cell.TotalTokens > heatmap.MaxTokens {
				heatmap.MaxTokens = cell.TotalTokens
			}
			if cell.TotalCost > heatmap.MaxCost {
				heatmap.MaxCost = cell.TotalCost
			}
			if cell.RequestCount > heatmap.MaxRequests {
				heatmap.MaxRequests = cell.RequestCount
			}
			if cell.FailureCount > heatmap.MaxFailures {
				heatmap.MaxFailures = cell.FailureCount
			}
		}
	}
	return heatmap, nil
}

func analyticsHeatmapCellInRange(bucketStart time.Time, bucketEnd time.Time, windowStart time.Time, windowEnd time.Time) bool {
	return bucketEnd.After(windowStart) && !bucketStart.After(windowEnd)
}

func fillAnalyticsHeatmapCell(db *gorm.DB, filter dto.UsageQueryFilter, cell *dto.AnalyticsHeatmapCellRecord) error {
	cellStart := cell.BucketStart
	if filter.StartTime != nil && filter.StartTime.UTC().After(cellStart) {
		cellStart = filter.StartTime.UTC()
	}
	cellEnd := cell.BucketEnd.Add(-time.Nanosecond)
	if filter.EndTime != nil && filter.EndTime.UTC().Before(cellEnd) {
		cellEnd = filter.EndTime.UTC()
	}
	if cellEnd.Before(cellStart) {
		return nil
	}

	cellFilter := filter
	cellFilter.StartTime = &cellStart
	cellFilter.EndTime = &cellEnd
	var row analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, cellFilter).
		Select(`
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events`).
		Scan(&row).Error; err != nil {
		return fmt.Errorf("build analytics heatmap cell: %w", err)
	}
	cell.TotalTokens = row.TotalTokens
	cell.TotalCost = row.TotalCost
	cell.RequestCount = row.RequestCount
	cell.FailureCount = row.FailureCount
	cell.CostAvailable, cell.CostStatus = analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	return nil
}

func localDateStart(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func buildAnalyticsTrend(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsTrendPointRecord, error) {
	bucketByDay := analyticsTrendBucketsByDay(filter)
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
	return applyAnalyticsQueryFilter(db.Model(&entities.UsageEvent{}), filter).
		Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_events.model)")
}

func applyAnalyticsQueryFilter(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	query = applyUsageOverviewQuery(query, filter)
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("TRIM(usage_events.provider) = ?", provider)
	}
	return query
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
	bucketByDay := analyticsTrendBucketsByDay(filter)
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

func buildAnalyticsModelBreakdown(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsModelBreakdownRecord, error) {
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

	breakdown := make([]dto.AnalyticsModelBreakdownRecord, 0, len(rows))
	for _, row := range rows {
		breakdown = append(breakdown, mapAnalyticsModelBreakdown(row))
	}
	return breakdown, nil
}

func buildAnalyticsProviderOptions(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsProviderOptionRecord, error) {
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

	options := make([]dto.AnalyticsProviderOptionRecord, 0, len(rows))
	for _, row := range rows {
		costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
		options = append(options, dto.AnalyticsProviderOptionRecord{
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

func analyticsCacheSavingsSQLExpression() string {
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND model_price_settings.id IS NOT NULL
			AND model_price_settings.prompt_price_per1_m >= model_price_settings.cache_price_per1_m
		THEN (` + cachedTokens + ` / 1000000.0) * (model_price_settings.prompt_price_per1_m - model_price_settings.cache_price_per1_m)
		ELSE 0
	END`
}

func analyticsCacheSavingsEligibleSQLExpression() string {
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND model_price_settings.id IS NOT NULL
			AND model_price_settings.prompt_price_per1_m >= model_price_settings.cache_price_per1_m
		THEN 1
		ELSE 0
	END`
}

func analyticsCacheSavingsIneligibleSQLExpression() string {
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND (
				model_price_settings.id IS NULL
				OR model_price_settings.prompt_price_per1_m < model_price_settings.cache_price_per1_m
			)
		THEN 1
		ELSE 0
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

func analyticsTrendBucketsByDay(filter dto.UsageQueryFilter) bool {
	return strings.TrimSpace(filter.Granularity) == "day"
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
		TotalCost:       row.TotalCost,
		TotalTokens:     row.TotalTokens,
		RequestCount:    row.RequestCount,
		SuccessCount:    row.SuccessCount,
		FailureCount:    row.FailureCount,
		InputTokens:     row.InputTokens,
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
	label := row.Bucket
	if !bucketByDay {
		label = bucketStart.In(time.Local).Format("2006-01-02 15:04 -0700")
	}
	return dto.AnalyticsTrendPointRecord{
		Label:         label,
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

func mapAnalyticsModelBreakdown(row analyticsModelAggregateRow) dto.AnalyticsModelBreakdownRecord {
	record := dto.AnalyticsModelBreakdownRecord{
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

func buildAnalyticsInsights(
	summary dto.AnalyticsSummaryRecord,
	trend []dto.AnalyticsTrendPointRecord,
	keyAliases []dto.AnalyticsKeyAliasBreakdownRecord,
	models []dto.AnalyticsModelBreakdownRecord,
) []dto.AnalyticsInsightRecord {
	if summary.RequestCount == 0 {
		return []dto.AnalyticsInsightRecord{}
	}

	insights := make([]dto.AnalyticsInsightRecord, 0, 6)
	insights = append(insights, metricCompletenessInsight(summary, models))
	insights = append(insights, cacheEfficiencyInsight(summary))
	if topCost, ok := topCostKeyAlias(keyAliases); ok {
		insights = append(insights, dto.AnalyticsInsightRecord{
			Type:        "top_cost_key",
			Severity:    "green",
			Title:       "Top Cost Key",
			Detail:      "Highest configured Cost contributor in this range.",
			Subject:     analyticsInsightKeyLabel(topCost),
			MetricLabel: "Cost",
			MetricValue: topCost.TotalCost,
			Count:       topCost.RequestCount,
			CostStatus:  topCost.CostStatus,
		})
	}
	if spike, ok := topTokenBucket(trend); ok {
		insights = append(insights, dto.AnalyticsInsightRecord{
			Type:        "token_spike",
			Severity:    "violet",
			Title:       "Token Spike",
			Detail:      "Highest token bucket in the selected range.",
			Subject:     spike.Label,
			MetricLabel: "Tokens",
			MetricValue: float64(spike.TotalTokens),
			Count:       spike.RequestCount,
			CostStatus:  spike.CostStatus,
		})
	}
	if failure, ok := failureConcentration(keyAliases); ok {
		insights = append(insights, dto.AnalyticsInsightRecord{
			Type:        "failure_concentration",
			Severity:    "amber",
			Title:       "Failure Cluster",
			Detail:      "Largest failure concentration by Key Alias.",
			Subject:     analyticsInsightKeyLabel(failure),
			MetricLabel: "Failures",
			MetricValue: float64(failure.FailureCount),
			Count:       failure.FailureCount,
			CostStatus:  failure.CostStatus,
		})
	}
	if summary.ReasoningTokens > 0 {
		insights = append(insights, dto.AnalyticsInsightRecord{
			Type:        "reasoning_tokens",
			Severity:    "blue",
			Title:       "Reasoning Tokens",
			Detail:      "Reasoning token volume is tracked separately from prompt cache reads.",
			Subject:     "Reasoning behavior",
			MetricLabel: "Tokens",
			MetricValue: float64(summary.ReasoningTokens),
			Count:       summary.ReasoningTokens,
			CostStatus:  summary.CostStatus,
		})
	}
	return insights
}

func metricCompletenessInsight(summary dto.AnalyticsSummaryRecord, models []dto.AnalyticsModelBreakdownRecord) dto.AnalyticsInsightRecord {
	incompleteModels := countModelsWithIncompletePricing(models)
	cacheComplete := summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateAvailable
	costComplete := summary.CostStatus == dto.AnalyticsCostStatusAvailable
	insight := dto.AnalyticsInsightRecord{
		Type:        "metric_completeness",
		Severity:    "green",
		Title:       "Metric Completeness",
		Detail:      "Cost and cache efficiency have the supporting data needed for complete interpretation.",
		Subject:     "Complete",
		MetricLabel: "Metric Completeness",
		MetricValue: 0,
		Count:       incompleteModels,
		CostStatus:  summary.CostStatus,
	}
	if costComplete && cacheComplete {
		return insight
	}
	insight.Severity = "amber"
	insight.Subject = metricCompletenessSubject(summary, incompleteModels)
	insight.Detail = "Some derived metrics are incomplete, but the underlying usage events remain valid."
	insight.MetricValue = float64(incompleteModels)
	return insight
}

func metricCompletenessSubject(summary dto.AnalyticsSummaryRecord, incompleteModels int64) string {
	if summary.CostStatus != dto.AnalyticsCostStatusAvailable {
		return "Cost " + summary.CostStatus
	}
	if summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateNoCacheData {
		return "No cache data"
	}
	if summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateNoPromptInput {
		return "No prompt input"
	}
	if incompleteModels == 1 {
		return "1 model"
	}
	if incompleteModels > 1 {
		return fmt.Sprintf("%d models", incompleteModels)
	}
	return "Incomplete"
}

func cacheEfficiencyInsight(summary dto.AnalyticsSummaryRecord) dto.AnalyticsInsightRecord {
	insight := dto.AnalyticsInsightRecord{
		Type:        "cache_efficiency",
		Severity:    "green",
		Title:       "Cache Read Share",
		Detail:      "Prompt cache reads are measured against prompt input tokens, separately from reasoning tokens.",
		Subject:     "Prompt input cache",
		MetricLabel: "Cache Read Share",
		MetricValue: summary.CacheReadShare,
		Count:       summary.CachedTokens,
		CostStatus:  summary.CostStatus,
	}
	switch summary.CacheReadShareState {
	case dto.AnalyticsCacheReadShareStateNoCacheData:
		insight.Severity = "amber"
		insight.Subject = "No cache data"
		insight.MetricLabel = "Cache state"
		insight.Detail = "Cached-token evidence is unavailable for this range; reasoning tokens are not counted as cache reads."
	case dto.AnalyticsCacheReadShareStateNoPromptInput:
		insight.Severity = "amber"
		insight.Subject = "No prompt input"
		insight.MetricLabel = "Cache state"
		insight.Detail = "Prompt input is zero for this range, so Cache Read Share has no denominator."
	}
	return insight
}

func topCostKeyAlias(rows []dto.AnalyticsKeyAliasBreakdownRecord) (dto.AnalyticsKeyAliasBreakdownRecord, bool) {
	var best dto.AnalyticsKeyAliasBreakdownRecord
	found := false
	for _, row := range rows {
		if row.CostAvailable == false || row.CostStatus == dto.AnalyticsCostStatusUnavailable || row.TotalCost <= 0 {
			continue
		}
		if !found || row.TotalCost > best.TotalCost {
			best = row
			found = true
		}
	}
	return best, found
}

func topTokenBucket(points []dto.AnalyticsTrendPointRecord) (dto.AnalyticsTrendPointRecord, bool) {
	var best dto.AnalyticsTrendPointRecord
	found := false
	for _, point := range points {
		if !found || point.TotalTokens > best.TotalTokens {
			best = point
			found = true
		}
	}
	return best, found && best.TotalTokens > 0
}

func failureConcentration(rows []dto.AnalyticsKeyAliasBreakdownRecord) (dto.AnalyticsKeyAliasBreakdownRecord, bool) {
	var best dto.AnalyticsKeyAliasBreakdownRecord
	found := false
	for _, row := range rows {
		if row.FailureCount == 0 {
			continue
		}
		if !found || row.FailureCount > best.FailureCount {
			best = row
			found = true
		}
	}
	return best, found
}

func analyticsInsightKeyLabel(row dto.AnalyticsKeyAliasBreakdownRecord) string {
	for _, value := range []string{row.Alias, row.Name, row.Identity} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "Unknown key"
}

func countModelsWithIncompletePricing(models []dto.AnalyticsModelBreakdownRecord) int64 {
	var count int64
	for _, model := range models {
		if model.CostStatus != dto.AnalyticsCostStatusAvailable {
			count++
		}
	}
	return count
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
