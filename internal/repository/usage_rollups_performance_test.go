package repository

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	populatedHourEventCount = 32_768
	newHourBatchEventCount  = 1_000
	backfillEventCount      = 32_768
)

func BenchmarkInsertUsageEventsIntoPopulatedHour(b *testing.B) {
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		db := openUsageRollupPerformanceDatabase(b, "populated-hour", iteration)
		bucket := time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)
		prefill := usageRollupPerformanceEvents("prefill", 0, populatedHourEventCount, bucket, 1)
		if err := db.CreateInBatches(&prefill, insertBatchSize(entities.UsageEvent{})).Error; err != nil {
			b.Fatalf("seed populated hour: %v", err)
		}
		if err := RebuildUsageRollupsForBucketRange(db, bucket, bucket); err != nil {
			b.Fatalf("build populated-hour rollup: %v", err)
		}
		batch := usageRollupPerformanceEvents("batch", populatedHourEventCount, newHourBatchEventCount, bucket, 1)
		b.ReportAllocs()
		b.ReportMetric(populatedHourEventCount, "existing_hour_events")
		b.ReportMetric(newHourBatchEventCount, "new_batch_events")
		b.StartTimer()
		inserted, deduped, err := InsertUsageEvents(db, batch)
		b.StopTimer()
		if err != nil || inserted != len(batch) || deduped != 0 {
			b.Fatalf("insert populated-hour batch: inserted=%d deduped=%d err=%v", inserted, deduped, err)
		}
		assertUsageRollupPerformanceCount(b, db, populatedHourEventCount+newHourBatchEventCount)
		closeUsageRollupPerformanceDatabase(b, db)
	}
}

func BenchmarkRebuildUsageRollupsTwentyFourHours(b *testing.B) {
	for iteration := 0; iteration < b.N; iteration++ {
		b.StopTimer()
		db := openUsageRollupPerformanceDatabase(b, "backfill-24h", iteration)
		start := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
		events := usageRollupPerformanceEvents("backfill", 0, backfillEventCount, start, 24)
		if err := db.CreateInBatches(&events, insertBatchSize(entities.UsageEvent{})).Error; err != nil {
			b.Fatalf("seed backfill events: %v", err)
		}
		b.ReportAllocs()
		b.ReportMetric(backfillEventCount, "raw_events")
		b.ReportMetric(24, "bucket_hours")
		b.StartTimer()
		err := RebuildUsageRollupsForBucketRange(db, start, start.Add(23*time.Hour))
		b.StopTimer()
		if err != nil {
			b.Fatalf("rebuild 24-hour rollups: %v", err)
		}
		assertUsageRollupPerformanceCount(b, db, backfillEventCount)
		closeUsageRollupPerformanceDatabase(b, db)
	}
}

func usageRollupPerformanceEvents(prefix string, offset int, count int, start time.Time, hours int) []entities.UsageEvent {
	events := make([]entities.UsageEvent, 0, count)
	for index := 0; index < count; index++ {
		absolute := offset + index
		event := entities.UsageEvent{
			EventKey: fmt.Sprintf("%s-%06d", prefix, absolute), Provider: fmt.Sprintf("provider-%02d", absolute%8),
			Model: fmt.Sprintf("model-%02d", absolute%16), AuthType: "apikey", AuthIndex: fmt.Sprintf("auth-%02d", absolute%64),
			APIGroupKey: fmt.Sprintf("sk-bench-%02d", absolute%64), Timestamp: start.Add(time.Duration(absolute%hours)*time.Hour + time.Duration(absolute%3_600)*time.Second),
			Failed: absolute%17 == 0, LatencyMS: int64(100 + absolute%1_000), InputTokens: int64(100 + absolute%500),
			OutputTokens: int64(50 + absolute%250), ReasoningTokens: int64(absolute % 50),
		}
		event.TotalTokens = event.InputTokens + event.OutputTokens
		event = canonicalUsageTestEvent(event)
		event.AccountingState = AccountingValid
		events = append(events, event)
	}
	return events
}

func openUsageRollupPerformanceDatabase(b *testing.B, name string, iteration int) *gorm.DB {
	b.Helper()
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(b.TempDir(), fmt.Sprintf("%s-%d.db", name, iteration))})
	if err != nil {
		b.Fatalf("open usage rollup performance database: %v", err)
	}
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func assertUsageRollupPerformanceCount(b *testing.B, db *gorm.DB, expected int) {
	b.Helper()
	var requestCount int64
	if err := db.Model(&entities.UsageRollupHourly{}).Select("COALESCE(SUM(request_count), 0)").Scan(&requestCount).Error; err != nil {
		b.Fatalf("read rollup request count: %v", err)
	}
	if requestCount != int64(expected) {
		b.Fatalf("expected %d rolled-up events, got %d", expected, requestCount)
	}
}

func closeUsageRollupPerformanceDatabase(b *testing.B, db *gorm.DB) {
	b.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get usage rollup database handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		b.Fatalf("close usage rollup database: %v", err)
	}
}
