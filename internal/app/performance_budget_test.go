package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sort"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/backup"
	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
	"gorm.io/gorm/logger"
)

type budgetQueue struct{ messages []string }

func (q budgetQueue) PopUsage(context.Context) ([]string, error) { return q.messages, nil }

type budgetWorkload struct {
	app         *App
	syncer      *service.SyncService
	end         time.Time
	seed, batch int
	backupDir   string
}
type budgetResult struct{ p95, ingestion, wait time.Duration }

func newBudgetWorkload(t testing.TB, seed, batch int) *budgetWorkload {
	t.Helper()
	cfg := config.Config{AppPort: "8080", CPABaseURL: "http://127.0.0.1:1", CPAManagementKey: "synthetic", SQLitePath: t.TempDir() + "/app.db", RedisQueueIdleInterval: time.Second, RedisQueueErrorBackoff: time.Second, MetadataSyncInterval: time.Minute, RequestTimeout: time.Second, LogLevel: "error"}
	app, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = app.Close() })
	app.DB.Logger = logger.Default.LogMode(logger.Silent)
	end := time.Now().UTC().Truncate(time.Hour)
	events := make([]entities.UsageEvent, seed)
	for i := range events {
		timestamp := end.Add(-23 * time.Hour).Add(time.Duration(i%82800) * time.Second)
		raw := budgetMessage(timestamp, i)
		event, _, err := service.DecodeRedisUsageMessage(raw, timestamp)
		if err != nil {
			t.Fatal(err)
		}
		event.EventKey = fmt.Sprintf("budget-seed-%d", i)
		events[i] = event
	}
	if err := app.DB.CreateInBatches(events, 200).Error; err != nil {
		t.Fatal(err)
	}
	start := end.Add(-24 * time.Hour)
	if err := repository.RebuildUsageRollupsForBucketRange(app.DB, start, end); err != nil {
		t.Fatal(err)
	}
	if err := repository.SaveUsageRollupBackfillStatus(app.DB, dto.RollupBackfillStatus{Status: "completed", TargetBucketStart: &start, CoveredBucketStart: &start, CompletedAt: &end}); err != nil {
		t.Fatal(err)
	}
	messages := make([]string, batch)
	for i := range messages {
		messages[i] = budgetMessage(end.Add(-time.Minute), i)
	}
	syncer := service.NewSyncServiceWithOptions(app.DB, service.SyncServiceOptions{RedisQueue: budgetQueue{messages}, Now: func() time.Time { return end }})
	return &budgetWorkload{app: app, syncer: syncer, end: end, seed: seed, batch: batch, backupDir: t.TempDir()}
}

func budgetMessage(timestamp time.Time, i int) string {
	return fmt.Sprintf(`{"timestamp":%q,"provider":"codex","model":"model-%d","source":"synthetic","auth_type":"oauth","request_id":"budget-%d","latency_ms":%d,"ttft_ms":100,"accounting_version":2,"generate":true,"stream":true,"token_breakdown":{"schema_version":2,"quality":"complete","total_tokens":120,"input":{"total_tokens":100,"uncached_tokens":70,"cache_read_tokens":20,"cache_write_tokens":10},"output":{"total_tokens":20,"non_reasoning_tokens":15,"reasoning_tokens":5},"unclassified_tokens":0}}`, timestamp.Format(time.RFC3339), i%12, i, 1000+i%10000)
}

