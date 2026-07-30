package entities

import "testing"

func TestUsageIdentityDisplayNameFormatsProviderNameAndPrefix(t *testing.T) {
	identity := UsageIdentity{
		Name:     "Provider Name",
		Prefix:   "Team Prefix",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}

	if got := identity.DisplayName(); got != "Provider Name(Team Prefix)" {
		t.Fatalf("expected provider displayName to include name and prefix, got %q", got)
	}
}

func TestUsageIdentityDisplayNameAddsProviderBaseURLQualifier(t *testing.T) {
	withPrefix := UsageIdentity{
		Name:     "Provider Name",
		Prefix:   "Team Prefix",
		BaseURL:  "https://api.openai.com/v1/",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}
	providerOnly := UsageIdentity{
		Name:     "codex",
		BaseURL:  "https://chatgpt.com/backend-api/codex/",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "codex-auth-index",
	}

	if got := withPrefix.DisplayName(); got != "Provider Name(Team Prefix @ api.openai.com/v1)" {
		t.Fatalf("expected base URL to be an extra display qualifier, got %q", got)
	}
	if got := providerOnly.DisplayName(); got != "codex(chatgpt.com/backend-api/codex)" {
		t.Fatalf("expected provider displayName to include base URL qualifier, got %q", got)
	}
}

func TestUsageIdentityDisplayNameKeepsOpenAICompatibilityName(t *testing.T) {
	identity := UsageIdentity{
		Name:     "OpenRouter",
		Prefix:   "openrouter",
		BaseURL:  "https://openrouter.ai/api/v1",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Type:     "openai",
		Provider: "OpenRouter",
		Identity: "openrouter-auth-index",
	}

	if got := identity.DisplayName(); got != "OpenRouter" {
		t.Fatalf("expected openai compatibility displayName to keep name without qualifiers, got %q", got)
	}
}

func TestUsageIdentityDisplayNameFallsBackWhenOpenAICompatibilityNameIsMissing(t *testing.T) {
	identity := UsageIdentity{
		Prefix:   "openrouter",
		BaseURL:  "https://openrouter.ai/api/v1",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Type:     "openai",
		Provider: "openai",
		Identity: "openrouter-auth-index",
	}

	if got := identity.DisplayName(); got != "openrouter(openrouter.ai/api/v1)" {
		t.Fatalf("expected unnamed openai compatibility displayName to fall back to provider qualifier rules, got %q", got)
	}
}

func TestUsageIdentityDisplayNameUsesProviderWhenAuthFileNameIsMissing(t *testing.T) {
	identity := UsageIdentity{
		AuthType: UsageIdentityAuthTypeAuthFile,
		Provider: "Claude",
	}

	if got := identity.DisplayName(); got != "Claude" {
		t.Fatalf("expected auth file displayName to fall back to provider, got %q", got)
	}
}

func TestUsageIdentityDisplayNameFallsBackWhenProviderNameOrPrefixIsMissing(t *testing.T) {
	prefixOnly := UsageIdentity{
		Prefix:   "Team Prefix",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}
	nameOnly := UsageIdentity{
		Name:     "Provider Name",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}
	providerOnly := UsageIdentity{
		Provider: "OpenAI",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}

	if got := prefixOnly.DisplayName(); got != "Team Prefix" {
		t.Fatalf("expected prefix-only provider displayName, got %q", got)
	}
	if got := nameOnly.DisplayName(); got != "Provider Name" {
		t.Fatalf("expected name-only provider displayName, got %q", got)
	}
	if got := providerOnly.DisplayName(); got != "OpenAI" {
		t.Fatalf("expected provider-only displayName, got %q", got)
	}
}

func TestUsageIdentityDisplayNameFallsBackToBaseURLWhenOnlyBaseURLPresent(t *testing.T) {
	identity := UsageIdentity{
		BaseURL:  "https://api.example.com/v1/",
		AuthType: UsageIdentityAuthTypeAIProvider,
		Identity: "provider-auth-index",
	}

	if got := identity.DisplayName(); got != "api.example.com/v1" {
		t.Fatalf("expected base-URL-only provider displayName, got %q", got)
	}
}
