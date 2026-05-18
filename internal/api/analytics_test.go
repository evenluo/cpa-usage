package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	repodto "cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/gin-gonic/gin"
)

type analyticsStub struct {
	snapshot *repodto.AnalyticsSummarySnapshot
	filter   servicedto.UsageFilter
	calls    int
}

func (s *analyticsStub) GetAnalyticsSummary(_ context.Context, filter servicedto.UsageFilter) (*repodto.AnalyticsSummarySnapshot, error) {
	s.calls++
	s.filter = filter
	return s.snapshot, nil
}

func TestBuildAnalyticsSummaryResponseMatchesContractFixture(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 18, 0, 0, 0, 0, time.UTC)
	filter := servicedto.UsageFilter{Range: "7d", Granularity: "day", StartTime: &start, EndTime: &end, Provider: "OpenAI"}
	response := buildAnalyticsSummaryResponse(filter, analyticsSummaryContractSnapshot(start))
	actual, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal analytics summary response: %v", err)
	}
	assertJSONMatchesContractFixture(t, actual, "analytics_summary.json")
}

func analyticsSummaryContractSnapshot(start time.Time) *repodto.AnalyticsSummarySnapshot {
	bucketEnd := start.Add(24 * time.Hour)
	return &repodto.AnalyticsSummarySnapshot{
		Summary: repodto.AnalyticsSummary{
			TotalCost:           2.45,
			TotalTokens:         2_100_100,
			RequestCount:        3,
			SuccessCount:        2,
			FailureCount:        1,
			InputTokens:         1_500_100,
			OutputTokens:        500_000,
			ReasoningTokens:     100_000,
			CachedTokens:        100_000,
			SuccessRate:         66.6666666667,
			CostAvailable:       false,
			CostStatus:          "partial",
			CacheReadShare:      6.666222251849877,
			CacheReadShareState: "available",
		},
		Trend: []repodto.AnalyticsTrendPoint{{
			Label:           "2026-05-11",
			BucketStart:     start,
			BucketEnd:       bucketEnd,
			TotalCost:       1.95,
			TotalTokens:     1_600_000,
			InputTokens:     1_000_000,
			OutputTokens:    500_000,
			ReasoningTokens: 100_000,
			CachedTokens:    100_000,
			RequestCount:    1,
			SuccessCount:    1,
			FailureCount:    0,
			CostAvailable:   true,
			CostStatus:      "available",
		}},
		KeyAliasBreakdown: []repodto.AnalyticsKeyAliasBreakdown{{
			Label:          "Shared Alias",
			Traceability:   "sk-a*******3456 · OpenAI",
			MaskedIdentity: "sk-a*******3456",
			AuthType:       int(entities.UsageIdentityAuthTypeAIProvider),
			Identity:       "sk-alpha-123456",
			Alias:          "Shared Alias",
			AuthTypeName:   "apikey",
			Type:           "openai",
			Provider:       "OpenAI",
			TotalCost:      2.45,
			TotalTokens:    2_100_100,
			RequestCount:   3,
			SuccessCount:   2,
			FailureCount:   1,
			SuccessRate:    66.6666666667,
			LastUsedAt:     &bucketEnd,
			CostAvailable:  false,
			CostStatus:     "partial",
			Trend: []repodto.AnalyticsKeyAliasTrendPoint{{
				Label:         "2026-05-11",
				TotalCost:     2.45,
				TotalTokens:   2_100_100,
				CostAvailable: false,
				CostStatus:    "partial",
			}},
		}},
		APIKeyBreakdown: []repodto.AnalyticsKeyAliasBreakdown{{
			Label:          "Raw API Key",
			Traceability:   "sk-a*****3456 · OpenAI",
			MaskedIdentity: "sk-a*****3456",
			AuthType:       int(entities.UsageIdentityAuthTypeAIProvider),
			Identity:       "sk-api-123456",
			Alias:          "Raw API Key",
			AuthTypeName:   "apikey",
			Provider:       "OpenAI",
			TotalCost:      1.25,
			TotalTokens:    1_000_000,
			RequestCount:   1,
			SuccessCount:   1,
			SuccessRate:    100,
			LastUsedAt:     &bucketEnd,
			CostAvailable:  true,
			CostStatus:     "available",
			Trend:          []repodto.AnalyticsKeyAliasTrendPoint{},
		}},
		ModelBreakdown: []repodto.AnalyticsModelBreakdown{{
			Model:               "priced-model",
			Provider:            "OpenAI",
			TotalCost:           2.45,
			TotalTokens:         2_100_100,
			RequestCount:        3,
			SuccessCount:        2,
			FailureCount:        1,
			InputTokens:         1_500_100,
			OutputTokens:        500_000,
			ReasoningTokens:     100_000,
			CachedTokens:        100_000,
			SuccessRate:         66.6666666667,
			TotalLatencyMS:      600,
			LatencySampleCount:  3,
			AverageLatencyMS:    200,
			CostAvailable:       false,
			CostStatus:          "partial",
			CacheReadShare:      6.666222251849877,
			CacheReadShareState: "available",
		}},
		TimeBreakdown: []repodto.AnalyticsTrendPoint{{
			Label:           "2026-05-11",
			BucketStart:     start,
			BucketEnd:       bucketEnd,
			TotalCost:       1.95,
			TotalTokens:     1_600_000,
			InputTokens:     1_000_000,
			OutputTokens:    500_000,
			ReasoningTokens: 100_000,
			CachedTokens:    100_000,
			RequestCount:    1,
			SuccessCount:    1,
			FailureCount:    0,
			CostAvailable:   true,
			CostStatus:      "available",
		}},
		Insights: []repodto.AnalyticsInsight{{
			Type:        "pricing_missing",
			Severity:    "amber",
			Title:       "Pricing Missing",
			Detail:      "Some Cost values are partial.",
			Subject:     "1 model",
			MetricLabel: "Cost status",
			MetricValue: 1,
			Count:       1,
			CostStatus:  "partial",
		}},
		ProviderOptions: []repodto.AnalyticsProviderOption{{
			Provider:      "OpenAI",
			RequestCount:  3,
			TotalTokens:   2_100_100,
			TotalCost:     2.45,
			CostAvailable: false,
			CostStatus:    "partial",
		}},
		Comparison: repodto.AnalyticsComparison{HasPreviousPeriod: false},
		Heatmap: repodto.AnalyticsHeatmap{
			Measure:     "tokens",
			MaxTokens:   1_600_000,
			MaxCost:     1.95,
			MaxRequests: 1,
			Rows: []repodto.AnalyticsHeatmapRow{{
				Date:  "2026-05-11",
				Label: "05/11 Mon",
				Cells: []repodto.AnalyticsHeatmapCell{},
			}},
		},
	}
}

