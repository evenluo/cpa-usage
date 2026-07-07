package repository

import (
	"context"
	"math"
	"reflect"
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

func TestBuildAnalyticsCoreWithFilterUsesRollupsForSummaryAndTrend(t *testing.T) {
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
		{EventKey: "priced-hour-1", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-alpha", Timestamp: start.Add(9*time.Hour + 15*time.Minute), InputTokens: 1_000_000, OutputTokens: 500_000, CachedTokens: 100_000, TotalTokens: 1_600_000},
		{EventKey: "priced-hour-2", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-alpha", Timestamp: start.AddDate(0, 0, 1).Add(10 * time.Hour), InputTokens: 500_000, TotalTokens: 500_000},
		{EventKey: "unpriced-hour-2", Provider: "OpenAI", Model: "missing-model", AuthType: "oauth", AuthIndex: "auth-2", Timestamp: start.AddDate(0, 0, 1).Add(11 * time.Hour), Failed: true, InputTokens: 100, TotalTokens: 100},
		{EventKey: "other-provider", Provider: "Claude", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-3", Timestamp: start.Add(12 * time.Hour), InputTokens: 700, TotalTokens: 700},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	target := end.UTC().Truncate(time.Hour)
	covered := target
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status:             dto.RollupBackfillStatusCompleted,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark rollup coverage: %v", err)
	}
	filter := dto.UsageQueryFilter{Range: "7d", Granularity: "day", Provider: "OpenAI", StartTime: &start, EndTime: &end, FixedWindowEnd: &end}

	rawSummary, err := buildAnalyticsSummary(db, filter)
	if err != nil {
		t.Fatalf("build raw summary: %v", err)
	}
	rawTrend, err := buildAnalyticsTrend(db, filter)
	if err != nil {
		t.Fatalf("build raw trend: %v", err)
	}
	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter returned error: %v", err)
	}

	if core.Summary != rawSummary {
		t.Fatalf("expected rollup summary to match raw summary\nrollup=%+v\nraw=%+v", core.Summary, rawSummary)
	}
	if len(core.Trend) != len(rawTrend) {
		t.Fatalf("expected %d trend points, got %+v", len(rawTrend), core.Trend)
	}
	for i := range rawTrend {
		if core.Trend[i] != rawTrend[i] {
			t.Fatalf("trend point %d mismatch\nrollup=%+v\nraw=%+v", i, core.Trend[i], rawTrend[i])
		}
	}
	if len(core.Heatmap.Rows) != 0 || core.Comparison.HasPreviousPeriod {
		t.Fatalf("core analytics should not return heatmap or comparison in this slice, got %+v", core)
	}
}

func TestBuildAnalyticsCoreWithFilterUsesRollupsForBreakdownsAndReadTimeEnrichment(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 11, 10, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 2,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if err := db.Create(&[]entities.UsageIdentity{
		{Name: "OpenAI Account", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-account", Type: "account", Provider: "OpenAI"},
		{Name: "Provider Key", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "auth-provider", Type: "openai", Provider: "OpenAI"},
	}).Error; err != nil {
		t.Fatalf("create usage identities: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAuthFile, "auth-account", "Account Alias", start); err != nil {
		t.Fatalf("set account alias: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-alpha", "Raw API Alias", start); err != nil {
		t.Fatalf("set api key alias: %v", err)
	}
	events := []entities.UsageEvent{
		{EventKey: "account-usage", Provider: "OpenAI", Model: "priced-model", AuthType: "oauth", AuthIndex: "auth-account", Timestamp: start.Add(15 * time.Minute), InputTokens: 1_000_000, OutputTokens: 100_000, CachedTokens: 100_000, ReasoningTokens: 50_000, TotalTokens: 1_250_000, LatencyMS: 120},
		{EventKey: "api-key-usage", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-provider", APIGroupKey: "sk-alpha", Timestamp: start.Add(time.Hour + 10*time.Minute), InputTokens: 500_000, OutputTokens: 100_000, TotalTokens: 600_000, LatencyMS: 240},
		{EventKey: "missing-price", Provider: "Claude", Model: "missing-model", AuthType: "oauth", AuthIndex: "auth-missing", Timestamp: start.Add(time.Hour + 20*time.Minute), Failed: true, InputTokens: 100, TotalTokens: 100},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	target := end.UTC().Truncate(time.Hour)
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status:             dto.RollupBackfillStatusCompleted,
		TargetBucketStart:  &target,
		CoveredBucketStart: &target,
	}); err != nil {
		t.Fatalf("mark rollup coverage: %v", err)
	}
	filter := dto.UsageQueryFilter{Range: "custom", Granularity: "hour", StartTime: &start, EndTime: &end, FixedWindowEnd: &end}

	raw, err := BuildAnalyticsSummaryWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}
	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter returned error: %v", err)
	}
	assertAnalyticsCoreBreakdownsMatchRaw(t, core, raw)

	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     3,
		CompletionPricePer1M: 2,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("update pricing: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAuthFile, "auth-account", "Renamed Account", start.Add(time.Hour)); err != nil {
		t.Fatalf("rename account alias: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-alpha", "Renamed Raw Key", start.Add(time.Hour)); err != nil {
		t.Fatalf("rename api key alias: %v", err)
	}

	raw, err = BuildAnalyticsSummaryWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter after enrichment changes returned error: %v", err)
	}
	core, err = BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter after enrichment changes returned error: %v", err)
	}
	assertAnalyticsCoreBreakdownsMatchRaw(t, core, raw)
	if core.Summary.TotalCost <= 0 || core.KeyAliasBreakdown[0].Label != "Renamed Account" || core.APIKeyBreakdown[0].Label != "Renamed Raw Key" {
		t.Fatalf("expected read-time pricing and alias enrichment after rollup rows exist, got summary=%+v key_aliases=%+v api_keys=%+v", core.Summary, core.KeyAliasBreakdown, core.APIKeyBreakdown)
	}
}

func assertAnalyticsCoreBreakdownsMatchRaw(t *testing.T, core *dto.AnalyticsSummarySnapshot, raw *dto.AnalyticsSummarySnapshot) {
	t.Helper()
	if !reflect.DeepEqual(core.ProviderOptions, raw.ProviderOptions) {
		t.Fatalf("provider options mismatch\ncore=%+v\nraw=%+v", core.ProviderOptions, raw.ProviderOptions)
	}
	if !reflect.DeepEqual(core.ModelBreakdown, raw.ModelBreakdown) {
		t.Fatalf("model breakdown mismatch\ncore=%+v\nraw=%+v", core.ModelBreakdown, raw.ModelBreakdown)
	}
	if !reflect.DeepEqual(core.KeyAliasBreakdown, raw.KeyAliasBreakdown) {
		t.Fatalf("key alias breakdown mismatch\ncore=%+v\nraw=%+v", core.KeyAliasBreakdown, raw.KeyAliasBreakdown)
	}
	if !reflect.DeepEqual(core.APIKeyBreakdown, raw.APIKeyBreakdown) {
		t.Fatalf("api key breakdown mismatch\ncore=%+v\nraw=%+v", core.APIKeyBreakdown, raw.APIKeyBreakdown)
	}
	if !reflect.DeepEqual(core.Insights, raw.Insights) {
		t.Fatalf("insights mismatch\ncore=%+v\nraw=%+v", core.Insights, raw.Insights)
	}
}

func TestBuildAnalyticsCoreWithFilterKeepsPartialHourWindowExact(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 9, 30, 0, 0, time.UTC)
	end := time.Date(2026, 5, 11, 11, 29, 59, 0, time.UTC)
	events := []entities.UsageEvent{
		{EventKey: "partial-before-start", Provider: "Alpha", Model: "model", APIGroupKey: "sk-mixed", Timestamp: start.Truncate(time.Hour).Add(10 * time.Minute), InputTokens: 100, TotalTokens: 100},
		{EventKey: "partial-start-edge", Provider: "Zulu", Model: "model", APIGroupKey: "sk-mixed", Timestamp: start.Add(15 * time.Minute), InputTokens: 10, TotalTokens: 10},
		{EventKey: "partial-full-hour", Provider: "Alpha", Model: "model", APIGroupKey: "sk-mixed", Timestamp: start.Truncate(time.Hour).Add(time.Hour + 15*time.Minute), InputTokens: 20, TotalTokens: 20},
		{EventKey: "partial-end-edge", Provider: "Zulu", Model: "model", APIGroupKey: "sk-mixed", Timestamp: end.Truncate(time.Hour).Add(15 * time.Minute), InputTokens: 30, TotalTokens: 30},
		{EventKey: "partial-after-end", Provider: "Alpha", Model: "model", APIGroupKey: "sk-mixed", Timestamp: end.Truncate(time.Hour).Add(45 * time.Minute), InputTokens: 100, TotalTokens: 100},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	target := end.UTC().Truncate(time.Hour)
	covered := target
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status:             dto.RollupBackfillStatusCompleted,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark rollup coverage: %v", err)
	}
	filter := dto.UsageQueryFilter{Range: "custom", Granularity: "hour", StartTime: &start, EndTime: &end, FixedWindowEnd: &end}

	raw, err := BuildAnalyticsSummaryWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}
	rawSummary, err := buildAnalyticsSummary(db, filter)
	if err != nil {
		t.Fatalf("build raw summary: %v", err)
	}
	rawTrend, err := buildAnalyticsTrend(db, filter)
	if err != nil {
		t.Fatalf("build raw trend: %v", err)
	}
	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter returned error: %v", err)
	}

	if rawSummary.RequestCount != 3 || rawSummary.TotalTokens != 60 {
		t.Fatalf("test setup expected three in-window events, got %+v", rawSummary)
	}
	if core.Summary != rawSummary {
		t.Fatalf("expected partial-hour core summary to match raw summary\ncore=%+v\nraw=%+v", core.Summary, rawSummary)
	}
	if len(core.Trend) != len(rawTrend) {
		t.Fatalf("expected %d trend points, got %+v", len(rawTrend), core.Trend)
	}
	for i := range rawTrend {
		if core.Trend[i] != rawTrend[i] {
			t.Fatalf("trend point %d mismatch\ncore=%+v\nraw=%+v", i, core.Trend[i], rawTrend[i])
		}
	}
	assertAnalyticsCoreBreakdownsMatchRaw(t, core, raw)
	if len(core.APIKeyBreakdown) != 1 || core.APIKeyBreakdown[0].Provider != "Alpha" {
		t.Fatalf("expected mixed raw/rollup API key provider to match raw MIN(provider), got %+v", core.APIKeyBreakdown)
	}
}

