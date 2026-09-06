package repository

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestUsageAttemptPerformanceSeparatesResultExecutionAndInvalidSamples(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "attempt-performance.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	knownTrue := true
	knownFalse := false
	events := make([]entities.UsageEvent, 0, 17)
	for index, latency := range []int64{100, 200, 300, 400, 500, 600, 700, 800, 900, 10_000} {
		ttft := int64((index + 1) * 10)
		events = append(events, entities.UsageEvent{
			EventKey: "tail-" + string(rune('a'+index)), Timestamp: start.Add(time.Duration(index+1) * time.Minute),
			Provider: "claude", Model: "sonnet", AuthIndex: "account-a", LatencyMS: latency, TTFTMS: &ttft,
			OutputTokens: 100, Generate: &knownTrue, Stream: &knownTrue,
		})
	}
	events = append(events,
		entities.UsageEvent{EventKey: "missing-ttft", Timestamp: start.Add(20 * time.Minute), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", LatencyMS: 1_100, OutputTokens: 100, Generate: &knownTrue, Stream: &knownTrue},
		entities.UsageEvent{EventKey: "non-generating", Timestamp: start.Add(21 * time.Minute), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", LatencyMS: 1_200, OutputTokens: 100, Generate: &knownFalse, Stream: &knownTrue},
		entities.UsageEvent{EventKey: "non-streaming", Timestamp: start.Add(22 * time.Minute), Provider: "claude", Model: "opus", AuthIndex: "account-b", LatencyMS: 1_300, OutputTokens: 100, Generate: &knownTrue, Stream: &knownFalse},
		entities.UsageEvent{EventKey: "unknown-valid", Timestamp: start.Add(23 * time.Minute), Provider: "claude", Model: "opus", AuthIndex: "account-b", LatencyMS: 1_400, TTFTMS: performanceInt64Pointer(100), OutputTokens: 100},
		entities.UsageEvent{EventKey: "unknown-qualified", Timestamp: start.Add(24 * time.Minute), Provider: "claude", Model: "opus", AuthIndex: "account-b", LatencyMS: 1_500, TTFTMS: performanceInt64Pointer(100), OutputTokens: 30, UsageAccounting: performanceAccounting("inconsistent")},
		entities.UsageEvent{EventKey: "failed-fast", Timestamp: start.Add(25 * time.Minute), Provider: "claude", Model: "opus", AuthIndex: "account-b", Failed: true, LatencyMS: 50, TTFTMS: performanceInt64Pointer(10), OutputTokens: 10, Generate: &knownTrue, Stream: &knownTrue},
		entities.UsageEvent{EventKey: "failed-tail", Timestamp: start.Add(26 * time.Minute), Provider: "claude", Model: "opus", AuthIndex: "account-b", Failed: true, LatencyMS: 5_000, TTFTMS: performanceInt64Pointer(100), OutputTokens: 100, Generate: &knownTrue, Stream: &knownTrue},
	)
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage attempts: %v", err)
	}

	filter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end, Provider: "claude"}}
	result, err := BuildUsageAttemptPerformanceWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build attempt performance: %v", err)
	}
	if result.TotalAttempts != 17 || result.SuccessfulAttempts != 15 || result.FailedAttempts != 2 {
		t.Fatalf("unexpected result populations: %+v", result)
	}
	if got := result.SuccessfulExecution; got.GeneratingStreaming != 11 || got.NonGenerating != 1 || got.NonStreaming != 1 || got.Unknown != 2 {
		t.Fatalf("unexpected successful execution partition: %+v", got)
	}
	assertUsagePercentiles(t, result.SuccessfulLatencyMS, 15, 15, 800, 10_000, 1)
	assertUsagePercentiles(t, result.FailedLatencyMS, 2, 2, 50, 5_000, 1)
	assertUsagePercentiles(t, result.StreamingTTFTMS, 11, 10, 50, 100, 10.0/11.0)
	if result.StreamingOutputTPS.SampleCount != 10 || result.StreamingOutputTPS.PopulationCount != 11 {
		t.Fatalf("missing TTFT must lower known-execution TPS coverage: %+v", result.StreamingOutputTPS)
	}
	assertUsagePercentiles(t, result.UnknownExecutionTTFTMS, 2, 2, 100, 100, 1)
	if result.UnknownExecutionOutputTPS.SampleCount != 1 || result.UnknownExecutionOutputTPS.Coverage == nil || math.Abs(*result.UnknownExecutionOutputTPS.Coverage-0.5) > 1e-9 {
		t.Fatalf("qualified canonical evidence must not yield false exact TPS: %+v", result.UnknownExecutionOutputTPS)
	}
	if len(result.Providers.Items) != 1 || result.Providers.Items[0].AttemptCount != 17 {
		t.Fatalf("expected provider breakdown over the same attempts: %+v", result.Providers)
	}
	assertUsagePercentiles(t, result.Providers.Items[0].SuccessfulLatencyMS, 15, 15, 800, 10_000, 1)
	assertUsagePerformanceBreakdownParity(t, result.TotalAttempts, result.Providers)
	assertUsagePerformanceBreakdownParity(t, result.TotalAttempts, result.Models)
	assertUsagePerformanceBreakdownParity(t, result.TotalAttempts, result.Accounts)
}

