package api

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cpa-usage/internal/service"
)

type modelSupportProviderStub struct {
	result service.ModelSupportResult
	err    error
	calls  int
	ids    []uint
}

func (s *modelSupportProviderStub) Load(_ context.Context, request service.ModelSupportRequest) (service.ModelSupportResult, error) {
	s.calls++
	s.ids = append([]uint(nil), request.IdentityIDs...)
	return s.result, s.err
}

func TestModelSupportRouteIsProtectedBeforeProviderCall(t *testing.T) {
	provider := &modelSupportProviderStub{}
	config := AuthConfig{Enabled: true, LoginPassword: "secret"}
	router := NewRouter(nil, nil, nil, nil, config, NewAuthHandler(config, nil), "", OptionalProviders{ModelSupport: provider})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/usage/identities/model-support", strings.NewReader(`{"identity_ids":[1]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusUnauthorized {
		t.Fatalf("expected protected route to return 401, got %d: %s", resp.Code, resp.Body.String())
	}
	if provider.calls != 0 {
		t.Fatalf("provider must not run before authentication, got %d calls", provider.calls)
	}
}

func TestModelSupportRouteMapsExplicitPartialAndCapabilityFields(t *testing.T) {
	contextLength := 200000
	zeroAllowed := false
	provider := &modelSupportProviderStub{result: service.ModelSupportResult{
		Complete: false, SelectedCount: 2, LoadedCount: 1,
		Accounts: []service.AccountModelSupport{{
			IdentityID: 1, AuthIndex: "auth-1", DisplayName: "Codex Account", Provider: "Codex", Channel: "codex",
			Status: "loaded", CatalogStatus: "loaded", RegisteredModels: []service.RegisteredModelSupport{{
				ID: "gpt-exact", DefinitionStatus: "available", Capability: &service.ModelCapability{
					ContextLength: &contextLength, SupportedInputModalities: []string{"TEXT", "IMAGE"},
					Thinking: &service.ModelThinkingSupport{ZeroAllowed: &zeroAllowed},
				},
			}},
		}, {
			IdentityID: 2, AuthIndex: "auth-2", DisplayName: "Unavailable Account", Provider: "Codex", Channel: "codex",
			Status: "failed", ErrorCode: "upstream_error", CatalogStatus: "loaded", RegisteredModels: []service.RegisteredModelSupport{},
		}},
		Models: []service.ModelSupportCoverage{{ModelID: "gpt-exact", ObservedSupportingAccounts: 1, SelectedAccounts: 2}},
		Limits: service.ModelSupportLimits{MaxAccounts: 12, MaxConcurrency: 4, TimeoutSeconds: 15, MaxUpstreamRequests: 36},
	}}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{ModelSupport: provider})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/usage/identities/model-support", bytes.NewBufferString(`{"identity_ids":[1,2]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", resp.Code, resp.Body.String())
	}
	body := resp.Body.String()
	for _, expected := range []string{`"scope_complete":false`, `"loaded_count":1`, `"definition_status":"available"`, `"context_length":200000`, `"zero_allowed":false`, `"error_code":"upstream_error"`, `"single_registered_account_in_scope":null`, `"max_upstream_requests":36`} {
		if !strings.Contains(body, expected) {
			t.Fatalf("expected %s in response: %s", expected, body)
		}
	}
	if strings.Contains(body, "provider unavailable") || strings.Contains(body, "management-secret") {
		t.Fatalf("response exposed upstream or credential data: %s", body)
	}
	if provider.calls != 1 || len(provider.ids) != 2 || provider.ids[1] != 2 {
		t.Fatalf("unexpected provider request: calls=%d ids=%v", provider.calls, provider.ids)
	}
}

func TestModelSupportRouteRejectsScopeErrors(t *testing.T) {
	provider := &modelSupportProviderStub{err: errors.Join(service.ErrModelSupportScopeTooLarge, errors.New("select at most 12 accounts"))}
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{ModelSupport: provider})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/usage/identities/model-support", strings.NewReader(`{"identity_ids":[1]}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()

	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusBadRequest || !strings.Contains(resp.Body.String(), "select at most 12 accounts") {
		t.Fatalf("expected bounded scope 400, got %d: %s", resp.Code, resp.Body.String())
	}
}
