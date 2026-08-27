package repository

import (
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

func buildAnalyticsTrend(db *gorm.DB, filter dto.AnalyticsFilter) ([]dto.AnalyticsTrendPoint, error) {
	bucketByDay := analyticsTrendBucketsByDay(filter)
	rows, err := buildAnalyticsAggregateRowsByBucket(db, filter, analyticsEventsAggregateSource())
	if err != nil {
		return nil, err
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

func analyticsEventsWithPricingQuery(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	return usageEventsWithPricingQuery(db, filter.UsageTimeScope)
}

func usageEventsWithPricingQuery(db *gorm.DB, scope dto.UsageTimeScope) *gorm.DB {
	return applyAnalyticsScopeFilter(db.Model(&entities.UsageEvent{}), scope).
		Joins("LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_events.model)")
}

func applyAnalyticsQueryFilter(query *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB {
	return applyAnalyticsScopeFilter(query, filter.UsageTimeScope)
}

func applyAnalyticsScopeFilter(query *gorm.DB, scope dto.UsageTimeScope) *gorm.DB {
	query = applyUsageQueryWindow(query, scope)
	if provider := strings.TrimSpace(scope.Provider); provider != "" {
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
		Label:           label,
		BucketStart:     bucketStart.UTC(),
		BucketEnd:       bucketEnd.UTC(),
		TotalCost:       row.TotalCost,
		TotalTokens:     row.TotalTokens,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		ReasoningTokens: row.ReasoningTokens,
		CachedTokens:    row.CachedTokens,
		RequestCount:    row.RequestCount,
		SuccessCount:    row.SuccessCount,
		FailureCount:    row.FailureCount,
		CostAvailable:   costAvailable,
		CostStatus:      costStatus,
	}, nil
}
