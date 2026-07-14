package service

import (
	"context"
	"math"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
)

func TestUsageServiceGetUsageWithFilterDelegatesToFilteredSnapshot(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-filter.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), TotalTokens: 10},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), TotalTokens: 20},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 9, 30, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 10, 30, 0, 0, time.UTC)
	provider := NewUsageService(db)
	snapshot, err := provider.GetUsageWithFilter(context.Background(), servicedto.UsageFilter{StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("GetUsageWithFilter returned error: %v", err)
	}
	if snapshot.TotalRequests != 1 || snapshot.TotalTokens != 20 {
		t.Fatalf("expected service filter to keep only in-range event, got %+v", snapshot)
	}
}

func TestUsageServiceListUsageEventsDerivesOutputTPSFromTTFT(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-events.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	ttftMS := int64(1052)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:     "event-tps",
		Model:        "gpt-5.6-sol",
		Timestamp:    time.Date(2026, 7, 14, 9, 7, 50, 0, time.UTC),
		LatencyMS:    21245,
		TTFTMS:       &ttftMS,
		OutputTokens: 976,
		TotalTokens:  105091,
	}}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	provider := NewUsageService(db)
	page, err := provider.ListUsageEvents(context.Background(), servicedto.UsageFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListUsageEvents returned error: %v", err)
	}
	if len(page.Events) != 1 {
		t.Fatalf("expected one usage event, got %+v", page.Events)
	}
	event := page.Events[0]
	if event.TTFTMS == nil || *event.TTFTMS != 1052 {
		t.Fatalf("expected ttft_ms 1052, got %+v", event.TTFTMS)
	}
	if event.OutputTPS == nil || math.Abs(*event.OutputTPS-48.33358094488189) > 0.000000001 {
		t.Fatalf("expected output TPS 48.33358094488189, got %+v", event.OutputTPS)
	}
}

func TestUsageServiceListUsageEventsLeavesInvalidOutputTPSUnavailable(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-invalid-tps.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	zeroTTFT := int64(0)
	equalTTFT := int64(21245)
	validTTFT := int64(1052)
	events := []entities.UsageEvent{
		{EventKey: "missing-ttft", Model: "missing-ttft", Timestamp: time.Date(2026, 7, 14, 9, 7, 50, 0, time.UTC), LatencyMS: 21245, OutputTokens: 976},
		{EventKey: "zero-ttft", Model: "zero-ttft", Timestamp: time.Date(2026, 7, 14, 9, 7, 51, 0, time.UTC), LatencyMS: 21245, TTFTMS: &zeroTTFT, OutputTokens: 976},
		{EventKey: "inconsistent-duration", Model: "inconsistent-duration", Timestamp: time.Date(2026, 7, 14, 9, 7, 52, 0, time.UTC), LatencyMS: 21245, TTFTMS: &equalTTFT, OutputTokens: 976},
		{EventKey: "zero-output", Model: "zero-output", Timestamp: time.Date(2026, 7, 14, 9, 7, 53, 0, time.UTC), LatencyMS: 21245, TTFTMS: &validTTFT},
		{EventKey: "failed-valid", Model: "failed-valid", Timestamp: time.Date(2026, 7, 14, 9, 7, 54, 0, time.UTC), Failed: true, LatencyMS: 21245, TTFTMS: &validTTFT, OutputTokens: 976},
	}
	if _, _, err := repository.InsertUsageEvents(db, events); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	provider := NewUsageService(db)
	page, err := provider.ListUsageEvents(context.Background(), servicedto.UsageFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("ListUsageEvents returned error: %v", err)
	}
	byModel := make(map[string]servicedto.UsageEventRecord, len(page.Events))
	for _, event := range page.Events {
		byModel[event.Model] = event
	}
	for _, model := range []string{"missing-ttft", "zero-ttft", "inconsistent-duration", "zero-output"} {
		if byModel[model].OutputTPS != nil {
			t.Fatalf("expected %s Output TPS to be unavailable, got %+v", model, byModel[model].OutputTPS)
		}
	}
	failedEvent := byModel["failed-valid"]
	if !failedEvent.Failed || failedEvent.OutputTPS == nil || math.Abs(*failedEvent.OutputTPS-48.33358094488189) > 0.000000001 {
		t.Fatalf("expected failed event with valid facts to retain Output TPS, got %+v", failedEvent)
	}
}

