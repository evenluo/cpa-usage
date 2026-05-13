package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/gin-gonic/gin"
)

type analyticsStub struct {
	snapshot *servicedto.AnalyticsSummarySnapshot
	filter   servicedto.UsageFilter
	calls    int
}

func (s *analyticsStub) GetAnalyticsSummary(_ context.Context, filter servicedto.UsageFilter) (*servicedto.AnalyticsSummarySnapshot, error) {
	s.calls++
	s.filter = filter
	return s.snapshot, nil
}

func TestAnalyticsSummaryRouteReturnsSummaryTrendAndRangeMetadata(t *testing.T) {
	previousLocal := time.Local
	time.Local = time.UTC
	t.Cleanup(func() { time.Local = previousLocal })

	start := time.Date(2026, 5, 11, 0, 0, 0, 0, time.UTC)
	end := time.Date(2026, 5, 12, 23, 59, 59, 0, time.UTC)
	provider := &analyticsStub{snapshot: &servicedto.AnalyticsSummarySnapshot{
		Summary: servicedto.AnalyticsSummary{
			TotalCost:     2.45,
			TotalTokens:   2_100_100,
			RequestCount:  3,
			SuccessCount:  2,
			FailureCount:  1,
			SuccessRate:   66.6666666667,
			CostAvailable: false,
			CostStatus:    "partial",
		},
		Trend: []servicedto.AnalyticsTrendPoint{{
			Label:         "2026-05-11",
			BucketStart:   start,
			BucketEnd:     end,
			TotalCost:     1.95,
			TotalTokens:   1_600_000,
			RequestCount:  1,
			SuccessCount:  1,
			FailureCount:  0,
			CostAvailable: true,
			CostStatus:    "available",
		}},
		KeyAliasBreakdown: []servicedto.AnalyticsKeyAliasBreakdown{{
			AuthType:      int(entities.UsageIdentityAuthTypeAIProvider),
			Identity:      "sk-alpha-123456",
			Alias:         "Shared Alias",
			Name:          "OpenAI Team",
			AuthTypeName:  "apikey",
			Type:          "openai",
			Provider:      "OpenAI",
			TotalCost:     2.45,
			TotalTokens:   2_100_100,
			RequestCount:  3,
			SuccessCount:  2,
			FailureCount:  1,
			SuccessRate:   66.6666666667,
			LastUsedAt:    &end,
			CostAvailable: false,
			CostStatus:    "partial",
			Trend: []servicedto.AnalyticsKeyAliasTrendPoint{{
				Label:         "2026-05-11",
				TotalCost:     2.45,
				TotalTokens:   2_100_100,
				CostAvailable: false,
				CostStatus:    "partial",
			}},
		}},
		ModelBreakdown: []servicedto.AnalyticsModelBreakdown{{
			Model:              "priced-model",
			Provider:           "OpenAI",
			TotalCost:          2.45,
			TotalTokens:        2_100_100,
			RequestCount:       3,
			SuccessCount:       2,
			FailureCount:       1,
			SuccessRate:        66.6666666667,
			TotalLatencyMS:     600,
			LatencySampleCount: 3,
			AverageLatencyMS:   200,
			CostAvailable:      false,
			CostStatus:         "partial",
		}},
		TimeBreakdown: []servicedto.AnalyticsTrendPoint{{
			Label:         "2026-05-11",
			BucketStart:   start,
			BucketEnd:     end,
			TotalCost:     1.95,
			TotalTokens:   1_600_000,
			RequestCount:  1,
			SuccessCount:  1,
			FailureCount:  0,
			CostAvailable: true,
			CostStatus:    "available",
		}},
		Insights: []servicedto.AnalyticsInsight{{
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
		`"provider":"OpenAI"`,
		`"range_start":"2026-05-11T00:00:00Z"`,
		`"range_end":"2026-05-12T23:59:59.999999999Z"`,
		`"total_cost":2.45`,
		`"total_tokens":2100100`,
		`"request_count":3`,
		`"cost_available":false`,
		`"cost_status":"partial"`,
		`"label":"2026-05-11"`,
		`"key_alias_breakdown":[`,
		`"label":"Shared Alias"`,
		`"traceability":"sk-a*******3456 · OpenAI"`,
		`"identity":"sk-a*******3456"`,
		`"last_used_at":"2026-05-12T23:59:59Z"`,
		`"model_distribution":[`,
		`"model":"priced-model"`,
		`"average_latency_ms":200`,
		`"time_breakdown":[`,
		`"insights":[`,
		`"type":"pricing_missing"`,
		`"subject":"1 model"`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, body)
		}
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
	if !contains(body, `"trend":[]`) || !contains(body, `"model_distribution":[]`) || !contains(body, `"time_breakdown":[]`) || !contains(body, `"cost_status":"available"`) {
		t.Fatalf("expected empty analytics payload, got %s", body)
	}
}