func TestAnalyticsSummaryRouteReturnsSummaryTrendAndRangeMetadata(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 0, time.UTC)
	previousStart := time.Date(2026, 5, 9, 0, 0, 0, 0, time.UTC)
	previousEnd := time.Date(2026, 5, 10, 23, 59, 59, 999999999, time.UTC)
	costChange := 12.5
	tokenChange := -4.25
	requestChange := 8.0
	successRateChange := 1.75
	provider := &analyticsStub{snapshot: &repodto.AnalyticsSummarySnapshot{
		Summary: repodto.AnalyticsSummary{
			TotalCost:           2.45,
			TotalTokens:         2_100_100,
			RequestCount:        3,
			SuccessCount:        2,
			FailureCount:        1,
			InputTokens:         1_500_100,
			OutputTokens:        500_000,
			ReasoningTokens:     100_000,
			CachedTokens:        100_000,
			SuccessRate:         66.6666666667,
			CostAvailable:       false,
			CostStatus:          "partial",
			CacheReadShare:      6.666222251849877,
			CacheReadShareState: "available",
		},
		Trend: []repodto.AnalyticsTrendPoint{{
			Label:           "2026-05-11",
			BucketStart:     start,
			BucketEnd:       end,
			TotalCost:       1.95,
			TotalTokens:     1_600_000,
			InputTokens:     1_000_000,
			OutputTokens:    500_000,
			ReasoningTokens: 100_000,
			CachedTokens:    100_000,
			RequestCount:    1,
			SuccessCount:    1,
			FailureCount:    0,
			CostAvailable:   true,
			CostStatus:      "available",
		}},
		KeyAliasBreakdown: []repodto.AnalyticsKeyAliasBreakdown{{
			Label:          "Shared Alias",
			Traceability:   "sk-a*******3456 · OpenAI",
			MaskedIdentity: "sk-a*******3456",
			AuthType:       int(entities.UsageIdentityAuthTypeAIProvider),
			Identity:       "sk-alpha-123456",
			Alias:          "Shared Alias",
			Name:           "OpenAI Team",
			AuthTypeName:   "apikey",
			Type:           "openai",
			Provider:       "OpenAI",
			TotalCost:      2.45,
			TotalTokens:    2_100_100,
			RequestCount:   3,
			SuccessCount:   2,
			FailureCount:   1,
			SuccessRate:    66.6666666667,
			LastUsedAt:     &end,
			CostAvailable:  false,
			CostStatus:     "partial",
			Trend: []repodto.AnalyticsKeyAliasTrendPoint{{
				Label:         "2026-05-11",
				TotalCost:     2.45,
				TotalTokens:   2_100_100,
				CostAvailable: false,
				CostStatus:    "partial",
			}},
		}},
		APIKeyBreakdown: []repodto.AnalyticsKeyAliasBreakdown{{
			Label:          "Raw API Key",
			Traceability:   "sk-a*****3456 · OpenAI",
			MaskedIdentity: "sk-a*****3456",
			AuthType:       int(entities.UsageIdentityAuthTypeAIProvider),
			Identity:       "sk-api-123456",
			Alias:          "Raw API Key",
			AuthTypeName:   "apikey",
			Provider:       "OpenAI",
			TotalCost:      1.25,
			TotalTokens:    1_000_000,
			RequestCount:   1,
			SuccessCount:   1,
			SuccessRate:    100,
			LastUsedAt:     &end,
			CostAvailable:  true,
			CostStatus:     "available",
		}},
		ModelBreakdown: []repodto.AnalyticsModelBreakdown{{
			Model:               "priced-model",
			Provider:            "OpenAI",
			TotalCost:           2.45,
			TotalTokens:         2_100_100,
			RequestCount:        3,
			SuccessCount:        2,
			FailureCount:        1,
			InputTokens:         1_500_100,
			OutputTokens:        500_000,
			ReasoningTokens:     100_000,
			CachedTokens:        100_000,
			SuccessRate:         66.6666666667,
			TotalLatencyMS:      600,
			LatencySampleCount:  3,
			AverageLatencyMS:    200,
			CostAvailable:       false,
			CostStatus:          "partial",
			CacheReadShare:      6.666222251849877,
			CacheReadShareState: "available",
		}},
		TimeBreakdown: []repodto.AnalyticsTrendPoint{{
			Label:           "2026-05-11",
			BucketStart:     start,
			BucketEnd:       end,
			TotalCost:       1.95,
			TotalTokens:     1_600_000,
			InputTokens:     1_000_000,
			OutputTokens:    500_000,
			ReasoningTokens: 100_000,
			CachedTokens:    100_000,
			RequestCount:    1,
			SuccessCount:    1,
			FailureCount:    0,
			CostAvailable:   true,
			CostStatus:      "available",
		}},
		Insights: []repodto.AnalyticsInsight{{
			Type:        "pricing_missing",
			Severity:    "amber",
			Title:       "Pricing Missing",
			Detail:      "Some Cost values are partial.",
			Subject:     "1 model",
			MetricLabel: "Cost status",
			MetricValue: 1,
			Count:       1,
			CostStatus:  "partial",
		}},
		ProviderOptions: []repodto.AnalyticsProviderOption{{
			Provider:      "OpenAI",
			RequestCount:  3,
			TotalTokens:   2_100_100,
			TotalCost:     2.45,
			CostAvailable: false,
			CostStatus:    "partial",
		}},
		PreviousRangeStart: &previousStart,
		PreviousRangeEnd:   &previousEnd,
		Comparison: repodto.AnalyticsComparison{
			HasPreviousPeriod:     true,
			TotalCostChangePct:    &costChange,
			TotalTokensChangePct:  &tokenChange,
			RequestCountChangePct: &requestChange,
			SuccessRateChangePP:   &successRateChange,
		},
		Heatmap: repodto.AnalyticsHeatmap{
			Measure:     "tokens",
			MaxTokens:   1_600_000,
			MaxCost:     1.95,
			MaxRequests: 1,
			MaxFailures: 1,
			Rows: []repodto.AnalyticsHeatmapRow{{
				Date:  "2026-05-11",
				Label: "Mon 05/11",
				Cells: []repodto.AnalyticsHeatmapCell{{
					Hour:          9,
					InRange:       true,
					BucketStart:   start.Add(9 * time.Hour),
					BucketEnd:     start.Add(10 * time.Hour),
					TotalTokens:   1_600_000,
					TotalCost:     1.95,
					RequestCount:  1,
					FailureCount:  0,
					CostAvailable: true,
					CostStatus:    "available",
				}},
			}},
		},
	}}
	router := gin.New()
	registerAnalyticsRoutes(router, provider)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=custom&start=2026-05-11&end=2026-05-12&provider=OpenAI", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"range":"custom"`,
		`"granularity":"hour"`,
		`"provider":"OpenAI"`,
		`"range_start":"2026-05-11T00:00:00Z"`,
		`"range_end":"2026-05-12T23:59:59.999999999Z"`,
		`"previous_range_start":"2026-05-09T00:00:00Z"`,
		`"previous_range_end":"2026-05-10T23:59:59.999999999Z"`,
		`"comparison":{`,
		`"has_previous_period":true`,
		`"total_cost_change_pct":12.5`,
		`"total_tokens_change_pct":-4.25`,
		`"request_count_change_pct":8`,
		`"success_rate_change_pp":1.75`,
		`"heatmap":{`,
		`"measure":"tokens"`,
		`"max_tokens":1600000`,
		`"max_cost":1.95`,
		`"max_requests":1`,
		`"max_failures":1`,
		`"date":"2026-05-11"`,
		`"hour":9`,
		`"in_range":true`,
		`"bucket_start":"2026-05-11T09:00:00Z"`,
		`"total_cost":2.45`,
		`"total_tokens":2100100`,
		`"request_count":3`,
		`"input_tokens":1500100`,
		`"output_tokens":500000`,
		`"reasoning_tokens":100000`,
		`"cached_tokens":100000`,
		`"cache_read_share":6.666222251849877`,
		`"cache_read_share_state":"available"`,
		`"cost_available":false`,
		`"cost_status":"partial"`,
		`"label":"2026-05-11"`,
		`"key_alias_breakdown":[`,
		`"label":"Shared Alias"`,
		`"traceability":"sk-a*******3456 · OpenAI"`,
		`"identity":"sk-a*******3456"`,
		`"api_key_breakdown":[`,
		`"label":"Raw API Key"`,
		`"identity":"sk-a*****3456"`,
		`"last_used_at":"2026-05-12T23:59:59Z"`,
		`"model_distribution":[`,
		`"model":"priced-model"`,
		`"average_latency_ms":200`,
		`"time_breakdown":[`,
		`"insights":[`,
		`"type":"pricing_missing"`,
		`"subject":"1 model"`,
		`"provider_options":[`,
		`"provider":"OpenAI"`,
		`"request_count":3`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, body)
		}
	}
	if contains(body, `"estimated_cache_savings"`) {
		t.Fatalf("expected partial pricing response to withhold estimated cache savings, got %s", body)
	}
	if provider.calls != 1 {
		t.Fatalf("expected provider to be called once, got %d", provider.calls)
	}
	if provider.filter.Range != "custom" || provider.filter.StartTime == nil || provider.filter.EndTime == nil {
		t.Fatalf("expected custom range filter, got %+v", provider.filter)
	}
	if provider.filter.Provider != "OpenAI" {
		t.Fatalf("expected provider filter to be passed through, got %+v", provider.filter)
	}
	if provider.filter.Granularity != "hour" {
		t.Fatalf("expected default hour granularity to be passed through, got %+v", provider.filter)
	}
}

