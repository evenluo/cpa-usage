package repository

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type analyticsCoreCardinalityFixture struct {
	start      time.Time
	end        time.Time
	events     []entities.UsageEvent
	prices     []entities.ModelPriceSetting
	identities []entities.UsageIdentity
	aliases    []entities.KeyAlias
}

func TestAnalyticsCoreRawFastPathMatchesSharedPlan(t *testing.T) {
	withRepositoryTestLocation(t, "UTC")
	db := openTestDatabase(t)
	fixture := newAnalyticsCoreCardinalityFixture(4_096, 8, 64, 128)
	seedAnalyticsCoreCardinalityFixture(t, db, fixture, false)

	tests := []struct {
		name        string
		granularity string
		provider    string
		start       time.Time
		end         time.Time
	}{
		{name: "hour unscoped", granularity: "hour", start: fixture.start, end: fixture.end},
		{name: "day provider scoped", granularity: "day", provider: "provider-03", start: fixture.start, end: fixture.end},
		{name: "no full hour", granularity: "hour", start: fixture.start.Add(15 * time.Minute), end: fixture.start.Add(45 * time.Minute)},
		{name: "empty", granularity: "hour", provider: "missing-provider", start: fixture.start, end: fixture.end},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			filter := dto.AnalyticsFilter{
				UsageTimeScope: dto.UsageTimeScope{StartTime: &test.start, EndTime: &test.end, Provider: test.provider},
				Range:          "custom",
				Granularity:    test.granularity,
				FixedWindowEnd: &test.end,
			}
			plan := analyticsCoreRawWindowPlan(filter)
			fast, err := buildAnalyticsCoreSnapshot(db, filter, plan)
			if err != nil {
				t.Fatalf("build raw fast snapshot: %v", err)
			}
			shared, err := buildAnalyticsCoreSharedCandidateForProof(db, filter, plan)
			if err != nil {
				t.Fatalf("build raw shared candidate: %v", err)
			}
			if !reflect.DeepEqual(fast, shared) {
				t.Fatalf("raw fast and shared candidate snapshots differ\nfast=%+v\nshared=%+v", fast, shared)
			}
		})
	}
}

func TestAnalyticsCoreWindowPlanKeepsNoFullHourRawOnly(t *testing.T) {
	start := time.Date(2026, 8, 27, 9, 15, 0, 0, time.UTC)
	end := start.Add(30 * time.Minute)
	filter := dto.AnalyticsFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}, Range: "custom", Granularity: "hour"}

	plan := analyticsCoreRollupWindowPlan(filter)
	rawFilter, ok := plan.rawOnlyFilter()
	if !ok || rawFilter.StartTime == nil || rawFilter.EndTime == nil || !rawFilter.StartTime.Equal(start) || !rawFilter.EndTime.Equal(end) {
		t.Fatalf("expected exact raw-only plan for a window without a full hour, got %+v", plan)
	}
}

