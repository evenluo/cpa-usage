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

func TestUsageFailuresPassesNormalizedFixedSelection(t *testing.T) {
	provider := &usageEventsStub{failureRecord: &dto.UsageFailureDistributionRecord{TotalFailures: 1}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/failures?range=24h&provider=claude&model=sonnet&account=auth-1&endpoint=/v1/messages&status=429", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	filter := provider.lastFailureFilter
	if provider.failureCalls != 1 || filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected one bounded repository call, calls=%d filter=%+v", provider.failureCalls, filter)
	}
	if filter.EndTime.Sub(*filter.StartTime) != 24*time.Hour || filter.Provider != "claude" || filter.Model != "sonnet" || filter.Account != "auth-1" || filter.Endpoint != "/v1/messages" || filter.Status != "429" {
		t.Fatalf("unexpected normalized filter: %+v", filter)
	}
}

func TestUsageFailuresRejectsInvalidSelectionBeforeRepositoryCall(t *testing.T) {
	provider := &usageEventsStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	for _, path := range []string{
		"/api/v1/usage/failures?range=7d",
		"/api/v1/usage/failures?status=600",
		"/api/v1/usage/failures?endpoint=/v1/messages%3Fsecret%3D1",
	} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 for %s, got %d: %s", path, resp.Code, resp.Body.String())
		}
	}
	if provider.failureCalls != 0 {
		t.Fatalf("invalid filters must not reach repository, got %d calls", provider.failureCalls)
	}
}

func TestUsageFailuresReturnsDistinctAPIError(t *testing.T) {
	provider := &usageEventsStub{err: errors.New("database unavailable")}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/failures", nil))
	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsageFailureDistributionMatchesReusableContractFixture(t *testing.T) {
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	filter := usageDiagnosticFilter{usageTimeFilter: usageTimeFilter{usageWindow: usageWindow{StartTime: &start, EndTime: &end}}}
	record := &dto.UsageFailureDistributionRecord{
		TotalFailures: 3,
		Categories:    dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "4xx", Count: 2}, {Value: "unknown", Count: 1}}},
		Statuses:      dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "429", Count: 2}, {Value: "unknown", Count: 1}}},
		Providers:     dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "claude", Count: 3}}},
		Accounts:      dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "auth-1", Count: 2}}, OtherCount: 1},
		Models:        dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "sonnet", Count: 3}}},
		Endpoints:     dto.UsageFailureBreakdownRecord{Items: []dto.UsageFailureBreakdownItemRecord{{Value: "/v1/messages", Count: 3}}},
	}
	resolver := newUsageIdentityResolver([]entities.UsageIdentity{{
		Name: "Claude Primary", Identity: "auth-1", AuthType: entities.UsageIdentityAuthTypeAuthFile,
	}})
	payload := buildUsageFailureDistributionPayload(filter, record, resolver)
	actual, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	assertJSONMatchesContractFixture(t, actual, "usage_failure_distribution.json")
}
