package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/service"
)

type usageIdentitiesStub struct {
	items            []entities.UsageIdentity
	activeItems      []entities.UsageIdentity
	pagedActiveItems []entities.UsageIdentity
	pagedActiveTotal int64
	pagedActiveReq   *service.ListUsageIdentitiesRequest
	err              error
}

func (s usageIdentitiesStub) ListUsageIdentities(context.Context) ([]entities.UsageIdentity, error) {
	return s.items, s.err
}

func (s usageIdentitiesStub) ListActiveUsageIdentities(context.Context) ([]entities.UsageIdentity, error) {
	if s.activeItems != nil {
		return s.activeItems, s.err
	}
	return s.items, s.err
}

func (s usageIdentitiesStub) ListActiveUsageIdentitiesPage(_ context.Context, request service.ListUsageIdentitiesRequest) (service.ListUsageIdentitiesResponse, error) {
	if s.pagedActiveReq != nil {
		*s.pagedActiveReq = request
	}
	if s.pagedActiveItems != nil || s.pagedActiveTotal != 0 {
		return service.ListUsageIdentitiesResponse{Items: s.pagedActiveItems, Total: s.pagedActiveTotal}, s.err
	}
	return service.ListUsageIdentitiesResponse{Items: s.items, Total: int64(len(s.items))}, s.err
}

type keyAliasStub struct {
	aliases        map[service.UsageIdentityAliasKey]string
	apiKeyItems    []service.APIKeyAliasTarget
	apiKeyTotal    int64
	apiKeyPageReq  *service.ListAPIKeyAliasTargetsRequest
	setID          uint
	setAlias       string
	clearID        uint
	getID          uint
	setAPIKeyID    string
	setAPIKeyAlias string
	clearAPIKeyID  string
	err            error
	invalidErr     error
}

func (s *keyAliasStub) ListAliasesForUsageIdentities(_ context.Context, identities []entities.UsageIdentity) (map[service.UsageIdentityAliasKey]string, error) {
	result := map[service.UsageIdentityAliasKey]string{}
	for _, identity := range identities {
		key := service.UsageIdentityAliasKey{AuthType: identity.AuthType, Identity: identity.Identity}
		if alias, ok := s.aliases[key]; ok {
			result[key] = alias
		}
	}
	return result, s.err
}

func (s *keyAliasStub) GetUsageIdentityAlias(context.Context, uint) (string, error) {
	return "Agent Research", s.err
}

func (s *keyAliasStub) ListAPIKeyAliasTargetsPage(_ context.Context, request service.ListAPIKeyAliasTargetsRequest) (service.ListAPIKeyAliasTargetsResponse, error) {
	if s.apiKeyPageReq != nil {
		*s.apiKeyPageReq = request
	}
	total := s.apiKeyTotal
	if total == 0 {
		total = int64(len(s.apiKeyItems))
	}
	return service.ListAPIKeyAliasTargetsResponse{Items: s.apiKeyItems, Total: total}, s.err
}

func (s *keyAliasStub) SetUsageIdentityAlias(_ context.Context, id uint, alias string) (string, error) {
	s.setID = id
	s.setAlias = alias
	if s.invalidErr != nil {
		return "", s.invalidErr
	}
	return strings.TrimSpace(alias), s.err
}

func (s *keyAliasStub) ClearUsageIdentityAlias(_ context.Context, id uint) error {
	s.clearID = id
	return s.err
}

func (s *keyAliasStub) SetAPIKeyAlias(_ context.Context, apiKeyID string, alias string) (string, error) {
	s.setAPIKeyID = apiKeyID
	s.setAPIKeyAlias = alias
	if s.invalidErr != nil {
		return "", s.invalidErr
	}
	return strings.TrimSpace(alias), s.err
}

func (s *keyAliasStub) ClearAPIKeyAlias(_ context.Context, apiKeyID string) error {
	s.clearAPIKeyID = apiKeyID
	return s.err
}

