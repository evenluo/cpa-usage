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

func TestAPIKeyAggregateFactsMatchAcrossAnalyticsRollupAndAliasTargets(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 5, 20, 9, 0, 0, 0, time.UTC)
	end := start.Add(59*time.Minute + 59*time.Second)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model:                "priced",
		PromptPricePer1M:     1,
		CompletionPricePer1M: 2,
		CachePricePer1M:      0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-shared-123456", "Shared Key", start); err != nil {
		t.Fatalf("set key alias: %v", err)
	}
	events := []entities.UsageEvent{
		{EventKey: "priced", APIGroupKey: "sk-shared-123456", Provider: "OpenAI", Model: "priced", Timestamp: start.Add(5 * time.Minute), InputTokens: 1_000_000, OutputTokens: 100_000, CachedTokens: 200_000, TotalTokens: 1_300_000},
		{EventKey: "unpriced", APIGroupKey: "sk-shared-123456", Provider: "OpenAI", Model: "missing", Timestamp: start.Add(25 * time.Minute), InputTokens: 10, TotalTokens: 10, Failed: true},
		{EventKey: "negative", APIGroupKey: "sk-shared-123456", Provider: "OpenAI", Model: "missing", Timestamp: start.Add(45 * time.Minute), InputTokens: -10, TotalTokens: -10},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert events: %v", err)
	}
	filter := dto.AnalyticsFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}, Range: "custom", Granularity: "hour"}

	analyticsRows, err := buildAnalyticsAPIKeyBreakdown(db, filter)
	if err != nil {
		t.Fatalf("build raw analytics api key breakdown: %v", err)
	}
	if len(analyticsRows) != 1 {
		t.Fatalf("expected one analytics row, got %+v", analyticsRows)
	}
	aliasRows, total, err := ListAPIKeyAliasTargetsPage(context.Background(), db, ListAPIKeyAliasTargetsPageRequest{Page: 1, PageSize: 1})
	if err != nil {
		t.Fatalf("list api key alias targets: %v", err)
	}
	if total != 1 || len(aliasRows) != 1 {
		t.Fatalf("expected one paginated alias target, total=%d rows=%+v", total, aliasRows)
	}
	analyticsRow := analyticsRows[0]
	aliasRow := aliasRows[0]
	if analyticsRow.Identity != aliasRow.Identity || analyticsRow.Alias != aliasRow.Alias || analyticsRow.Provider != aliasRow.Provider {
		t.Fatalf("identity facts differ\nanalytics=%+v\nalias=%+v", analyticsRow, aliasRow)
	}
	if analyticsRow.RequestCount != aliasRow.RequestCount || analyticsRow.SuccessCount != aliasRow.SuccessCount || analyticsRow.FailureCount != aliasRow.FailureCount || analyticsRow.TotalTokens != aliasRow.TotalTokens {
		t.Fatalf("usage totals differ\nanalytics=%+v\nalias=%+v", analyticsRow, aliasRow)
	}
	if math.Abs(analyticsRow.TotalCost-aliasRow.TotalCost) > 0.000000001 || analyticsRow.CostAvailable != aliasRow.CostAvailable || analyticsRow.CostStatus != aliasRow.CostStatus {
		t.Fatalf("cost facts differ\nanalytics=%+v\nalias=%+v", analyticsRow, aliasRow)
	}
	if analyticsRow.LastUsedAt == nil || aliasRow.LastUsedAt == nil || !analyticsRow.LastUsedAt.Equal(*aliasRow.LastUsedAt) {
		t.Fatalf("last-used facts differ\nanalytics=%+v\nalias=%+v", analyticsRow.LastUsedAt, aliasRow.LastUsedAt)
	}
	if aliasRow.FirstUsedAt == nil || !aliasRow.FirstUsedAt.Equal(events[0].Timestamp) {
		t.Fatalf("raw alias target must retain exact first-used fact, got %+v", aliasRow.FirstUsedAt)
	}

	if err := RebuildUsageRollupsForEvents(db, events); err != nil {
		t.Fatalf("rebuild usage rollups: %v", err)
	}
	rawFacts, err := buildAnalyticsAPIKeySegmentRows(db, filter, analyticsEventsAggregateSource())
	if err != nil {
		t.Fatalf("build raw api key fact segment: %v", err)
	}
	rollupFacts, err := buildAnalyticsAPIKeySegmentRows(db, filter, analyticsRollupsAggregateSource())
	if err != nil {
		t.Fatalf("build rollup api key fact segment: %v", err)
	}
	if len(rawFacts) != 1 || len(rollupFacts) != 1 {
		t.Fatalf("expected one raw and rollup fact row\nraw=%+v\nrollup=%+v", rawFacts, rollupFacts)
	}
	rawReadModel := mapAnalyticsKeyAliasBreakdown(rawFacts[0])
	rollupReadModel := mapAnalyticsKeyAliasBreakdown(rollupFacts[0])
	if !reflect.DeepEqual(rawReadModel, rollupReadModel) {
		t.Fatalf("raw and rollup api key read facts differ\nraw=%+v\nrollup=%+v", rawReadModel, rollupReadModel)
	}
	var rollupRows []apiKeyAggregateFactRow
	if err := apiKeyAggregateFactsQuery(db, filter, analyticsRollupsAggregateSource()).Scan(&rollupRows).Error; err != nil {
		t.Fatalf("read rollup api key facts: %v", err)
	}
	if len(rollupRows) != 1 || rollupRows[0].FirstUsedAt != "" {
		t.Fatalf("rollup must not fabricate first-used, got %+v", rollupRows)
	}
}
