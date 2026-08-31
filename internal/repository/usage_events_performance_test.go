package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	requestEvidencePerformanceEventCount = 65_536
	usageAttemptBenchmarkRequestCount    = 250
)

var (
	requestEvidenceBenchmarkResult *dto.UsageEventsPageRecord
	usageAttemptBenchmarkInserted  int
)

// BenchmarkListUsageEventsHighCardinalityCombinedFilters measures Request
// Evidence's count, model-options, and page queries together at the same
// 65,536-event cardinality used by the analytics convergence benchmark.
// Fixture construction and ANALYZE run before the timer so results describe
// request-time query and projection work only.
func BenchmarkListUsageEventsHighCardinalityCombinedFilters(b *testing.B) {
	db, fixture := prepareRequestEvidencePerformanceFixture(b, requestEvidencePerformanceEventCount)
	target := fixture.events[len(fixture.events)/2]
	filter := dto.UsageEventListFilter{
		UsageTimeScope: dto.UsageTimeScope{
			StartTime: &fixture.start,
			EndTime:   &fixture.end,
			Provider:  target.Provider,
		},
		Page:     1,
		PageSize: 100,
		Model:    target.Model,
		Result:   usageEventBenchmarkResultFilter(target.Failed),
	}

	warm, err := ListUsageEventsWithFilter(context.Background(), db, filter)
	if err != nil {
		b.Fatalf("warm request evidence benchmark: %v", err)
	}
	expectedModelOptions := analyticsCoreBenchmarkModelCount / analyticsCoreBenchmarkProviderCount
	if warm.TotalCount == 0 || len(warm.Models) != expectedModelOptions {
		b.Fatalf("benchmark fixture did not exercise combined filter and model options: %+v", warm)
	}

	b.ReportMetric(requestEvidencePerformanceEventCount, "fixture_events")
	b.ReportMetric(analyticsCoreBenchmarkProviderCount, "fixture_providers")
	b.ReportMetric(analyticsCoreBenchmarkModelCount, "fixture_models")
	b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
	b.ReportMetric(float64(warm.TotalCount), "matching_events")
	b.ReportMetric(float64(len(warm.Models)), "model_options")
	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		result, err := ListUsageEventsWithFilter(context.Background(), db, filter)
		if err != nil {
			b.Fatalf("list request evidence benchmark: %v", err)
		}
		requestEvidenceBenchmarkResult = result
	}
}

// BenchmarkInsertUsageEventsAttemptAmplification measures current storage and
// write cost as attempt rows increase. It does not attribute a baseline delta:
// preserving retries intentionally changes upstream attempt cardinality. Each
// sub-benchmark reuses request IDs while increasing distinct attempt event keys
// from one to four; its 1,000-attempt maximum equals the live inbox processing
// limit, and database construction is excluded from the timed region.
func BenchmarkInsertUsageEventsAttemptAmplification(b *testing.B) {
	for _, attemptsPerRequest := range []int{1, 2, 4} {
		b.Run(fmt.Sprintf("%dx_attempts", attemptsPerRequest), func(b *testing.B) {
			events := benchmarkUsageAttempts(usageAttemptBenchmarkRequestCount, attemptsPerRequest)
			b.ReportMetric(float64(usageAttemptBenchmarkRequestCount), "unique_request_ids")
			b.ReportMetric(float64(attemptsPerRequest), "attempts_per_request")
			b.ReportMetric(float64(len(events)), "usage_attempts")
			b.ReportAllocs()
			b.ResetTimer()
			for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
				b.StopTimer()
				db := openUsageEventsPerformanceDatabase(b, "attempt-amplification", benchmarkIndex)
				b.StartTimer()
				inserted, deduped, err := InsertUsageEvents(db, events)
				if err != nil {
					b.Fatalf("insert attempts benchmark: %v", err)
				}
				b.StopTimer()

				if inserted != len(events) || deduped != 0 {
					b.Fatalf("attempt benchmark did not preserve every attempt: inserted=%d deduped=%d events=%d", inserted, deduped, len(events))
				}
				usageAttemptBenchmarkInserted = inserted
				b.ReportMetric(float64(usageEventsDatabaseBytes(b, db)), "database_bytes")
				closeUsageEventsPerformanceDatabase(b, db)
			}
		})
	}
}

// TestRequestEvidenceQueryPlansAvoidFullScans uses a deterministic, ANALYZE'd
// fixture to protect all three queries issued by Request Evidence: the
// filtered count, time/provider-scoped model options, and paginated event page.
// It deliberately does not pin a specific index or reject SQLite's reasonable
// temporary B-trees for DISTINCT/ORDER.
func TestRequestEvidenceQueryPlansAvoidFullScans(t *testing.T) {
	db, fixture := prepareRequestEvidencePerformanceFixture(t, 4_096)
	target := fixture.events[len(fixture.events)/2]

	assertUsageEventsQueryPlanUsesSearch(t, db, "combined filtered count", `
		EXPLAIN QUERY PLAN SELECT COUNT(*) FROM usage_events
		WHERE timestamp >= ? AND timestamp <= ?
		  AND TRIM(provider) = ?
		  AND TRIM(model) = ?
		  AND failed = ?`, fixture.start, fixture.end, target.Provider, target.Model, target.Failed)

	assertUsageEventsQueryPlanUsesSearch(t, db, "time/provider scoped model options", `
		EXPLAIN QUERY PLAN SELECT DISTINCT TRIM(model) FROM usage_events
		WHERE timestamp >= ? AND timestamp <= ?
		  AND TRIM(provider) = ?
		ORDER BY TRIM(model) ASC`, fixture.start, fixture.end, target.Provider)

	assertUsageEventsQueryPlanUsesSearch(t, db, "combined page", `
		EXPLAIN QUERY PLAN SELECT id FROM usage_events
		WHERE timestamp >= ? AND timestamp <= ?
		  AND TRIM(provider) = ?
		  AND TRIM(model) = ?
		  AND failed = ?
		ORDER BY timestamp DESC, id DESC
		LIMIT 100`, fixture.start, fixture.end, target.Provider, target.Model, target.Failed)
}

