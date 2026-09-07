package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

func TestUsageModelMappingsPassesSharedFixedSelectionAndPreservesWindowPrecision(t *testing.T) {
	provider := &usageEventsStub{mappingRecord: &dto.UsageModelMappingDistributionRecord{
		TotalAttempts: 1, ObservedAliasAttempts: 1,
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	windowEnd := "2026-09-07T12:34:56.123456789Z"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/model-mappings?range=24h&window_end="+windowEnd+"&provider=claude&model=sonnet&model_alias=route-a&account=auth-1&endpoint=/v1/messages&status=429", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	filter := provider.lastMappingFilter
	if provider.mappingCalls != 1 || filter.StartTime == nil || filter.EndTime == nil || filter.EndTime.Sub(*filter.StartTime) != 24*time.Hour {
		t.Fatalf("expected one bounded repository call, calls=%d filter=%+v", provider.mappingCalls, filter)
	}
	if filter.Provider != "claude" || filter.Model != "sonnet" || filter.ModelAlias != "route-a" || filter.Account != "auth-1" || filter.Endpoint != "/v1/messages" || filter.Status != "429" {
		t.Fatalf("unexpected normalized filter: %+v", filter)
	}
	if !strings.Contains(resp.Body.String(), `"window_end":"`+windowEnd+`"`) {
		t.Fatalf("expected exact RFC3339Nano window_end, got %s", resp.Body.String())
	}
}

func TestUsageModelMappingsMatchesReusableContractFixture(t *testing.T) {
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filter := usageDiagnosticFilter{usageTimeFilter: usageTimeFilter{usageWindow: usageWindow{StartTime: &start, EndTime: &end}}}
	record := &dto.UsageModelMappingDistributionRecord{
		TotalAttempts: 6, CanonicalValidAttempts: 6, ObservedAliasAttempts: 5, MissingAliasAttempts: 1,
		ObservedTotalCost: 1, ObservedCostStatus: dto.CostStatusPartial,
		Mappings: []dto.UsageModelMappingRecord{
			{ModelAlias: "route-a", Model: "actual-a", Provider: "provider-a", AttemptCount: 2, CanonicalValidAttempts: 2, FailureCount: 1, FailureShare: 50, LatencySampleCount: 1, MeanLatencyMS: 100, TotalCost: 1, CostAvailable: true, CostStatus: dto.CostStatusAvailable},
			{ModelAlias: "route-a", Model: "actual-b", Provider: "provider-b", AttemptCount: 1, CanonicalValidAttempts: 1, FailureCount: 1, FailureShare: 100, LatencySampleCount: 1, MeanLatencyMS: 300, CostStatus: dto.CostStatusUnavailable},
			{ModelAlias: "actual-b", Model: "actual-b", Provider: "provider-b", AttemptCount: 1, CanonicalValidAttempts: 1, LatencySampleCount: 1, MeanLatencyMS: 200, CostStatus: dto.CostStatusUnavailable},
			{ModelAlias: "route-missing-provider", Model: "actual-a", AttemptCount: 1, CanonicalValidAttempts: 1, CostAvailable: true, CostStatus: dto.CostStatusAvailable},
		},
	}
	actual, err := json.Marshal(buildUsageModelMappingDistributionPayload(filter, record))
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	assertJSONMatchesContractFixture(t, actual, "usage_model_mappings.json")
}

func TestUsageModelMappingsReturnsExplicitCoverageAndMappingSemantics(t *testing.T) {
	provider := &usageEventsStub{mappingRecord: &dto.UsageModelMappingDistributionRecord{
		TotalAttempts: 5, ObservedAliasAttempts: 4, MissingAliasAttempts: 1,
		ObservedTotalCost: 1.25, ObservedCostStatus: dto.CostStatusPartial,
		Mappings: []dto.UsageModelMappingRecord{
			{ModelAlias: "route-a", Model: "actual-a", Provider: "provider-a", AttemptCount: 2, CanonicalValidAttempts: 2, FailureCount: 1, FailureShare: 50, LatencySampleCount: 1, MeanLatencyMS: 100, TotalCost: 1.25, CostAvailable: true, CostStatus: dto.CostStatusAvailable},
			{ModelAlias: "actual-b", Model: "actual-b", Provider: "", AttemptCount: 1, CanonicalValidAttempts: 1, CostStatus: dto.CostStatusUnavailable},
		},
		OtherAttempts: 1,
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/model-mappings", nil))

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	for _, want := range []string{
		`"total_attempts":5`, `"observed_alias_attempts":4`, `"missing_alias_attempts":1`, `"alias_coverage":80`,
		`"observed_cost_status":"partial"`, `"model_alias":"route-a"`, `"failure_share":50`,
		`"model_alias":"actual-b"`, `"provider":""`, `"other_attempts":1`,
	} {
		if !strings.Contains(resp.Body.String(), want) {
			t.Fatalf("expected %s in response: %s", want, resp.Body.String())
		}
	}
}

func TestUsageModelMappingsRejectsInvalidAliasBeforeRepositoryCall(t *testing.T) {
	provider := &usageEventsStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	path := "/api/v1/usage/model-mappings?model_alias=" + strings.Repeat("x", 129)
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
	if resp.Code != http.StatusBadRequest || provider.mappingCalls != 0 {
		t.Fatalf("expected 400 before repository call, code=%d calls=%d body=%s", resp.Code, provider.mappingCalls, resp.Body.String())
	}
}
