package repository

import (
	"math"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestAnalyticsCacheReadMetricsUseTokenWeightedCoverage(t *testing.T) {
	tests := []struct {
		name          string
		inputTokens   int64
		observedInput int64
		cacheRead     int64
		wantShare     float64
		wantCoverage  float64
		wantState     string
	}{
		{name: "exact", inputTokens: 1_000, observedInput: 1_000, cacheRead: 250, wantShare: 25, wantCoverage: 100, wantState: dto.AnalyticsCacheReadShareStateAvailable},
		{name: "partial", inputTokens: 1_000, observedInput: 400, cacheRead: 100, wantShare: 25, wantCoverage: 40, wantState: dto.AnalyticsCacheReadShareStatePartial},
		{name: "explicit zero is observed", inputTokens: 1_000, observedInput: 1_000, cacheRead: 0, wantShare: 0, wantCoverage: 100, wantState: dto.AnalyticsCacheReadShareStateAvailable},
		{name: "no exact cache data", inputTokens: 1_000, observedInput: 0, cacheRead: 0, wantShare: 0, wantCoverage: 0, wantState: dto.AnalyticsCacheReadShareStateNoCacheData},
		{name: "no prompt input", inputTokens: 0, observedInput: 0, cacheRead: 0, wantShare: 0, wantCoverage: 0, wantState: dto.AnalyticsCacheReadShareStateNoPromptInput},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			share, coverage, state := analyticsCacheReadMetrics(tt.inputTokens, tt.observedInput, tt.cacheRead)
			if math.Abs(share-tt.wantShare) > 1e-9 || math.Abs(coverage-tt.wantCoverage) > 1e-9 || state != tt.wantState {
				t.Fatalf("expected share=%v coverage=%v state=%q, got share=%v coverage=%v state=%q", tt.wantShare, tt.wantCoverage, tt.wantState, share, coverage, state)
			}
		})
	}
}

func TestAnalyticsCacheReadInvalidObservationsStayRawAndMatchRollups(t *testing.T) {
	tests := []struct {
		name        string
		inputTokens int64
		cacheRead   int64
	}{
		{name: "cache read exceeds input", inputTokens: 100, cacheRead: 101},
		{name: "cache read with zero input", inputTokens: 0, cacheRead: 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDatabase(t)
			start := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
			end := start.Add(time.Hour).Add(-time.Nanosecond)
			event := entities.UsageEvent{
				EventKey:        "invalid-cache-read",
				Provider:        "provider",
				Model:           "model",
				Timestamp:       start.Add(5 * time.Minute),
				InputTokens:     tt.inputTokens,
				CacheReadTokens: &tt.cacheRead,
				TotalTokens:     tt.inputTokens,
			}
			if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{event}); err != nil {
				t.Fatalf("insert invalid cache-read observation: %v", err)
			}

			var stored entities.UsageEvent
			if err := db.Where("event_key = ?", event.EventKey).First(&stored).Error; err != nil {
				t.Fatalf("load raw cache-read evidence: %v", err)
			}
			if stored.CacheReadTokens == nil || *stored.CacheReadTokens != tt.cacheRead || stored.InputTokens != tt.inputTokens {
				t.Fatalf("expected raw evidence to remain unchanged, got %+v", stored)
			}

			filter := dto.AnalyticsFilter{
				UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end},
				Range:          "custom",
				Granularity:    "hour",
				FixedWindowEnd: &end,
			}
			raw, err := buildAnalyticsAggregateRow(db, filter, analyticsEventsAggregateSource())
			if err != nil {
				t.Fatalf("build raw analytics aggregate: %v", err)
			}
			rollup, err := buildAnalyticsAggregateRow(db, filter, analyticsRollupsAggregateSource())
			if err != nil {
				t.Fatalf("build rollup analytics aggregate: %v", err)
			}
			if raw.CacheReadTokens != 0 || raw.CacheReadObservedInputTokens != 0 {
				t.Fatalf("expected invalid raw observation to be excluded, got %+v", raw)
			}
			if rollup.CacheReadTokens != raw.CacheReadTokens || rollup.CacheReadObservedInputTokens != raw.CacheReadObservedInputTokens {
				t.Fatalf("expected raw/rollup cache-read parity, raw=%+v rollup=%+v", raw, rollup)
			}
		})
	}
}
