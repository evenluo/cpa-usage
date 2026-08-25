package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/service"
	servicedto "cpa-usage/internal/service/dto"
)

type pricingStub struct {
	usedModels []string
	pricing    []entities.ModelPriceSetting
	updated    *entities.ModelPriceSetting
	lastUpdate *servicedto.UpdatePricingInput
	deleted    string
	err        error
}

func (s pricingStub) ListUsedModels(context.Context) ([]string, error) {
	return s.usedModels, s.err
}

func (s pricingStub) ListPricing(context.Context) ([]entities.ModelPriceSetting, error) {
	return s.pricing, s.err
}

func (s *pricingStub) UpdatePricing(_ context.Context, input servicedto.UpdatePricingInput) (*entities.ModelPriceSetting, error) {
	s.lastUpdate = &input
	return s.updated, s.err
}

func (s *pricingStub) DeletePricing(_ context.Context, model string) error {
	s.deleted = model
	return s.err
}

func TestPricingRoutesReturnEmptyResponsesWithoutProvider(t *testing.T) {
	router := NewRouter(nil, nil, nil, nil, AuthConfig{}, nil, "", OptionalProviders{})

	usedReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/used", nil)
	usedResp := httptest.NewRecorder()
	router.ServeHTTP(usedResp, usedReq)
	if usedResp.Code != http.StatusOK || !contains(usedResp.Body.String(), `"models":[]`) {
		t.Fatalf("unexpected used models response: %d %s", usedResp.Code, usedResp.Body.String())
	}

	pricingReq := httptest.NewRequest(http.MethodGet, "/api/v1/pricing", nil)
	pricingResp := httptest.NewRecorder()
	router.ServeHTTP(pricingResp, pricingReq)
	if pricingResp.Code != http.StatusOK || !contains(pricingResp.Body.String(), `"pricing":[]`) {
		t.Fatalf("unexpected pricing response: %d %s", pricingResp.Code, pricingResp.Body.String())
	}
}

func TestPricingRoutesReturnConfiguredData(t *testing.T) {
	router := NewRouter(nil, nil, nil, &pricingStub{
		usedModels: []string{"claude-sonnet"},
		pricing: []entities.ModelPriceSetting{{
			Model:                "claude-sonnet",
			PromptPricePer1M:     3,
			CompletionPricePer1M: 15,
			CachePricePer1M:      0.3,
		}},
	}, AuthConfig{}, nil, "", OptionalProviders{})

	usedReq := httptest.NewRequest(http.MethodGet, "/api/v1/models/used", nil)
	usedResp := httptest.NewRecorder()
	router.ServeHTTP(usedResp, usedReq)
	if usedResp.Code != http.StatusOK || !contains(usedResp.Body.String(), `claude-sonnet`) {
		t.Fatalf("unexpected used models response: %d %s", usedResp.Code, usedResp.Body.String())
	}

	pricingReq := httptest.NewRequest(http.MethodGet, "/api/v1/pricing", nil)
	pricingResp := httptest.NewRecorder()
	router.ServeHTTP(pricingResp, pricingReq)
	if pricingResp.Code != http.StatusOK || !contains(pricingResp.Body.String(), `"prompt_price_per_1m":3`) {
		t.Fatalf("unexpected pricing response: %d %s", pricingResp.Code, pricingResp.Body.String())
	}
}

func TestUpdatePricingRoute(t *testing.T) {
	provider := &pricingStub{
		updated: &entities.ModelPriceSetting{
			Model:                "claude-sonnet",
			PromptPricePer1M:     3,
			CompletionPricePer1M: 15,
			CachePricePer1M:      0.3,
		},
	}
	router := NewRouter(nil, nil, nil, provider, AuthConfig{}, nil, "", OptionalProviders{})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/pricing/claude-sonnet", strings.NewReader(`{"prompt_price_per_1m":3,"completion_price_per_1m":15,"cache_price_per_1m":0.3}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !contains(resp.Body.String(), `"model":"claude-sonnet"`) {
		t.Fatalf("unexpected update response: %d %s", resp.Code, resp.Body.String())
	}
}

