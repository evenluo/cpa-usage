package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository/dto"
)

type usageAnalysisStub struct {
	apiRows       []dto.UsageAnalysisAPIStatRecord
	modelRows     []dto.UsageAnalysisModelStatRecord
	err           error
	lastFilter    dto.UsageTimeScope
	analysisCalls int
}

func (s *usageAnalysisStub) GetUsageOverview(context.Context, dto.UsageOverviewFilter) (*dto.UsageOverviewRecord, error) {
	return nil, nil
}

func (s *usageAnalysisStub) GetRequestHealth(context.Context, dto.UsageOverviewFilter) (*dto.UsageOverviewHealthRecord, error) {
	return nil, nil
}

func (s *usageAnalysisStub) ListUsageEvents(context.Context, dto.UsageEventListFilter) (*dto.UsageEventsPageRecord, error) {
	return nil, nil
}

func (s *usageAnalysisStub) ListUsageEventFilterOptions(context.Context, dto.UsageTimeScope) (*dto.UsageEventFilterOptionsRecord, error) {
	return nil, nil
}

func (s *usageAnalysisStub) GetUsageAnalysis(_ context.Context, filter dto.UsageTimeScope) ([]dto.UsageAnalysisAPIStatRecord, []dto.UsageAnalysisModelStatRecord, error) {
	s.lastFilter = filter
	s.analysisCalls++
	return s.apiRows, s.modelRows, s.err
}

func TestUsageAnalysisReturnsAggregatedRows(t *testing.T) {
	provider := &usageAnalysisStub{
		apiRows: []dto.UsageAnalysisAPIStatRecord{{
			APIGroupKey:   "provider-a",
			DisplayName:   "provider-a",
			TotalRequests: 2,
			SuccessCount:  1,
			FailureCount:  1,
			TotalTokens:   42,
			Models: []dto.UsageAnalysisModelStatRecord{{
				Model:              "claude-sonnet",
				TotalRequests:      2,
				SuccessCount:       1,
				FailureCount:       1,
				InputTokens:        30,
				OutputTokens:       9,
				ReasoningTokens:    2,
				CachedTokens:       1,
				TotalTokens:        42,
				TotalLatencyMS:     350,
				LatencySampleCount: 2,
			}},
		}},
		modelRows: []dto.UsageAnalysisModelStatRecord{{
			Model:              "claude-sonnet",
			TotalRequests:      2,
			SuccessCount:       1,
			FailureCount:       1,
			InputTokens:        30,
			OutputTokens:       9,
			ReasoningTokens:    2,
			CachedTokens:       1,
			TotalTokens:        42,
			TotalLatencyMS:     350,
			LatencySampleCount: 2,
		}},
	}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/analysis?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"apis":[`) || !contains(body, `"models":[`) {
		t.Fatalf("unexpected response body: %s", body)
	}
	if !contains(body, `"display_name":"prov**er-a"`) {
		t.Fatalf("expected display name in response body: %s", body)
	}
	if !contains(body, `"api_key":"`+redact.APIAlias("provider-a")+`"`) {
		t.Fatalf("expected redacted api key alias in response body: %s", body)
	}
	if !contains(body, `"model":"claude-sonnet"`) || !contains(body, `"latency_sample_count":2`) || !contains(body, `"total_latency_ms":350`) {
		t.Fatalf("expected model latency aggregates in response body: %s", body)
	}
	if provider.analysisCalls != 1 {
		t.Fatalf("expected GetUsageAnalysis to be called once, got %d", provider.analysisCalls)
	}
	if provider.lastFilter.StartTime == nil || provider.lastFilter.EndTime == nil {
		t.Fatalf("expected resolved time bounds in filter, got %+v", provider.lastFilter)
	}
}

func TestUsageAnalysisRequiresAuthWhenEnabled(t *testing.T) {
	router := NewRouter(nil, nil, &usageAnalysisStub{}, nil, AuthConfig{Enabled: true, LoginPassword: "secret", SessionTTL: time.Hour}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/analysis", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected status 401, got %d", resp.Code)
	}
}
