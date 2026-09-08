package repository

import (
	"context"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestUsageModelMappingSummaryMatchesFullSnapshotPopulation(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 8, 8, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	aliasA, aliasB := "route-a", "route-b"
	events := []entities.UsageEvent{
		{EventKey: "a-1", Timestamp: start, Provider: "provider-a", ModelAlias: &aliasA, Model: "actual-a"},
		{EventKey: "a-2", Timestamp: start.Add(time.Hour), Provider: "provider-a", ModelAlias: &aliasA, Model: "actual-b"},
		{EventKey: "a-missing", Timestamp: start.Add(2 * time.Hour), Provider: "provider-a", Model: "actual-a"},
		{EventKey: "b", Timestamp: start.Add(3 * time.Hour), Provider: "provider-b", ModelAlias: &aliasB, Model: "actual-b"},
		{EventKey: "outside", Timestamp: end.Add(time.Nanosecond), Provider: "provider-a", ModelAlias: &aliasA, Model: "actual-a"},
	}
	if _, _, err := insertCanonicalUsageTestEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	filter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end, Provider: "provider-a"}}

	summary, err := BuildUsageModelMappingsSummaryWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build summary: %v", err)
	}
	full, err := BuildUsageModelMappingsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build full mappings: %v", err)
	}
	if summary.TotalAttempts != full.TotalAttempts || summary.ObservedAliasAttempts != full.ObservedAliasAttempts || summary.MissingAliasAttempts != full.MissingAliasAttempts || summary.DisplayedMappings != int64(len(full.Mappings)) {
		t.Fatalf("summary/full population mismatch: summary=%+v full=%+v", summary, full)
	}
	if summary.TotalAttempts != 3 || summary.ObservedAliasAttempts != 2 || summary.MissingAliasAttempts != 1 || summary.DisplayedMappings != 2 {
		t.Fatalf("unexpected scoped summary: %+v", summary)
	}
}

func TestUsageModelMappingSummaryRequiresBoundedWindow(t *testing.T) {
	if _, err := BuildUsageModelMappingsSummaryWithFilter(context.Background(), openTestDatabase(t), dto.UsageDiagnosticFilter{}); err == nil {
		t.Fatal("expected an unbounded summary query to fail")
	}
}