func TestUsageIdentitiesRouteReturnsMetadataStatsAndActiveRows(t *testing.T) {
	firstUsedAt := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	lastUsedAt := time.Date(2026, 5, 4, 9, 0, 0, 0, time.UTC)
	statsUpdatedAt := time.Date(2026, 5, 4, 10, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 3, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 4, 10, 30, 0, 0, time.UTC)
	deletedAt := time.Date(2026, 5, 4, 11, 0, 0, 0, time.UTC)

	activeIdentity := entities.UsageIdentity{
		ID:                         1,
		Name:                       "Claude Desktop",
		AuthType:                   entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName:               "oauth",
		Identity:                   "2",
		Type:                       "auth-file",
		Provider:                   "anthropic",
		TotalRequests:              10,
		SuccessCount:               8,
		FailureCount:               2,
		InputTokens:                100,
		OutputTokens:               200,
		ReasoningTokens:            30,
		CachedTokens:               40,
		TotalTokens:                370,
		LastAggregatedUsageEventID: 99,
		FirstUsedAt:                &firstUsedAt,
		LastUsedAt:                 &lastUsedAt,
		StatsUpdatedAt:             &statsUpdatedAt,
		CreatedAt:                  createdAt,
		UpdatedAt:                  updatedAt,
	}
	deletedIdentity := entities.UsageIdentity{
		ID:           2,
		Name:         "Deleted Provider",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "sk-deleted-provider-secret",
		Type:         "openai",
		Provider:     "OpenAI",
		IsDeleted:    true,
		DeletedAt:    &deletedAt,
		CreatedAt:    createdAt,
		UpdatedAt:    updatedAt,
	}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{
		items:       []entities.UsageIdentity{activeIdentity, deletedIdentity},
		activeItems: []entities.UsageIdentity{activeIdentity},
	}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"identities":[`) || !contains(body, `"id":1`) || !contains(body, `"identity":"2"`) {
		t.Fatalf("expected auth file identity row in response, got %s", body)
	}
	if contains(body, "Deleted Provider") || contains(body, "sk-deleted-provider-secret") || contains(body, `"deleted_at"`) {
		t.Fatalf("expected deleted identities to be filtered from response, got %s", body)
	}
	for _, expected := range []string{
		`"name":"Claude Desktop"`,
		`"auth_type":1`,
		`"auth_type_name":"oauth"`,
		`"type":"auth-file"`,
		`"provider":"anthropic"`,
		`"total_requests":10`,
		`"success_count":8`,
		`"failure_count":2`,
		`"input_tokens":100`,
		`"output_tokens":200`,
		`"reasoning_tokens":30`,
		`"cached_tokens":40`,
		`"total_tokens":370`,
		`"last_aggregated_usage_event_id":99`,
		`"first_used_at":"2026-05-04T08:00:00Z"`,
		`"last_used_at":"2026-05-04T09:00:00Z"`,
		`"stats_updated_at":"2026-05-04T10:00:00Z"`,
		`"is_deleted":false`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected %s in response body: %s", expected, body)
		}
	}
}

func TestUsageIdentitiesPageRouteIncludesLocalAlias(t *testing.T) {
	identity := entities.UsageIdentity{
		ID:            42,
		Name:          "Provider Name",
		AuthType:      entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName:  "apikey",
		Identity:      "sk-live-secret-value",
		Type:          "openai",
		Provider:      "OpenAI",
		TotalCost:     18.45,
		CostAvailable: true,
	}
	aliasProvider := &keyAliasStub{aliases: map[service.UsageIdentityAliasKey]string{
		{AuthType: identity.AuthType, Identity: identity.Identity}: "Agent Research",
	}}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{
		UsageIdentity: usageIdentitiesStub{pagedActiveItems: []entities.UsageIdentity{identity}, pagedActiveTotal: 1},
		KeyAlias:      aliasProvider,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities/page?page=1&page_size=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"alias":"Agent Research"`) {
		t.Fatalf("expected alias in response body: %s", body)
	}
	if !contains(body, `"total_cost":18.45`) || !contains(body, `"cost_available":true`) {
		t.Fatalf("expected cost fields in response body: %s", body)
	}
}