func TestAnalyticsSummaryRawAndRollupPathsMatchTimeContracts(t *testing.T) {
	tests := []struct {
		name         string
		location     string
		start        time.Time
		end          time.Time
		granularity  string
		eventTimes   []time.Time
		wantMixed    bool
		wantPrevious bool
	}{
		{
			name:        "non-UTC daily partial mixed",
			location:    "Asia/Shanghai",
			start:       time.Date(2026, 5, 11, 0, 30, 0, 0, time.UTC),
			end:         time.Date(2026, 5, 12, 23, 29, 59, 999_999_999, time.UTC),
			granularity: "day",
			eventTimes: []time.Time{
				time.Date(2026, 5, 11, 0, 40, 0, 0, time.UTC),
				time.Date(2026, 5, 11, 5, 15, 0, 0, time.UTC),
				time.Date(2026, 5, 11, 16, 30, 0, 0, time.UTC),
				time.Date(2026, 5, 12, 23, 20, 0, 0, time.UTC),
			},
			wantMixed: true,
		},
		{
			name:        "fall-back repeated hour",
			location:    "America/New_York",
			start:       time.Date(2026, 11, 1, 4, 0, 0, 0, time.UTC),
			end:         time.Date(2026, 11, 1, 9, 59, 59, 999_999_999, time.UTC),
			granularity: "hour",
			eventTimes: []time.Time{
				time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC),
				time.Date(2026, 11, 1, 6, 30, 0, 0, time.UTC),
				time.Date(2026, 11, 1, 8, 30, 0, 0, time.UTC),
			},
		},
		{
			name:        "spring-forward missing hour",
			location:    "America/New_York",
			start:       time.Date(2026, 3, 8, 5, 0, 0, 0, time.UTC),
			end:         time.Date(2026, 3, 8, 9, 59, 59, 999_999_999, time.UTC),
			granularity: "hour",
			eventTimes: []time.Time{
				time.Date(2026, 3, 8, 6, 30, 0, 0, time.UTC),
				time.Date(2026, 3, 8, 7, 30, 0, 0, time.UTC),
				time.Date(2026, 3, 8, 8, 30, 0, 0, time.UTC),
			},
		},
		{
			name:        "previous-period comparison",
			location:    "Asia/Shanghai",
			start:       time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC),
			end:         time.Date(2026, 5, 12, 23, 59, 59, 999_999_999, time.UTC),
			granularity: "day",
			eventTimes: []time.Time{
				time.Date(2026, 5, 9, 1, 0, 0, 0, time.UTC),
				time.Date(2026, 5, 10, 1, 0, 0, 0, time.UTC),
				time.Date(2026, 5, 11, 1, 0, 0, 0, time.UTC),
				time.Date(2026, 5, 12, 1, 0, 0, 0, time.UTC),
				time.Date(2026, 5, 12, 20, 0, 0, 0, time.UTC),
			},
			wantPrevious: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			withRepositoryTestLocation(t, test.location)
			db := openTestDatabase(t)
			seedAnalyticsTimeContractFixture(t, db, test.eventTimes)
			filter := dto.AnalyticsFilter{
				UsageTimeScope: dto.UsageTimeScope{StartTime: &test.start, EndTime: &test.end},
				Range:          "custom",
				Granularity:    test.granularity,
				FixedWindowEnd: &test.end,
			}

			plan := analyticsCoreRollupWindowPlan(filter)
			if plan.rollupFilter == nil {
				t.Fatalf("test setup expected a rollup-capable source plan, got %+v", plan)
			}
			if test.wantMixed != (len(plan.rawFilters) > 0) {
				t.Fatalf("test setup mixed-plan mismatch: want_mixed=%t plan=%+v", test.wantMixed, plan)
			}

			raw := buildAnalyticsSummaryWithForcedBackfillState(t, db, filter, false)
			rollup := buildAnalyticsSummaryWithForcedBackfillState(t, db, filter, true)
			assertAnalyticsSummarySnapshotsEquivalent(t, raw, rollup, 1e-9)
			if test.wantPrevious && (!raw.Comparison.HasPreviousPeriod || !rollup.Comparison.HasPreviousPeriod) {
				t.Fatalf("test setup expected previous-period comparison: raw=%+v rollup=%+v", raw.Comparison, rollup.Comparison)
			}
		})
	}
}

func seedAnalyticsTimeContractFixture(t *testing.T, db *gorm.DB, eventTimes []time.Time) {
	t.Helper()
	if err := db.Create([]entities.ModelPriceSetting{
		{Model: "priced-a", PromptPricePer1M: 1, CompletionPricePer1M: 2, CachePricePer1M: 0.5},
		{Model: "priced-b", PromptPricePer1M: 3, CompletionPricePer1M: 4, CachePricePer1M: 1},
	}).Error; err != nil {
		t.Fatalf("seed time-contract prices: %v", err)
	}
	if err := db.Create([]entities.UsageIdentity{
		{AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: entities.UsageIdentityAuthTypeNameOAuth, Identity: "oauth-time", Name: "Time OAuth", Provider: "OpenAI"},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: entities.UsageIdentityAuthTypeNameAPIKey, Identity: "provider-time", Name: "Time Provider", Provider: "Claude"},
	}).Error; err != nil {
		t.Fatalf("seed time-contract identities: %v", err)
	}
	if err := db.Create([]entities.KeyAlias{
		{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "oauth-time", Alias: "OAuth Alias"},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "provider-time", Alias: "Provider Alias"},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "sk-time-123456", Alias: "API Key Alias"},
	}).Error; err != nil {
		t.Fatalf("seed time-contract aliases: %v", err)
	}

	events := make([]entities.UsageEvent, 0, len(eventTimes))
	for index, eventTime := range eventTimes {
		event := entities.UsageEvent{
			EventKey:        fmt.Sprintf("time-contract-%02d", index),
			Provider:        "OpenAI",
			Model:           "priced-a",
			AuthType:        entities.UsageIdentityAuthTypeNameOAuth,
			AuthIndex:       "oauth-time",
			Timestamp:       eventTime,
			InputTokens:     int64(1_000 + index*100),
			OutputTokens:    int64(200 + index*20),
			ReasoningTokens: int64(index * 10),
			CachedTokens:    int64(index * 25),
			LatencyMS:       int64(100 + index*10),
			Failed:          index%3 == 2,
		}
		if index%2 == 1 {
			event.Provider = "Claude"
			event.Model = "priced-b"
			event.AuthType = entities.UsageIdentityAuthTypeNameAPIKey
			event.AuthIndex = "provider-time"
			event.APIGroupKey = "sk-time-123456"
		}
		event.TotalTokens = event.InputTokens + event.OutputTokens + event.ReasoningTokens
		events = append(events, event)
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("seed time-contract events: %v", err)
	}
}