func TestAnalyticsSummaryRouteOmitsUnavailableComparisonDeltas(t *testing.T) {
	provider := &analyticsStub{snapshot: &repodto.AnalyticsSummarySnapshot{
		Comparison: repodto.AnalyticsComparison{HasPreviousPeriod: false},
	}}
	router := gin.New()
	registerAnalyticsRoutes(router, provider)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=7d", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"comparison":{`,
		`"has_previous_period":false`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, body)
		}
	}
	for _, unexpected := range []string{
		`"total_cost_change_pct"`,
		`"total_tokens_change_pct"`,
		`"request_count_change_pct"`,
		`"success_rate_change_pp"`,
	} {
		if contains(body, unexpected) {
			t.Fatalf("expected response to omit %s, got %s", unexpected, body)
		}
	}
}

func TestAnalyticsSummaryRouteAcceptsDayGranularity(t *testing.T) {
	provider := &analyticsStub{snapshot: &repodto.AnalyticsSummarySnapshot{}}
	router := gin.New()
	registerAnalyticsRoutes(router, provider)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=7d&granularity=day", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `"granularity":"day"`) {
		t.Fatalf("expected day granularity in response, got %s", rec.Body.String())
	}
	if provider.filter.Granularity != "day" {
		t.Fatalf("expected day granularity to be passed through, got %+v", provider.filter)
	}
}