func TestUsageServiceGetUsageOverviewDelegatesToFilteredOverview(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-overview.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, err := repository.UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "claude-sonnet",
		PromptPricePer1M:     3,
		CompletionPricePer1M: 15,
		CachePricePer1M:      0.3,
	}); err != nil {
		t.Fatalf("UpsertModelPriceSetting returned error: %v", err)
	}
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "event-1", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), InputTokens: 1000, OutputTokens: 500, CachedTokens: 100, ReasoningTokens: 50, TotalTokens: 1650},
		{EventKey: "event-2", APIGroupKey: "provider-a", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), InputTokens: 500, OutputTokens: 250, CachedTokens: 0, ReasoningTokens: 25, TotalTokens: 775},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 16, 23, 59, 59, 0, time.UTC)
	provider := NewUsageService(db)
	overview, err := provider.GetUsageOverview(context.Background(), servicedto.UsageFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("GetUsageOverview returned error: %v", err)
	}
	if overview.Summary.RequestCount != 2 || overview.Summary.TokenCount != 2425 {
		t.Fatalf("expected overview summary counts, got %+v", overview.Summary)
	}
	if overview.Summary.WindowMinutes != 1440 {
		t.Fatalf("expected 24h overview to use exact 1440 minute window, got %+v", overview.Summary)
	}
	if overview.Series.Requests["2026-04-16T09:00:00Z"] != 1 || overview.Series.Requests["2026-04-16T10:00:00Z"] != 1 {
		t.Fatalf("expected hourly request series values, got %+v", overview.Series)
	}
	if math.Abs(overview.Series.Cost["2026-04-16T09:00:00Z"]-0.01023) > 0.000000001 || math.Abs(overview.Series.Cost["2026-04-16T10:00:00Z"]-0.00525) > 0.000000001 {
		t.Fatalf("expected hourly cost series values, got %+v", overview.Series)
	}
}

func TestUsageServiceGetRequestHealthDelegatesToFilteredProjection(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "usage-service-request-health.db")})
	if err != nil {
		t.Fatalf("OpenDatabase returned error: %v", err)
	}
	closeTestDatabase(t, db)
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "event-codex-success", Provider: "codex", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 9, 0, 0, 0, time.UTC), Failed: false, TotalTokens: 10},
		{EventKey: "event-codex-failed", Provider: "codex", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 10, 0, 0, 0, time.UTC), Failed: true, TotalTokens: 20},
		{EventKey: "event-other", Provider: "claude", Model: "claude-sonnet", Timestamp: time.Date(2026, 4, 16, 11, 0, 0, 0, time.UTC), Failed: true, TotalTokens: 30},
	}); err != nil {
		t.Fatalf("InsertUsageEvents returned error: %v", err)
	}

	start := time.Date(2026, 4, 16, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 4, 17, 0, 0, 0, 0, time.UTC)
	provider := NewUsageService(db)
	health, err := provider.GetRequestHealth(context.Background(), servicedto.UsageFilter{Range: "24h", StartTime: &start, EndTime: &end, Provider: "codex"})
	if err != nil {
		t.Fatalf("GetRequestHealth returned error: %v", err)
	}

	if health.TotalSuccess != 1 || health.TotalFailure != 1 || health.SuccessRate != 50 {
		t.Fatalf("expected provider-scoped request health totals, got %+v", health)
	}
	if health.Rows != 8 || health.Columns != 60 || health.BucketSeconds != 180 {
		t.Fatalf("expected fixed 24h request health grid, got %+v", health)
	}
}