func buildAnalyticsSummaryWithForcedBackfillState(t *testing.T, db *gorm.DB, filter dto.AnalyticsFilter, covered bool) *dto.AnalyticsSummarySnapshot {
	t.Helper()
	target := filter.EndTime.UTC().Truncate(time.Hour)
	coveredBucket := target.Add(-time.Hour)
	status := dto.RollupBackfillStatus{Status: dto.RollupBackfillStatusRunning, TargetBucketStart: &target, CoveredBucketStart: &coveredBucket}
	if covered {
		coveredBucket = target
		status.Status = dto.RollupBackfillStatusCompleted
		status.CoveredBucketStart = &coveredBucket
	}
	if err := SaveUsageRollupBackfillStatus(db, status); err != nil {
		t.Fatalf("force rollup backfill state: %v", err)
	}
	snapshot, err := BuildAnalyticsSummaryWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build analytics summary with covered=%t: %v", covered, err)
	}
	return snapshot
}

func assertAnalyticsSummarySnapshotsEquivalent(t *testing.T, raw *dto.AnalyticsSummarySnapshot, rollup *dto.AnalyticsSummarySnapshot, floatTolerance float64) {
	t.Helper()
	if diff := analyticsSnapshotValueDiff("snapshot", reflect.ValueOf(raw), reflect.ValueOf(rollup), floatTolerance); diff != "" {
		t.Fatal(diff)
	}
}

var analyticsSnapshotTimeType = reflect.TypeOf(time.Time{})

func analyticsSnapshotValueDiff(path string, raw reflect.Value, rollup reflect.Value, floatTolerance float64) string {
	if !raw.IsValid() || !rollup.IsValid() {
		if raw.IsValid() == rollup.IsValid() {
			return ""
		}
		return fmt.Sprintf("%s validity differs", path)
	}
	if raw.Type() != rollup.Type() {
		return fmt.Sprintf("%s type differs: raw=%s rollup=%s", path, raw.Type(), rollup.Type())
	}
	if raw.Type() == analyticsSnapshotTimeType {
		rawTime := raw.Interface().(time.Time)
		rollupTime := rollup.Interface().(time.Time)
		if rawTime.Equal(rollupTime) {
			return ""
		}
		return fmt.Sprintf("%s UTC instant differs: raw=%s rollup=%s", path, rawTime.UTC(), rollupTime.UTC())
	}

	switch raw.Kind() {
	case reflect.Interface, reflect.Pointer:
		if raw.IsNil() || rollup.IsNil() {
			if raw.IsNil() == rollup.IsNil() {
				return ""
			}
			return fmt.Sprintf("%s nil shape differs: raw_nil=%t rollup_nil=%t", path, raw.IsNil(), rollup.IsNil())
		}
		return analyticsSnapshotValueDiff(path, raw.Elem(), rollup.Elem(), floatTolerance)
	case reflect.Struct:
		for fieldIndex := 0; fieldIndex < raw.NumField(); fieldIndex++ {
			fieldPath := path + "." + raw.Type().Field(fieldIndex).Name
			if diff := analyticsSnapshotValueDiff(fieldPath, raw.Field(fieldIndex), rollup.Field(fieldIndex), floatTolerance); diff != "" {
				return diff
			}
		}
		return ""
	case reflect.Slice:
		if raw.IsNil() != rollup.IsNil() {
			return fmt.Sprintf("%s nil/empty shape differs: raw_nil=%t rollup_nil=%t", path, raw.IsNil(), rollup.IsNil())
		}
		if raw.Len() != rollup.Len() {
			return fmt.Sprintf("%s length differs: raw=%d rollup=%d", path, raw.Len(), rollup.Len())
		}
		for itemIndex := 0; itemIndex < raw.Len(); itemIndex++ {
			if diff := analyticsSnapshotValueDiff(fmt.Sprintf("%s[%d]", path, itemIndex), raw.Index(itemIndex), rollup.Index(itemIndex), floatTolerance); diff != "" {
				return diff
			}
		}
		return ""
	case reflect.Float32, reflect.Float64:
		rawFloat := raw.Float()
		rollupFloat := rollup.Float()
		if (math.IsNaN(rawFloat) && math.IsNaN(rollupFloat)) || math.Abs(rawFloat-rollupFloat) <= floatTolerance {
			return ""
		}
		return fmt.Sprintf("%s float differs: raw=%.12g rollup=%.12g tolerance=%g", path, rawFloat, rollupFloat, floatTolerance)
	default:
		if reflect.DeepEqual(raw.Interface(), rollup.Interface()) {
			return ""
		}
		return fmt.Sprintf("%s differs: raw=%v rollup=%v", path, raw.Interface(), rollup.Interface())
	}
}

