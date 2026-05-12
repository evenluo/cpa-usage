package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

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
	}}
	router := gin.New()
	registerAnalyticsRoutes(router, provider)

	req := httptest.NewRequest(http.MethodGet, "/analytics/summary?range=custom&start=2026-05-11&end=2026-05-12", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, expected := range []string{
		`"range":"custom"`,
		`"range_start":"2026-05-11T00:00:00Z"`,
		`"range_end":"2026-05-12T23:59:59.999999999Z"`,
		`"total_cost":2.45`,
		`"total_tokens":2100100`,
		`"request_count":3`,
		`"cost_available":false`,
		`"cost_status":"partial"`,
		`"label":"2026-05-11"`,
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
	if !contains(body, `"trend":[]`) || !contains(body, `"cost_status":"available"`) {
		t.Fatalf("expected empty analytics payload, got %s", body)
	}
}