func TestUsageAttemptPerformanceTopNAndSlowEvidencePreserveCounts(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "attempt-performance-selection.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events := make([]entities.UsageEvent, 0, 11)
	for index := 0; index < 10; index++ {
		events = append(events, entities.UsageEvent{
			EventKey: "dimension-" + string(rune('a'+index)), Timestamp: start.Add(time.Duration(index) * time.Minute),
			Provider: "provider-" + string(rune('a'+index)), Model: "model-" + string(rune('a'+index)), AuthIndex: "account-" + string(rune('a'+index)), LatencyMS: int64(100 + index*100),
		})
	}
	events = append(events, entities.UsageEvent{EventKey: "blank-dimensions", Timestamp: start.Add(time.Hour), LatencyMS: 500})
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage attempts: %v", err)
	}
	allFilter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}}
	all, err := BuildUsageAttemptPerformanceWithFilter(context.Background(), db, allFilter)
	if err != nil {
		t.Fatalf("build full attempt performance: %v", err)
	}
	for _, breakdown := range []dto.UsagePerformanceBreakdownRecord{all.Providers, all.Models, all.Accounts} {
		assertUsagePerformanceBreakdownParity(t, all.TotalAttempts, breakdown)
		if len(breakdown.Items) != dto.UsagePerformanceBreakdownLimit || breakdown.OtherCount != 3 {
			t.Fatalf("expected Top-N plus blank/below-rank conservation: %+v", breakdown)
		}
	}

	threshold := int64(500)
	filter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}, MinLatencyMS: &threshold}
	result, err := BuildUsageAttemptPerformanceWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build slow attempt performance: %v", err)
	}
	evidence, err := ListUsageEventsWithFilter(context.Background(), db, dto.UsageEventListFilter{
		UsageTimeScope: filter.UsageTimeScope, MinLatencyMS: &threshold, Page: 1, PageSize: 100,
	})
	if err != nil {
		t.Fatalf("list slow evidence: %v", err)
	}
	if result.TotalAttempts != 7 || evidence.TotalCount != 7 {
		t.Fatalf("inclusive threshold must preserve aggregate/evidence parity: performance=%d evidence=%d", result.TotalAttempts, evidence.TotalCount)
	}
	for _, breakdown := range []dto.UsagePerformanceBreakdownRecord{result.Providers, result.Models, result.Accounts} {
		assertUsagePerformanceBreakdownParity(t, result.TotalAttempts, breakdown)
		if len(breakdown.Items) > dto.UsagePerformanceBreakdownLimit {
			t.Fatalf("breakdown exceeded backend Top-N: %+v", breakdown)
		}
	}
}