// buildAnalyticsCoreSharedCandidateForProof is benchmark/test evidence for the
// rejected all-groups raw candidate. Production keeps the SQL-limited raw
// Adapter and does not expose a strategy switch merely for proof.
func buildAnalyticsCoreSharedCandidateForProof(db *gorm.DB, filter dto.AnalyticsFilter, plan analyticsCoreWindowPlan) (*dto.AnalyticsSummarySnapshot, error) {
	summary, err := buildAnalyticsCoreSummary(db, plan)
	if err != nil {
		return nil, err
	}
	trend, err := buildAnalyticsCoreTrend(db, plan, filter)
	if err != nil {
		return nil, err
	}
	keyAliasBreakdown, err := buildAnalyticsCoreKeyAliasBreakdown(db, plan, filter)
	if err != nil {
		return nil, err
	}
	apiKeyBreakdown, err := buildAnalyticsCoreAPIKeyBreakdown(db, plan, filter)
	if err != nil {
		return nil, err
	}
	modelBreakdown, err := buildAnalyticsCoreModelBreakdown(db, plan)
	if err != nil {
		return nil, err
	}
	providerOptions, err := buildAnalyticsCoreProviderOptions(db, plan)
	if err != nil {
		return nil, err
	}
	return &dto.AnalyticsSummarySnapshot{
		Summary:           summary,
		Trend:             trend,
		KeyAliasBreakdown: keyAliasBreakdown,
		APIKeyBreakdown:   apiKeyBreakdown,
		ModelBreakdown:    modelBreakdown,
		TimeBreakdown:     trend,
		Insights:          buildAnalyticsInsights(summary, trend, keyAliasBreakdown, modelBreakdown),
		ProviderOptions:   providerOptions,
	}, nil
}