func TestUsageIdentitiesPageRouteMatchesContractFixture(t *testing.T) {
	firstUsedAt := time.Date(2026, 5, 18, 7, 0, 0, 0, time.UTC)
	lastUsedAt := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	statsUpdatedAt := time.Date(2026, 5, 18, 9, 0, 0, 0, time.UTC)
	createdAt := time.Date(2026, 5, 17, 8, 0, 0, 0, time.UTC)
	updatedAt := time.Date(2026, 5, 18, 9, 30, 0, 0, time.UTC)
	identity := entities.UsageIdentity{
		ID:                         42,
		Name:                       "Provider Name",
		AuthType:                   entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName:               "apikey",
		Identity:                   "sk-live-secret-value",
		Type:                       "openai",
		Provider:                   "OpenAI",
		TotalRequests:              3,
		SuccessCount:               2,
		FailureCount:               1,
		InputTokens:                10,
		OutputTokens:               5,
		CachedTokens:               2,
		TotalTokens:                17,
		TotalCost:                  1.5,
		CostAvailable:              true,
		LastAggregatedUsageEventID: 99,
		FirstUsedAt:                &firstUsedAt,
		LastUsedAt:                 &lastUsedAt,
		StatsUpdatedAt:             &statsUpdatedAt,
		CreatedAt:                  createdAt,
		UpdatedAt:                  updatedAt,
	}
	aliasProvider := &keyAliasStub{aliases: map[service.UsageIdentityAliasKey]string{
		{AuthType: identity.AuthType, Identity: identity.Identity}: "Agent Research",
	}}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{
		UsageIdentity: usageIdentitiesStub{pagedActiveItems: []entities.UsageIdentity{identity}, pagedActiveTotal: 1},
		KeyAlias:      aliasProvider,
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities/page?page=1&page_size=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	assertJSONMatchesContractFixture(t, resp.Body.Bytes(), "usage_identities_page.json")
}

func TestUsageIdentityAliasRoutesSetReadAndClear(t *testing.T) {
	aliasProvider := &keyAliasStub{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/usage/identities/42/alias", strings.NewReader(`{"alias":" Agent Research "}`))
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected PUT status 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if aliasProvider.setID != 42 || aliasProvider.setAlias != " Agent Research " || !contains(putResp.Body.String(), `"alias":"Agent Research"`) {
		t.Fatalf("expected set alias call and response, got id=%d alias=%q body=%s", aliasProvider.setID, aliasProvider.setAlias, putResp.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities/42/alias", nil)
	getResp := httptest.NewRecorder()
	router.ServeHTTP(getResp, getReq)
	if getResp.Code != http.StatusOK || !contains(getResp.Body.String(), `"alias":"Agent Research"`) {
		t.Fatalf("expected GET alias response, got %d: %s", getResp.Code, getResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/usage/identities/42/alias", nil)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected DELETE status 204, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	if aliasProvider.clearID != 42 {
		t.Fatalf("expected clear alias for identity 42, got %d", aliasProvider.clearID)
	}
}

func TestUsageAPIKeyAliasRoutesListSetAndClearByOpaqueID(t *testing.T) {
	lastUsedAt := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	apiKeyID := redact.APIAlias("sk-live-secret-value")
	request := service.ListAPIKeyAliasTargetsRequest{}
	aliasProvider := &keyAliasStub{
		apiKeyPageReq: &request,
		apiKeyItems: []service.APIKeyAliasTarget{{
			ID:            apiKeyID,
			Identity:      redact.APIKeyDisplayName("sk-live-secret-value"),
			Alias:         "Agent API Key",
			Provider:      "OpenAI",
			TotalRequests: 3,
			TotalTokens:   1234,
			TotalCost:     1.25,
			CostAvailable: true,
			CostStatus:    "available",
			LastUsedAt:    &lastUsedAt,
		}},
		apiKeyTotal: 1,
	}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})

	listReq := httptest.NewRequest(http.MethodGet, "/api/v1/usage/api-keys/page?page=2&page_size=10", nil)
	listResp := httptest.NewRecorder()
	router.ServeHTTP(listResp, listReq)
	if listResp.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", listResp.Code, listResp.Body.String())
	}
	body := listResp.Body.String()
	if request.Page != 2 || request.PageSize != 10 {
		t.Fatalf("expected page request to be passed through, got %+v", request)
	}
	for _, expected := range []string{
		`"api_keys":[`,
		`"id":"` + apiKeyID + `"`,
		`"identity":"sk-l************alue"`,
		`"displayName":"Agent API Key"`,
		`"alias":"Agent API Key"`,
		`"total_tokens":1234`,
		`"total_cost":1.25`,
		`"last_used_at":"2026-05-18T08:00:00Z"`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected response to contain %s, got %s", expected, body)
		}
	}
	if contains(body, "sk-live-secret-value") {
		t.Fatalf("expected raw api key not to be exposed, got %s", body)
	}

	putReq := httptest.NewRequest(http.MethodPut, "/api/v1/usage/api-keys/"+apiKeyID+"/alias", strings.NewReader(`{"alias":" Raw Alias "}`))
	putReq.Header.Set("Content-Type", "application/json")
	putResp := httptest.NewRecorder()
	router.ServeHTTP(putResp, putReq)
	if putResp.Code != http.StatusOK {
		t.Fatalf("expected PUT status 200, got %d: %s", putResp.Code, putResp.Body.String())
	}
	if aliasProvider.setAPIKeyID != apiKeyID || aliasProvider.setAPIKeyAlias != " Raw Alias " || !contains(putResp.Body.String(), `"alias":"Raw Alias"`) {
		t.Fatalf("expected api key alias set, got id=%q alias=%q body=%s", aliasProvider.setAPIKeyID, aliasProvider.setAPIKeyAlias, putResp.Body.String())
	}

	deleteReq := httptest.NewRequest(http.MethodDelete, "/api/v1/usage/api-keys/"+apiKeyID+"/alias", nil)
	deleteResp := httptest.NewRecorder()
	router.ServeHTTP(deleteResp, deleteReq)
	if deleteResp.Code != http.StatusNoContent {
		t.Fatalf("expected DELETE status 204, got %d: %s", deleteResp.Code, deleteResp.Body.String())
	}
	if aliasProvider.clearAPIKeyID != apiKeyID {
		t.Fatalf("expected api key alias clear for %q, got %q", apiKeyID, aliasProvider.clearAPIKeyID)
	}
}

func TestUsageAPIKeyAliasPageRouteMatchesContractFixture(t *testing.T) {
	firstUsedAt := time.Date(2026, 5, 18, 7, 0, 0, 0, time.UTC)
	lastUsedAt := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	apiKeyID := redact.APIAlias("sk-live-secret-value")
	aliasProvider := &keyAliasStub{
		apiKeyItems: []service.APIKeyAliasTarget{{
			ID:            apiKeyID,
			Identity:      redact.APIKeyDisplayName("sk-live-secret-value"),
			Alias:         "Agent API Key",
			Provider:      "OpenAI",
			TotalRequests: 3,
			SuccessCount:  2,
			FailureCount:  1,
			InputTokens:   10,
			OutputTokens:  5,
			CachedTokens:  2,
			TotalTokens:   17,
			TotalCost:     1.5,
			CostAvailable: true,
			CostStatus:    "available",
			FirstUsedAt:   &firstUsedAt,
			LastUsedAt:    &lastUsedAt,
		}},
		apiKeyTotal: 1,
	}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/api-keys/page?page=1&page_size=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected list status 200, got %d: %s", resp.Code, resp.Body.String())
	}
	assertJSONMatchesContractFixture(t, resp.Body.Bytes(), "api_key_alias_targets_page.json")
}

