package test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"cpa-usage/internal/cpa/dto/apicall"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/quota"
)

type recordingProviderHandler struct {
	inputs []quota.ProviderInput
	output quota.ProviderOutput
	err    error
}

func (h *recordingProviderHandler) Check(ctx context.Context, input quota.ProviderInput) (quota.ProviderOutput, error) {
	h.inputs = append(h.inputs, input)
	if h.err != nil {
		return quota.ProviderOutput{}, h.err
	}
	return h.output, nil
}

// fakeAuthFileIdentityLookup 以内存身份表实现 quota.AuthFileIdentityLookup，dispatch 测试不再需要真实数据库。
type fakeAuthFileIdentityLookup struct {
	identities map[string]entities.UsageIdentity
}

func newFakeAuthFileIdentityLookup(identities ...entities.UsageIdentity) fakeAuthFileIdentityLookup {
	lookup := fakeAuthFileIdentityLookup{identities: make(map[string]entities.UsageIdentity, len(identities))}
	for _, identity := range identities {
		lookup.identities[identity.Identity] = identity
	}
	return lookup
}

func (f fakeAuthFileIdentityLookup) FindActiveAuthFileIdentity(_ context.Context, authIndex string) (entities.UsageIdentity, bool, error) {
	identity, ok := f.identities[strings.TrimSpace(authIndex)]
	if !ok || identity.AuthType != entities.UsageIdentityAuthTypeAuthFile {
		return entities.UsageIdentity{}, false, nil
	}
	return identity, true, nil
}

func (f fakeAuthFileIdentityLookup) HasActiveIdentity(_ context.Context, authIndex string) (bool, error) {
	_, ok := f.identities[authIndex]
	return ok, nil
}

func TestServiceRejectsEmptyAuthIndex(t *testing.T) {
	service := quota.NewServiceWithRegistry(newFakeAuthFileIdentityLookup(), quota.NewProviderRegistry(nil))

	_, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "   "})
	if !errors.Is(err, quota.ErrValidation) {
		t.Fatalf("expected validation error, got %v", err)
	}
}

func TestServiceIgnoresProviderOnlyIdentity(t *testing.T) {
	lookup := newFakeAuthFileIdentityLookup(entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "shared-auth", Type: "codex", Name: "provider"})
	handler := &recordingProviderHandler{}
	service := quota.NewServiceWithRegistry(lookup, quota.NewProviderRegistry(map[string]quota.ProviderHandler{"codex": handler}))

	_, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "shared-auth"})
	if !errors.Is(err, quota.ErrNotFound) {
		t.Fatalf("expected not found error, got %v", err)
	}
	if len(handler.inputs) != 0 {
		t.Fatalf("expected provider not to be called, got %d calls", len(handler.inputs))
	}
}

func TestServiceDispatchesAuthFileIdentityByProviderBeforeType(t *testing.T) {
	lookup := newFakeAuthFileIdentityLookup(entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "codex-auth", Provider: "codex", Type: "unknown", Name: "auth file"})
	handler := &recordingProviderHandler{output: quota.ProviderOutput{Provider: "codex", Result: quota.CodexResult{Usage: &quota.CodexUsagePayload{RateLimit: &quota.CodexRateLimitInfo{PrimaryWindow: &quota.CodexUsageWindow{UsedPercent: 25}}}}}}
	service := quota.NewServiceWithRegistry(lookup, quota.NewProviderRegistry(map[string]quota.ProviderHandler{"codex": handler}))

	response, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "codex-auth"})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if response.ID != "codex-auth" || len(response.Quota) != 1 || response.Quota[0].Key != "rate_limit.primary_window" || response.Quota[0].UsedPercent == nil || *response.Quota[0].UsedPercent != 25 {
		t.Fatalf("unexpected check response: %+v", response)
	}
	if len(handler.inputs) != 1 || handler.inputs[0].Identity.Identity != "codex-auth" || handler.inputs[0].Identity.AuthType != entities.UsageIdentityAuthTypeAuthFile {
		t.Fatalf("unexpected provider inputs: %+v", handler.inputs)
	}
}

func TestServiceFallsBackToTypeWhenProviderMissing(t *testing.T) {
	lookup := newFakeAuthFileIdentityLookup(entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "gemini-auth", Provider: "Gemini", Type: "gemini-cli", Name: "auth file"})
	handler := &recordingProviderHandler{output: quota.ProviderOutput{Provider: "gemini-cli", Result: quota.GeminiCLIResult{Quota: &quota.GeminiCliQuotaPayload{Buckets: []quota.GeminiCliQuotaBucket{{ModelID: "gemini-2.5-pro_vertex", TokenType: "PROMPT", RemainingAmount: 42}}}}}}
	service := quota.NewServiceWithRegistry(lookup, quota.NewProviderRegistry(map[string]quota.ProviderHandler{"gemini-cli": handler}))

	response, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "gemini-auth"})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if response.ID != "gemini-auth" || len(response.Quota) != 1 || response.Quota[0].Key != "bucket.gemini-2.5-pro_vertex.PROMPT" {
		t.Fatalf("unexpected check response: %+v", response)
	}
	if len(handler.inputs) != 1 {
		t.Fatalf("unexpected provider inputs: %+v", handler.inputs)
	}
}

func TestServiceReturnsUnsupportedType(t *testing.T) {
	lookup := newFakeAuthFileIdentityLookup(entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "unknown-auth", Type: "unknown", Name: "auth file"})
	service := quota.NewServiceWithRegistry(lookup, quota.NewProviderRegistry(nil))

	_, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "unknown-auth"})
	if !errors.Is(err, quota.ErrUnsupportedType) {
		t.Fatalf("expected unsupported type error, got %v", err)
	}
}

func TestServiceAllowsCodexQuotaWithoutAccountID(t *testing.T) {
	lookup := newFakeAuthFileIdentityLookup(entities.UsageIdentity{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "codex-auth", Type: "codex", Name: "auth file"})
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   `{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false}}`,
		Body:       json.RawMessage(`{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false}}`),
	}}}
	service := quota.NewServiceWithRegistry(lookup, quota.NewDefaultProviderRegistry(caller, quota.DefaultProviderConfigs()))

	response, err := service.Check(context.Background(), quota.CheckRequest{AuthIndex: "codex-auth"})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if response.ID != "codex-auth" || len(caller.requests) != 1 {
		t.Fatalf("expected codex quota request without account_id, got response=%+v requests=%d", response, len(caller.requests))
	}
}
