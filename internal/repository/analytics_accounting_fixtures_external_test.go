package repository_test

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
)

func TestPinnedAccountingFixturesPreserveRawHybridRollupAndEvidenceSemantics(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "fixtures.db")})
	if err != nil {
		t.Fatalf("open fixture database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get fixture database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	start := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
	fixtures := []struct {
		name string
		at   time.Time
	}{
		{name: "v7.2.62-legacy.json", at: start.Add(5 * time.Minute)},
		{name: "v7.2.152-complete.json", at: start.Add(10 * time.Minute)},
		{name: "v7.2.152-independent.json", at: start.Add(15 * time.Minute)},
		{name: "v7.2.152-separate-reasoning.json", at: start.Add(20 * time.Minute)},
		{name: "v7.2.152-inconsistent.json", at: start.Add(time.Hour + 5*time.Minute)},
		{name: "v7.2.152-unclassified.json", at: start.Add(time.Hour + 10*time.Minute)},
	}
	events := make([]entities.UsageEvent, 0, len(fixtures))
	for _, fixture := range fixtures {
		raw, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "usage", fixture.name))
		if err != nil {
			t.Fatalf("read %s: %v", fixture.name, err)
		}
		event, _, err := service.DecodeRedisUsageMessage(string(raw), fixture.at)
		if err != nil {
			t.Fatalf("decode %s: %v", fixture.name, err)
		}
		event.EventKey = "fixture:" + fixture.name
		event.Timestamp = fixture.at
		events = append(events, event)
	}
	if _, _, err := repository.InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert pinned fixtures: %v", err)
	}
	if _, err := repository.UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model: "fixture-model", PromptPricePer1M: 1, CompletionPricePer1M: 2, CachePricePer1M: 0.5,
	}); err != nil {
		t.Fatalf("set fixture cost rate: %v", err)
	}

	end := start.Add(time.Hour + 30*time.Minute)
	filter := dto.AnalyticsFilter{
		UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end, Provider: "openai"},
		Range:          "custom", Granularity: "hour", FixedWindowEnd: &end,
	}
	raw, err := repository.BuildAnalyticsCoreWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build raw fixture summary: %v", err)
	}
	covered := start
	if err := repository.SaveUsageRollupBackfillStatus(db, dto.RollupBackfillStatus{
		Status: dto.RollupBackfillStatusCompleted, TargetBucketStart: &covered, CoveredBucketStart: &covered,
	}); err != nil {
		t.Fatalf("mark first fixture hour covered: %v", err)
	}
	hybrid, err := repository.BuildAnalyticsCoreWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build hybrid fixture summary: %v", err)
	}
	if !reflect.DeepEqual(raw.Summary, hybrid.Summary) {
		t.Fatalf("expected provider-scoped raw/hybrid summary parity\nraw=%+v\nhybrid=%+v", raw.Summary, hybrid.Summary)
	}
	if hybrid.Summary.RequestCount != 4 || hybrid.Summary.TotalTokens != 580 || math.Abs(hybrid.Summary.TotalCost-0.00056) > 1e-12 {
		t.Fatalf("legacy scalar and Cost meanings changed: %+v", hybrid.Summary)
	}
	accounting := hybrid.Summary.Accounting
	if accounting.ValidAttempts != 3 || accounting.CoveragePct == nil || math.Abs(*accounting.CoveragePct-75) > 1e-9 || accounting.States.Absent != 1 || accounting.States.Valid != 3 {
		t.Fatalf("unexpected accounting coverage: %+v", accounting)
	}
	if accounting.ValidQuality != (dto.AnalyticsAccountingValidQuality{Complete: 1, Inconsistent: 1, Unclassified: 1}) ||
		accounting.Composition.TotalTokens != 450 || accounting.Composition.Input.TotalTokens != 200 ||
		accounting.Composition.Output.TotalTokens != 60 || accounting.Composition.UnclassifiedTokens != 190 {
		t.Fatalf("unexpected canonical composition or quality: %+v", accounting)
	}

	page, err := repository.ListUsageEventsWithFilter(context.Background(), db, dto.UsageEventListFilter{
		UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}, Page: 1, PageSize: 20,
	})
	if err != nil {
		t.Fatalf("list pinned fixture evidence: %v", err)
	}
	byProvider := map[string]dto.UsageEventRecord{}
	for _, event := range page.Events {
		if event.Provider == "gemini" || event.Provider == "claude" {
			byProvider[event.Provider] = event
		}
	}
	for _, provider := range []string{"gemini", "claude"} {
		evidence := byProvider[provider]
		if evidence.AttemptFacts.OutputTPS == nil || math.Abs(*evidence.AttemptFacts.OutputTPS-30) > 1e-9 {
			t.Fatalf("%s evidence must retain provider output scalar TPS, got %+v", provider, evidence.AttemptFacts)
		}
		if evidence.AttemptFacts.Accounting.Output.ReasoningTokens == nil || *evidence.AttemptFacts.Accounting.Output.ReasoningTokens != 12 {
			t.Fatalf("%s evidence lost canonical reasoning bucket: %+v", provider, evidence.AttemptFacts.Accounting)
		}
	}
}
