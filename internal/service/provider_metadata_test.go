package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"cpa-usage/internal/cpa/dto/providerconfig"
	servicedto "cpa-usage/internal/service/dto"
)

func TestProviderKeyConfigAdapterFetchesAndFlattensPayload(t *testing.T) {
	adapter := newProviderKeyConfigAdapter("gemini", MetadataFetcher.FetchGeminiAPIKeys)
	fetcher := stubMetadataFetcher{providerConfig: providerconfig.ProviderMetadataConfig{
		GeminiAPIKeys: []providerconfig.ProviderKeyConfig{
			{APIKey: "gemini-key", Prefix: "gemini-prefix", Name: "Gemini Team", AuthIndex: "gemini-auth", BaseURL: "https://gemini.example.com"},
			{APIKey: "unnamed-key", AuthIndex: "unnamed-auth"},
		},
	}}

	inputs, err := adapter.fetch(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("adapter fetch returned error: %v", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %+v", inputs)
	}
	first := inputs[0]
	if first.LookupKey != "gemini-key" || first.Prefix != "gemini-prefix" || first.ProviderType != "gemini" || first.DisplayName != "Gemini Team" || first.AuthIndex != "gemini-auth" || first.BaseURL != "https://gemini.example.com" {
		t.Fatalf("unexpected gemini input: %+v", first)
	}
	if inputs[1].DisplayName != "gemini" {
		t.Fatalf("expected unnamed key to fall back to provider type display name, got %+v", inputs[1])
	}
}

func TestProviderKeyConfigAdapterWrapsFetchFailure(t *testing.T) {
	adapter := newProviderKeyConfigAdapter("gemini", MetadataFetcher.FetchGeminiAPIKeys)

	_, err := adapter.fetch(context.Background(), stubMetadataFetcher{geminiErr: errors.New("gemini unavailable")})
	if err == nil || !strings.Contains(err.Error(), "fetch gemini api keys: gemini unavailable") {
		t.Fatalf("expected wrapped fetch error, got %v", err)
	}

	_, err = adapter.fetch(context.Background(), stubMetadataFetcher{geminiNilResult: true})
	if err == nil || !strings.Contains(err.Error(), "gemini api keys response is nil") {
		t.Fatalf("expected nil response error, got %v", err)
	}
}

func TestOpenAICompatibilityAdapterFetchesAndFlattensEntries(t *testing.T) {
	adapter := newOpenAICompatibilityAdapter()
	fetcher := stubMetadataFetcher{providerConfig: providerconfig.ProviderMetadataConfig{
		OpenAICompatibility: []providerconfig.OpenAICompatibilityConfig{{
			Name:    "OpenRouter",
			Prefix:  "openrouter",
			BaseURL: "https://openrouter.ai/api/v1",
			APIKeyEntries: []providerconfig.OpenAIApiKeyEntry{
				{APIKey: "openrouter-key", AuthIndex: "openrouter-auth"},
				{APIKey: "fallback-key"},
			},
		}},
	}}

	inputs, err := adapter.fetch(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("adapter fetch returned error: %v", err)
	}
	if len(inputs) != 2 {
		t.Fatalf("expected 2 inputs, got %+v", inputs)
	}
	first := inputs[0]
	if first.ProviderType != "openai" || first.DisplayName != "OpenRouter" || first.AuthIndex != "openrouter-auth" || first.Prefix != "openrouter" || first.BaseURL != "https://openrouter.ai/api/v1" {
		t.Fatalf("unexpected openai input: %+v", first)
	}
	if inputs[1].AuthIndex != "fallback-key" {
		t.Fatalf("expected entry without auth index to fall back to api key, got %+v", inputs[1])
	}
}

func TestOpenAICompatibilityAdapterUsesProviderTypeAsDisplayNameFallback(t *testing.T) {
	adapter := newOpenAICompatibilityAdapter()
	fetcher := stubMetadataFetcher{providerConfig: providerconfig.ProviderMetadataConfig{
		OpenAICompatibility: []providerconfig.OpenAICompatibilityConfig{{
			Prefix:        "custom",
			APIKeyEntries: []providerconfig.OpenAIApiKeyEntry{{APIKey: "raw-key", AuthIndex: "raw-auth"}},
		}},
	}}

	inputs, err := adapter.fetch(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("adapter fetch returned error: %v", err)
	}
	if len(inputs) != 1 || inputs[0].DisplayName != "openai" {
		t.Fatalf("expected display name to fall back to openai, got %+v", inputs)
	}
}

func TestOpenAICompatibilityAdapterWrapsFetchFailure(t *testing.T) {
	adapter := newOpenAICompatibilityAdapter()

	_, err := adapter.fetch(context.Background(), stubMetadataFetcher{openAIErr: errors.New("openai unavailable")})
	if err == nil || !strings.Contains(err.Error(), "fetch openai compatibility: openai unavailable") {
		t.Fatalf("expected wrapped fetch error, got %v", err)
	}
}

func TestNewProviderMetadataRegistrySkipsIncompleteAdapters(t *testing.T) {
	registry := NewProviderMetadataRegistry([]ProviderMetadataAdapter{
		{providerType: "", fetch: stubProviderMetadataFetch(nil, nil)},
		{providerType: "gemini", fetch: nil},
		{providerType: "claude", fetch: stubProviderMetadataFetch(nil, nil)},
	})
	if len(registry.adapters) != 1 || registry.adapters[0].providerType != "claude" {
		t.Fatalf("expected only the complete adapter to register, got %+v", registry.adapters)
	}
}

func TestProviderMetadataRegistryFetchAggregatesPartialFailures(t *testing.T) {
	registry := NewProviderMetadataRegistry([]ProviderMetadataAdapter{
		{providerType: "gemini", fetch: stubProviderMetadataFetch(nil, errors.New("gemini unavailable"))},
		{providerType: "claude", fetch: stubProviderMetadataFetch([]servicedto.ProviderMetadataInput{
			{LookupKey: "claude-key", ProviderType: "claude", DisplayName: "Claude", AuthIndex: "claude-auth"},
		}, nil)},
	})

	inputs, fetchedTypes, err := registry.fetch(context.Background(), stubMetadataFetcher{})
	if err == nil || !strings.Contains(err.Error(), "gemini unavailable") {
		t.Fatalf("expected aggregated fetch error, got %v", err)
	}
	if len(fetchedTypes) != 1 || fetchedTypes[0] != "claude" {
		t.Fatalf("expected only the successful provider type, got %+v", fetchedTypes)
	}
	if len(inputs) != 1 || inputs[0].AuthIndex != "claude-auth" {
		t.Fatalf("expected successful provider inputs, got %+v", inputs)
	}
}

func TestProviderMetadataRegistryFetchDedupesAuthIndexAcrossAdapters(t *testing.T) {
	registry := NewProviderMetadataRegistry([]ProviderMetadataAdapter{
		{providerType: "gemini", fetch: stubProviderMetadataFetch([]servicedto.ProviderMetadataInput{
			{LookupKey: "gemini-key", ProviderType: "gemini", DisplayName: "Gemini", AuthIndex: "shared-auth"},
		}, nil)},
		{providerType: "claude", fetch: stubProviderMetadataFetch([]servicedto.ProviderMetadataInput{
			{LookupKey: "claude-key", ProviderType: "claude", DisplayName: "Claude", AuthIndex: "shared-auth"},
			{LookupKey: "claude-key-2", ProviderType: "claude", DisplayName: "Claude", AuthIndex: "claude-auth"},
		}, nil)},
	})

	inputs, fetchedTypes, err := registry.fetch(context.Background(), stubMetadataFetcher{})
	if err != nil {
		t.Fatalf("registry fetch returned error: %v", err)
	}
	if len(fetchedTypes) != 2 {
		t.Fatalf("expected both provider types fetched, got %+v", fetchedTypes)
	}
	if len(inputs) != 2 || inputs[0].ProviderType != "gemini" || inputs[1].AuthIndex != "claude-auth" {
		t.Fatalf("expected first adapter to win auth index dedup, got %+v", inputs)
	}
}

func TestDefaultProviderMetadataRegistryFetchesEveryProvider(t *testing.T) {
	metadata := &trackingMetadataFetcher{}
	registry := NewDefaultProviderMetadataRegistry()

	_, fetchedTypes, err := registry.fetch(context.Background(), metadata)
	if err != nil {
		t.Fatalf("registry fetch returned error: %v", err)
	}
	if metadata.providerCalls() != 5 {
		t.Fatalf("expected 5 provider fetches, got %d", metadata.providerCalls())
	}
	expected := []string{"gemini", "claude", "codex", "vertex", "openai"}
	if len(fetchedTypes) != len(expected) {
		t.Fatalf("expected provider types %v, got %v", expected, fetchedTypes)
	}
	for i, providerType := range expected {
		if fetchedTypes[i] != providerType {
			t.Fatalf("expected provider types %v, got %v", expected, fetchedTypes)
		}
	}
}

func stubProviderMetadataFetch(inputs []servicedto.ProviderMetadataInput, err error) providerMetadataFetchFunc {
	return func(context.Context, MetadataFetcher) ([]servicedto.ProviderMetadataInput, error) {
		return inputs, err
	}
}
