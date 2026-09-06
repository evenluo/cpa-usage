package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository/dto"
)

type usageEventsStub struct {
	events                []dto.UsageEventRecord
	eventsPage            *dto.UsageEventsPageRecord
	eventFilterOptions    *dto.UsageEventFilterOptionsRecord
	err                   error
	lastFilter            dto.UsageEventListFilter
	lastOptionsFilter     dto.UsageTimeScope
	filterCalls           int
	filterOptionCalls     int
	failureRecord         *dto.UsageFailureDistributionRecord
	lastFailureFilter     dto.UsageDiagnosticFilter
	failureCalls          int
	mappingRecord         *dto.UsageModelMappingDistributionRecord
	lastMappingFilter     dto.UsageDiagnosticFilter
	mappingCalls          int
	performanceRecord     *dto.UsageAttemptPerformanceRecord
	lastPerformanceFilter dto.UsageDiagnosticFilter
	performanceCalls      int
}

func (s *usageEventsStub) GetUsageOverview(context.Context, dto.UsageOverviewFilter) (*dto.UsageOverviewRecord, error) {
	return nil, nil
}

func (s *usageEventsStub) GetRequestHealth(context.Context, dto.UsageOverviewFilter) (*dto.UsageOverviewHealthRecord, error) {
	return nil, nil
}

func (s *usageEventsStub) ListUsageEvents(_ context.Context, filter dto.UsageEventListFilter) (*dto.UsageEventsPageRecord, error) {
	s.lastFilter = filter
	s.filterCalls++
	if s.eventsPage != nil {
		return s.eventsPage, s.err
	}
	return &dto.UsageEventsPageRecord{Events: s.events, TotalCount: int64(len(s.events)), Page: 1, PageSize: dto.DefaultUsageEventsLimit, TotalPages: 1}, s.err
}

func (s *usageEventsStub) GetUsageFailureDistribution(_ context.Context, filter dto.UsageDiagnosticFilter) (*dto.UsageFailureDistributionRecord, error) {
	s.lastFailureFilter = filter
	s.failureCalls++
	if s.failureRecord != nil {
		return s.failureRecord, s.err
	}
	return &dto.UsageFailureDistributionRecord{}, s.err
}

func (s *usageEventsStub) GetUsageModelMappings(_ context.Context, filter dto.UsageDiagnosticFilter) (*dto.UsageModelMappingDistributionRecord, error) {
	s.lastMappingFilter = filter
	s.mappingCalls++
	if s.mappingRecord != nil {
		return s.mappingRecord, s.err
	}
	return &dto.UsageModelMappingDistributionRecord{}, s.err
}

func (s *usageEventsStub) GetUsageAttemptPerformance(_ context.Context, filter dto.UsageDiagnosticFilter) (*dto.UsageAttemptPerformanceRecord, error) {
	s.lastPerformanceFilter = filter
	s.performanceCalls++
	if s.performanceRecord != nil {
		return s.performanceRecord, s.err
	}
	return &dto.UsageAttemptPerformanceRecord{}, s.err
}

func (s *usageEventsStub) ListUsageEventFilterOptions(_ context.Context, filter dto.UsageTimeScope) (*dto.UsageEventFilterOptionsRecord, error) {
	s.lastOptionsFilter = filter
	s.filterOptionCalls++
	if s.eventFilterOptions != nil {
		return s.eventFilterOptions, s.err
	}
	return &dto.UsageEventFilterOptionsRecord{}, s.err
}

func (s *usageEventsStub) GetUsageAnalysis(context.Context, dto.UsageTimeScope) ([]dto.UsageAnalysisAPIStatRecord, []dto.UsageAnalysisModelStatRecord, error) {
	return nil, nil, s.err
}