func TestBuildAnalyticsCoreWithFilterFallsBackWhenWindowPrecedesCoverage(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 11, 9, 59, 59, 0, time.UTC)
	event := entities.UsageEvent{
		EventKey: "uncovered-window", Provider: "OpenAI", Model: "model", Timestamp: start.Add(15 * time.Minute), InputTokens: 100, TotalTokens: 100,
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{event}); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	if err := db.Where("1 = 1").Delete(&entities.UsageRollupHourly{}).Error; err != nil {
		t.Fatalf("remove rollups to prove raw fallback: %v", err)
	}
	target := start.Add(-time.Hour)
	covered := target
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status:             dto.RollupBackfillStatusCompleted,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark rollup coverage: %v", err)
	}
	filter := dto.UsageQueryFilter{Range: "custom", Granularity: "hour", Provider: "OpenAI", StartTime: &start, EndTime: &end, FixedWindowEnd: &end}

	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter returned error: %v", err)
	}

	if core.Summary.RequestCount != 1 || core.Summary.TotalTokens != 100 {
		t.Fatalf("expected uncovered window to use raw fallback, got %+v", core.Summary)
	}
}

func TestBuildAnalyticsCoreWithFilterPreservesRawPromptCostClamp(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 11, 9, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 2,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	events := []entities.UsageEvent{
		{EventKey: "clamped-cache-heavy", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-alpha", Timestamp: start.Add(10 * time.Minute), InputTokens: 100, CachedTokens: 200, TotalTokens: 300},
		{EventKey: "clamped-prompt-heavy", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-alpha", Timestamp: start.Add(20 * time.Minute), InputTokens: 200, TotalTokens: 200},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	target := start
	covered := start
	if err := SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status:             dto.RollupBackfillStatusCompleted,
		TargetBucketStart:  &target,
		CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark rollup coverage: %v", err)
	}
	filter := dto.UsageQueryFilter{Range: "custom", Granularity: "hour", Provider: "OpenAI", StartTime: &start, EndTime: &end, FixedWindowEnd: &end}

	rawSummary, err := buildAnalyticsSummary(db, filter)
	if err != nil {
		t.Fatalf("build raw summary: %v", err)
	}
	core, err := BuildAnalyticsCoreWithFilter(db, filter)
	if err != nil {
		t.Fatalf("BuildAnalyticsCoreWithFilter returned error: %v", err)
	}

	if math.Abs(core.Summary.TotalCost-rawSummary.TotalCost) > 0.000000001 || core.Summary.TotalTokens != rawSummary.TotalTokens || core.Summary.RequestCount != rawSummary.RequestCount {
		t.Fatalf("expected rollup cost to preserve raw prompt clamp\ncore=%+v\nraw=%+v", core.Summary, rawSummary)
	}
}

func TestInsertUsageEventsRebuildsHourlyRollupsIdempotently(t *testing.T) {
	db := openTestDatabase(t)
	bucket := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	event := entities.UsageEvent{
		EventKey: "rollup-idempotent", Provider: "OpenAI", Model: "priced-model", AuthType: "apikey", AuthIndex: "auth-1", APIGroupKey: "sk-alpha",
		Timestamp: bucket.Add(5 * time.Minute), InputTokens: 100, OutputTokens: 50, CachedTokens: 10, TotalTokens: 160,
	}
	inserted, deduped, err := InsertUsageEvents(db, []entities.UsageEvent{event})
	if err != nil {
		t.Fatalf("first InsertUsageEvents returned error: %v", err)
	}
	if inserted != 1 || deduped != 0 {
		t.Fatalf("expected first insert inserted=1 deduped=0, got inserted=%d deduped=%d", inserted, deduped)
	}
	inserted, deduped, err = InsertUsageEvents(db, []entities.UsageEvent{event})
	if err != nil {
		t.Fatalf("second InsertUsageEvents returned error: %v", err)
	}
	if inserted != 0 || deduped != 1 {
		t.Fatalf("expected second insert inserted=0 deduped=1, got inserted=%d deduped=%d", inserted, deduped)
	}

	var rollups []entities.UsageRollupHourly
	if err := db.Find(&rollups).Error; err != nil {
		t.Fatalf("load rollups: %v", err)
	}
	if len(rollups) != 1 {
		t.Fatalf("expected one rollup row after retry, got %+v", rollups)
	}
	rollup := rollups[0]
	if rollup.RequestCount != 1 || rollup.SuccessCount != 1 || rollup.FailureCount != 0 || rollup.TotalTokens != 160 || rollup.InputTokens != 100 || rollup.BillablePromptTokens != 90 || rollup.OutputTokens != 50 || rollup.CachedTokens != 10 {
		t.Fatalf("unexpected rollup metrics after retry: %+v", rollup)
	}
	if !rollup.BucketStart.Equal(bucket) || rollup.APIKeyIdentity != "sk-alpha" {
		t.Fatalf("unexpected rollup dimensions after retry: %+v", rollup)
	}
}

func TestInsertUsageEventsDoesNotRebuildDuplicateEventBuckets(t *testing.T) {
	db := openTestDatabase(t)
	oldBucket := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	oldEvent := entities.UsageEvent{EventKey: "duplicate-old", Provider: "OpenAI", Model: "model", Timestamp: oldBucket.Add(5 * time.Minute), InputTokens: 10, TotalTokens: 10}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{oldEvent}); err != nil {
		t.Fatalf("insert old event: %v", err)
	}
	if err := db.Model(&entities.UsageRollupHourly{}).Where("bucket_start = ?", oldBucket).Update("total_tokens", 999).Error; err != nil {
		t.Fatalf("mark old rollup bucket: %v", err)
	}
	newBucket := time.Date(2026, 5, 11, 9, 0, 0, 0, time.UTC)
	newEvent := entities.UsageEvent{EventKey: "new-event", Provider: "OpenAI", Model: "model", Timestamp: newBucket.Add(5 * time.Minute), InputTokens: 20, TotalTokens: 20}

	inserted, deduped, err := InsertUsageEvents(db, []entities.UsageEvent{oldEvent, newEvent})
	if err != nil {
		t.Fatalf("insert mixed duplicate and new event: %v", err)
	}
	if inserted != 1 || deduped != 1 {
		t.Fatalf("expected inserted=1 deduped=1, got inserted=%d deduped=%d", inserted, deduped)
	}

	var oldRollup entities.UsageRollupHourly
	if err := db.Where("bucket_start = ?", oldBucket).First(&oldRollup).Error; err != nil {
		t.Fatalf("load old rollup: %v", err)
	}
	if oldRollup.TotalTokens != 999 {
		t.Fatalf("duplicate old event should not rebuild old bucket, got %+v", oldRollup)
	}
	var newRollup entities.UsageRollupHourly
	if err := db.Where("bucket_start = ?", newBucket).First(&newRollup).Error; err != nil {
		t.Fatalf("load new rollup: %v", err)
	}
	if newRollup.TotalTokens != 20 {
		t.Fatalf("expected new bucket to be rebuilt, got %+v", newRollup)
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

			snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

func TestBuildAnalyticsSummaryWithFilterBucketsHourlyTrendWhenRequested(t *testing.T) {
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
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{
			EventKey: "local-hour-0", Model: "priced-model",
			Timestamp:   time.Date(2026, 5, 11, 16, 30, 0, 0, time.UTC),
			InputTokens: 1000, TotalTokens: 1000,
		},
		{
			EventKey: "local-hour-1", Model: "priced-model",
			Timestamp:   time.Date(2026, 5, 11, 17, 15, 0, 0, time.UTC),
			InputTokens: 2000, TotalTokens: 2000,
		},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "hour", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Trend) != 2 {
		t.Fatalf("expected two hourly trend points, got %+v", snapshot.Trend)
	}
	if snapshot.Trend[0].Label != "2026-05-12 00:00 +0800" || !snapshot.Trend[0].BucketStart.Equal(time.Date(2026, 5, 11, 16, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected first local hour bucket, got %+v", snapshot.Trend[0])
	}
	if snapshot.Trend[1].Label != "2026-05-12 01:00 +0800" || !snapshot.Trend[1].BucketStart.Equal(time.Date(2026, 5, 11, 17, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected second local hour bucket, got %+v", snapshot.Trend[1])
	}
}

func TestBuildAnalyticsSummaryWithFilterKeepsRepeatedDSTHoursSeparate(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	withRepositoryTestLocation(t, "America/New_York")
	db := openTestDatabase(t)
	start := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 11, 1, 23, 59, 59, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{
			EventKey: "fall-back-hour-early", Model: "priced-model",
			Timestamp:   time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC),
			InputTokens: 1000, TotalTokens: 1000,
		},
		{
			EventKey: "fall-back-hour-late", Model: "priced-model",
			Timestamp:   time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC),
			InputTokens: 2000, TotalTokens: 2000,
		},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", Granularity: "hour", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Trend) != 2 {
		t.Fatalf("expected repeated DST hours to remain distinct, got %+v", snapshot.Trend)
	}
	if snapshot.Trend[0].Label != "2026-11-01 01:00 -0400" || !snapshot.Trend[0].BucketStart.Equal(time.Date(2026, 11, 1, 5, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected first fall-back hour with EDT offset, got %+v", snapshot.Trend[0])
	}
	if snapshot.Trend[1].Label != "2026-11-01 01:00 -0500" || !snapshot.Trend[1].BucketStart.Equal(time.Date(2026, 11, 1, 6, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected second fall-back hour with EST offset, got %+v", snapshot.Trend[1])
	}
	var fallBackRow *dto.AnalyticsHeatmapRow
	for index := range snapshot.Heatmap.Rows {
		if snapshot.Heatmap.Rows[index].Date == "2026-11-01" {
			fallBackRow = &snapshot.Heatmap.Rows[index]
		}
	}
	if fallBackRow == nil {
		t.Fatalf("expected local fall-back heatmap row, got %+v", snapshot.Heatmap.Rows)
	}
	fallBackCell := fallBackRow.Cells[1]
	if fallBackCell.TotalTokens != 3000 || fallBackCell.RequestCount != 2 {
		t.Fatalf("expected heatmap to aggregate both repeated local 01:00 hours, got %+v", fallBackCell)
	}
}

func TestBuildAnalyticsSummaryWithFilterHandlesSpringForwardHeatmapHour(t *testing.T) {
	t.Setenv("TZ", "America/New_York")
	withRepositoryTestLocation(t, "America/New_York")
	db := openTestDatabase(t)
	localStart := time.Date(2026, 3, 8, 0, 0, 0, 0, time.Local)
	localEnd := time.Date(2026, 3, 8, 23, 59, 59, 999999999, time.Local)
	start := localStart.UTC()
	end := localEnd.UTC()
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:    "spring-forward-hour",
		Model:       "missing-model",
		Timestamp:   time.Date(2026, 3, 8, 3, 30, 0, 0, time.Local).UTC(),
		TotalTokens: 90,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "custom", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	var springRow *dto.AnalyticsHeatmapRow
	for index := range snapshot.Heatmap.Rows {
		if snapshot.Heatmap.Rows[index].Date == "2026-03-08" {
			springRow = &snapshot.Heatmap.Rows[index]
		}
	}
	if springRow == nil {
		t.Fatalf("expected local spring-forward heatmap row, got %+v", snapshot.Heatmap.Rows)
	}
	missingHour := springRow.Cells[2]
	if !missingHour.BucketStart.Equal(missingHour.BucketEnd) || missingHour.InRange || missingHour.TotalTokens != 0 {
		t.Fatalf("expected nonexistent 02:00 hour to be a zero-duration empty cell, got %+v", missingHour)
	}
	activeHour := springRow.Cells[3]
	if !activeHour.BucketStart.Equal(time.Date(2026, 3, 8, 7, 0, 0, 0, time.UTC)) || !activeHour.BucketEnd.Equal(time.Date(2026, 3, 8, 8, 0, 0, 0, time.UTC)) {
		t.Fatalf("expected 03:00 hour to start at DST transition, got %+v", activeHour)
	}
	if activeHour.TotalTokens != 90 || activeHour.RequestCount != 1 {
		t.Fatalf("expected 03:30 event in hour 3 cell, got %+v", activeHour)
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
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

func TestBuildAnalyticsSummaryWithFilterReturnsProviderOptionsForCurrentScope(t *testing.T) {
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
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "openai-a", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "openai-b", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(2 * time.Hour), OutputTokens: 500_000, TotalTokens: 500_000},
		{EventKey: "anthropic", Provider: "Anthropic", Model: "priced-model", Timestamp: start.Add(3 * time.Hour), InputTokens: 3_000_000, TotalTokens: 3_000_000},
		{EventKey: "blank-provider", Provider: " ", Model: "priced-model", Timestamp: start.Add(4 * time.Hour), InputTokens: 10_000_000, TotalTokens: 10_000_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	allProviders, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}
	if len(allProviders.ProviderOptions) != 2 {
		t.Fatalf("expected non-empty provider options only, got %+v", allProviders.ProviderOptions)
	}
	if allProviders.ProviderOptions[0].Provider != "Anthropic" || allProviders.ProviderOptions[0].RequestCount != 1 || allProviders.ProviderOptions[0].TotalTokens != 3_000_000 {
		t.Fatalf("expected Anthropic first by cost and tokens, got %+v", allProviders.ProviderOptions)
	}
	if allProviders.ProviderOptions[1].Provider != "OpenAI" || allProviders.ProviderOptions[1].RequestCount != 2 || allProviders.ProviderOptions[1].TotalTokens != 1_500_000 {
		t.Fatalf("expected OpenAI option totals, got %+v", allProviders.ProviderOptions)
	}

	openAIOnly, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}
	if len(openAIOnly.ProviderOptions) != 1 || openAIOnly.ProviderOptions[0].Provider != "OpenAI" || openAIOnly.ProviderOptions[0].RequestCount != 2 {
		t.Fatalf("expected provider options to follow selected provider scope, got %+v", openAIOnly.ProviderOptions)
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	insights := map[string]dto.AnalyticsInsight{}
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

func TestBuildAnalyticsSummaryWithFilterDoesNotRenderUnavailableCacheInsightAsZeroShare(t *testing.T) {
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
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "no-cache-data", Model: "priced-model", Timestamp: start.Add(time.Hour),
		InputTokens: 1000, TotalTokens: 1000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Insights) < 2 || snapshot.Insights[1].Type != "cache_efficiency" {
		t.Fatalf("expected cache insight after completeness, got %+v", snapshot.Insights)
	}
	cacheInsight := snapshot.Insights[1]
	if cacheInsight.Subject != "No cache data" || cacheInsight.MetricLabel != "Cache state" || cacheInsight.MetricValue != 0 {
		t.Fatalf("expected unavailable cache state instead of zero share, got %+v", cacheInsight)
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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

func TestBuildAnalyticsSummaryWithFilterReturnsPreviousPeriodComparison(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour).Add(-time.Nanosecond)
	previousStart := start.Add(-7 * 24 * time.Hour)
	previousEnd := start.Add(-time.Nanosecond)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced-model",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 1,
		CachePricePer1M:      1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "previous-success", Provider: "OpenAI", Model: "priced-model", Timestamp: previousStart.Add(24 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "previous-failure", Provider: "OpenAI", Model: "priced-model", Timestamp: previousStart.Add(48 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000, Failed: true},
		{EventKey: "previous-other-provider", Provider: "Anthropic", Model: "priced-model", Timestamp: previousStart.Add(72 * time.Hour), InputTokens: 20_000_000, TotalTokens: 20_000_000},
		{EventKey: "current-success-1", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(24 * time.Hour), InputTokens: 1_500_000, TotalTokens: 1_500_000},
		{EventKey: "current-success-2", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(48 * time.Hour), InputTokens: 1_500_000, TotalTokens: 1_500_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.PreviousRangeStart == nil || !snapshot.PreviousRangeStart.Equal(previousStart) {
		t.Fatalf("expected previous range start %s, got %v", previousStart, snapshot.PreviousRangeStart)
	}
	if snapshot.PreviousRangeEnd == nil || !snapshot.PreviousRangeEnd.Equal(previousEnd) {
		t.Fatalf("expected previous range end %s, got %v", previousEnd, snapshot.PreviousRangeEnd)
	}
	if !snapshot.Comparison.HasPreviousPeriod {
		t.Fatalf("expected previous period comparison to be available, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.TotalCostChangePct == nil || math.Abs(*snapshot.Comparison.TotalCostChangePct-50) > 0.000000001 {
		t.Fatalf("expected 50%% cost change, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.TotalTokensChangePct == nil || math.Abs(*snapshot.Comparison.TotalTokensChangePct-50) > 0.000000001 {
		t.Fatalf("expected 50%% token change, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.RequestCountChangePct == nil || math.Abs(*snapshot.Comparison.RequestCountChangePct) > 0.000000001 {
		t.Fatalf("expected 0%% request change, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.SuccessRateChangePP == nil || math.Abs(*snapshot.Comparison.SuccessRateChangePP-50) > 0.000000001 {
		t.Fatalf("expected 50pp success rate change, got %+v", snapshot.Comparison)
	}
}

func TestBuildAnalyticsSummaryWithFilterReturnsMissingPreviousPeriodComparison(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour).Add(-time.Nanosecond)
	previousStart := start.Add(-7 * 24 * time.Hour)
	previousEnd := start.Add(-time.Nanosecond)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:            "priced-model",
		PromptPricePer1M: 1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "current-only", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(24 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.PreviousRangeStart == nil || !snapshot.PreviousRangeStart.Equal(previousStart) {
		t.Fatalf("expected previous range start %s, got %v", previousStart, snapshot.PreviousRangeStart)
	}
	if snapshot.PreviousRangeEnd == nil || !snapshot.PreviousRangeEnd.Equal(previousEnd) {
		t.Fatalf("expected previous range end %s, got %v", previousEnd, snapshot.PreviousRangeEnd)
	}
	if snapshot.Comparison.HasPreviousPeriod {
		t.Fatalf("expected missing previous period, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.TotalCostChangePct != nil || snapshot.Comparison.TotalTokensChangePct != nil || snapshot.Comparison.RequestCountChangePct != nil || snapshot.Comparison.SuccessRateChangePP != nil {
		t.Fatalf("expected nil comparison deltas for missing previous period, got %+v", snapshot.Comparison)
	}
}

func TestBuildAnalyticsSummaryWithFilterIncludesPreviousPeriodStartBoundary(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 999999999, time.UTC)
	previousStart := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:            "priced-model",
		PromptPricePer1M: 1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "previous-boundary", Provider: "OpenAI", Model: "priced-model", Timestamp: previousStart, InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "current-boundary", Provider: "OpenAI", Model: "priced-model", Timestamp: start, InputTokens: 1_000_000, TotalTokens: 1_000_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "custom", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if snapshot.PreviousRangeStart == nil || !snapshot.PreviousRangeStart.Equal(previousStart) {
		t.Fatalf("expected previous range start %s, got %v", previousStart, snapshot.PreviousRangeStart)
	}
	if !snapshot.Comparison.HasPreviousPeriod {
		t.Fatalf("expected previous boundary event to count, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.RequestCountChangePct == nil || math.Abs(*snapshot.Comparison.RequestCountChangePct) > 0.000000001 {
		t.Fatalf("expected boundary comparison to use previous event, got %+v", snapshot.Comparison)
	}
}

func TestBuildAnalyticsSummaryWithFilterOmitsCostComparisonWhenPricingIsIncomplete(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 8, 0, 0, 0, 0, time.UTC)
	end := start.Add(7 * 24 * time.Hour).Add(-time.Nanosecond)
	previousStart := start.Add(-7 * 24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:            "priced-model",
		PromptPricePer1M: 1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "previous-priced", Provider: "OpenAI", Model: "priced-model", Timestamp: previousStart.Add(24 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "current-priced", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(24 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "current-unpriced", Provider: "OpenAI", Model: "missing-model", Timestamp: start.Add(48 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if !snapshot.Comparison.HasPreviousPeriod {
		t.Fatalf("expected previous period comparison to be available, got %+v", snapshot.Comparison)
	}
	if snapshot.Summary.CostStatus != dto.AnalyticsCostStatusPartial {
		t.Fatalf("expected current cost to be partial, got %+v", snapshot.Summary)
	}
	if snapshot.Comparison.TotalCostChangePct != nil {
		t.Fatalf("expected nil cost comparison for incomplete pricing, got %+v", snapshot.Comparison)
	}
	if snapshot.Comparison.TotalTokensChangePct == nil || math.Abs(*snapshot.Comparison.TotalTokensChangePct-100) > 0.000000001 {
		t.Fatalf("expected non-cost comparison fields to remain available, got %+v", snapshot.Comparison)
	}
}

func analyticsHeatmapRowByDate(t *testing.T, heatmap dto.AnalyticsHeatmap, date string) dto.AnalyticsHeatmapRow {
	t.Helper()
	for _, row := range heatmap.Rows {
		if row.Date == date {
			return row
		}
	}
	t.Fatalf("expected heatmap row %s, got %+v", date, heatmap.Rows)
	return dto.AnalyticsHeatmapRow{}
}

func TestBuildAnalyticsSummaryWithFilterReturnsCompleteHourlyHeatmap(t *testing.T) {
	withRepositoryTestLocation(t, "UTC")

	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 999999999, time.UTC)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:            "priced-model",
		PromptPricePer1M: 1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "priced-cell", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(9 * time.Hour), InputTokens: 120, TotalTokens: 120},
		{EventKey: "unpriced-cell", Provider: "OpenAI", Model: "missing-model", Timestamp: start.AddDate(0, 0, 1).Add(10 * time.Hour), InputTokens: 80, TotalTokens: 80, Failed: true},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	selectedStart := start.AddDate(0, 0, 1)
	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "custom", StartTime: &selectedStart, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	heatmap := snapshot.Heatmap
	if heatmap.Measure != "tokens" || heatmap.MaxTokens != 120 || heatmap.MaxRequests != 1 || heatmap.MaxFailures != 1 {
		t.Fatalf("unexpected heatmap max values: %+v", heatmap)
	}
	if len(heatmap.Rows) < 30 {
		t.Fatalf("expected fixed 30-day heatmap rows, got %+v", heatmap.Rows)
	}
	for _, row := range heatmap.Rows {
		if len(row.Cells) != 24 {
			t.Fatalf("expected 24 cells for row %+v", row)
		}
	}
	firstRow := analyticsHeatmapRowByDate(t, heatmap, "2026-05-11")
	if firstRow.Cells[9].TotalTokens != 120 || firstRow.Cells[9].RequestCount != 1 {
		t.Fatalf("expected priced event before selected range in fixed heatmap row hour 9, got %+v", firstRow)
	}
	if !firstRow.Cells[9].InRange {
		t.Fatalf("expected populated fixed-window cell to be in range, got %+v", firstRow.Cells[9])
	}
	if !firstRow.Cells[8].CostAvailable || firstRow.Cells[8].CostStatus != dto.AnalyticsCostStatusAvailable || firstRow.Cells[8].TotalTokens != 0 {
		t.Fatalf("expected empty bucket to be explicit available zero cell, got %+v", firstRow.Cells[8])
	}
	secondRow := analyticsHeatmapRowByDate(t, heatmap, "2026-05-12")
	unpriced := secondRow.Cells[10]
	if unpriced.TotalTokens != 80 || unpriced.RequestCount != 1 || unpriced.FailureCount != 1 || unpriced.CostStatus != dto.AnalyticsCostStatusUnavailable {
		t.Fatalf("expected unpriced failed event to preserve cost completeness, got %+v", unpriced)
	}
	if !firstRow.Cells[9].BucketStart.Equal(start.Add(9*time.Hour)) || !firstRow.Cells[9].BucketEnd.Equal(start.Add(10*time.Hour)) {
		t.Fatalf("unexpected bucket boundaries: %+v", firstRow.Cells[9])
	}
}

func TestBuildAnalyticsSummaryWithFilterHeatmapMarksRollingRangeBoundaryCells(t *testing.T) {
	withRepositoryTestLocation(t, "UTC")

	db := openTestDatabase(t)
	start := time.Date(2026, 5, 11, 10, 30, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 12, 15, 0, 0, time.UTC)

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.Heatmap.Rows) < 30 {
		t.Fatalf("expected fixed 30-day boundary date rows, got %+v", snapshot.Heatmap.Rows)
	}
	startBoundaryRow := analyticsHeatmapRowByDate(t, snapshot.Heatmap, "2026-04-12")
	if startBoundaryRow.Cells[11].InRange {
		t.Fatalf("expected hour before fixed window start to be out of range, got %+v", startBoundaryRow.Cells[11])
	}
	if !startBoundaryRow.Cells[12].InRange {
		t.Fatalf("expected overlapping fixed window start hour to be in range, got %+v", startBoundaryRow.Cells[12])
	}
	endBoundaryRow := analyticsHeatmapRowByDate(t, snapshot.Heatmap, "2026-05-12")
	if !endBoundaryRow.Cells[12].InRange {
		t.Fatalf("expected overlapping fixed window end hour to be in range, got %+v", endBoundaryRow.Cells[12])
	}
	if endBoundaryRow.Cells[13].InRange {
		t.Fatalf("expected hour after fixed window end to be out of range, got %+v", endBoundaryRow.Cells[13])
	}
}

func TestBuildAnalyticsSummaryWithFilterHeatmapBucketsFractionalOffsetLocalHour(t *testing.T) {
	withRepositoryTestLocation(t, "Asia/Kathmandu")

	db := openTestDatabase(t)
	localStart := time.Date(2026, 5, 11, 0, 0, 0, 0, time.Local)
	localEnd := time.Date(2026, 5, 11, 23, 59, 59, 999999999, time.Local)
	start := localStart.UTC()
	end := localEnd.UTC()
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:    "fractional-offset-local-midnight",
		Provider:    "OpenAI",
		Model:       "missing-model",
		Timestamp:   time.Date(2026, 5, 11, 0, 30, 0, 0, time.Local).UTC(),
		TotalTokens: 75,
	}}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "custom", StartTime: &start, EndTime: &end, FixedWindowEnd: &end, Provider: "OpenAI"})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	row := analyticsHeatmapRowByDate(t, snapshot.Heatmap, "2026-05-11")
	cell := row.Cells[0]
	if cell.TotalTokens != 75 || cell.RequestCount != 1 {
		t.Fatalf("expected local 00:30 event in hour 0 cell, got %+v", cell)
	}
	if row.Cells[23].TotalTokens != 0 {
		t.Fatalf("expected event not to be shifted into another local hour, got %+v", row.Cells[23])
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "7d", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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
	if rows[0].Label != "Shared Alias" || rows[0].MaskedIdentity != "sk-a*******3456" || rows[0].Traceability != "sk-a*******3456 · OpenAI" {
		t.Fatalf("expected repository read model to include display fields, got %+v", rows[0])
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

func TestBuildAnalyticsSummaryWithFilterReturnsAPIKeyBreakdownByClientKey(t *testing.T) {
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
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-alpha-123456", "Alpha API Key", start); err != nil {
		t.Fatalf("set alpha alias: %v", err)
	}
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{
		{EventKey: "alpha", APIGroupKey: "sk-alpha-123456", AuthType: "oauth", AuthIndex: "account-key", Source: "operator@example.com", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(time.Hour), InputTokens: 2_000_000, TotalTokens: 2_000_000},
		{EventKey: "beta", APIGroupKey: "sk-beta-123456", AuthType: "oauth", AuthIndex: "account-key", Source: "operator@example.com", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(2 * time.Hour), InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "provider-fallback-ignored", APIGroupKey: "OpenAI", AuthType: "oauth", AuthIndex: "oauth-account", Source: "sk-oauth-source", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(3 * time.Hour), InputTokens: 5_000_000, TotalTokens: 5_000_000},
		{EventKey: "source-key-compat", APIGroupKey: "OpenAI", AuthType: "apikey", AuthIndex: "legacy-account-key", Source: "sk-source-123456", Provider: "OpenAI", Model: "priced-model", Timestamp: start.Add(4 * time.Hour), InputTokens: 500_000, TotalTokens: 500_000},
	}); err != nil {
		t.Fatalf("insert events: %v", err)
	}

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
	if err != nil {
		t.Fatalf("BuildAnalyticsSummaryWithFilter returned error: %v", err)
	}

	if len(snapshot.KeyAliasBreakdown) != 3 || snapshot.KeyAliasBreakdown[0].Identity != "oauth-account" || snapshot.KeyAliasBreakdown[1].Identity != "account-key" || snapshot.KeyAliasBreakdown[2].Identity != "legacy-account-key" {
		t.Fatalf("expected account leaderboard to stay grouped by auth_index, got %+v", snapshot.KeyAliasBreakdown)
	}
	if len(snapshot.APIKeyBreakdown) != 3 {
		t.Fatalf("expected three client key rows, got %+v", snapshot.APIKeyBreakdown)
	}
	if snapshot.APIKeyBreakdown[0].Identity != "sk-alpha-123456" || snapshot.APIKeyBreakdown[0].Alias != "Alpha API Key" || snapshot.APIKeyBreakdown[0].TotalCost != 2 {
		t.Fatalf("expected alpha client key first by cost with alias, got %+v", snapshot.APIKeyBreakdown[0])
	}
	if snapshot.APIKeyBreakdown[0].Label != "Alpha API Key" || snapshot.APIKeyBreakdown[0].MaskedIdentity != "sk-a*******3456" {
		t.Fatalf("expected api key display fields to be built by repository read model, got %+v", snapshot.APIKeyBreakdown[0])
	}
	if snapshot.APIKeyBreakdown[1].Identity != "sk-beta-123456" || snapshot.APIKeyBreakdown[1].TotalTokens != 1_000_000 {
		t.Fatalf("expected beta client key row, got %+v", snapshot.APIKeyBreakdown[1])
	}
	if snapshot.APIKeyBreakdown[2].Identity != "sk-source-123456" || snapshot.APIKeyBreakdown[2].TotalTokens != 500_000 {
		t.Fatalf("expected source-key compatibility row, got %+v", snapshot.APIKeyBreakdown[2])
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

	snapshot, err := BuildAnalyticsSummaryWithFilter(db, dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end})
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
		dto.UsageQueryFilter{Range: "24h", StartTime: &start, EndTime: &end, FixedWindowEnd: &end},
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
