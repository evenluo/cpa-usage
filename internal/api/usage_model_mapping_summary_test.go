package api

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

type usageModelMappingSummaryStub struct {
	UsageProvider
	filter dto.UsageDiagnosticFilter
	record *dto.UsageModelMappingSummaryRecord
	calls  int
}

func (s *usageModelMappingSummaryStub) GetUsageModelMappingsSummary(_ context.Context, filter dto.UsageDiagnosticFilter) (*dto.UsageModelMappingSummaryRecord, error) {
	s.filter = filter
	s.calls++
	return s.record, nil
}

func TestUsageModelMappingSummaryPreservesFixedSnapshotAndProvider(t *testing.T) {
	provider := &usageModelMappingSummaryStub{record: &dto.UsageModelMappingSummaryRecord{
		TotalAttempts: 6, ObservedAliasAttempts: 5, MissingAliasAttempts: 1, DisplayedMappings: 4,
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	windowEnd := "2026-09-08T12:34:56Z"
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/model-mappings/summary?range=24h&provider=OpenAI&window_end="+windowEnd, nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.calls != 1 || provider.filter.Provider != "OpenAI" || provider.filter.StartTime == nil || provider.filter.EndTime == nil || provider.filter.EndTime.Sub(*provider.filter.StartTime) != 24*time.Hour {
		t.Fatalf("unexpected repository selection: calls=%d filter=%+v", provider.calls, provider.filter)
	}
	var payload usageModelMappingSummaryResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.WindowEnd != windowEnd || payload.TotalAttempts != 6 || math.Abs(payload.AliasCoverage-5.0/6.0*100) > 1e-9 || payload.DisplayedMappings != 4 {
		t.Fatalf("unexpected payload: %+v", payload)
	}
}
