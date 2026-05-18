package repository

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
)

func TestSetKeyAliasTrimsAllowsDuplicatesAndClears(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	now := time.Date(2026, 5, 13, 8, 0, 0, 0, time.UTC)
	first, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-one", "  产品分析 Key  ", now)
	if err != nil {
		t.Fatalf("set first alias: %v", err)
	}
	second, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-two", "产品分析 Key", now)
	if err != nil {
		t.Fatalf("set duplicate alias value: %v", err)
	}
	if first.Alias != "产品分析 Key" || second.Alias != "产品分析 Key" {
		t.Fatalf("expected trimmed duplicate aliases, got %+v and %+v", first, second)
	}

	updated, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-one", "Agent Research", now.Add(time.Hour))
	if err != nil {
		t.Fatalf("update alias: %v", err)
	}
	if updated.ID != first.ID || updated.Alias != "Agent Research" {
		t.Fatalf("expected update in place, got %+v from first %+v", updated, first)
	}

	if err := ClearKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-one"); err != nil {
		t.Fatalf("clear alias: %v", err)
	}
	if _, err := GetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-one"); err == nil {
		t.Fatal("expected cleared alias to be absent")
	}
}

func TestSetKeyAliasRejectsInvalidInput(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-validation.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-one", strings.Repeat("a", 81), time.Now()); err == nil {
		t.Fatal("expected 81-character alias to be rejected")
	}
	if _, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, " ", "Alias", time.Now()); err == nil {
		t.Fatal("expected blank identity to be rejected")
	}
}

func TestKeyAliasDoesNotDependOnUsageIdentityActiveState(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-deleted-identity.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	deletedAt := time.Date(2026, 5, 12, 8, 0, 0, 0, time.UTC)
	if err := db.Create(&entities.UsageIdentity{
		Name:         "Deleted Provider",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "sk-deleted",
		Type:         "openai",
		Provider:     "OpenAI",
		IsDeleted:    true,
		DeletedAt:    &deletedAt,
	}).Error; err != nil {
		t.Fatalf("seed deleted usage identity: %v", err)
	}

	alias, err := SetKeyAlias(context.Background(), db, entities.UsageIdentityAuthTypeAIProvider, "sk-deleted", "Historical Key", time.Now())
	if err != nil {
		t.Fatalf("set alias for deleted identity: %v", err)
	}
	if alias.Alias != "Historical Key" {
		t.Fatalf("expected alias for deleted identity, got %+v", alias)
	}
}
