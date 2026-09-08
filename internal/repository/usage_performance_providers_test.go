package repository

import (
	"context"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestUsagePerformanceProvidersPreserveExactWindowAndRawRollupParity(t *testing.T) {
	withRepositoryTestLocation(t, "UTC")
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 8, 10, 15, 0, 0, time.UTC)
	end := time.Date(2026, 9, 8, 12, 15, 0, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "before", Timestamp: start.Add(-time.Nanosecond), Provider: "excluded"},
		{EventKey: "start-a", Timestamp: start, Provider: "alpha"},
		{EventKey: "head-b", Timestamp: start.Add(30 * time.Minute), Provider: "beta"},
		{EventKey: "full-a", Timestamp: time.Date(2026, 9, 8, 11, 5, 0, 0, time.UTC), Provider: "alpha"},
		{EventKey: "full-b", Timestamp: time.Date(2026, 9, 8, 11, 10, 0, 0, time.UTC), Provider: "beta"},
		{EventKey: "full-c", Timestamp: time.Date(2026, 9, 8, 11, 15, 0, 0, time.UTC), Provider: "charlie"},
		{EventKey: "tail-c", Timestamp: end, Provider: "charlie"},
		{EventKey: "after", Timestamp: end.Add(time.Nanosecond), Provider: "excluded"},
		{EventKey: "blank", Timestamp: start.Add(time.Minute), Provider: "  "},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	scope := dto.UsageTimeScope{StartTime: &start, EndTime: &end}
	raw, err := ListUsagePerformanceProvidersWithFilter(context.Background(), db, scope)
	if err != nil {
		t.Fatalf("list raw providers: %v", err)
	}
	want := []dto.UsagePerformanceProviderOptionRecord{
		{Provider: "alpha", RequestCount: 2},
		{Provider: "beta", RequestCount: 2},
		{Provider: "charlie", RequestCount: 2},
	}
	if !reflect.DeepEqual(raw.ProviderOptions, want) {
		t.Fatalf("unexpected raw provider options: got=%+v want=%+v", raw.ProviderOptions, want)
	}

	target := end.UTC().Truncate(time.Hour)
	covered := target
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status: dto.RollupBackfillStatusCompleted, TargetBucketStart: &target, CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark rollups covered: %v", err)
	}
	rollup, err := ListUsagePerformanceProvidersWithFilter(context.Background(), db, scope)
	if err != nil {
		t.Fatalf("list mixed providers: %v", err)
	}
	if !reflect.DeepEqual(rollup.ProviderOptions, want) {
		t.Fatalf("raw/mixed provider options differ: got=%+v want=%+v", rollup.ProviderOptions, want)
	}
}

func TestUsagePerformanceProvidersRequiresBoundedWindow(t *testing.T) {
	if _, err := ListUsagePerformanceProvidersWithFilter(context.Background(), openTestDatabase(t), dto.UsageTimeScope{}); err == nil {
		t.Fatal("expected an unbounded provider catalog to fail")
	}
}
