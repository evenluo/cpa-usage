package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestUsagePerformancePassesNormalizedFixedSelection(t *testing.T) {
	provider := &usageEventsStub{performanceRecord: &dto.UsageAttemptPerformanceRecord{TotalAttempts: 1}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/performance?range=24h&provider=claude&model=sonnet&account=auth-1&min_latency_ms=500", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	filter := provider.lastPerformanceFilter
	if provider.performanceCalls != 1 || filter.StartTime == nil || filter.EndTime == nil || filter.MinLatencyMS == nil {
		t.Fatalf("expected one bounded repository call, calls=%d filter=%+v", provider.performanceCalls, filter)
	}
	if filter.EndTime.Sub(*filter.StartTime) != 24*time.Hour || filter.Provider != "claude" || filter.Model != "sonnet" || filter.Account != "auth-1" || *filter.MinLatencyMS != 500 {
		t.Fatalf("unexpected normalized filter: %+v", filter)
	}
}

func TestUsagePerformanceRejectsInvalidSelectionBeforeRepositoryCall(t *testing.T) {
	provider := &usageEventsStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	for _, path := range []string{
		"/api/v1/usage/performance?range=7d",
		"/api/v1/usage/performance?min_latency_ms=0",
		"/api/v1/usage/performance?min_latency_ms=0500",
	} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d: %s", path, resp.Code, resp.Body.String())
		}
	}
	if provider.performanceCalls != 0 {
		t.Fatalf("invalid filters must not reach repository, got %d calls", provider.performanceCalls)
	}
}

func TestUsagePerformanceReturnsDistinctAPIError(t *testing.T) {
	provider := &usageEventsStub{err: errors.New("database unavailable")}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/performance", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsagePerformancePayloadPreservesCoverageTopNAndSafeAccountLabel(t *testing.T) {
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	p50, p95, coverage := 100.0, 900.0, 0.5
	counts := make([]int64, dto.UsagePerformanceHistogramBins)
	counts[0], counts[23] = 1, 1
	metric := dto.UsagePercentileRecord{PopulationCount: 4, SampleCount: 2, Coverage: &coverage, P50: &p50, P95: &p95, Histogram: &dto.UsageHistogramRecord{UpperBound: 900, Counts: counts}}
	item := dto.UsagePerformanceBreakdownItemRecord{
		Value: "auth-1", AttemptCount: 5, SuccessfulAttempts: 4, FailedAttempts: 1,
		SuccessfulExecution: dto.UsageExecutionPopulationRecord{GeneratingStreaming: 2, NonGenerating: 1, Unknown: 1},
		SuccessfulLatencyMS: metric, StreamingTTFTMS: metric, StreamingOutputTPS: metric,
	}
	record := &dto.UsageAttemptPerformanceRecord{
		TotalAttempts: 5, SuccessfulAttempts: 4, FailedAttempts: 1,
		SuccessfulExecution: dto.UsageExecutionPopulationRecord{GeneratingStreaming: 2, NonGenerating: 1, Unknown: 1},
		SuccessfulLatencyMS: metric,
		Accounts:            dto.UsagePerformanceBreakdownRecord{Items: []dto.UsagePerformanceBreakdownItemRecord{item}, OtherCount: 3},
	}
	filter := usageDiagnosticFilter{usageTimeFilter: usageTimeFilter{usageWindow: usageWindow{StartTime: &start, EndTime: &end}}}
	resolver := newUsageIdentityResolver([]entities.UsageIdentity{{Name: "Claude Primary", Identity: "auth-1", AuthType: entities.UsageIdentityAuthTypeAuthFile}})
	payload := buildUsageAttemptPerformancePayload(filter, record, resolver)
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal performance payload: %v", err)
	}
	var decoded struct {
		WindowEnd string `json:"window_end"`
		LatencyMS struct {
			Successful usagePercentilePayload `json:"successful"`
		} `json:"latency_ms"`
		Accounts usagePerformanceBreakdownPayload `json:"accounts"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal performance payload: %v", err)
	}
	if decoded.WindowEnd != end.Format(time.RFC3339Nano) || decoded.LatencyMS.Successful.Coverage == nil || *decoded.LatencyMS.Successful.Coverage != 0.5 {
		t.Fatalf("lost frozen window or coverage: %s", encoded)
	}
	if len(decoded.Accounts.Items) != 1 || decoded.Accounts.Items[0].Label != "Claude Primary" || decoded.Accounts.OtherCount != 3 {
		t.Fatalf("lost safe account label or excluded count: %s", encoded)
	}
	if got := decoded.LatencyMS.Successful.Histogram; got == nil || got.UpperBound != 900 || len(got.Counts) != 24 || got.Counts[0] != 1 || got.Counts[23] != 1 {
		t.Fatalf("lost histogram counts or shared range: %s", encoded)
	}
}