// Exercise real HTTP handlers, durable inbox processing, hourly backfill and an
// online backup concurrently. No remote service or app background runner starts.
func (w *budgetWorkload) run(t testing.TB) budgetResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	sqlDB, err := w.app.DB.DB()
	if err != nil {
		t.Fatal(err)
	}
	before := sqlDB.Stats()
	start := make(chan struct{})
	errs := make(chan error, 5)
	var wg sync.WaitGroup
	var durations []time.Duration
	var mu sync.Mutex
	var ingestTime time.Duration
	paths := []string{"/api/v1/analytics/core?range=24h", "/api/v1/usage/performance?range=24h&provider=codex", "/api/v1/usage/events?range=24h&page=1&page_size=50"}
	for _, path := range paths {
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			<-start
			for i := 0; i < 3; i++ {
				began := time.Now()
				req := httptest.NewRequest(http.MethodGet, path+"&window_end="+url.QueryEscape(w.end.Format(time.RFC3339)), nil).WithContext(ctx)
				rec := httptest.NewRecorder()
				w.app.Router.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					errs <- fmt.Errorf("%s: %d %s", path, rec.Code, rec.Body.String())
					return
				}
				mu.Lock()
				durations = append(durations, time.Since(began))
				mu.Unlock()
			}
		}(path)
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		began := time.Now()
		for i := 0; i < 4; i++ {
			if _, err := w.syncer.PullRedisUsageInbox(ctx); err != nil {
				errs <- err
				return
			}
			result, err := w.syncer.ProcessRedisUsageInbox(ctx)
			if err != nil {
				errs <- err
				return
			}
			if result.InsertedEvents != w.batch {
				errs <- fmt.Errorf("inserted %d, want %d", result.InsertedEvents, w.batch)
				return
			}
		}
		ingestTime = time.Since(began)
	}()
	wg.Add(1)
	go func() {
		defer wg.Done()
		<-start
		if err := repository.RebuildUsageRollupsForBucketRange(w.app.DB.WithContext(ctx), w.end.Add(-24*time.Hour), w.end); err != nil {
			errs <- err
			return
		}
		if _, err := backup.NewWriter(w.backupDir).WriteDatabase(ctx, sqlDB, time.Now()); err != nil {
			errs <- err
		}
	}()
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if t.Failed() {
		t.FailNow()
	}
	var raw, rolled, pending int64
	if err := w.app.DB.Model(&entities.UsageEvent{}).Count(&raw).Error; err != nil {
		t.Fatal(err)
	}
	if err := w.app.DB.Model(&entities.UsageRollupHourly{}).Select("COALESCE(SUM(request_count),0)").Scan(&rolled).Error; err != nil {
		t.Fatal(err)
	}
	if err := w.app.DB.Model(&entities.RedisUsageInbox{}).Where("status = ?", repository.RedisUsageInboxStatusPending).Count(&pending).Error; err != nil {
		t.Fatal(err)
	}
	if raw != int64(w.seed+4*w.batch) || rolled != raw || pending != 0 {
		t.Fatalf("mixed workload inconsistent: raw=%d rollup=%d pending=%d", raw, rolled, pending)
	}
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	return budgetResult{p95: durations[len(durations)-1], ingestion: ingestTime, wait: sqlDB.Stats().WaitDuration - before.WaitDuration}
}

func TestPerformanceBudgetMixedWorkload(t *testing.T) { newBudgetWorkload(t, 1024, 64).run(t) }

// Manual equal-work evidence, not a machine-dependent CI timing gate. With nine
// HTTP observations the nearest-rank p95 is their maximum; report this explicitly.
func BenchmarkPerformanceBudgetMixedWorkload(b *testing.B) {
	b.ResetTimer()
	var p95, ingestion, wait time.Duration
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		w := newBudgetWorkload(b, 32768, 250)
		b.StartTimer()
		r := w.run(b)
		b.StopTimer()
		if err := w.app.Close(); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
		p95 += r.p95
		ingestion += r.ingestion
		wait += r.wait
	}
	b.ReportMetric(float64(p95.Microseconds())/float64(b.N)/1000, "http-p95-ms/op")
	b.ReportMetric(float64(ingestion.Microseconds())/float64(b.N)/1000, "ingest-ms/op")
	b.ReportMetric(float64(wait.Microseconds())/float64(b.N)/1000, "db-wait-ms/op")
}