func TestUsageAttemptPerformanceRequiresBoundedWindow(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "attempt-performance-unbounded.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	if _, err := BuildUsageAttemptPerformanceWithFilter(context.Background(), db, dto.UsageDiagnosticFilter{}); err == nil {
		t.Fatal("expected unbounded attempt performance to be rejected")
	}
}

func TestUsageAttemptPerformanceLeavesInvalidTimingAndTokensUnavailable(t *testing.T) {
	knownTrue := true
	knownFalse := false
	zero := int64(0)
	tooLarge := int64(150)
	validTTFT := int64(50)
	item := summarizeUsagePerformanceGroup("edge-cases", []usagePerformanceAttempt{
		{LatencyMS: 0, TTFTMS: nil, OutputTokens: 10, Generate: &knownTrue, Stream: &knownTrue},
		{LatencyMS: 100, TTFTMS: &zero, OutputTokens: 10, Generate: &knownTrue, Stream: &knownTrue},
		{LatencyMS: 100, TTFTMS: &tooLarge, OutputTokens: 10, Generate: &knownTrue, Stream: &knownTrue},
		{LatencyMS: 200, TTFTMS: &validTTFT, OutputTokens: 0, Generate: &knownTrue, Stream: &knownTrue},
		{LatencyMS: 300, TTFTMS: &validTTFT, OutputTokens: 10, Generate: &knownFalse, Stream: &knownTrue},
		{Failed: true, LatencyMS: 400, TTFTMS: &validTTFT, OutputTokens: 10, Generate: &knownTrue, Stream: &knownTrue},
	})
	if item.AttemptCount != 6 || item.SuccessfulAttempts != 5 || item.FailedAttempts != 1 {
		t.Fatalf("unexpected edge-case populations: %+v", item)
	}
	if item.SuccessfulLatencyMS.SampleCount != 4 || item.StreamingTTFTMS.PopulationCount != 4 || item.StreamingTTFTMS.SampleCount != 1 || item.StreamingOutputTPS.SampleCount != 0 {
		t.Fatalf("invalid timing/tokens must lower the applicable sample coverage: %+v", item)
	}
	if item.SuccessfulExecution.NonGenerating != 1 || item.FailedLatencyMS.SampleCount != 1 {
		t.Fatalf("non-generating and failed attempts must remain explicitly qualified: %+v", item)
	}
}

func assertUsagePercentiles(t *testing.T, got dto.UsagePercentileRecord, population, samples int64, p50, p95, coverage float64) {
	t.Helper()
	if got.PopulationCount != population || got.SampleCount != samples || got.P50 == nil || got.P95 == nil || got.Coverage == nil || *got.P50 != p50 || *got.P95 != p95 || math.Abs(*got.Coverage-coverage) > 1e-9 {
		t.Fatalf("unexpected percentile record: %+v", got)
	}
}

func assertUsagePerformanceBreakdownParity(t *testing.T, total int64, breakdown dto.UsagePerformanceBreakdownRecord) {
	t.Helper()
	count := breakdown.OtherCount
	for _, item := range breakdown.Items {
		count += item.AttemptCount
	}
	if count != total {
		t.Fatalf("performance breakdown does not preserve total %d: %+v", total, breakdown)
	}
}

func performanceInt64Pointer(value int64) *int64 { return &value }

func performanceAccounting(quality string) entities.UsageAccounting {
	return entities.UsageAccounting{
		AccountingVersion: performanceInt64Pointer(2), TokenBreakdownPresent: true, TokenSchemaVersion: performanceInt64Pointer(2), TokenQuality: &quality,
		CanonicalTotalTokens: performanceInt64Pointer(130), CanonicalInputTokens: performanceInt64Pointer(100), CanonicalUncachedTokens: performanceInt64Pointer(50), CanonicalCacheReadTokens: performanceInt64Pointer(40), CanonicalCacheWriteTokens: performanceInt64Pointer(10),
		CanonicalOutputTokens: performanceInt64Pointer(30), CanonicalNonReasoningTokens: performanceInt64Pointer(18), CanonicalReasoningTokens: performanceInt64Pointer(12), CanonicalUnclassifiedTokens: performanceInt64Pointer(0),
	}
}