func TestAnalyticsSummaryFilterUsesRequestAnchorForFixedOperationalWindow(t *testing.T) {
	anchor := time.Date(2026, 5, 18, 14, 30, 0, 0, time.UTC)
	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=custom&start=2026-05-01&end=2026-05-02", nil)

	filter, err := parseAnalyticsSummaryFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseAnalyticsSummaryFilterQuery returned error: %v", err)
	}

	if filter.FixedWindowEnd == nil || !filter.FixedWindowEnd.Equal(anchor) {
		t.Fatalf("expected fixed operational window to use request anchor, got %+v", filter)
	}
	if filter.EndTime == nil || filter.EndTime.Equal(anchor) {
		t.Fatalf("expected selected analysis window end to remain independent, got %+v", filter)
	}
}

func TestAnalyticsSummaryRouteRejectsUnsupportedGranularity(t *testing.T) {
	provider := &analyticsStub{snapshot: &repodto.AnalyticsSummarySnapshot{}}
	router := gin.New()
	registerAnalyticsRoutes(router, provider)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=7d&granularity=week", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", rec.Code, rec.Body.String())
	}
	if !contains(rec.Body.String(), `unsupported granularity \"week\"`) {
		t.Fatalf("expected unsupported granularity error, got %s", rec.Body.String())
	}
	if provider.calls != 0 {
		t.Fatalf("expected provider not to be called, got %d calls", provider.calls)
	}
}

func TestAnalyticsSummaryRouteReturnsEmptyPayloadWithoutProvider(t *testing.T) {
	router := gin.New()
	registerAnalyticsRoutes(router, nil)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !contains(body, `"trend":[]`) || !contains(body, `"model_distribution":[]`) || !contains(body, `"time_breakdown":[]`) || !contains(body, `"provider_options":[]`) || !contains(body, `"cost_status":"available"`) {
		t.Fatalf("expected empty analytics payload, got %s", body)
	}
}