func TestUsageEventsReturnsFilteredRows(t *testing.T) {
	ttftMS := int64(1052)
	outputTPS := 48.33358094488189
	staleOutputTPS := 999.0
	statusCode := 200
	cacheReadTokens := int64(1)
	cacheCreationTokens := int64(3)
	canonicalInput := int64(10)
	canonicalUncached := int64(6)
	canonicalOutput := int64(4)
	canonicalNonReasoning := int64(2)
	canonicalReasoning := int64(2)
	canonicalZero := int64(0)
	canonicalTotal := int64(14)
	accountingVersion := int64(2)
	quality := "complete"
	generate := true
	stream := false
	requestTier := "priority"
	responseTier := "default"
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		AttemptFacts: dto.UsageAttemptFacts{
			Generate: &generate, Stream: &stream, RequestServiceTier: &requestTier, ResponseServiceTier: &responseTier, OutputTPS: &outputTPS,
			Accounting: dto.UsageAccountingRecord{
				State: "valid", AccountingVersion: &accountingVersion, SchemaVersion: &accountingVersion, Quality: &quality, TotalTokens: &canonicalTotal,
				Input:              dto.UsageTokenInput{TotalTokens: &canonicalInput, UncachedTokens: &canonicalUncached, CacheReadTokens: &canonicalZero, CacheWriteTokens: &canonicalOutput},
				Output:             dto.UsageTokenOutput{TotalTokens: &canonicalOutput, NonReasoningTokens: &canonicalNonReasoning, ReasoningTokens: &canonicalReasoning},
				UnclassifiedTokens: &canonicalZero,
			},
		},
		ID:                  42,
		Timestamp:           time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:               "claude-sonnet",
		ModelAlias:          "claude-sonnet-requested",
		Endpoint:            "/v1/messages?api_key=secret",
		RequestID:           "request-42",
		StatusCode:          &statusCode,
		ExecutorType:        "openai",
		ReasoningEffort:     "high",
		ServiceTier:         "priority",
		AuthType:            "apikey",
		Provider:            "OpenAI Mirror",
		Source:              "sk-provider-key",
		AuthIndex:           "2",
		Failed:              false,
		LatencyMS:           21245,
		TTFTMS:              &ttftMS,
		OutputTPS:           &staleOutputTPS,
		InputTokens:         10,
		OutputTokens:        976,
		ReasoningTokens:     2,
		CachedTokens:        1,
		CacheReadTokens:     &cacheReadTokens,
		CacheCreationTokens: &cacheCreationTokens,
		TotalTokens:         105091,
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?range=24h", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"events":[`) || !contains(body, `"model":"claude-sonnet"`) {
		t.Fatalf("unexpected response body: %s", body)
	}
	if !contains(body, `"id":42`) || !contains(body, `"total_count":1`) || !contains(body, `"page":1`) || !contains(body, `"page_size":100`) || !contains(body, `"total_pages":1`) {
		t.Fatalf("expected pagination metadata and event id in response body: %s", body)
	}
	if !contains(body, `"source":"OpenAI Mirror"`) {
		t.Fatalf("expected resolved source display in response body: %s", body)
	}
	if contains(body, `sk-provider-key`) || contains(body, `sk-provider-prefix`) {
		t.Fatalf("expected raw source values to be redacted from response body: %s", body)
	}
	if contains(body, `"source_type"`) || contains(body, `"source_key"`) {
		t.Fatalf("expected source metadata fields to stay omitted, got %s", body)
	}
	if !contains(body, `"auth_index":"2"`) {
		t.Fatalf("expected auth index in response body: %s", body)
	}
	if !contains(body, `"ttft_ms":1052`) || !contains(body, `"output_tps":48.33358094488189`) {
		t.Fatalf("expected TTFT and Output TPS in response body: %s", body)
	}
	if contains(body, `"output_tps":999`) {
		t.Fatalf("expected HTTP projection to use AttemptFacts Output TPS, got %s", body)
	}
	for _, fact := range []string{
		`"attempt_facts":{`, `"generate":true`, `"stream":false`, `"request_service_tier":"priority"`, `"response_service_tier":"default"`,
		`"accounting":{"state":"valid"`, `"accounting_version":2`, `"schema_version":2`, `"quality":"complete"`,
		`"input":{"total_tokens":10,"uncached_tokens":6,"cache_read_tokens":0,"cache_write_tokens":4}`,
		`"output":{"total_tokens":4,"non_reasoning_tokens":2,"reasoning_tokens":2}`, `"unclassified_tokens":0`,
	} {
		if !contains(body, fact) {
			t.Fatalf("expected accounting attempt fact %s in response body: %s", fact, body)
		}
	}
	if !contains(body, `"model_alias":"claude-sonnet-requested"`) || !contains(body, `"endpoint":"/v1/messages"`) || !contains(body, `"request_id":"request-42"`) {
		t.Fatalf("expected requested model, endpoint, and request trace in response body: %s", body)
	}
	if contains(body, "api_key=secret") {
		t.Fatalf("expected endpoint query to stay hidden: %s", body)
	}
	for _, token := range []string{`"input_tokens":10`, `"output_tokens":976`, `"reasoning_tokens":2`, `"cached_tokens":1`, `"total_tokens":105091`} {
		if !contains(body, token) {
			t.Fatalf("expected complete token evidence %s in response body: %s", token, body)
		}
	}
	for _, evidence := range []string{`"status_code":200`, `"executor_type":"openai"`, `"reasoning_effort":"high"`, `"service_tier":"priority"`, `"cache_read_tokens":1`, `"cache_creation_tokens":3`} {
		if !contains(body, evidence) {
			t.Fatalf("expected attempt evidence %s in response body: %s", evidence, body)
		}
	}
	if provider.filterCalls != 1 {
		t.Fatalf("expected ListUsageEvents to be called once, got %d", provider.filterCalls)
	}
	// 事件列表查询只下传解析后的时间窗口，不携带 Range 标签。
	if provider.lastFilter.Page != 1 || provider.lastFilter.PageSize != 100 || provider.lastFilter.Offset != 0 {
		t.Fatalf("expected default pagination to be passed through, got %+v", provider.lastFilter)
	}
	if provider.lastFilter.StartTime == nil || provider.lastFilter.EndTime == nil {
		t.Fatalf("expected resolved time bounds in filter, got %+v", provider.lastFilter)
	}
}