func TestUsageIdentityAliasRouteValidatesAliasLength(t *testing.T) {
	aliasProvider := &keyAliasStub{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/usage/identities/42/alias", strings.NewReader(`{"alias":"`+strings.Repeat("a", 81)+`"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
	if aliasProvider.setID != 0 {
		t.Fatalf("expected invalid alias not to reach provider, got set id %d", aliasProvider.setID)
	}
}

func TestUsageIdentityAliasRouteMapsValidationErrors(t *testing.T) {
	aliasProvider := &keyAliasStub{invalidErr: service.ErrInvalidKeyAlias}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/usage/identities/42/alias", strings.NewReader(`{"alias":"Alias"}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsageIdentityAliasRouteMapsUnexpectedErrors(t *testing.T) {
	aliasProvider := &keyAliasStub{err: errors.New("boom")}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{KeyAlias: aliasProvider})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/usage/identities/42/alias", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d: %s", resp.Code, resp.Body.String())
	}
}

func TestUsageIdentitiesRouteDoesNotReturnUnpublishedMetadataFields(t *testing.T) {
	activeStart := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	activeUntil := time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC)
	accountID := "acct_123"
	planType := "team"
	baseURL := "https://api.openai.com/v1"
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:           1,
		Name:         "Codex Account",
		AuthType:     entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName: "oauth",
		Identity:     "codex-auth",
		Type:         "codex",
		Provider:     "Codex",
		Prefix:       "codex-prefix",
		BaseURL:      baseURL,
		AccountID:    &accountID,
		ActiveStart:  &activeStart,
		ActiveUntil:  &activeUntil,
		PlanType:     &planType,
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	for _, expected := range []string{
		`"plan_type":"team"`,
		`"active_start":"2026-05-01T00:00:00Z"`,
		`"active_until":"2026-06-01T00:00:00Z"`,
	} {
		if !contains(body, expected) {
			t.Fatalf("expected API response to include %s, got %s", expected, body)
		}
	}
	for _, forbidden := range []string{
		`"prefix"`,
		`"base_url"`,
		`"account_id"`,
	} {
		if contains(body, forbidden) {
			t.Fatalf("expected API response not to include %s, got %s", forbidden, body)
		}
	}
}

