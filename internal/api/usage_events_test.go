package api

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
)

func TestUsageEventsExportUsesTheExactSelectionAndSafeCSVProjection(t *testing.T) {
	statusCode := 429
	ttftMS := int64(12)
	outputTPS := 34.5
	canonicalInput, canonicalOutput, canonicalReasoning := int64(6), int64(3), int64(1)
	canonicalCacheRead, canonicalCacheWrite, canonicalUnclassified, canonicalTotal := int64(1), int64(2), int64(1), int64(10)
	provider := &usageEventsStub{events: []dto.UsageEventRecord{{
		ID: 99, Timestamp: time.Date(2026, 9, 7, 11, 59, 59, 0, time.UTC),
		Provider: "\t=provider", Source: "sk-live-secret-value", AuthType: "apikey", AuthIndex: "auth-1",
		APIKeyIdentity: "sk-live-secret-value", Model: "\n=model", ModelAlias: "@别名\"quoted", Endpoint: "/v1/messages?api_key=secret",
		RequestID: "\r+request", Failed: true, StatusCode: &statusCode, LatencyMS: 80, TTFTMS: &ttftMS,
		AttemptFacts: dto.UsageAttemptFacts{OutputTPS: &outputTPS, Accounting: dto.UsageAccountingRecord{
			State: "valid", TotalTokens: &canonicalTotal,
			Input:              dto.UsageTokenInput{TotalTokens: &canonicalInput, CacheReadTokens: &canonicalCacheRead, CacheWriteTokens: &canonicalCacheWrite},
			Output:             dto.UsageTokenOutput{TotalTokens: &canonicalOutput, ReasoningTokens: &canonicalReasoning},
			UnclassifiedTokens: &canonicalUnclassified,
		}}, InputTokens: 1, OutputTokens: 2, ReasoningTokens: 3,
		CachedTokens: 4, TotalTokens: 10,
	}}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{
		UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
			Name: "\t=account", AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "auth-1",
		}}},
		KeyAlias: &keyAliasStub{apiKeyAliases: map[string]string{"sk-live-secret-value": "=alias"}},
	})
	windowEnd := "2026-09-07T12:00:00.123456789Z"
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/export?range=24h&provider=claude&model=sonnet&model_alias=route-a&account=auth-1&endpoint=%2Fv1%2Fmessages&status=4xx&request_id=request-42&min_latency_ms=500&window_end="+windowEnd+"&result=failed", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	if contentType := resp.Header().Get("Content-Type"); !strings.HasPrefix(contentType, "text/csv") {
		t.Fatalf("expected CSV content type, got %q", contentType)
	}
	if disposition := resp.Header().Get("Content-Disposition"); disposition != `attachment; filename="request-evidence.csv"` {
		t.Fatalf("unexpected disposition %q", disposition)
	}
	if provider.lastFilter.Page != 1 || provider.lastFilter.PageSize != usageEventsCSVExportLimit+1 || provider.lastFilter.Offset != 0 {
		t.Fatalf("expected cap+1 export read, got %+v", provider.lastFilter)
	}
	if provider.lastFilter.Provider != "claude" || provider.lastFilter.Model != "sonnet" || provider.lastFilter.ModelAlias != "route-a" || provider.lastFilter.Account != "auth-1" || provider.lastFilter.Endpoint != "/v1/messages" || provider.lastFilter.Status != "4xx" || provider.lastFilter.RequestID != "request-42" || provider.lastFilter.MinLatencyMS == nil || *provider.lastFilter.MinLatencyMS != 500 || provider.lastFilter.Result != "failed" {
		t.Fatalf("expected complete normalized selection, got %+v", provider.lastFilter)
	}
	if provider.lastFilter.EndTime == nil || provider.lastFilter.EndTime.Format(time.RFC3339Nano) != windowEnd {
		t.Fatalf("expected exact RFC3339Nano window end, got %+v", provider.lastFilter.EndTime)
	}

	rows, err := csv.NewReader(strings.NewReader(resp.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 2 || strings.Join(rows[0], "\x00") != strings.Join(usageEventsCSVHeader, "\x00") {
		t.Fatalf("unexpected CSV rows: %#v", rows)
	}
	row := rows[1]
	if row[1] != "'=account" || row[2] != "'=alias" || row[4] != "'\n=model" || row[5] != "'@别名\"quoted" || row[7] != "'\r+request" {
		t.Fatalf("expected formula-safe values, got %#v", row)
	}
	if row[6] != "/v1/messages" || strings.Contains(resp.Body.String(), "sk-live-secret-value") || strings.Contains(resp.Body.String(), "api_key=secret") || strings.Contains(resp.Body.String(), "auth-1") || strings.Contains(resp.Body.String(), "provider") {
		t.Fatalf("expected only safe endpoint and no raw identity material, got %q", resp.Body.String())
	}
	if row[8] != "failed" || row[9] != "429" || row[10] != "80" || row[11] != "12" || row[12] != "34.5" ||
		strings.Join(row[13:20], ",") != "6,3,1,1,2,1,10" {
		t.Fatalf("unexpected CSV units or empty semantics: %#v", row)
	}
}

func TestUsageEventsExportRejectsOversizeAndNeverWritesCSV(t *testing.T) {
	events := make([]dto.UsageEventRecord, usageEventsCSVExportLimit+1)
	router := NewRouter(nil, nil, &usageEventsStub{events: events}, nil, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00.123456789Z", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnprocessableEntity || !contains(resp.Body.String(), "5000-row limit") || resp.Header().Get("Content-Disposition") != "" {
		t.Fatalf("expected explicit non-download oversize failure, got status=%d headers=%v body=%s", resp.Code, resp.Header(), resp.Body.String())
	}
}

func TestUsageEventsExportDoesNotUseUnresolvedSourceOrProviderFallbacks(t *testing.T) {
	provider := &usageEventsStub{events: []dto.UsageEventRecord{
		{ID: 1, Timestamp: time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC), Provider: "raw-provider-label", Source: "raw-provider-source", AuthType: "apikey", AuthIndex: "missing-provider", Model: "model"},
		{ID: 2, Timestamp: time.Date(2026, 9, 7, 11, 1, 0, 0, time.UTC), Provider: "raw-authfile-provider", Source: "raw-authfile-source", AuthType: "auth_file", AuthIndex: "missing-auth-file", Model: "model"},
		{ID: 3, Timestamp: time.Date(2026, 9, 7, 11, 2, 0, 0, time.UTC), Provider: "raw-oauth-provider", Source: "raw-oauth-source", AuthType: "oauth", AuthIndex: "missing-oauth", Model: "model"},
	}}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00Z", nil))
	if resp.Code != http.StatusOK {
		t.Fatalf("expected export success, got %d: %s", resp.Code, resp.Body.String())
	}
	rows, err := csv.NewReader(strings.NewReader(resp.Body.String())).ReadAll()
	if err != nil {
		t.Fatalf("parse CSV: %v", err)
	}
	if len(rows) != 4 || rows[1][1] != "" || rows[2][1] != "" || rows[3][1] != "" {
		t.Fatalf("expected unresolved account cells to be empty, got %#v", rows)
	}
	for _, raw := range []string{"raw-provider-label", "raw-provider-source", "raw-authfile-provider", "raw-authfile-source", "raw-oauth-provider", "raw-oauth-source", "missing-provider", "missing-auth-file", "missing-oauth"} {
		if strings.Contains(resp.Body.String(), raw) {
			t.Fatalf("expected unresolved transport data to stay out of CSV: %s", raw)
		}
	}
}

