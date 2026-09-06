package repository

import (
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func TestAnalyticsAccountingRawRollupParityForHistoricalCanonicalAndMixedWindows(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 7, 0, 0, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "historical-absent", Provider: "provider", Model: "model", Timestamp: start.Add(5 * time.Minute), InputTokens: 9, OutputTokens: 1, TotalTokens: 10},
	}
	for index, quality := range []string{"complete", "inconsistent", "unclassified"} {
		events = append(events, accountingAnalyticsEvent(
			"canonical-"+quality,
			start.Add(time.Hour+time.Duration(index)*time.Minute),
			validAnalyticsAccounting(quality),
		))
	}
	mixedStart := start.Add(2 * time.Hour)
	events = append(events,
		entities.UsageEvent{EventKey: "mixed-absent", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(time.Minute)},
		entities.UsageEvent{EventKey: "mixed-malformed", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(2 * time.Minute), UsageAccounting: entities.UsageAccounting{AccountingMalformed: true}},
		entities.UsageEvent{EventKey: "mixed-unsupported-version", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(3 * time.Minute), UsageAccounting: entities.UsageAccounting{AccountingVersion: accountingInt64Pointer(3)}},
		entities.UsageEvent{EventKey: "mixed-unsupported-schema", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(4 * time.Minute), UsageAccounting: entities.UsageAccounting{AccountingVersion: accountingInt64Pointer(2), TokenBreakdownPresent: true, TokenSchemaVersion: accountingInt64Pointer(3)}},
		entities.UsageEvent{EventKey: "mixed-missing", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(5 * time.Minute), UsageAccounting: entities.UsageAccounting{AccountingVersion: accountingInt64Pointer(2), TokenBreakdownPresent: true}},
		entities.UsageEvent{EventKey: "mixed-unknown-quality", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(6 * time.Minute), UsageAccounting: entities.UsageAccounting{AccountingVersion: accountingInt64Pointer(2), TokenBreakdownPresent: true, TokenSchemaVersion: accountingInt64Pointer(2), TokenQuality: accountingStringPointer("future")}},
		entities.UsageEvent{EventKey: "mixed-invalid", Provider: "provider", Model: "model", Timestamp: mixedStart.Add(7 * time.Minute), UsageAccounting: invalidAnalyticsAccounting()},
	)
	for index, quality := range []string{"complete", "inconsistent", "unclassified"} {
		events = append(events, accountingAnalyticsEvent(
			"mixed-valid-"+quality,
			mixedStart.Add(time.Duration(8+index)*time.Minute),
			validAnalyticsAccounting(quality),
		))
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert accounting analytics fixtures: %v", err)
	}

	assertAnalyticsAccountingParity(t, db, start, start.Add(time.Hour-time.Nanosecond), dto.AnalyticsAccountingSummary{
		TotalAttempts: 1,
		CoveragePct:   accountingFloat64Pointer(0),
		States:        dto.AnalyticsAccountingStates{Absent: 1},
	})
	assertAnalyticsAccountingParity(t, db, start.Add(time.Hour), start.Add(2*time.Hour-time.Nanosecond), expectedValidAnalyticsAccountingSummary(3))
	mixedExpected := expectedValidAnalyticsAccountingSummary(3)
	mixedExpected.TotalAttempts = 10
	mixedExpected.CoveragePct = accountingFloat64Pointer(30)
	mixedExpected.States = dto.AnalyticsAccountingStates{
		Absent: 1, Malformed: 1, UnsupportedAccountingVersion: 1, UnsupportedSchemaVersion: 1,
		Missing: 1, UnknownQuality: 1, Invalid: 1, Valid: 3,
	}
	assertAnalyticsAccountingParity(t, db, mixedStart, mixedStart.Add(time.Hour-time.Nanosecond), mixedExpected)

	var factsBefore []entities.UsageEvent
	if err := db.Order("id ASC").Find(&factsBefore).Error; err != nil {
		t.Fatalf("load raw accounting facts before backfill: %v", err)
	}
	if err := db.Model(&entities.UsageRollupHourly{}).Where("1 = 1").Updates(map[string]any{
		"accounting_absent_attempts": 0, "accounting_malformed_attempts": 0,
		"accounting_unsupported_version_attempts": 0, "accounting_unsupported_schema_attempts": 0,
		"accounting_missing_attempts": 0, "accounting_unknown_quality_attempts": 0,
		"accounting_invalid_attempts": 0, "accounting_valid_attempts": 0,
		"accounting_valid_complete_attempts": 0, "accounting_valid_inconsistent_attempts": 0,
		"accounting_valid_unclassified_attempts": 0, "canonical_total_tokens": 0,
		"canonical_input_tokens": 0, "canonical_uncached_tokens": 0,
		"canonical_cache_read_tokens": 0, "canonical_cache_write_tokens": 0,
		"canonical_output_tokens": 0, "canonical_non_reasoning_tokens": 0,
		"canonical_reasoning_tokens": 0, "canonical_unclassified_tokens": 0,
	}).Error; err != nil {
		t.Fatalf("clear accounting rollup projections: %v", err)
	}
	target := mixedStart
	covered := start.Add(-time.Hour)
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status: dto.RollupBackfillStatusPending, TargetBucketStart: &target, CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("schedule accounting rollup backfill: %v", err)
	}
	result, err := BackfillUsageRollupsBatch(db, mixedStart.Add(2*time.Hour), 3)
	if err != nil {
		t.Fatalf("backfill accounting rollups: %v", err)
	}
	if !result.Done || result.Status.Status != dto.RollupBackfillStatusCompleted || result.RebuiltBucketCount != 3 {
		t.Fatalf("expected one bounded accounting backfill batch to complete, got %+v", result)
	}
	var factsAfter []entities.UsageEvent
	if err := db.Order("id ASC").Find(&factsAfter).Error; err != nil {
		t.Fatalf("load raw accounting facts after backfill: %v", err)
	}
	if !reflect.DeepEqual(factsBefore, factsAfter) {
		t.Fatal("rollup backfill changed immutable raw accounting facts")
	}
	assertAnalyticsAccountingParity(t, db, start, start.Add(time.Hour-time.Nanosecond), dto.AnalyticsAccountingSummary{
		TotalAttempts: 1,
		CoveragePct:   accountingFloat64Pointer(0),
		States:        dto.AnalyticsAccountingStates{Absent: 1},
	})
	assertAnalyticsAccountingParity(t, db, start.Add(time.Hour), start.Add(2*time.Hour-time.Nanosecond), expectedValidAnalyticsAccountingSummary(3))
	assertAnalyticsAccountingParity(t, db, mixedStart, mixedStart.Add(time.Hour-time.Nanosecond), mixedExpected)
}

