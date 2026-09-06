package service

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The same harness and fixtures run unmodified on the frozen baseline and this
// implementation. Timing includes projection, inbox/event writes, existing
// rollup rebuild, and processed marks; fixture/DB setup is outside the timer.
// No CPA network or production queue is involved.
func BenchmarkRedisUsageAccountingBatchEndToEnd(b *testing.B) {
	for _, version := range []string{"v7.2.62-legacy", "v7.2.152-complete"} {
		b.Run(version, func(b *testing.B) {
			data, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "usage", version+".json"))
			if err != nil {
				b.Fatal(err)
			}
			messages := make([]string, redisUsagePerformanceBatchSize)
			var totalBytes int64
			for i := range messages {
				messages[i] = strings.NewReplacer(
					"fixture-request", fmt.Sprintf("request-%04d", i/4),
					"fixture-model", fmt.Sprintf("model-%03d", i%32),
					"fixture-account", fmt.Sprintf("source-%03d", i%128),
					"fixture-auth", fmt.Sprintf("auth-%03d", i%128),
					"fixture-key", fmt.Sprintf("key-%03d", i%64),
				).Replace(string(data))
				totalBytes += int64(len(messages[i]))
			}
			b.SetBytes(totalBytes)
			b.ReportAllocs()
			b.ResetTimer()
			b.ReportMetric(redisUsagePerformanceBatchSize, "batch_messages")
			for n := 0; n < b.N; n++ {
				b.StopTimer()
				db := openRedisUsagePerformanceDatabase(b, version, n)
				queue := &redisUsagePerformanceQueue{messages: messages}
				svc := NewSyncServiceWithOptions(db, SyncServiceOptions{BaseURL: "https://cpa.invalid", RedisQueue: queue, RedisQueueKey: "usage", Now: func() time.Time { return time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC) }})
				b.StartTimer()
				pull, err := svc.PullRedisUsageInbox(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				process, err := svc.ProcessRedisUsageInbox(context.Background())
				if err != nil {
					b.Fatal(err)
				}
				b.StopTimer()
				if queue.calls != 1 || pull.InsertedRows != len(messages) || process.InsertedEvents != len(messages) {
					b.Fatalf("incomplete batch: %d %+v %+v", queue.calls, pull, process)
				}
				b.ReportMetric(float64(redisUsageDatabaseBytes(b, db)), "database_bytes")
				closeRedisUsagePerformanceDatabase(b, db)
			}
		})
	}
}