func TestUsageEventsReturnsExactCorrelationWindowAndNormalizedRequestIDFilter(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID: 7, Timestamp: time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC), RequestID: "request-42",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	windowEnd := "2026-09-07T12:00:00.123456789Z"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?range=24h&provider=claude&request_id=%20request-42%20&window_end="+windowEnd, nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.lastFilter.RequestID != "request-42" || provider.lastFilter.Provider != "claude" || provider.lastFilter.StartTime == nil || provider.lastFilter.EndTime == nil {
		t.Fatalf("expected normalized bounded correlation filter, got %+v", provider.lastFilter)
	}
	fragment := `"window_end":"2026-09-07T12:00:00.123456789Z"`
	if !contains(resp.Body.String(), fragment) {
		t.Fatalf("expected exact correlation window end %s in response: %s", fragment, resp.Body.String())
	}
}

func TestUsageEventsReturnsUnavailableOutputTPSAsNull(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		AttemptFacts: dto.UsageAttemptFacts{Accounting: dto.UsageAccountingRecord{State: "absent"}},
		ID:           43,
		Timestamp:    time.Date(2026, 4, 22, 11, 1, 0, 0, time.UTC),
		Model:        "historical-model",
		LatencyMS:    21245,
		OutputTokens: 976,
		TotalTokens:  105091,
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"ttft_ms":null`) || !contains(body, `"output_tps":null`) {
		t.Fatalf("expected missing TTFT and Output TPS to remain null: %s", body)
	}
	if !contains(body, `"attempt_facts":{"generate":null,"stream":null,"request_service_tier":null,"response_service_tier":null,"output_tps":null,"accounting":{"state":"absent"`) ||
		!contains(body, `"accounting_version":null`) || !contains(body, `"schema_version":null`) || !contains(body, `"quality":null`) || !contains(body, `"unclassified_tokens":null`) {
		t.Fatalf("expected absent attempt facts to remain explicit nulls: %s", body)
	}
	if !contains(body, `"tokens":{"input_tokens":0,"output_tokens":976,"reasoning_tokens":0,"cached_tokens":0,"total_tokens":105091}`) {
		t.Fatalf("expected missing legacy exact-cache facts to stay omitted from tokens: %s", body)
	}
}

// usage identity 展示名规则的行为测试已随 DisplayName 收拢到 internal/entities 包。

func TestUsageEventsResponseDoesNotExposeSourceKey(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        48,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "Fallback Provider",
		AuthIndex: "provider-auth-index",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:           12,
		Name:         "Provider Name",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "provider-auth-index",
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if contains(body, `"source_key"`) {
		t.Fatalf("expected source_key to be removed from usage event response, got %s", body)
	}
}

func TestUsageEventPublicSourcePreservesUnknownTransportProjection(t *testing.T) {
	source, isDelete := usageEventPublicSource(dto.UsageEventRecord{
		AuthType:  "future_auth",
		Provider:  "Future Provider",
		AuthIndex: "future-index",
	}, resolvedUsageIdentity{}, false)
	if source != "Future Provider" || !isDelete {
		t.Fatalf("expected unknown transport projection to preserve provider and deleted marker, got source=%q isDelete=%t", source, isDelete)
	}
}

func TestUsageEventsIncludesAPIKeyAliasAndMaskedKey(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:             49,
		Timestamp:      time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:          "claude-sonnet",
		AuthType:       "apikey",
		Provider:       "Fallback Provider",
		Source:         "sk-live-secret-value",
		AuthIndex:      "provider-auth-index",
		APIKeyIdentity: "sk-live-secret-value",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{
		KeyAlias: &keyAliasStub{apiKeyAliases: map[string]string{"sk-live-secret-value": "Agent API Key"}},
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"api_key_alias":"Agent API Key"`) {
		t.Fatalf("expected api key alias in response body: %s", body)
	}
	if !contains(body, `"api_key_display":"`+redact.APIKeyDisplayName("sk-live-secret-value")+`"`) {
		t.Fatalf("expected masked api key display in response body: %s", body)
	}
	if contains(body, `sk-live-secret-value`) {
		t.Fatalf("expected raw api key to stay hidden, got %s", body)
	}
}

func TestUsageEventsResolvesAPIKeySourceFromProviderIdentity(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        44,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "Fallback Provider",
		Source:    "sk-provider-key",
		AuthIndex: "provider-auth-index",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:            12,
		Name:          "Provider Name",
		Prefix:        "Team Prefix",
		AuthType:      entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName:  "apikey",
		Identity:      "provider-auth-index",
		Type:          "openai",
		Provider:      "Provider",
		TotalRequests: 1,
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"source":"Provider Name(Team Prefix)"`) {
		t.Fatalf("expected source to use provider identity displayName, got %s", body)
	}
	if !contains(body, `"source_type":"openai"`) {
		t.Fatalf("expected source_type to use provider identity type, got %s", body)
	}
	if contains(body, `"source_key"`) {
		t.Fatalf("expected source_key to stay omitted, got %s", body)
	}
	if contains(body, `Fallback Provider`) || contains(body, `sk-provider-key`) {
		t.Fatalf("expected fallback and raw source to be hidden, got %s", body)
	}
}

func TestUsageEventsDoesNotResolveProviderIdentityFromSource(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        45,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "Fallback Provider",
		Source:    "provider-auth-index",
		AuthIndex: "missing-auth-index",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:            12,
		Name:          "Provider Name",
		Prefix:        "Team Prefix",
		AuthType:      entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName:  "apikey",
		Identity:      "provider-auth-index",
		Type:          "openai",
		Provider:      "Provider",
		TotalRequests: 1,
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if contains(body, `"source":"Provider Name(Team Prefix)"`) || contains(body, `"source_key"`) {
		t.Fatalf("expected event source not to resolve identity through usage event source, got %s", body)
	}
	if !contains(body, `"source":"Fallback Provider"`) {
		t.Fatalf("expected auth_index fallback when identity is missing, got %s", body)
	}
}

func TestUsageEventsMarksRowDeletedWhenAuthIndexHasNoIdentity(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        46,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "Fallback Provider",
		AuthIndex: "missing-auth-index",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:           12,
		Name:         "Provider Name",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "other-auth-index",
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"isDelete":true`) {
		t.Fatalf("expected missing identity row to be marked deleted, got %s", body)
	}
}

