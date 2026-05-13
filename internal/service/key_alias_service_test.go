package service

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
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