func TestUsageIdentitiesPageRouteFiltersByAuthTypeAndPaginates(t *testing.T) {
	captured := service.ListUsageIdentitiesRequest{}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{
		pagedActiveReq:   &captured,
		pagedActiveTotal: 25,
		pagedActiveItems: []entities.UsageIdentity{{
			ID:           11,
			Name:         "Codex Account",
			AuthType:     entities.UsageIdentityAuthTypeAuthFile,
			AuthTypeName: "oauth",
			Identity:     "codex-auth",
			Type:         "codex",
			Provider:     "Codex",
		}},
	}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities/page?auth_type=1&page=2&page_size=10", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if captured.AuthType == nil || *captured.AuthType != entities.UsageIdentityAuthTypeAuthFile || captured.Page != 2 || captured.PageSize != 10 {
		t.Fatalf("expected auth_type/page/page_size request, got %+v", captured)
	}
	for _, expected := range []string{`"identities":[`, `"id":11`, `"total_count":25`, `"page":2`, `"page_size":10`, `"total_pages":3`} {
		if !contains(body, expected) {
			t.Fatalf("expected %s in response body: %s", expected, body)
		}
	}
}

func TestUsageIdentitiesRouteReturnsProviderDisplayName(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{{
		ID:           1,
		Name:         "Provider Name",
		Prefix:       "Team Prefix",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "provider-auth-index",
		Type:         "openai",
		Provider:     "OpenAI",
	}}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if !contains(body, `"displayName":"Provider Name(Team Prefix)"`) {
		t.Fatalf("expected displayName with name and prefix, got %s", body)
	}
	if contains(body, `"prefix"`) {
		t.Fatalf("expected raw prefix field to stay unpublished, got %s", body)
	}
}

func TestUsageIdentitiesRouteMasksAIProviderIdentity(t *testing.T) {
	rawLookupKey := "sk-live-secret-value"
	maskedLookupKey := redact.APIKeyDisplayName(rawLookupKey)
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{items: []entities.UsageIdentity{
		{ID: 1, Name: "Provider Name", Prefix: "Team Prefix", AuthType: entities.UsageIdentityAuthTypeAIProvider, AuthTypeName: "apikey", Identity: rawLookupKey, Type: "openai", Provider: "OpenAI"},
	}}})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/usage/identities", nil)
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	body := resp.Body.String()
	if resp.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d: %s", resp.Code, body)
	}
	if contains(body, rawLookupKey) {
		t.Fatalf("expected raw AI provider lookup key to be hidden, got %s", body)
	}
	if !contains(body, `"identity":"`+maskedLookupKey+`"`) {
		t.Fatalf("expected masked AI provider identity %q in response body: %s", maskedLookupKey, body)
	}
	if !contains(body, `"name":"Provider Name"`) || !contains(body, `"provider":"OpenAI"`) || !contains(body, `"displayName":"Provider Name(Team Prefix)"`) {
		t.Fatalf("expected AI provider display fields to use usage_identities values directly, got %s", body)
	}
}

func TestUsageIdentityReplacesLegacyMetadataRoutes(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{UsageIdentity: usageIdentitiesStub{}})
	for _, path := range []string{"/api/v1/auth-files", "/api/v1/provider-metadata"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		resp := httptest.NewRecorder()

		router.ServeHTTP(resp, req)

		if resp.Code != http.StatusNotFound {
			t.Fatalf("expected %s to return 404, got %d: %s", path, resp.Code, resp.Body.String())
		}
	}
}
