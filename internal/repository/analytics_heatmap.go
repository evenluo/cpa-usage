package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"gorm.io/gorm"
	"strings"
	"time"
)

const analyticsHeatmapWindow = 30 * 24 * time.Hour

func buildAnalyticsHeatmap(db *gorm.DB, filter dto.UsageQueryFilter) (dto.AnalyticsHeatmap, error) {
	heatmap := dto.AnalyticsHeatmap{Measure: "tokens", Rows: []dto.AnalyticsHeatmapRow{}}
	heatmapFilter, ok := analyticsFixedHeatmapFilter(filter)
	if !ok {
		return heatmap, nil
	}

	windowStart := heatmapFilter.StartTime.UTC()
	windowEnd := heatmapFilter.EndTime.UTC()
	startDay := localDateStart(windowStart.In(time.Local))
	endDay := localDateStart(windowEnd.In(time.Local))
	aggregates, err := buildAnalyticsHeatmapAggregates(db, heatmapFilter, startDay, endDay)
	if err != nil {
		return dto.AnalyticsHeatmap{}, err
	}
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		row := dto.AnalyticsHeatmapRow{
			Date:  day.Format(time.DateOnly),
			Label: day.Format("01/02 Mon"),
			Cells: make([]dto.AnalyticsHeatmapCell, 0, 24),
		}
		for hour := 0; hour < 24; hour++ {
			bucketStart, bucketEnd := localHourBucket(day, hour)
			cell := dto.AnalyticsHeatmapCell{
				Hour:          hour,
				InRange:       analyticsHeatmapCellInRange(bucketStart.UTC(), bucketEnd.UTC(), windowStart, windowEnd),
				BucketStart:   bucketStart.UTC(),
				BucketEnd:     bucketEnd.UTC(),
				CostAvailable: true,
				CostStatus:    dto.AnalyticsCostStatusAvailable,
			}
			if aggregate, ok := aggregates[analyticsHeatmapCellKey(row.Date, hour)]; ok {
				cell.TotalTokens = aggregate.TotalTokens
				cell.TotalCost = aggregate.TotalCost
				cell.RequestCount = aggregate.RequestCount
				cell.FailureCount = aggregate.FailureCount
				cell.CostAvailable, cell.CostStatus = analyticsCostAvailability(aggregate.MissingPricingEvents, aggregate.PricedBillableEvents)
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

func analyticsFixedHeatmapFilter(filter dto.UsageQueryFilter) (dto.UsageQueryFilter, bool) {
	if filter.FixedWindowEnd == nil {
		return dto.UsageQueryFilter{}, false
	}
	windowEnd := filter.FixedWindowEnd.UTC()
	windowStart := windowEnd.Add(-analyticsHeatmapWindow)
	heatmapFilter := filter
	heatmapFilter.StartTime = &windowStart
	heatmapFilter.EndTime = &windowEnd
	return heatmapFilter, true
}

func buildAnalyticsHeatmapAggregates(db *gorm.DB, filter dto.UsageQueryFilter, startDay time.Time, endDay time.Time) (map[string]analyticsHeatmapAggregateRow, error) {
	var rows []analyticsHeatmapAggregateRow
	query := `
		WITH heatmap_buckets(bucket_key, bucket_start_epoch, bucket_end_epoch) AS (VALUES ` + analyticsHeatmapBucketValues(startDay, endDay) + `)
		SELECT
			heatmap_buckets.bucket_key AS bucket_key,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCostSQLExpression() + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsMissingPricingSQLExpression() + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsPricedBillableSQLExpression() + `), 0) AS priced_billable_events
		FROM usage_events
		JOIN heatmap_buckets
			ON unixepoch(usage_events.timestamp) >= heatmap_buckets.bucket_start_epoch
			AND unixepoch(usage_events.timestamp) < heatmap_buckets.bucket_end_epoch
		LEFT JOIN model_price_settings ON TRIM(model_price_settings.model) = TRIM(usage_events.model)
		WHERE usage_events.timestamp >= ? AND usage_events.timestamp <= ?`
	args := []any{filter.StartTime.UTC(), filter.EndTime.UTC()}
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query += " AND TRIM(usage_events.provider) = ?"
		args = append(args, provider)
	}
	query += " GROUP BY heatmap_buckets.bucket_key"
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics heatmap aggregates: %w", err)
	}
	aggregates := make(map[string]analyticsHeatmapAggregateRow, len(rows))
	for _, row := range rows {
		aggregate := aggregates[row.BucketKey]
		aggregate.RequestCount += row.RequestCount
		aggregate.FailureCount += row.FailureCount
		aggregate.TotalTokens += row.TotalTokens
		aggregate.TotalCost += row.TotalCost
		aggregate.MissingPricingEvents += row.MissingPricingEvents
		aggregate.PricedBillableEvents += row.PricedBillableEvents
		aggregates[row.BucketKey] = aggregate
	}
	return aggregates, nil
}

func analyticsHeatmapBucketValues(startDay time.Time, endDay time.Time) string {
	values := make([]string, 0, 24)
	for day := startDay; !day.After(endDay); day = day.AddDate(0, 0, 1) {
		date := day.Format(time.DateOnly)
		for hour := 0; hour < 24; hour++ {
			bucketStart, bucketEnd := localHourBucket(day, hour)
			values = append(values, fmt.Sprintf(
				"(%s, %d, %d)",
				analyticsSQLString(analyticsHeatmapCellKey(date, hour)),
				bucketStart.UTC().Unix(),
				bucketEnd.UTC().Unix(),
			))
		}
	}
	return strings.Join(values, ", ")
}

func analyticsSQLString(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func analyticsHeatmapCellKey(date string, hour int) string {
	return fmt.Sprintf("%s-%02d", date, hour)
}

func analyticsHeatmapCellInRange(bucketStart time.Time, bucketEnd time.Time, windowStart time.Time, windowEnd time.Time) bool {
	return bucketEnd.After(bucketStart) && bucketEnd.After(windowStart) && !bucketStart.After(windowEnd)
}

func localDateStart(value time.Time) time.Time {
	local := value.In(time.Local)
	return time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.Local)
}

func localHourBucket(day time.Time, hour int) (time.Time, time.Time) {
	start := localHourBoundary(day, hour)
	end := localHourBoundary(day, hour+1)
	return start, end
}

func localHourBoundary(day time.Time, hour int) time.Time {
	localDay := day.In(time.Local)
	dayStart := time.Date(localDay.Year(), localDay.Month(), localDay.Day(), 0, 0, 0, 0, time.Local)
	dayEnd := dayStart.AddDate(0, 0, 1)
	targetYear, targetMonth, targetDay := localDay.Date()
	targetHour := hour
	if hour >= 24 {
		nextDay := dayStart.AddDate(0, 0, 1)
		targetYear, targetMonth, targetDay = nextDay.Date()
		targetHour = hour - 24
	}
	low := dayStart.UTC().UnixNano()
	high := dayEnd.UTC().UnixNano()
	for low < high {
		mid := low + (high-low)/2
		if localDateTimeAtOrAfter(time.Unix(0, mid).In(time.Local), targetYear, targetMonth, targetDay, targetHour) {
			high = mid
		} else {
			low = mid + 1
		}
	}
	return time.Unix(0, low).In(time.Local)
}

func localDateTimeAtOrAfter(value time.Time, targetYear int, targetMonth time.Month, targetDay int, targetHour int) bool {
	year, month, day := value.Date()
	if year != targetYear {
		return year > targetYear
	}
	if month != targetMonth {
		return month > targetMonth
	}
	if day != targetDay {
		return day > targetDay
	}
	return value.Hour() >= targetHour
}
