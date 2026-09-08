package repository

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestUsageHistogramPreservesSamplesAndBoundaryValues(t *testing.T) {
	metric := usagePercentileRecordWithHistogram([]float64{240, 10, 10, 20, 30}, 8, 240)
	want := make([]int64, dto.UsagePerformanceHistogramBins)
	want[1], want[2], want[3], want[23] = 2, 1, 1, 1
	if metric.Histogram == nil || metric.Histogram.UpperBound != 240 || !reflect.DeepEqual(metric.Histogram.Counts, want) {
		t.Fatalf("histogram must use left-closed intervals and include the upper endpoint: %+v", metric.Histogram)
	}
	assertUsagePercentiles(t, metric, 8, 5, 20, 240, 5.0/8.0)
	if empty := usagePercentileRecord(nil, 10); empty.Histogram != nil || empty.P50 != nil || empty.SampleCount != 0 {
		t.Fatalf("missing samples must not become zero-valued density: %+v", empty)
	}
	constant := usagePercentileRecord([]float64{42, 42, 42}, 3)
	if constant.Histogram.UpperBound != 42 || constant.Histogram.Counts[23] != 3 {
		t.Fatalf("identical samples must remain a real concentration: %+v", constant)
	}
}

type performanceReadCounter struct {
	logger.Interface
	reads int
}

func (l *performanceReadCounter) Trace(_ context.Context, _ time.Time, sql func() (string, int64), _ error) {
	statement, _ := sql()
	if strings.HasPrefix(strings.TrimSpace(strings.ToUpper(statement)), "SELECT") {
		l.reads++
	}
}

func TestUsageHistogramSharesBoundsWithoutExtraReads(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	streaming := true
	events := []entities.UsageEvent{
		{EventKey: "fast-a", Timestamp: start.Add(time.Minute), Provider: "claude", Model: "a", AuthIndex: "account-a", LatencyMS: 100, TTFTMS: performanceInt64Pointer(10), Generate: &streaming, Stream: &streaming, UsageAccounting: performanceAccountingWithOutput("complete", 100)},
		{EventKey: "fast-b", Timestamp: start.Add(2 * time.Minute), Provider: "claude", Model: "a", AuthIndex: "account-a", LatencyMS: 200, TTFTMS: performanceInt64Pointer(20), Generate: &streaming, Stream: &streaming, UsageAccounting: performanceAccountingWithOutput("complete", 100)},
		{EventKey: "slow", Timestamp: start.Add(3 * time.Minute), Provider: "claude", Model: "b", AuthIndex: "account-b", LatencyMS: 10_000, TTFTMS: performanceInt64Pointer(100), Generate: &streaming, Stream: &streaming, UsageAccounting: performanceAccountingWithOutput("complete", 100)},
		{EventKey: "invalid-output", Timestamp: start.Add(4 * time.Minute), Provider: "claude", Model: "b", AuthIndex: "account-b", LatencyMS: 300, TTFTMS: performanceInt64Pointer(30), Generate: &streaming, Stream: &streaming, UsageAccounting: performanceAccountingWithOutput("inconsistent", 10)},
		{EventKey: "failed", Timestamp: start.Add(5 * time.Minute), Provider: "claude", Model: "b", AuthIndex: "account-b", Failed: true, LatencyMS: 500},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatal(err)
	}
	counter := &performanceReadCounter{Interface: logger.Default.LogMode(logger.Silent)}
	result, err := BuildUsageAttemptPerformanceWithFilter(context.Background(), db.Session(&gorm.Session{Logger: counter}), dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end, Provider: "claude"}})
	if err != nil {
		t.Fatal(err)
	}
	if counter.reads != 13 {
		t.Fatalf("heatmaps must reuse the existing 13 reads; got %d", counter.reads)
	}
	for _, breakdown := range []dto.UsagePerformanceBreakdownRecord{result.Providers, result.Models, result.Accounts} {
		for _, item := range breakdown.Items {
			pairs := [][2]dto.UsagePercentileRecord{
				{item.SuccessfulLatencyMS, result.SuccessfulLatencyMS},
				{item.FailedLatencyMS, result.FailedLatencyMS},
				{item.StreamingTTFTMS, result.StreamingTTFTMS},
				{item.UnknownExecutionTTFTMS, result.UnknownExecutionTTFTMS},
				{item.StreamingOutputTPS, result.StreamingOutputTPS},
			}
			for _, pair := range pairs {
				metric, overall := pair[0], pair[1]
				if metric.SampleCount == 0 {
					if metric.Histogram != nil {
						t.Fatal("empty population received histogram bins")
					}
					continue
				}
				if metric.Histogram == nil || overall.Histogram == nil || metric.Histogram.UpperBound != overall.Histogram.UpperBound {
					t.Fatalf("row %s does not share the overall metric range", item.Value)
				}
				var samples int64
				for _, count := range metric.Histogram.Counts {
					samples += count
				}
				if samples != metric.SampleCount || len(metric.Histogram.Counts) != 24 {
					t.Fatalf("histogram sample conservation failed for %s: %+v", item.Value, metric)
				}
			}
		}
	}
	if result.StreamingOutputTPS.SampleCount != 3 {
		t.Fatalf("failed/incomplete attempts entered TPS density: %+v", result.StreamingOutputTPS)
	}
	if result.SuccessfulLatencyMS.Histogram.UpperBound != 10_000 || *result.Models.Items[1].SuccessfulLatencyMS.P95 != 200 {
		t.Fatalf("full range and exact row percentiles must remain distinct: %+v", result.Models)
	}
}