func TestUpdatePricingRouteAcceptsModelInBody(t *testing.T) {
	provider := &pricingStub{
		updated: &entities.ModelPriceSetting{
			Model:                "openai/gpt-4.1",
			PromptPricePer1M:     3,
			CompletionPricePer1M: 15,
			CachePricePer1M:      0.3,
		},
	}
	router := NewRouter(nil, nil, nil, provider, AuthConfig{}, nil, "", OptionalProviders{})

	req := httptest.NewRequest(http.MethodPut, "/api/v1/pricing", strings.NewReader(`{"model":"openai/gpt-4.1","prompt_price_per_1m":3,"completion_price_per_1m":15,"cache_price_per_1m":0.3}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusOK || !contains(resp.Body.String(), `"model":"openai/gpt-4.1"`) {
		t.Fatalf("unexpected update response: %d %s", resp.Code, resp.Body.String())
	}
	if provider.lastUpdate == nil || provider.lastUpdate.Model != "openai/gpt-4.1" {
		t.Fatalf("expected model from body to be passed through, got %+v", provider.lastUpdate)
	}
}

func TestDeletePricingRoute(t *testing.T) {
	provider := &pricingStub{}
	router := NewRouter(nil, nil, nil, provider, AuthConfig{}, nil, "", OptionalProviders{})

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pricing?model=openai%2Fgpt-4.1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusNoContent {
		t.Fatalf("expected status 204, got %d %s", resp.Code, resp.Body.String())
	}
	if provider.deleted != "openai/gpt-4.1" {
		t.Fatalf("expected model to be deleted, got %q", provider.deleted)
	}
}

func TestUpdatePricingRouteClassifiesSentinelErrorsAsBadRequest(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want string
	}{
		{name: "unused model", err: service.ModelNotUsedError("ghost-model"), want: `model "ghost-model" has not been used`},
		{name: "model required", err: service.ErrModelRequired, want: "model is required"},
		{name: "negative prices", err: service.ErrPricesMustBeNonNegative, want: "prices must be non-negative"},
		{name: "wrapped unused model", err: fmt.Errorf("update pricing: %w", service.ModelNotUsedError("ghost-model")), want: `update pricing: model "ghost-model" has not been used`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter(nil, nil, nil, &pricingStub{err: tc.err}, AuthConfig{}, nil, "", OptionalProviders{})
			req := httptest.NewRequest(http.MethodPut, "/api/v1/pricing/ghost-model", strings.NewReader(`{"prompt_price_per_1m":3,"completion_price_per_1m":15,"cache_price_per_1m":0.3}`))
			req.Header.Set("Content-Type", "application/json")
			resp := httptest.NewRecorder()
			router.ServeHTTP(resp, req)

			var body struct {
				Error string `json:"error"`
			}
			if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
			}
			if resp.Code != http.StatusBadRequest || body.Error != tc.want {
				t.Fatalf("unexpected sentinel 400 response: %d %s", resp.Code, resp.Body.String())
			}
		})
	}
}

func TestDeletePricingRouteClassifiesModelRequiredAsBadRequest(t *testing.T) {
	router := NewRouter(nil, nil, nil, &pricingStub{err: service.ErrModelRequired}, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodDelete, "/api/v1/pricing?model=ghost-model", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(resp.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v body=%s", err, resp.Body.String())
	}
	if resp.Code != http.StatusBadRequest || body.Error != "model is required" {
		t.Fatalf("unexpected delete 400 response: %d %s", resp.Code, resp.Body.String())
	}
}

func TestUpdatePricingRouteMapsUnexpectedErrorsAsInternal(t *testing.T) {
	router := NewRouter(nil, nil, nil, &pricingStub{err: errors.New("database is nil")}, AuthConfig{}, nil, "", OptionalProviders{})
	req := httptest.NewRequest(http.MethodPut, "/api/v1/pricing/ghost-model", strings.NewReader(`{"prompt_price_per_1m":3,"completion_price_per_1m":15,"cache_price_per_1m":0.3}`))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	if resp.Code != http.StatusInternalServerError || !contains(resp.Body.String(), "internal server error") {
		t.Fatalf("unexpected update 500 response: %d %s", resp.Code, resp.Body.String())
	}
}