func TestUsageEventsExportRequiresFrozenSelectionAndRemainsProtected(t *testing.T) {
	provider := &usageEventsStub{}
	router := NewRouter(nil, nil, provider, nil, AuthConfig{Enabled: true}, nil, "", OptionalProviders{})
	unauthenticated := httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00.123456789Z", nil)
	unauthenticatedResponse := httptest.NewRecorder()
	router.ServeHTTP(unauthenticatedResponse, unauthenticated)
	if unauthenticatedResponse.Code != http.StatusUnauthorized || provider.filterCalls != 0 {
		t.Fatalf("expected protected export before any provider call, got status=%d calls=%d", unauthenticatedResponse.Code, provider.filterCalls)
	}

	router = NewRouter(nil, nil, provider, nil, AuthConfig{}, nil, "", OptionalProviders{})
	for _, path := range []string{
		"/api/v1/usage/events/export?range=24h",
		"/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00Z&page=2",
		"/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00Z&source=raw-source",
	} {
		resp := httptest.NewRecorder()
		router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, path, nil))
		if resp.Code != http.StatusBadRequest || resp.Header().Get("Content-Disposition") != "" {
			t.Fatalf("expected non-download invalid selection for %s, got status=%d headers=%v", path, resp.Code, resp.Header())
		}
	}
}

func TestUsageEventsExportDoesNotDownloadARepositoryFailure(t *testing.T) {
	router := NewRouter(nil, nil, &usageEventsStub{err: errors.New("db unavailable")}, nil, AuthConfig{}, nil, "", OptionalProviders{})
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, httptest.NewRequest(http.MethodGet, "/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00Z", nil))
	if resp.Code != http.StatusInternalServerError || resp.Header().Get("Content-Disposition") != "" {
		t.Fatalf("expected failure without a download header, got status=%d headers=%v", resp.Code, resp.Header())
	}
}