func newAnalyticsCoreCardinalityFixture(eventCount int, providerCount int, modelCount int, identityCount int) analyticsCoreCardinalityFixture {
	start := time.Date(2026, 8, 24, 0, 0, 0, 0, time.UTC)
	end := start.Add(48*time.Hour - time.Nanosecond)
	fixture := analyticsCoreCardinalityFixture{
		start:  start,
		end:    end,
		events: make([]entities.UsageEvent, 0, eventCount),
		prices: make([]entities.ModelPriceSetting, 0, modelCount),
	}

	for modelIndex := 0; modelIndex < modelCount; modelIndex++ {
		if modelIndex%7 == 0 {
			continue
		}
		price := entities.ModelPriceSetting{
			Model:                fmt.Sprintf("model-%03d", modelIndex),
			PromptPricePer1M:     float64(modelIndex%11 + 1),
			CompletionPricePer1M: float64(modelIndex%13 + 1),
			CachePricePer1M:      float64(modelIndex%5 + 1),
		}
		if modelIndex%19 == 0 {
			price.PromptPricePer1M = 0
			price.CompletionPricePer1M = 0
			price.CachePricePer1M = 0
		}
		fixture.prices = append(fixture.prices, price)
	}

	fixture.identities = make([]entities.UsageIdentity, 0, identityCount*2)
	fixture.aliases = make([]entities.KeyAlias, 0, identityCount*3)
	for identityIndex := 0; identityIndex < identityCount; identityIndex++ {
		oauthIdentity := fmt.Sprintf("oauth-%04d", identityIndex)
		providerIdentity := fmt.Sprintf("provider-account-%04d", identityIndex)
		apiKeyIdentity := fmt.Sprintf("sk-bench-%04d", identityIndex)
		fixture.identities = append(fixture.identities,
			entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: entities.UsageIdentityAuthTypeNameOAuth, Identity: oauthIdentity, Name: "OAuth " + oauthIdentity, Provider: "oauth"},
			entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: entities.UsageIdentityAuthTypeNameAPIKey, Identity: providerIdentity, Name: "Provider " + providerIdentity, Provider: fmt.Sprintf("provider-%02d", identityIndex%providerCount)},
		)
		fixture.aliases = append(fixture.aliases,
			entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: oauthIdentity, Alias: "Alias " + oauthIdentity},
			entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: providerIdentity, Alias: "Alias " + providerIdentity},
			entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: apiKeyIdentity, Alias: "Alias " + apiKeyIdentity},
		)
	}

	for eventIndex := 0; eventIndex < eventCount; eventIndex++ {
		identityIndex := (eventIndex * 17) % identityCount
		inputTokens := int64(100 + eventIndex%1_000)
		outputTokens := int64(50 + eventIndex%300)
		reasoningTokens := int64(eventIndex % 80)
		cachedTokens := inputTokens / 3
		if eventIndex%991 == 0 {
			cachedTokens = inputTokens * 2
		}
		if eventIndex%997 == 0 {
			inputTokens = -inputTokens
			outputTokens = -outputTokens
			reasoningTokens = -reasoningTokens
			cachedTokens = -cachedTokens
		}
		var cacheReadTokens *int64
		if eventIndex%5 != 0 {
			value := positiveInt64(cachedTokens)
			cacheReadTokens = &value
		}
		authType := entities.UsageIdentityAuthTypeNameOAuth
		authIndex := fmt.Sprintf("oauth-%04d", identityIndex)
		apiGroupKey := ""
		if eventIndex%2 == 1 {
			authType = entities.UsageIdentityAuthTypeNameAPIKey
			authIndex = fmt.Sprintf("provider-account-%04d", identityIndex)
			apiGroupKey = fmt.Sprintf("sk-bench-%04d", (eventIndex*31)%identityCount)
		}
		fixture.events = append(fixture.events, entities.UsageEvent{
			EventKey:        fmt.Sprintf("benchmark-event-%06d", eventIndex),
			APIGroupKey:     apiGroupKey,
			Provider:        fmt.Sprintf("provider-%02d", (eventIndex*11)%providerCount),
			AuthType:        authType,
			AuthIndex:       authIndex,
			Model:           fmt.Sprintf("model-%03d", (eventIndex*29)%modelCount),
			Timestamp:       start.Add(time.Duration(eventIndex%48)*time.Hour + time.Duration((eventIndex*37)%3_600)*time.Second),
			Failed:          eventIndex%17 == 0,
			LatencyMS:       int64(20 + eventIndex%2_000),
			InputTokens:     inputTokens,
			OutputTokens:    outputTokens,
			ReasoningTokens: reasoningTokens,
			CachedTokens:    cachedTokens,
			CacheReadTokens: cacheReadTokens,
			TotalTokens:     inputTokens + outputTokens + reasoningTokens,
		})
	}
	return fixture
}

func seedAnalyticsCoreCardinalityFixture(tb testing.TB, db *gorm.DB, fixture analyticsCoreCardinalityFixture, includeRollups bool) {
	tb.Helper()
	for name, rows := range map[string]any{
		"prices":     &fixture.prices,
		"identities": &fixture.identities,
		"aliases":    &fixture.aliases,
		"events":     &fixture.events,
	} {
		if err := db.CreateInBatches(rows, 100).Error; err != nil {
			tb.Fatalf("seed analytics benchmark %s: %v", name, err)
		}
	}
	if !includeRollups {
		return
	}
	buckets := hourlyBucketRange(fixture.start, fixture.end)
	rollups := buildUsageRollupsForBuckets(buckets, fixture.events)
	if err := db.CreateInBatches(&rollups, 100).Error; err != nil {
		tb.Fatalf("seed analytics benchmark rollups: %v", err)
	}
}