func assertAnalyticsAccountingParity(t *testing.T, db *gorm.DB, start time.Time, end time.Time, want dto.AnalyticsAccountingSummary) {
	t.Helper()
	filter := dto.AnalyticsFilter{
		UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end},
		Range:          "custom",
		Granularity:    "hour",
		FixedWindowEnd: &end,
	}
	rawRow, err := buildAnalyticsAggregateRow(db, filter, analyticsEventsAggregateSource())
	if err != nil {
		t.Fatalf("build raw accounting analytics: %v", err)
	}
	rollupRow, err := buildAnalyticsAggregateRow(db, filter, analyticsRollupsAggregateSource())
	if err != nil {
		t.Fatalf("build rollup accounting analytics: %v", err)
	}
	raw := mapAnalyticsSummary(rawRow)
	rollup := mapAnalyticsSummary(rollupRow)
	if !reflect.DeepEqual(raw.Accounting, rollup.Accounting) {
		t.Fatalf("expected raw/rollup accounting parity\nraw=%+v\nrollup=%+v", raw.Accounting, rollup.Accounting)
	}
	if !reflect.DeepEqual(raw.Accounting, want) {
		t.Fatalf("unexpected accounting summary\nwant=%+v\ngot=%+v", want, raw.Accounting)
	}
}

func accountingAnalyticsEvent(key string, timestamp time.Time, accounting entities.UsageAccounting) entities.UsageEvent {
	return entities.UsageEvent{
		EventKey: key, Provider: "provider", Model: "model", Timestamp: timestamp,
		InputTokens: 900, OutputTokens: 90, TotalTokens: 990, UsageAccounting: accounting,
	}
}

func validAnalyticsAccounting(quality string) entities.UsageAccounting {
	unclassified := int64(0)
	total := int64(130)
	if quality == "unclassified" {
		unclassified = 5
		total = 135
	}
	return entities.UsageAccounting{
		AccountingVersion: accountingInt64Pointer(2), TokenBreakdownPresent: true,
		TokenSchemaVersion: accountingInt64Pointer(2), TokenQuality: accountingStringPointer(quality),
		CanonicalTotalTokens: accountingInt64Pointer(total), CanonicalInputTokens: accountingInt64Pointer(100),
		CanonicalUncachedTokens: accountingInt64Pointer(50), CanonicalCacheReadTokens: accountingInt64Pointer(40),
		CanonicalCacheWriteTokens: accountingInt64Pointer(10), CanonicalOutputTokens: accountingInt64Pointer(30),
		CanonicalNonReasoningTokens: accountingInt64Pointer(18), CanonicalReasoningTokens: accountingInt64Pointer(12),
		CanonicalUnclassifiedTokens: accountingInt64Pointer(unclassified),
	}
}

func invalidAnalyticsAccounting() entities.UsageAccounting {
	accounting := validAnalyticsAccounting("complete")
	accounting.CanonicalUncachedTokens = accountingInt64Pointer(51)
	return accounting
}

func expectedValidAnalyticsAccountingSummary(attempts int64) dto.AnalyticsAccountingSummary {
	return dto.AnalyticsAccountingSummary{
		TotalAttempts: attempts, ValidAttempts: attempts, CoveragePct: accountingFloat64Pointer(100),
		States:       dto.AnalyticsAccountingStates{Valid: attempts},
		ValidQuality: dto.AnalyticsAccountingValidQuality{Complete: 1, Inconsistent: 1, Unclassified: 1},
		Composition: dto.AnalyticsAccountingComposition{
			TotalTokens:        395,
			Input:              dto.AnalyticsAccountingInputComposition{TotalTokens: 300, UncachedTokens: 150, CacheReadTokens: 120, CacheWriteTokens: 30},
			Output:             dto.AnalyticsAccountingOutputComposition{TotalTokens: 90, NonReasoningTokens: 54, ReasoningTokens: 36},
			UnclassifiedTokens: 5,
		},
	}
}

func accountingInt64Pointer(value int64) *int64       { return &value }
func accountingFloat64Pointer(value float64) *float64 { return &value }
func accountingStringPointer(value string) *string    { return &value }
