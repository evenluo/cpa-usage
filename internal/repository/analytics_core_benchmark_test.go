package repository

import (
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const (
	analyticsCoreBenchmarkEventCount    = 65_536
	analyticsCoreBenchmarkProviderCount = 32
	analyticsCoreBenchmarkModelCount    = 512
	analyticsCoreBenchmarkIdentityCount = 2_048
)

var analyticsCoreBenchmarkResult *dto.AnalyticsSummarySnapshot

func BenchmarkAnalyticsCoreRawFastHighCardinality(b *testing.B) {
	db, filter, plan := prepareAnalyticsCoreBenchmark(b, false)
	benchmarkAnalyticsCoreSnapshot(b, func() (*dto.AnalyticsSummarySnapshot, error) {
		return buildAnalyticsCoreSnapshot(db, filter, plan)
	})
}

func BenchmarkAnalyticsCoreRawSharedPlanHighCardinality(b *testing.B) {
	db, filter, plan := prepareAnalyticsCoreBenchmark(b, false)
	benchmarkAnalyticsCoreSnapshot(b, func() (*dto.AnalyticsSummarySnapshot, error) {
		return buildAnalyticsCoreSharedCandidateForProof(db, filter, plan)
	})
}

func BenchmarkAnalyticsCoreCoveredRollupHighCardinality(b *testing.B) {
	db, filter, plan := prepareAnalyticsCoreBenchmark(b, true)
	if plan.rollupFilter == nil {
		b.Fatal("benchmark setup expected a covered rollup plan")
	}
	benchmarkAnalyticsCoreSnapshot(b, func() (*dto.AnalyticsSummarySnapshot, error) {
		return buildAnalyticsCoreSnapshot(db, filter, plan)
	})
}

func prepareAnalyticsCoreBenchmark(b *testing.B, includeRollups bool) (*gorm.DB, dto.AnalyticsFilter, analyticsCoreWindowPlan) {
	b.Helper()
	previousLocal := time.Local
	time.Local = time.UTC
	b.Cleanup(func() { time.Local = previousLocal })

	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(b.TempDir(), "analytics-core-benchmark.db")})
	if err != nil {
		b.Fatalf("open analytics benchmark database: %v", err)
	}
	db = db.Session(&gorm.Session{Logger: logger.Default.LogMode(logger.Silent)})
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("get analytics benchmark database: %v", err)
	}
	b.Cleanup(func() {
		if err := sqlDB.Close(); err != nil {
			b.Errorf("close analytics benchmark database: %v", err)
		}
	})

	fixture := newAnalyticsCoreCardinalityFixture(
		analyticsCoreBenchmarkEventCount,
		analyticsCoreBenchmarkProviderCount,
		analyticsCoreBenchmarkModelCount,
		analyticsCoreBenchmarkIdentityCount,
	)
	seedAnalyticsCoreCardinalityFixture(b, db, fixture, includeRollups)
	filter := dto.AnalyticsFilter{
		UsageTimeScope: dto.UsageTimeScope{StartTime: &fixture.start, EndTime: &fixture.end},
		Range:          "custom",
		Granularity:    "hour",
		FixedWindowEnd: &fixture.end,
	}
	plan := analyticsCoreRawWindowPlan(filter)
	if includeRollups {
		plan = analyticsCoreRollupWindowPlan(filter)
	}
	b.ReportMetric(analyticsCoreBenchmarkEventCount, "fixture_events")
	b.ReportMetric(analyticsCoreBenchmarkProviderCount, "fixture_providers")
	b.ReportMetric(analyticsCoreBenchmarkModelCount, "fixture_models")
	b.ReportMetric(analyticsCoreBenchmarkIdentityCount, "fixture_identities_per_kind")
	b.ReportMetric(float64(runtime.GOMAXPROCS(0)), "gomaxprocs")
	return db, filter, plan
}

func benchmarkAnalyticsCoreSnapshot(b *testing.B, build func() (*dto.AnalyticsSummarySnapshot, error)) {
	b.Helper()
	warm, err := build()
	if err != nil {
		b.Fatalf("warm analytics benchmark: %v", err)
	}
	if len(warm.KeyAliasBreakdown) != analyticsKeyAliasBreakdownLimit || len(warm.APIKeyBreakdown) != analyticsKeyAliasBreakdownLimit || len(warm.ModelBreakdown) != 20 {
		b.Fatalf("benchmark fixture did not exercise top-N: aliases=%d api_keys=%d models=%d", len(warm.KeyAliasBreakdown), len(warm.APIKeyBreakdown), len(warm.ModelBreakdown))
	}
	if len(warm.ProviderOptions) != analyticsCoreBenchmarkProviderCount {
		b.Fatalf("benchmark fixture did not exercise every provider: got %d", len(warm.ProviderOptions))
	}
	b.ReportMetric(float64(len(warm.KeyAliasBreakdown)), "result_alias_rows")
	b.ReportMetric(float64(len(warm.APIKeyBreakdown)), "result_api_key_rows")
	b.ReportMetric(float64(len(warm.ModelBreakdown)), "result_model_rows")
	b.ReportMetric(float64(len(warm.ProviderOptions)), "result_provider_rows")

	b.ReportAllocs()
	b.ResetTimer()
	for benchmarkIndex := 0; benchmarkIndex < b.N; benchmarkIndex++ {
		result, err := build()
		if err != nil {
			b.Fatalf("build analytics benchmark snapshot: %v", err)
		}
		analyticsCoreBenchmarkResult = result
	}
}
