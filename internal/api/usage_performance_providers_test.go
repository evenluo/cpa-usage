package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

type usagePerformanceProvidersStub struct {
	UsageProvider
	filter dto.UsageTimeScope
	record *dto.UsagePerformanceProviderOptionsRecord
	calls  int
}

func (s *usagePerformanceProvidersStub) ListUsagePerformanceProviders(_ context.Context, filter dto.UsageTimeScope) (*dto.UsagePerformanceProviderOptionsRecord, error) {
	s.filter = filter
	s.calls++
	return s.record, nil
}

func TestUsagePerformanceProvidersReturnsCompleteSortedCatalogWindow(t *testing.T) {
	provider := &usagePerformanceProvidersStub{record: &dto.UsagePerformanceProviderOptionsRecord{
		ProviderOptions: []dto.UsagePerformanceProviderOptionRecord{
			{Provider: "alpha", RequestCount: 9},
			{Provider: "beta", RequestCount: 4},
		},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	windowEnd := "2026-09-08T12:34:56Z"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/performance/providers?range=24h&window_end="+windowEnd, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.calls != 1 || provider.filter.Provider != "" || provider.filter.StartTime == nil || provider.filter.EndTime == nil || provider.filter.EndTime.Sub(*provider.filter.StartTime) != 24*time.Hour {
		t.Fatalf("expected one unscoped bounded call, calls=%d filter=%+v", provider.calls, provider.filter)
	}
	var payload usagePerformanceProvidersResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.WindowEnd != windowEnd || len(payload.ProviderOptions) != 2 || payload.ProviderOptions[0].Provider != "alpha" || payload.ProviderOptions[0].RequestCount != 9 {
		t.Fatalf("unexpected provider payload: %+v", payload)
	}
}

func TestUsagePerformanceProvidersRejectsNon24HourRangeBeforeRead(t *testing.T) {
	provider := &usagePerformanceProvidersStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/performance/providers?range=7d", nil))
	if resp.Code != http.StatusBadRequest || provider.calls != 0 {
		t.Fatalf("expected 400 without repository read, code=%d calls=%d body=%s", resp.Code, provider.calls, resp.Body.String())
	}
}
