package repository

import (
	"context"
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
	if snapshot.Summary.InputTokens != 1_500_100 || snapshot.Summary.CachedTokens != 100_000 || snapshot.Summary.CacheReadShareState != "available" {
		t.Fatalf("expected cache token summary from real fields, got %+v", snapshot.Summary)
	}
	if math.Abs(snapshot.Summary.CacheReadShare-6.666222251849877) > 0.000000001 {
		t.Fatalf("unexpected cache read share: %+v", snapshot.Summary)
	}
	if snapshot.Summary.EstimatedCacheSavings != nil {
		t.Fatalf("expected estimated cache savings to be withheld when pricing is partial, got %+v", *snapshot.Summary.EstimatedCacheSavings)
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

func TestBuildAnalyticsSummaryWithFilterExposesCacheEfficiencyWhenPricingIsComplete(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     2,
		CompletionPricePer1M: 4,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "cached", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(time.Hour),
		InputTokens: 1_000_000, CachedTokens: 250_000, OutputTokens: 100_000, TotalTokens: 1_350_000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.InputTokens != 1_000_000 || snapshot.Summary.CachedTokens != 250_000 {
		t.Fatalf("expected input and cached tokens in summary, got %+v", snapshot.Summary)
	}
	if snapshot.Summary.CacheReadShareState != dto.AnalyticsCacheReadShareStateAvailable {
		t.Fatalf("expected available cache read share, got %+v", snapshot.Summary)
	}
	if math.Abs(snapshot.Summary.CacheReadShare-25) > 0.000000001 {
		t.Fatalf("expected 25%% cache read share, got %+v", snapshot.Summary)
	}
	if snapshot.Summary.EstimatedCacheSavings == nil || math.Abs(*snapshot.Summary.EstimatedCacheSavings-0.375) > 0.000000001 {
		t.Fatalf("expected cache savings estimate, got %+v", snapshot.Summary.EstimatedCacheSavings)
	}
	if len(snapshot.ModelBreakdown) != 1 {
		t.Fatalf("expected one model row, got %+v", snapshot.ModelBreakdown)
	}
	model := snapshot.ModelBreakdown[0]
	if model.InputTokens != 1_000_000 || model.CachedTokens != 250_000 || model.CacheReadShareState != dto.AnalyticsCacheReadShareStateAvailable {
		t.Fatalf("expected model cache fields from same source, got %+v", model)
	}
	if model.EstimatedCacheSavings == nil || math.Abs(*model.EstimatedCacheSavings-0.375) > 0.000000001 {
		t.Fatalf("expected model cache savings estimate, got %+v", model.EstimatedCacheSavings)
	}
}

func TestBuildAnalyticsSummaryWithFilterSplitsCacheUnavailableStates(t *testing.T) {
	tests := []struct {
		name          string
		event         entities.UsageEvent
		expectedState string
	}{
		{
			name: "no cache data",
			event: entities.UsageEvent{
				EventKey: "no-cache-data", Model: "priced-model", InputTokens: 1000, TotalTokens: 1000,
			},
			expectedState: dto.AnalyticsCacheReadShareStateNoCacheData,
		},
		{
			name: "no prompt input",
			event: entities.UsageEvent{
				EventKey: "no-prompt-input", Model: "priced-model", OutputTokens: 1000, TotalTokens: 1000,
			},
			expectedState: dto.AnalyticsCacheReadShareStateNoPromptInput,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openTestDatabase(t)
			start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
			end := start.Add(24 * time.Hour)
			if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
				Model:                "priced-model",
				PromptPricePer1M:     1,
				CompletionPricePer1M: 1,
				CachePricePer1M:      0.5,
			}); err != nil {
				t.Fatalf("upsert pricing: %v", err)
			}
			tt.event.Timestamp = start.Add(time.Hour)
			if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{tt.event}); err != nil {
				t.Fatalf("insert events: %v", err)
			}

			snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
			if err != nil {
				t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
			}
			if snapshot.Summary.CacheReadShareState != tt.expectedState || snapshot.Summary.CacheReadShare != 0 || snapshot.Summary.EstimatedCacheSavings != nil {
				t.Fatalf("unexpected cache state for %s: %+v", tt.name, snapshot.Summary)
			}
		})
	}
}

