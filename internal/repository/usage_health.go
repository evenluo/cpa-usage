package repository

import (
	"context"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// BuildUsageRequestHealthWithFilter builds the fixed Request Health grid with bounded SQL aggregates.
func BuildUsageRequestHealthWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) (*dto.UsageOverviewHealthRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	health := buildUsageOverviewHealth(filter)
	type usageRequestHealthTotalRow struct {
		Success int64
		Failure int64
	}
	var totals usageRequestHealthTotalRow
	if err := applyUsageOverviewQuery(db.Model(&entities.UsageEvent{}), filter).
		Select(`
			COALESCE(SUM(CASE WHEN failed THEN 0 ELSE 1 END), 0) AS success,
			COALESCE(SUM(CASE WHEN failed THEN 1 ELSE 0 END), 0) AS failure`).
		Scan(&totals).Error; err != nil {
		return nil, fmt.Errorf("build usage request health totals: %w", err)
	}
	health.TotalSuccess = totals.Success
	health.TotalFailure = totals.Failure
	if total := health.TotalSuccess + health.TotalFailure; total > 0 {
		health.SuccessRate = (float64(health.TotalSuccess) / float64(total)) * 100
	}

	if len(health.BlockDetails) == 0 {
		return &health, nil
	}

	type usageRequestHealthBucketRow struct {
		BucketIndex int
		Success     int64
		Failure     int64
	}
	var rows []usageRequestHealthBucketRow
	if err := applyUsageOverviewQuery(db.Model(&entities.UsageEvent{}), filter).
		Where("timestamp >= ? AND timestamp < ?", health.WindowStart, health.WindowEnd).
		Select(`
			CAST((unixepoch(timestamp) - unixepoch(?)) / ? AS INTEGER) AS bucket_index,
			COALESCE(SUM(CASE WHEN failed THEN 0 ELSE 1 END), 0) AS success,
			COALESCE(SUM(CASE WHEN failed THEN 1 ELSE 0 END), 0) AS failure`,
			health.WindowStart.UTC(), health.BucketSeconds).
		Group("bucket_index").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build usage request health buckets: %w", err)
	}
	for _, row := range rows {
		if row.BucketIndex < 0 || row.BucketIndex >= len(health.BlockDetails) {
			continue
		}
		block := &health.BlockDetails[row.BucketIndex]
		block.Success = row.Success
		block.Failure = row.Failure
		if total := block.Success + block.Failure; total > 0 {
			block.Rate = float64(block.Success) / float64(total)
		}
	}

	return &health, nil
}

const (
	usageOverviewHealthRows           = 7
	usageOverviewHealthDefaultColumns = 96
	usageOverviewHealthDefaultSpan    = 15 * time.Minute
	usageOverviewHealthPresetWindow   = 24 * time.Hour
	usageOverviewHealthPresetSpan     = (usageOverviewHealthPresetWindow + time.Duration(usageOverviewHealthRows*usageOverviewHealthDefaultColumns) - 1) / time.Duration(usageOverviewHealthRows*usageOverviewHealthDefaultColumns)
)

func buildUsageOverviewHealth(filter dto.UsageQueryFilter) dto.UsageOverviewHealthRecord {
	rows, columns, span := usageOverviewHealthGrid(filter)
	totalBlocks := rows * columns
	windowStart, windowEnd := usageOverviewHealthWindow(filter, totalBlocks, span)
	blocks := make([]dto.UsageOverviewHealthBlockRecord, totalBlocks)
	for index := range blocks {
		startTime := windowStart.Add(time.Duration(index) * span)
		blocks[index] = dto.UsageOverviewHealthBlockRecord{
			StartTime: startTime,
			EndTime:   startTime.Add(span),
			Rate:      -1,
		}
	}
	return dto.UsageOverviewHealthRecord{
		Rows:          rows,
		Columns:       columns,
		BucketSeconds: int64((span + time.Second - 1) / time.Second),
		WindowStart:   windowStart,
		WindowEnd:     windowEnd,
		BlockDetails:  blocks,
	}
}

func usageOverviewHealthGrid(filter dto.UsageQueryFilter) (int, int, time.Duration) {
	if !isUsageOverviewShortHealthRange(filter.Range) {
		return usageOverviewHealthRows, usageOverviewHealthDefaultColumns, usageOverviewHealthDefaultSpan
	}
	if filter.Range == "24h" {
		return 8, 60, 3 * time.Minute
	}
	return usageOverviewHealthRows, usageOverviewHealthDefaultColumns, usageOverviewHealthPresetSpan
}

func isUsageOverviewShortHealthRange(value string) bool {
	switch value {
	case "4h", "8h", "12h", "24h", "today":
		return true
	default:
		return false
	}
}

func usageOverviewHealthWindow(filter dto.UsageQueryFilter, totalBlocks int, span time.Duration) (time.Time, time.Time) {
	end := time.Now().UTC()
	if filter.EndTime != nil {
		end = filter.EndTime.UTC()
	}
	if isUsageOverviewShortHealthRange(filter.Range) {
		return end.Add(-usageOverviewHealthPresetWindow), end
	}
	currentBucketStart := end.Truncate(span)
	windowEnd := currentBucketStart.Add(span)
	return windowEnd.Add(-time.Duration(totalBlocks) * span), windowEnd
}

func updateUsageOverviewHealthBlock(blocks []dto.UsageOverviewHealthBlockRecord, event entities.UsageEvent) {
	timestamp := event.Timestamp.UTC()
	for index := range blocks {
		block := &blocks[index]
		if timestamp.Before(block.StartTime) || !timestamp.Before(block.EndTime) {
			continue
		}
		if event.Failed {
			block.Failure++
		} else {
			block.Success++
		}
		total := block.Success + block.Failure
		if total > 0 {
			block.Rate = float64(block.Success) / float64(total)
		}
		return
	}
}
