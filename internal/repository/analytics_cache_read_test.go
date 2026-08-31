package repository

import (
	"math"
	"testing"

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