func TestBuildAnalyticsSummaryWithFilterWithholdsCacheSavingsWhenPromptCachePricingIsInvalid(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "inverted-cache-price",
		PromptPricePer1M:     0.25,
		CompletionPricePer1M: 1,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "invalid-savings", Model: "inverted-cache-price", Timestamp: start.Add(time.Hour),
		InputTokens: 1_000_000, CachedTokens: 250_000, TotalTokens: 1_250_000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}
	if snapshot.Summary.CacheReadShareState != dto.AnalyticsCacheReadShareStateAvailable || snapshot.Summary.EstimatedCacheSavings != nil {
		t.Fatalf("expected share but no savings estimate for invalid pricing, got %+v", snapshot.Summary)
	}
}

func TestBuildAnalyticsSummaryWithFilterBucketsDailyTrendByLocalDay(t *testing.T) {
	t.Setenv("TZ", "Asia/Shanghai")
	withRepositoryTestLocation(t, "Asia/Shanghai")
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "local-day-event", Model: "priced-model",
		Timestamp:   time.Date(2026, 5, 11, 16, 30, 0, 0, time.UTC),
		InputTokens: 1000, TotalTokens: 1000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Trend) != 1 {
		t.Fatalf("expected one trend point, got %+v", snapshot.Trend)
	}
	if snapshot.Trend[0].Label != "2026-05-12" {
		t.Fatalf("expected local day bucket 2026-05-12, got %+v", snapshot.Trend[0])
	}
	if !snapshot.Trend[0].BucketStart.Equal(time.Date(2026, 5, 11, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected bucket start at local midnight, got %+v", snapshot.Trend[0])
	}
}

