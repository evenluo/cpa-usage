package repository

import (
	"math"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestBuildAnalyticsSummaryWithFilterAggregatesSummaryAndTrend(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 2,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	events := []entities.UsageEvent{
		{EventKey: "priced-day-1", Model: "priced-model", Timestamp: start.Add(9 * time.Hour), InputTokens: 1_000_000, OutputTokens: 500_000, CachedTokens: 100_000, TotalTokens: 1_600_000},
		{EventKey: "priced-day-2", Model: "priced-model", Timestamp: start.AddDate(0, 0, 1).Add(10 * time.Hour), InputTokens: 500_000, TotalTokens: 500_000},
		{EventKey: "unpriced-day-2", Model: "missing-model", Timestamp: start.AddDate(0, 0, 1).Add(11 * time.Hour), Failed: true, InputTokens: 100, TotalTokens: 100},
		{EventKey: "outside", Model: "priced-model", Timestamp: start.AddDate(0, 0, -1), InputTokens: 1_000_000, TotalTokens: 1_000_000},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.RequestCount != 3 || snapshot.Summary.TotalTokens != 2_100_100 {
		t.Fatalf("unexpected summary counts: %+v", snapshot.Summary)
	}
	if snapshot.Summary.SuccessCount != 2 || snapshot.Summary.FailureCount != 1 {
		t.Fatalf("unexpected success/failure counts: %+v", snapshot.Summary)
	}
	if diff := snapshot.Summary.SuccessRate - (2.0/3.0)*100; diff < -1e-9 || diff > 1e-9 {
		t.Fatalf("unexpected success rate: %+v", snapshot.Summary)
	}
	if math.Abs(snapshot.Summary.TotalCost-2.45) > 0.000000001 {
		t.Fatalf("unexpected total cost: %+v", snapshot.Summary)
	}
	if snapshot.Summary.CostAvailable || snapshot.Summary.CostStatus != "partial" {
		t.Fatalf("expected partial cost status, got %+v", snapshot.Summary)
	}
	if len(snapshot.Trend) != 2 {
		t.Fatalf("expected two trend points, got %+v", snapshot.Trend)
	}
	if snapshot.Trend[0].Label != "2026-05-11" || math.Abs(snapshot.Trend[0].TotalCost-1.95) > 0.000000001 || !snapshot.Trend[0].CostAvailable {
		t.Fatalf("unexpected first trend point: %+v", snapshot.Trend[0])
	}
	if snapshot.Trend[1].Label != "2026-05-12" || snapshot.Trend[1].CostAvailable || snapshot.Trend[1].CostStatus != "partial" {
		t.Fatalf("unexpected second trend point: %+v", snapshot.Trend[1])
	}
}

func TestBuildAnalyticsSummaryWithFilterMarksCostUnavailableWhenNoPricedCostExists(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "unpriced-only", Model: "missing-model", Timestamp: start.Add(time.Hour), InputTokens: 100, TotalTokens: 100,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.CostAvailable || snapshot.Summary.CostStatus != "unavailable" || snapshot.Summary.TotalCost != 0 {
		t.Fatalf("expected unavailable cost status, got %+v", snapshot.Summary)
	}
}

func TestBuildAnalyticsSummaryWithFilterReturnsEmptyState(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.RequestCount != 0 || snapshot.Summary.TotalTokens != 0 || snapshot.Summary.TotalCost != 0 {
		t.Fatalf("unexpected empty summary: %+v", snapshot.Summary)
	}
	if !snapshot.Summary.CostAvailable || snapshot.Summary.CostStatus != "available" {
		t.Fatalf("expected available cost status for empty state, got %+v", snapshot.Summary)
	}
	if len(snapshot.Trend) != 0 {
		t.Fatalf("expected empty trend, got %+v", snapshot.Trend)
	}
}
