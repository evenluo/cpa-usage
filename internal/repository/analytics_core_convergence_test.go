package repository

import (
	"fmt"
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