func TestBuildAnalyticsSummaryWithFilterBucketsDailyTrendAcrossDSTChange(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	withRepositoryTestLocation(t, "America/New_York")
	db := openTestDatabase(t)
	start := time.Date(2026, 3, 8, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 3, 10, 23, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "after-dst-change", Model: "priced-model",
		Timestamp:   time.Date(2026, 3, 9, 4, 30, 0, 0, time.UTC),
		InputTokens: 1000, TotalTokens: 1000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Trend) != 1 {
		t.Fatalf("expected one trend point, got %+v", snapshot.Trend)
	}
	if snapshot.Trend[0].Label != "2026-03-09" {
		t.Fatalf("expected event after DST change to use its local day, got %+v", snapshot.Trend[0])
	}
	if !snapshot.Trend[0].BucketStart.Equal(time.Date(2026, 3, 9, 4, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected bucket start at DST local midnight, got %+v", snapshot.Trend[0])
	}
}

func TestBuildAnalyticsSummaryWithFilterMarksCostUnavailableWhenNoPricedCostExists(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "unpriced-only", Model: "missing-model", Timestamp: start.Add(time.Hour), InputTokens: 100, TotalTokens: 100,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.CostAvailable || snapshot.Summary.CostStatus != "unavailable" || snapshot.Summary.TotalCost != 0 {
		t.Fatalf("expected unavailable cost status, got %+v", snapshot.Summary)
	}
}

func TestBuildAnalyticsSummaryWithFilterReturnsModelAndTimeBreakdowns(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	for _, input := range []dto.ModelPriceSettingInput{
		{Model: "priced-model", PromptPricePer1M: 1, CompletionPricePer1M: 1, CachePricePer1M: 1},
		{Model: "other-model", PromptPricePer1M: 2, CompletionPricePer1M: 1, CachePricePer1M: 1},
	} {
		if _, err := UpsertModelPriceSetting(db, input); err != nil {
			t.Fatalf("upsert pricing: %v", err)
		}
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "priced-openai-1", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000, LatencyMS: 100},
		{EventKey: "priced-openai-2", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(3 * time.Hour), OutputTokens: 500_000, TotalTokens: 500_000, Failed: true, LatencyMS: 300},
		{EventKey: "unpriced-openai", Provider: "OpenAI", Model: "missing-model", Timestamp: start.Add(26 * time.Hour), InputTokens: 2_000_000, TotalTokens: 2_000_000, LatencyMS: 200},
		{EventKey: "priced-anthropic", Provider: "Anthropic", Model: "other-model", Timestamp: start.Add(4 * time.Hour), InputTokens: 4_000_000, TotalTokens: 4_000_000, LatencyMS: 400},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.RequestCount != 3 || snapshot.Summary.TotalTokens != 3_500_000 {
		t.Fatalf("expected provider filter to limit summary to OpenAI events, got %+v", snapshot.Summary)
	}
	if len(snapshot.ModelBreakdown) != 2 {
		t.Fatalf("expected two OpenAI model rows, got %+v", snapshot.ModelBreakdown)
	}
	if snapshot.ModelBreakdown[0].Model != "priced-model" || snapshot.ModelBreakdown[0].Provider != "OpenAI" || snapshot.ModelBreakdown[0].FailureCount != 1 {
		t.Fatalf("expected priced model row first, got %+v", snapshot.ModelBreakdown[0])
	}
	if snapshot.ModelBreakdown[0].LatencySampleCount != 2 || snapshot.ModelBreakdown[0].AverageLatencyMS != 200 {
		t.Fatalf("expected latency samples to be preserved, got %+v", snapshot.ModelBreakdown[0])
	}
	if snapshot.ModelBreakdown[1].Model != "missing-model" || snapshot.ModelBreakdown[1].CostStatus != "unavailable" {
		t.Fatalf("expected missing model to expose unavailable cost, got %+v", snapshot.ModelBreakdown[1])
	}
	if len(snapshot.TimeBreakdown) != len(snapshot.Trend) || len(snapshot.TimeBreakdown) != 2 {
		t.Fatalf("expected explicit time breakdown to mirror trend buckets, got trend=%+v time=%+v", snapshot.Trend, snapshot.TimeBreakdown)
	}
	if snapshot.TimeBreakdown[1].Label != "2026-05-12" || snapshot.TimeBreakdown[1].TotalTokens != 2_000_000 || snapshot.TimeBreakdown[1].CostStatus != "unavailable" {
		t.Fatalf("expected second time bucket to expose token and cost status, got %+v", snapshot.TimeBreakdown[1])
	}
}

func TestBuildAnalyticsSummaryWithFilterReturnsDeterministicInsights(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(48 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	identities := []entities.UsageIdentity{
		{Name: "Alpha Team", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-alpha-123456", Type: "openai", Provider: "OpenAI"},
		{Name: "Beta Team", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-beta-123456", Type: "openai", Provider: "OpenAI"},
	}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("create identities: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-alpha-123456", "Alpha Ops", start); err != nil {
		t.Fatalf("set alias: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "alpha-priced", AuthType: "apikey", AuthIndex: "sk-alpha-123456", Model: "priced-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1_000_000, CachedTokens: 200_000, ReasoningTokens: 300_000, TotalTokens: 1_500_000},
		{EventKey: "beta-unpriced", AuthType: "apikey", AuthIndex: "sk-beta-123456", Model: "missing-model", Timestamp: start.Add(26 * time.Hour), InputTokens: 2_000_000, TotalTokens: 2_000_000, Failed: true},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	insights := map[string]dto.AnalyticsInsightRecord{}
	for _, insight := range snapshot.Insights {
		insights[insight.Type] = insight
	}
	expectedOrder := []string{"metric_completeness", "cache_efficiency", "top_cost_key", "token_spike", "failure_concentration", "reasoning_tokens"}
	if len(snapshot.Insights) != len(expectedOrder) {
		t.Fatalf("expected ordered insights %v, got %+v", expectedOrder, snapshot.Insights)
	}
	for index, expectedType := range expectedOrder {
		if snapshot.Insights[index].Type != expectedType {
			t.Fatalf("expected insight %q at index %d, got %+v", expectedType, index, snapshot.Insights)
		}
		if _, ok := insights[expectedType]; !ok {
			t.Fatalf("expected insight %q in %+v", expectedType, snapshot.Insights)
		}
	}
	if insights["top_cost_key"].Subject != "Alpha Ops" || insights["top_cost_key"].MetricValue <= 0 {
		t.Fatalf("expected top cost key to use alias and configured cost, got %+v", insights["top_cost_key"])
	}
	if insights["metric_completeness"].Title != "Metric Completeness" || insights["metric_completeness"].CostStatus != dto.AnalyticsCostStatusPartial {
		t.Fatalf("expected completeness insight to expose partial interpretation, got %+v", snapshot.Insights)
	}
	if insights["cache_efficiency"].Title != "Cache Read Share" || insights["cache_efficiency"].Count != 200_000 {
		t.Fatalf("expected cache insight to use cache fields, got %+v", snapshot.Insights)
	}
	if insights["reasoning_tokens"].Count != 300_000 || insights["failure_concentration"].Count != 1 {
		t.Fatalf("expected reasoning and failure counts, got %+v", snapshot.Insights)
	}
}

func TestBuildAnalyticsSummaryWithFilterMarksCostPartialWhenPricedRowsHaveZeroRates(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "zero-rate-model",
		PromptPricePer1M:     0,
		CompletionPricePer1M: 0,
		CachePricePer1M:      0,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "priced-zero-rate", Model: "zero-rate-model", Timestamp: start.Add(time.Hour), InputTokens: 1000, TotalTokens: 1000},
		{EventKey: "unpriced", Model: "missing-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1000, TotalTokens: 1000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.Summary.TotalCost != 0 || snapshot.Summary.CostAvailable || snapshot.Summary.CostStatus != "partial" {
		t.Fatalf("expected partial cost status with zero-rate priced row, got %+v", snapshot.Summary)
	}
	if len(snapshot.Trend) != 1 || snapshot.Trend[0].CostStatus != "partial" {
		t.Fatalf("expected partial trend status, got %+v", snapshot.Trend)
	}
}

func TestBuildAnalyticsSummaryWithFilterClampsTokenFieldsBeforeCostCalculation(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "negative-cached", Model: "priced-model", Timestamp: start.Add(time.Hour),
		InputTokens: 100, OutputTokens: -20, CachedTokens: -50, TotalTokens: 30,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if math.Abs(snapshot.Summary.TotalCost-0.0001) > 0.000000001 {
		t.Fatalf("expected independently clamped token cost, got %+v", snapshot.Summary)
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

func TestBuildAnalyticsSummaryWithFilterAggregatesKeyAliasBreakdownByStableIdentity(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour)
	deletedAt := start.Add(-time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	identities := []entities.UsageIdentity{
		{Name: "OpenAI Team", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-alpha-123456", Type: "openai", Provider: "OpenAI"},
		{Name: "Anthropic Team", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-beta-123456", Type: "claude", Provider: "Anthropic"},
		{Name: "Deleted Team", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "sk-deleted-123456", Type: "codex", Provider: "Codex", IsDeleted: true, DeletedAt: &deletedAt},
	}
	if err := db.Create(&identities).Error; err != nil {
		t.Fatalf("create identities: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-alpha-123456", "Shared Alias", start); err != nil {
		t.Fatalf("set alpha alias: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-beta-123456", "Shared Alias", start); err != nil {
		t.Fatalf("set beta alias: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-deleted-123456", "Historical Alias", start); err != nil {
		t.Fatalf("set deleted alias: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "alpha-1", AuthType: "apikey", AuthIndex: "sk-alpha-123456", Model: "priced-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 2_000_000, TotalTokens: 2_000_000},
		{EventKey: "beta-1", AuthType: "apikey", AuthIndex: "sk-beta-123456", Model: "priced-model", Timestamp: start.Add(3 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000, Failed: true},
		{EventKey: "missing-alias-1", AuthType: "apikey", AuthIndex: "sk-missing-123456", Model: "missing-model", Timestamp: start.Add(4 * time.Hour), InputTokens: 3_000_000, TotalTokens: 3_000_000},
		{EventKey: "deleted-1", AuthType: "apikey", AuthIndex: "sk-deleted-123456", Model: "priced-model", Timestamp: start.Add(5 * time.Hour), InputTokens: 500_000, TotalTokens: 500_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	rows := snapshot.KeyAliasBreakdown
	if len(rows) != 4 {
		t.Fatalf("expected four key alias rows, got %+v", rows)
	}
	if rows[0].Identity != "sk-alpha-123456" || rows[0].Alias != "Shared Alias" || rows[0].TotalCost != 2 || rows[0].RequestCount != 1 {
		t.Fatalf("expected first row ordered by cost for alpha, got %+v", rows[0])
	}
	if rows[1].Identity != "sk-beta-123456" || rows[1].Alias != "Shared Alias" || rows[1].FailureCount != 1 {
		t.Fatalf("expected duplicate alias to remain traceable as beta row, got %+v", rows[1])
	}
	if rows[2].Identity != "sk-deleted-123456" || !rows[2].IsDeleted || rows[2].Alias != "Historical Alias" {
		t.Fatalf("expected deleted identity to stay in historical breakdown, got %+v", rows[2])
	}
	if rows[3].Identity != "sk-missing-123456" || rows[3].Alias != "" || rows[3].CostStatus != "unavailable" || rows[3].Trend[0].TotalTokens != 3_000_000 {
		t.Fatalf("expected missing alias row with unavailable cost and token trend, got %+v", rows[3])
	}
}

func TestBuildAnalyticsSummaryWithFilterOrdersKeyAliasBreakdownByTokensWhenCostUnavailable(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "low-token-unpriced", AuthType: "apikey", AuthIndex: "sk-low-123456", Model: "missing-model", Timestamp: start.Add(time.Hour), InputTokens: 10, TotalTokens: 10},
		{EventKey: "high-token-unpriced", AuthType: "apikey", AuthIndex: "sk-high-123456", Model: "missing-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1000, TotalTokens: 1000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.KeyAliasBreakdown) != 2 {
		t.Fatalf("expected two key alias rows, got %+v", snapshot.KeyAliasBreakdown)
	}
	if snapshot.KeyAliasBreakdown[0].Identity != "sk-high-123456" || snapshot.KeyAliasBreakdown[0].CostStatus != "unavailable" {
		t.Fatalf("expected unavailable cost rows to fall back to token ordering, got %+v", snapshot.KeyAliasBreakdown)
	}
}

func TestBuildAnalyticsKeyAliasTrendsRestrictsRowsToSelectedIdentities(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "included", AuthType: "apikey", AuthIndex: "sk-included-123456", Model: "missing-model", Timestamp: start.Add(time.Hour), InputTokens: 10, TotalTokens: 10},
		{EventKey: "excluded", AuthType: "apikey", AuthIndex: "sk-excluded-123456", Model: "missing-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1000, TotalTokens: 1000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	trends, err := buildAnalyticsKeyAliasTrends(
		db,
		dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end},
		[]analyticsIdentityKey{{AuthType: int(entities.UsageIdentityAuthTypeAIProvider), Identity: "sk-included-123456"}},
	)
	if err != nil {
		t.Fatalf("buildAnalyticsKeyAliasTrends returned error: %v", err)
	}

	if len(trends) != 1 {
		t.Fatalf("expected trends for only selected identity, got %+v", trends)
	}
	if _, ok := trends[analyticsIdentityKey{AuthType: int(entities.UsageIdentityAuthTypeAIProvider), Identity: "sk-excluded-123456"}]; ok {
		t.Fatalf("expected excluded identity to be absent, got %+v", trends)
	}
}