func TestUsageEventsDoesNotMarkRowDeletedWhenAuthIndexMatchesIdentity(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        47,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "Fallback Provider",
		AuthIndex: "provider-auth-index",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:           12,
		Name:         "Provider Name",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "provider-auth-index",
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if contains(body, `"isDelete":true`) {
		t.Fatalf("expected matched identity row not to be marked deleted, got %s", body)
	}
}

func TestUsageEventsKeepsFallbackSourceWhenAuthIndexIsMissing(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID:        43,
		Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC),
		Model:     "claude-sonnet",
		AuthType:  "apikey",
		Provider:  "OpenAI Mirror",
		Source:    "sk-provider-key",
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if !contains(body, `"source":"OpenAI Mirror"`) || contains(body, `"source_key"`) {
		t.Fatalf("expected provider source fallback without source_key, got %s", body)
	}
}

func TestUsageEventsPassesPaginationAndAuthIndexSourceFilter(t *testing.T) {
	provider := &usageEventsStub{eventsPage: &dto.UsageEventsPageRecord{Events: []dto.UsageEventRecord{}, TotalCount: 0, Page: 3, PageSize: 100, TotalPages: 0}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?page=3&page_size=100&model=claude-sonnet&source=authidx-openai-main&result=failed", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if provider.lastFilter.Page != 3 || provider.lastFilter.PageSize != 100 || provider.lastFilter.Offset != 200 {
		t.Fatalf("expected pagination filter, got %+v", provider.lastFilter)
	}
	if provider.lastFilter.Model != "claude-sonnet" || provider.lastFilter.AuthIndex != "authidx-openai-main" || provider.lastFilter.Source != "" || provider.lastFilter.Result != "failed" {
		t.Fatalf("expected source filter to be translated to auth_index only, got %+v", provider.lastFilter)
	}
	body := resp.Body.String()
	if !contains(body, `"page":1`) || !contains(body, `"page_size":100`) || !contains(body, `"total_count":0`) || !contains(body, `"total_pages":1`) {
		t.Fatalf("expected normalized empty response pagination metadata, got %s", body)
	}
}

