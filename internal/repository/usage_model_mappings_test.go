package repository

import (
	"context"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type afterSQLTestLogger struct {
	logger.Interface
	match string
	once  sync.Once
	after func()
}

func (l *afterSQLTestLogger) Trace(ctx context.Context, begin time.Time, fc func() (string, int64), err error) {
	sql, rows := fc()
	l.Interface.Trace(ctx, begin, func() (string, int64) { return sql, rows }, err)
	if strings.Contains(sql, l.match) {
		l.once.Do(l.after)
	}
}

func TestUsageModelMappingsPreserveObservedPopulationSplitsAndEvidenceParity(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	if _, err := UpsertModelPriceSetting(db, dto.ModelPriceSettingInput{
		Model: "actual-a", PromptPricePer1M: 1, CompletionPricePer1M: 2, CachePricePer1M: 0.5,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	alias := func(value string) *string { return &value }
	events := []entities.UsageEvent{
		{EventKey: "mapped-a-1", Timestamp: start.Add(time.Hour), ModelAlias: alias("route-a"), Model: "actual-a", Provider: "provider-a", LatencyMS: 100, InputTokens: 1_000_000, TotalTokens: 1_000_000},
		{EventKey: "mapped-a-2", Timestamp: start.Add(2 * time.Hour), ModelAlias: alias("route-a"), Model: "actual-a", Provider: "provider-a", Failed: true, LatencyMS: 0},
		{EventKey: "mapped-b", Timestamp: start.Add(3 * time.Hour), ModelAlias: alias("route-a"), Model: "actual-b", Provider: "provider-b", Failed: true, LatencyMS: 300, InputTokens: 100, TotalTokens: 100},
		// CPA serializes an upstream blank alias as the actual model. This remains
		// an observed alias==model fact, not proof of the client-requested name.
		{EventKey: "canonicalized", Timestamp: start.Add(4 * time.Hour), ModelAlias: alias("actual-b"), Model: "actual-b", Provider: "provider-b", LatencyMS: 200},
		{EventKey: "missing-alias", Timestamp: start.Add(5 * time.Hour), Model: "actual-a", Provider: "provider-a"},
		{EventKey: "outside", Timestamp: start.Add(-time.Second), ModelAlias: alias("route-a"), Model: "actual-a", Provider: "provider-a"},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	filter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}}
	result, err := BuildUsageModelMappingsWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build usage model mappings: %v", err)
	}
	if result.TotalAttempts != 5 || result.ObservedAliasAttempts != 4 || result.MissingAliasAttempts != 1 {
		t.Fatalf("unexpected alias population: %+v", result)
	}
	if result.ObservedCostStatus != dto.CostStatusPartial || result.ObservedCostAvailable || math.Abs(result.ObservedTotalCost-1) > 1e-9 {
		t.Fatalf("expected partial observed cost with current formulas, got %+v", result)
	}
	if len(result.Mappings) != 3 || result.OtherAttempts != 0 {
		t.Fatalf("expected three separate mappings, got %+v", result)
	}
	first := result.Mappings[0]
	if first.ModelAlias != "route-a" || first.Model != "actual-a" || first.Provider != "provider-a" || first.AttemptCount != 2 || first.FailureCount != 1 || first.FailureShare != 50 || first.LatencySampleCount != 1 || first.MeanLatencyMS != 100 || first.CostStatus != dto.CostStatusAvailable {
		t.Fatalf("unexpected first mapping: %+v", first)
	}

	selection := dto.UsageDiagnosticFilter{UsageTimeScope: filter.UsageTimeScope, Model: "actual-a", ModelAlias: "route-a"}
	mappings, err := BuildUsageModelMappingsWithFilter(context.Background(), db, selection)
	if err != nil {
		t.Fatalf("build selected mappings: %v", err)
	}
	evidence, err := ListUsageEventsWithFilter(context.Background(), db, dto.UsageEventListFilter{
		UsageTimeScope: selection.UsageTimeScope, Model: selection.Model, ModelAlias: selection.ModelAlias, Page: 1, PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list selected evidence: %v", err)
	}
	if mappings.TotalAttempts != 2 || mappings.ObservedAliasAttempts != 2 || evidence.TotalCount != 2 || len(evidence.Events) != 2 {
		t.Fatalf("expected aggregate/evidence parity for exact observed alias, mappings=%+v evidence=%+v", mappings, evidence)
	}
}

func TestUsageModelMappingsRequiresBoundedWindow(t *testing.T) {
	db := openTestDatabase(t)
	if _, err := BuildUsageModelMappingsWithFilter(context.Background(), db, dto.UsageDiagnosticFilter{}); err == nil {
		t.Fatal("expected unbounded model mapping query to be rejected")
	}
}

func TestUsageModelMappingsUsesSingleReadSnapshot(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "model-mapping-snapshot.db")
	db, err := OpenDatabase(config.Config{SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("open reader database: %v", err)
	}
	closeTestDatabase(t, db)
	writer, err := OpenDatabase(config.Config{SQLitePath: dbPath})
	if err != nil {
		t.Fatalf("open writer database: %v", err)
	}
	closeTestDatabase(t, writer)

	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	alias := "route-a"
	if _, _, err := InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey: "before-snapshot", Timestamp: start.Add(time.Hour), ModelAlias: &alias, Model: "actual-a", Provider: "provider-a",
	}}); err != nil {
		t.Fatalf("seed usage event: %v", err)
	}

	reader := db.Session(&gorm.Session{Logger: &afterSQLTestLogger{
		Interface: db.Logger,
		match:     "AS observed_alias_attempts",
		after: func() {
			if _, _, insertErr := InsertUsageEvents(writer, []entities.UsageEvent{{
				EventKey: "during-snapshot", Timestamp: start.Add(2 * time.Hour), ModelAlias: &alias, Model: "actual-b", Provider: "provider-b",
			}}); insertErr != nil {
				t.Errorf("insert concurrent usage event: %v", insertErr)
			}
		},
	}})

	result, err := BuildUsageModelMappingsWithFilter(context.Background(), reader, dto.UsageDiagnosticFilter{
		UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end},
	})
	if err != nil {
		t.Fatalf("build usage model mappings: %v", err)
	}
	if result.TotalAttempts != 1 || result.ObservedAliasAttempts != 1 {
		t.Fatalf("expected the initial snapshot summary, got %+v", result)
	}
	visibleAttempts := result.OtherAttempts
	for _, mapping := range result.Mappings {
		visibleAttempts += mapping.AttemptCount
	}
	if visibleAttempts != result.ObservedAliasAttempts {
		t.Fatalf("mapping rows do not preserve observed attempts %d: %+v", result.ObservedAliasAttempts, result)
	}
}

func TestUsageModelMappingsBoundsRowsAndPreservesExcludedAttempts(t *testing.T) {
	db := openTestDatabase(t)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events := make([]entities.UsageEvent, 0, dto.UsageModelMappingLimit+2)
	for index := 0; index < dto.UsageModelMappingLimit+2; index++ {
		alias := "route-" + string(rune('a'+index))
		events = append(events, entities.UsageEvent{
			EventKey: alias, Timestamp: start.Add(time.Hour), ModelAlias: &alias,
			Model: "actual", Provider: "provider",
		})
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	result, err := BuildUsageModelMappingsWithFilter(context.Background(), db, dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}})
	if err != nil {
		t.Fatalf("build usage model mappings: %v", err)
	}
	visible := result.OtherAttempts
	for _, row := range result.Mappings {
		visible += row.AttemptCount
	}
	if len(result.Mappings) != dto.UsageModelMappingLimit || result.OtherAttempts != 2 || visible != result.ObservedAliasAttempts {
		t.Fatalf("expected bounded rows with conserved excluded attempts, got %+v", result)
	}
}
