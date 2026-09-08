package repository

import (
	"reflect"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func TestIncrementalUsageRollupsMatchFullRebuildAcrossAccountingAndLateEvents(t *testing.T) {
	db := openTestDatabase(t)
	bucket := time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)
	complete := canonicalUsageTestEvent(entities.UsageEvent{
		EventKey: "complete", Provider: "\t OpenAI\u2003\n", Model: "\u00a0model\v", AuthType: "\r apikey\u2029", AuthIndex: "\u3000auth-1\f", APIGroupKey: "\u205fsk-primary\u202f",
		Timestamp: bucket.Add(40 * time.Minute), LatencyMS: 120, InputTokens: 100, OutputTokens: 40, ReasoningTokens: 10, TotalTokens: 140,
	})
	inconsistent := usageRollupAccountingEvent("inconsistent", "inconsistent", bucket.Add(50*time.Minute), true, 0)
	unclassified := usageRollupAccountingEvent("unclassified", "unclassified", bucket.Add(55*time.Minute), false, 80)
	absent := entities.UsageEvent{
		EventKey: "absent", Provider: "OpenAI", Model: "model", AuthType: "\tapikey\n", AuthIndex: "auth-1", Source: "\u1680sk-primary\u0085",
		Timestamp: bucket.Add(45 * time.Minute), Failed: true, LatencyMS: -1,
	}
	firstBatch := []entities.UsageEvent{complete, inconsistent, unclassified, absent}
	if inserted, deduped, err := InsertUsageEvents(db, firstBatch); err != nil || inserted != len(firstBatch) || deduped != 0 {
		t.Fatalf("insert first batch: inserted=%d deduped=%d err=%v", inserted, deduped, err)
	}

	late := canonicalUsageTestEvent(entities.UsageEvent{
		EventKey: "late", Provider: "OpenAI", Model: "model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-primary",
		Timestamp: bucket.Add(5 * time.Minute), LatencyMS: 30, InputTokens: 12, TotalTokens: 12,
	})
	if inserted, deduped, err := InsertUsageEvents(db, []entities.UsageEvent{late}); err != nil || inserted != 1 || deduped != 0 {
		t.Fatalf("insert late event: inserted=%d deduped=%d err=%v", inserted, deduped, err)
	}
	if inserted, deduped, err := InsertUsageEvents(db, append(firstBatch, late)); err != nil || inserted != 0 || deduped != len(firstBatch)+1 {
		t.Fatalf("replay events: inserted=%d deduped=%d err=%v", inserted, deduped, err)
	}

	incremental := loadNormalizedUsageRollups(t, db)
	if len(incremental) != 1 || !incremental[0].LastEventAt.Equal(unclassified.Timestamp) {
		t.Fatalf("late event must not move last_event_at backwards: %+v", incremental)
	}
	if err := RebuildUsageRollupsForEvents(db, append(firstBatch, late)); err != nil {
		t.Fatalf("full rebuild: %v", err)
	}
	rebuilt := loadNormalizedUsageRollups(t, db)
	if !reflect.DeepEqual(incremental, rebuilt) {
		t.Fatalf("incremental rollups differ from full rebuild\nincremental=%+v\nrebuilt=%+v", incremental, rebuilt)
	}
	if err := db.Where("bucket_start = ?", bucket).Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		t.Fatalf("clear rollup before SQL rebuild: %v", err)
	}
	if err := rebuildUsageRollupsForBucketRange(db, bucket, bucket); err != nil {
		t.Fatalf("SQL range rebuild: %v", err)
	}
	sqlRebuilt := loadNormalizedUsageRollups(t, db)
	if !reflect.DeepEqual(rebuilt, sqlRebuilt) {
		t.Fatalf("SQL range rebuild differs from Go rebuild\ngo=%+v\nsql=%+v", rebuilt, sqlRebuilt)
	}
}

