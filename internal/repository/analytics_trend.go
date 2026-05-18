package repository

import (
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

func buildAnalyticsTrend(db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.AnalyticsTrendPoint, error) {
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

	trend := make([]dto.AnalyticsTrendPoint, 0, len(rows))
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
	query = applyUsageQueryWindow(query, filter)
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("TRIM(usage_events.provider) = ?", provider)
	}
	return query
}
func mapAnalyticsTrendPoint(row analyticsAggregateRow, bucketByDay bool) (dto.AnalyticsTrendPoint, error) {
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
		return dto.AnalyticsTrendPoint{}, fmt.Errorf("parse analytics trend bucket %q: %w", row.Bucket, err)
	}
	costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	label := row.Bucket
	if !bucketByDay {
		label = bucketStart.In(time.Local).Format("2006-01-02 15:04 -0700")
	}
	return dto.AnalyticsTrendPoint{
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
