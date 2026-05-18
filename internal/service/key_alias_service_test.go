package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"
)

func TestKeyAliasServiceResolvesDeletedUsageIdentityByID(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-service.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	deletedAt := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	identity := entities.UsageIdentity{
		Name:         "Deleted Provider",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "sk-deleted",
		Type:         "openai",
		Provider:     "OpenAI",
		IsDeleted:    true,
		DeletedAt:    &deletedAt,
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed deleted usage identity: %v", err)
	}

	provider := NewKeyAliasService(db)
	alias, err := provider.SetUsageIdentityAlias(context.Background(), identity.ID, "  Historical Key  ")
	if err != nil {
		t.Fatalf("set alias for deleted identity: %v", err)
	}
	if alias != "Historical Key" {
		t.Fatalf("expected trimmed alias, got %q", alias)
	}

	aliases, err := provider.ListAliasesForUsageIdentities(context.Background(), []entities.UsageIdentity{identity})
	if err != nil {
		t.Fatalf("list aliases: %v", err)
	}
	if aliases[UsageIdentityAliasKey{AuthType: identity.AuthType, Identity: identity.Identity}] != "Historical Key" {
		t.Fatalf("expected alias map to include deleted identity alias, got %+v", aliases)
	}
}

func TestKeyAliasServiceValidatesAliasLength(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-service-validation.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	identity := entities.UsageIdentity{
		Name:         "Provider",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "sk-provider",
		Type:         "openai",
		Provider:     "OpenAI",
	}
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed usage identity: %v", err)
	}

	provider := NewKeyAliasService(db)
	if _, err := provider.SetUsageIdentityAlias(context.Background(), identity.ID, strings.Repeat("a", 81)); err == nil {
		t.Fatal("expected 81-character alias to be rejected")
	}
}

func TestKeyAliasServiceManagesRawAPIKeyAliasesByOpaqueID(t *testing.T) {
	db, err := repository.OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "api-key-alias-service.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	now := time.Date(2026, 5, 18, 8, 0, 0, 0, time.UTC)
	if _, err := repository.UpsertModelPriceSetting(db, repodto.ModelPriceSettingInput{
		Model:            "priced-model",
		PromptPricePer1M: 1,
	}); err != nil {
		t.Fatalf("upsert pricing: %v", err)
	}
	if _, _, err := repository.InsertUsageEvents(db, []entities.UsageEvent{{
		EventKey:    "api-key-event",
		APIGroupKey: "sk-live-secret-value",
		AuthType:    "oauth",
		AuthIndex:   "account-key",
		Source:      "operator@example.com",
		Provider:    "OpenAI",
		Model:       "priced-model",
		Timestamp:   now,
		InputTokens: 1_000_000,
		TotalTokens: 1_000_000,
	}, {
		EventKey:    "provider-fallback",
		APIGroupKey: "OpenAI",
		AuthType:    "oauth",
		AuthIndex:   "account-key",
		Source:      "operator@example.com",
		Provider:    "OpenAI",
		Model:       "priced-model",
		Timestamp:   now.Add(time.Minute),
		InputTokens: 1_000_000,
		TotalTokens: 1_000_000,
	}}); err != nil {
		t.Fatalf("insert usage event: %v", err)
	}

	provider := NewKeyAliasService(db)
	apiKeyID := redact.APIAlias("sk-live-secret-value")
	alias, err := provider.SetAPIKeyAlias(context.Background(), apiKeyID, "  Raw API Key  ")
	if err != nil {
		t.Fatalf("set api key alias: %v", err)
	}
	if alias != "Raw API Key" {
		t.Fatalf("expected trimmed alias, got %q", alias)
	}

	page, err := provider.ListAPIKeyAliasTargetsPage(context.Background(), ListAPIKeyAliasTargetsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list api key aliases: %v", err)
	}
	if len(page.Items) != 1 || page.Total != 1 {
		t.Fatalf("expected one api key alias target, got %+v", page)
	}
	item := page.Items[0]
	if item.ID != apiKeyID || item.Identity != redact.APIKeyDisplayName("sk-live-secret-value") || item.Alias != "Raw API Key" {
		t.Fatalf("expected masked api key alias target, got %+v", item)
	}
	if item.TotalCost != 1 || !item.CostAvailable || item.CostStatus != "available" {
		t.Fatalf("expected priced api key totals, got %+v", item)
	}

	if err := provider.ClearAPIKeyAlias(context.Background(), apiKeyID); err != nil {
		t.Fatalf("clear api key alias: %v", err)
	}
	page, err = provider.ListAPIKeyAliasTargetsPage(context.Background(), ListAPIKeyAliasTargetsRequest{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("list api key aliases after clear: %v", err)
	}
	if len(page.Items) != 1 || page.Items[0].Alias != "" {
		t.Fatalf("expected alias to be cleared, got %+v", page)
	}
}