func TestBackfillAndIncrementalIngestionConvergeWhenStartedTogether(t *testing.T) {
	for iteration := 0; iteration < 8; iteration++ {
		db := openTestDatabase(t)
		bucket := time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)
		base := canonicalUsageTestEvent(entities.UsageEvent{
			EventKey: "base", Provider: "OpenAI", Model: "model", Timestamp: bucket.Add(40 * time.Minute), InputTokens: 100, TotalTokens: 100,
		})
		if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{base}); err != nil {
			t.Fatalf("iteration %d insert base event: %v", iteration, err)
		}
		if err := db.Where("1 = 1").Delete(&entities.UsageRollupHourly{}).Error; err != nil {
			t.Fatalf("iteration %d clear initial rollup: %v", iteration, err)
		}
		if err := SaveUsageRollupBackfillStatus(db, repodto.RollupBackfillStatus{Status: repodto.RollupBackfillStatusPending, TargetBucketStart: &bucket}); err != nil {
			t.Fatalf("iteration %d seed backfill status: %v", iteration, err)
		}
		late := canonicalUsageTestEvent(entities.UsageEvent{
			EventKey: "late", Provider: "OpenAI", Model: "model", Timestamp: bucket.Add(5 * time.Minute), InputTokens: 25, TotalTokens: 25,
		})

		start := make(chan struct{})
		errs := make(chan error, 2)
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, err := BackfillUsageRollupsBatch(db, bucket.Add(2*time.Hour), 24)
			errs <- err
		}()
		go func() {
			defer wg.Done()
			<-start
			_, _, err := InsertUsageEvents(db, []entities.UsageEvent{late})
			errs <- err
		}()
		close(start)
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("iteration %d concurrent operation: %v", iteration, err)
			}
		}

		concurrentResult := loadNormalizedUsageRollups(t, db)
		if err := RebuildUsageRollupsForEvents(db, []entities.UsageEvent{base, late}); err != nil {
			t.Fatalf("iteration %d rebuild final oracle: %v", iteration, err)
		}
		oracle := loadNormalizedUsageRollups(t, db)
		if !reflect.DeepEqual(concurrentResult, oracle) {
			t.Fatalf("iteration %d concurrent backfill/ingestion did not converge\nconcurrent=%+v\noracle=%+v", iteration, concurrentResult, oracle)
		}
	}
}

func usageRollupAccountingEvent(eventKey, quality string, timestamp time.Time, failed bool, latencyMS int64) entities.UsageEvent {
	total := int64(45)
	input := int64(20)
	uncached := int64(12)
	cacheRead := int64(5)
	cacheWrite := int64(3)
	output := int64(15)
	nonReasoning := int64(11)
	reasoning := int64(4)
	unclassified := int64(10)
	return entities.UsageEvent{
		EventKey: eventKey, Provider: "OpenAI", Model: "model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-primary",
		Timestamp: timestamp, Failed: failed, LatencyMS: latencyMS,
		UsageAccounting: entities.UsageAccounting{
			TokenQuality: &quality, CanonicalTotalTokens: &total, CanonicalInputTokens: &input,
			CanonicalUncachedTokens: &uncached, CanonicalCacheReadTokens: &cacheRead, CanonicalCacheWriteTokens: &cacheWrite,
			CanonicalOutputTokens: &output, CanonicalNonReasoningTokens: &nonReasoning, CanonicalReasoningTokens: &reasoning,
			CanonicalUnclassifiedTokens: &unclassified,
		},
	}
}

func loadNormalizedUsageRollups(t *testing.T, db *gorm.DB) []entities.UsageRollupHourly {
	t.Helper()
	var rollups []entities.UsageRollupHourly
	if err := db.Order("bucket_start ASC, provider ASC, model ASC, auth_type ASC, auth_index ASC, api_key_identity ASC").Find(&rollups).Error; err != nil {
		t.Fatalf("load usage rollups: %v", err)
	}
	for index := range rollups {
		rollups[index].ID = 0
		rollups[index].CreatedAt = time.Time{}
		rollups[index].UpdatedAt = time.Time{}
	}
	return rollups
}
