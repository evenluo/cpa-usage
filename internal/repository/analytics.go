package repository

import (
	"fmt"
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

	return &dto.AnalyticsSummarySnapshot{Summary: summary, Trend: trend}, nil
}

func buildAnalyticsSummary(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsSummaryRecord, error) {
	var row analyticsAggregateRow
	if err := analyticsEventsWithPricingQuery(db, filter).
		Select(`
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
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
			COALESCE(SUM(CASE WHEN failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(total_tokens), 0) AS total_tokens,
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

func analyticsCostAvailability(missingPricingEvents int64, pricedBillableEvents int64) (bool, string) {
	if missingPricingEvents == 0 {
		return true, dto.AnalyticsCostStatusAvailable
	}
	if pricedBillableEvents > 0 {
		return false, dto.AnalyticsCostStatusPartial
	}
	return false, dto.AnalyticsCostStatusUnavailable
}