func TestUsageEventsAcceptsCompactEvidencePageSize(t *testing.T) {
	provider := &usageEventsStub{eventsPage: &dto.UsageEventsPageRecord{Events: []dto.UsageEventRecord{}, TotalCount: 0, Page: 1, PageSize: 10, TotalPages: 0}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?range=24h&page_size=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.lastFilter.Page != 1 || provider.lastFilter.PageSize != 10 || provider.lastFilter.Offset != 0 {
		t.Fatalf("expected compact evidence pagination, got %+v", provider.lastFilter)
	}
}

func TestUsageEventsAcceptsLatestEvidencePageSize(t *testing.T) {
	provider := &usageEventsStub{eventsPage: &dto.UsageEventsPageRecord{Events: []dto.UsageEventRecord{}, TotalCount: 0, Page: 1, PageSize: 1, TotalPages: 0}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?range=24h&page_size=1", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.lastFilter.Page != 1 || provider.lastFilter.PageSize != 1 || provider.lastFilter.Offset != 0 {
		t.Fatalf("expected latest evidence pagination, got %+v", provider.lastFilter)
	}
}

func TestUsageEventsPassesAuthFileIdentitySourceFilterAsAuthIndex(t *testing.T) {
	provider := &usageEventsStub{eventsPage: &dto.UsageEventsPageRecord{Events: []dto.UsageEventRecord{}, TotalCount: 0, Page: 1, PageSize: 100, TotalPages: 0}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events?source=auth-file-index", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if provider.lastFilter.AuthIndex != "auth-file-index" || provider.lastFilter.Source != "" {
		t.Fatalf("expected auth file identity source filter to use auth_index only, got %+v", provider.lastFilter)
	}
}

func TestUsageEventsDoesNotReturnFilterOptions(t *testing.T) {
	provider := &usageEventsStub{eventsPage: &dto.UsageEventsPageRecord{
		Events: []dto.UsageEventRecord{{
			ID: 7, Timestamp: time.Date(2026, 4, 22, 11, 0, 0, 0, time.UTC), Model: "gpt-5", AuthType: "apikey", Provider: "Provider A", Source: "source-a", Failed: true,
		}},
		TotalCount: 2, Page: 1, PageSize: 20, TotalPages: 1,
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	body := resp.Body.String()
	if contains(body, `"models":`) || contains(body, `"sources":`) {
		t.Fatalf("expected events response to omit filter options, got %s", body)
	}
}

func TestUsageEventModelFilterOptionsReturnsStableModels(t *testing.T) {
	provider := &usageEventsStub{eventFilterOptions: &dto.UsageEventFilterOptionsRecord{
		Models: []string{"claude-sonnet", "gpt-5"},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/filters/models?range=24h&provider=OpenAI&model=ignored&source=ignored&result=failed&page=3&page_size=20", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if provider.filterOptionCalls != 1 || provider.filterCalls != 0 {
		t.Fatalf("expected model filter options endpoint only, events=%d filterOptions=%d", provider.filterCalls, provider.filterOptionCalls)
	}
	if provider.lastOptionsFilter.StartTime == nil || provider.lastOptionsFilter.EndTime == nil || provider.lastOptionsFilter.Provider != "OpenAI" {
		t.Fatalf("expected model filters endpoint to preserve the selected time scope, got %+v", provider.lastOptionsFilter)
	}
	body := resp.Body.String()
	if body != `{"models":["claude-sonnet","gpt-5"]}` {
		t.Fatalf("expected stable model filter options, got %s", body)
	}
}

func TestUsageEventSourceFilterOptionsReturnsIdentitySources(t *testing.T) {
	provider := &usageEventsStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{ID: 1, Name: "Claude Main", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "authidx-source-a", Type: "openai", Provider: "Provider A", TotalRequests: 3}, {ID: 2, Name: "Provider A", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "authidx-source-b", Type: "openai", Provider: "Provider A"}, {ID: 3, Name: "Auth User", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-1", Type: "claude", Provider: "Claude", TotalRequests: 2}, {ID: 4, Name: "Zero Request User", AuthType: entities.UsageIdentityAuthTypeAuthFile, AuthTypeName: "oauth", Identity: "auth-zero", Type: "claude", Provider: "Claude"}, {ID: 5, Name: "Zero Provider", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "authidx-source-zero", Type: "openai", Provider: "Zero Provider"}, {ID: 6, Name: "Deleted Source", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: "authidx-deleted", Type: "openai", Provider: "Deleted Provider", TotalRequests: 5, IsDeleted: true}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/filters/sources?range=24h&model=ignored&source=ignored&result=failed&page=3&page_size=20", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", resp.Code)
	}
	if provider.filterOptionCalls != 0 || provider.filterCalls != 0 {
		t.Fatalf("expected source filter options endpoint to use identities only, events=%d filterOptions=%d", provider.filterCalls, provider.filterOptionCalls)
	}
	body := resp.Body.String()
	if !contains(body, `"sources":[`) || !contains(body, `"value":"authidx-source-a"`) || !contains(body, `"label":"Claude Main"`) || !contains(body, `"displayName":"Claude Main"`) || !contains(body, `"value":"auth-1"`) || !contains(body, `"label":"Auth User"`) {
		t.Fatalf("expected stable identity source filter options with display names, got %s", body)
	}
	if contains(body, `"models"`) {
		t.Fatalf("expected source filter options endpoint not to return models, got %s", body)
	}
	if contains(body, `"value":"auth:auth-1"`) || contains(body, `"value":"provider:Provider A"`) || contains(body, `"value":"provider:1"`) || contains(body, `"value":"provider:2"`) {
		t.Fatalf("expected source filter values without prefixes, got %s", body)
	}
	if contains(body, `Zero Request User`) || contains(body, `Zero Provider`) || contains(body, `auth-zero`) || contains(body, `authidx-source-zero`) {
		t.Fatalf("expected zero-request source filter options to be omitted, got %s", body)
	}
	if contains(body, `Deleted Source`) || contains(body, `Deleted Provider`) || contains(body, `authidx-deleted`) {
		t.Fatalf("expected deleted source filter options to be omitted, got %s", body)
	}
}