func BenchmarkEncodeUsageEventsCSVAtLimit(b *testing.B) {
	events := make([]usageEventPayload, usageEventsCSVExportLimit)
	for index := range events {
		events[index] = usageEventPayload{
			Timestamp: "2026-09-07T12:00:00Z", Source: "Account", APIKeyAlias: "Key Alias", APIKeyDisplay: "sk-a***********z",
			Model: "model", ModelAlias: "route", Endpoint: "/v1/messages", RequestID: "request", LatencyMS: 100,
			Tokens: usageEventTokenPayload{InputTokens: testInt64Pointer(1), OutputTokens: testInt64Pointer(2), TotalTokens: testInt64Pointer(3)},
		}
	}
	b.ReportMetric(float64(len(events)), "export_rows")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		body, err := encodeUsageEventsCSV(events)
		if err != nil {
			b.Fatalf("encode CSV: %v", err)
		}
		if len(body) == 0 {
			b.Fatal("expected CSV body")
		}
	}
}

func BenchmarkUsageEventsExportHandlerAtLimit(b *testing.B) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(b.TempDir(), "usage-export-benchmark.db")})
	if err != nil {
		b.Fatalf("open benchmark database: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		b.Fatalf("open benchmark sql database: %v", err)
	}
	b.Cleanup(func() { _ = sqlDB.Close() })

	const rowCount = usageEventsCSVExportLimit
	observedAt := time.Date(2026, 9, 7, 11, 0, 0, 0, time.UTC)
	events := make([]entities.UsageEvent, 0, rowCount)
	aliases := make([]entities.KeyAlias, 0, rowCount)
	for index := 0; index < rowCount; index++ {
		identity := fmt.Sprintf("sk-export-%04d", index)
		events = append(events, entities.UsageEvent{
			EventKey: fmt.Sprintf("export-event-%04d", index), Timestamp: observedAt, AuthType: entities.UsageIdentityAuthTypeNameAPIKey,
			Source: identity, AuthIndex: "benchmark-account", Model: "benchmark-model", InputTokens: 1, OutputTokens: 2, TotalTokens: 3,
		})
		aliases = append(aliases, entities.KeyAlias{
			AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: identity, Alias: fmt.Sprintf("Alias %04d", index), CreatedAt: observedAt, UpdatedAt: observedAt,
		})
	}
	if err := db.CreateInBatches(&events, 400).Error; err != nil {
		b.Fatalf("seed benchmark events: %v", err)
	}
	if err := db.CreateInBatches(&aliases, 400).Error; err != nil {
		b.Fatalf("seed benchmark aliases: %v", err)
	}
	if err := db.Create(&entities.UsageIdentity{
		Name: "Benchmark account", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: entities.UsageIdentityAuthTypeNameAPIKey,
		Identity: "benchmark-account", Provider: "Benchmark provider",
	}).Error; err != nil {
		b.Fatalf("seed benchmark usage identity: %v", err)
	}

	router := NewRouter(nil, nil, repository.NewUsageReader(db), nil, AuthConfig{}, nil, "", OptionalProviders{
		UsageIdentity: repository.NewUsageIdentityReader(db),
		KeyAlias:      service.NewKeyAliasService(db),
	})
	requestPath := "/api/v1/usage/events/export?range=24h&window_end=2026-09-07T12:00:00Z"
	warmResponse := httptest.NewRecorder()
	router.ServeHTTP(warmResponse, httptest.NewRequest(http.MethodGet, requestPath, nil))
	if warmResponse.Code != http.StatusOK || !strings.Contains(warmResponse.Body.String(), "Alias 0000") || strings.Contains(warmResponse.Body.String(), "sk-export-0000") {
		b.Fatalf("warm export did not exercise safe alias enrichment: status=%d bytes=%d", warmResponse.Code, warmResponse.Body.Len())
	}

	b.ReportMetric(rowCount, "export_rows")
	b.ReportMetric(rowCount, "distinct_api_keys")
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, requestPath, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			b.Fatalf("export handler failed: status=%d bytes=%d", response.Code, response.Body.Len())
		}
	}
}

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
	for _, token := range []string{`"input_tokens":10`, `"output_tokens":4`, `"reasoning_tokens":2`, `"cache_read_tokens":0`, `"cache_write_tokens":4`, `"unclassified_tokens":0`, `"total_tokens":14`} {
		if !contains(body, token) {
			t.Fatalf("expected complete token evidence %s in response body: %s", token, body)
		}
	}
	for _, evidence := range []string{`"status_code":200`, `"executor_type":"openai"`, `"reasoning_effort":"high"`, `"service_tier":"priority"`} {
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
	if !contains(body, `"tokens":{"input_tokens":null,"output_tokens":null,"reasoning_tokens":null,"cache_read_tokens":null,"cache_write_tokens":null,"unclassified_tokens":null,"total_tokens":null}`) {
		t.Fatalf("expected historical scalar tokens to remain unavailable: %s", body)
	}
}

func testInt64Pointer(value int64) *int64 { return &value }

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