func assertUsageEventsQueryPlanUsesSearch(t *testing.T, db *gorm.DB, name string, query string, args ...any) {
	t.Helper()
	var rows []struct {
		ID     int
		Parent int
		Unused int
		Detail string
	}
	if err := db.Raw(query, args...).Scan(&rows).Error; err != nil {
		t.Fatalf("explain %s query: %v", name, err)
	}
	if len(rows) == 0 {
		t.Fatalf("expected a query plan for %s", name)
	}
	details := make([]string, 0, len(rows))
	hasSearch := false
	for _, row := range rows {
		detail := strings.ToUpper(row.Detail)
		details = append(details, row.Detail)
		if strings.Contains(detail, "SEARCH USAGE_EVENTS USING") {
			hasSearch = true
		}
		if strings.Contains(detail, "SCAN USAGE_EVENTS") {
			t.Fatalf("%s regressed to a table or full-index scan: %s", name, strings.Join(details, " | "))
		}
	}
	if !hasSearch {
		t.Fatalf("%s did not use an indexed search: %s", name, strings.Join(details, " | "))
	}
}

func prepareRequestEvidencePerformanceFixture(tb testing.TB, eventCount int) (*gorm.DB, analyticsCoreCardinalityFixture) {
	tb.Helper()
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(tb.TempDir(), "request-evidence-performance.db")})
	if err != nil {
		tb.Fatalf("open request evidence performance database: %v", err)
	}
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	sqlDB, err := db.DB()
	if err != nil {
		tb.Fatalf("get request evidence performance database handle: %v", err)
	}
	tb.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			tb.Errorf("close request evidence performance database: %v", err)
		}
	})

	fixture := newAnalyticsCoreCardinalityFixture(
		eventCount,
		analyticsCoreBenchmarkProviderCount,
		analyticsCoreBenchmarkModelCount,
		analyticsCoreBenchmarkIdentityCount,
	)
	seedAnalyticsCoreCardinalityFixture(tb, db, fixture, false)
	if err := db.Exec("ANALYZE").Error; err != nil {
		tb.Fatalf("analyze request evidence performance fixture: %v", err)
	}
	return db, fixture
}

func usageEventBenchmarkResultFilter(failed bool) string {
	if failed {
		return "failed"
	}
	return "success"
}

func benchmarkUsageAttempts(requestCount int, attemptsPerRequest int) []entities.UsageEvent {
	start := time.Date(2026, 8, 31, 0, 0, 0, 0, time.UTC)
	events := make([]entities.UsageEvent, 0, requestCount*attemptsPerRequest)
	for requestIndex := 0; requestIndex < requestCount; requestIndex++ {
		requestID := fmt.Sprintf("request-%05d", requestIndex)
		for attemptIndex := 0; attemptIndex < attemptsPerRequest; attemptIndex++ {
			eventIndex := requestIndex*attemptsPerRequest + attemptIndex
			events = append(events, entities.UsageEvent{
				EventKey:        fmt.Sprintf("redis-inbox:%06d", eventIndex),
				RequestID:       requestID,
				APIGroupKey:     fmt.Sprintf("sk-bench-%04d", requestIndex%512),
				Provider:        fmt.Sprintf("provider-%02d", requestIndex%32),
				Model:           fmt.Sprintf("model-%03d", requestIndex%512),
				Timestamp:       start.Add(time.Duration(eventIndex%48) * time.Hour),
				Source:          fmt.Sprintf("source-%03d", requestIndex%128),
				AuthIndex:       fmt.Sprintf("auth-%04d", requestIndex%2_048),
				Failed:          eventIndex%17 == 0,
				LatencyMS:       int64(100 + eventIndex%1_000),
				InputTokens:     int64(100 + eventIndex%500),
				OutputTokens:    int64(50 + eventIndex%250),
				ReasoningTokens: int64(eventIndex % 80),
				CachedTokens:    int64(eventIndex % 100),
				TotalTokens:     int64(150 + eventIndex%900),
			})
		}
	}
	return events
}

func openUsageEventsPerformanceDatabase(b *testing.B, name string, benchmarkIndex int) *gorm.DB {
	b.Helper()
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(b.TempDir(), fmt.Sprintf("%s-%d.db", name, benchmarkIndex))})
	if err != nil {
		b.Fatalf("open usage events performance database: %v", err)
	}
	return db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
}

func closeUsageEventsPerformanceDatabase(b *testing.B, db *gorm.DB) {
	b.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get usage events performance database handle: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		b.Fatalf("close usage events performance database: %v", err)
	}
}

func usageEventsDatabaseBytes(b *testing.B, db *gorm.DB) int64 {
	b.Helper()
	var pageCount, pageSize int64
	if err := db.Raw("PRAGMA page_count").Scan(&pageCount).Error; err != nil {
		b.Fatalf("read usage events benchmark page count: %v", err)
	}
	if err := db.Raw("PRAGMA page_size").Scan(&pageSize).Error; err != nil {
		b.Fatalf("read usage events benchmark page size: %v", err)
	}
	return pageCount * pageSize
}
