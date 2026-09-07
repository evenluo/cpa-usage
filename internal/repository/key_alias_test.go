package repository

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"gorm.io/gorm"
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

func TestListKeyAliasesPreservesMixedTypeKeysAndSkipsMissingOrInvalidInput(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-list.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	rows := []entities.KeyAlias{
		{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "shared", Alias: "Auth file", CreatedAt: now, UpdatedAt: now},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "shared", Alias: "API key", CreatedAt: now, UpdatedAt: now},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "only-api", Alias: "Only API", CreatedAt: now, UpdatedAt: now},
	}
	if err := db.Create(&rows).Error; err != nil {
		t.Fatalf("seed key aliases: %v", err)
	}

	aliases, err := ListKeyAliases(context.Background(), db, []KeyAliasKey{
		{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: " shared "},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "shared"},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: " only-api "},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "only-api"},
		{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "missing"},
		{AuthType: 0, Identity: "invalid-auth-type"},
		{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: " "},
	})
	if err != nil {
		t.Fatalf("list key aliases: %v", err)
	}
	if len(aliases) != 3 {
		t.Fatalf("expected three normalized matches, got %#v", aliases)
	}
	if aliases[KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "shared"}].Alias != "Auth file" ||
		aliases[KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "shared"}].Alias != "API key" ||
		aliases[KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "only-api"}].Alias != "Only API" {
		t.Fatalf("expected auth type and normalized identity to remain the lookup key, got %#v", aliases)
	}
}

func TestListKeyAliasesUsesBoundedQueriesForFiveThousandDistinctKeys(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "key-alias-batch.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeTestDatabase(t, db)

	const keyCount = 5000
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	rows := make([]entities.KeyAlias, 0, keyCount)
	keys := make([]KeyAliasKey, 0, keyCount)
	for index := 0; index < keyCount; index++ {
		identity := fmt.Sprintf("sk-list-%04d", index)
		rows = append(rows, entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: identity, Alias: fmt.Sprintf("Alias %04d", index), CreatedAt: now, UpdatedAt: now})
		keys = append(keys, KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: identity})
	}
	if err := db.CreateInBatches(&rows, 400).Error; err != nil {
		t.Fatalf("seed key aliases: %v", err)
	}

	queryCount := 0
	if err := db.Callback().Query().Before("gorm:query").Register("test:count-key-alias-list-queries", func(*gorm.DB) {
		queryCount++
	}); err != nil {
		t.Fatalf("register query counter: %v", err)
	}
	aliases, err := ListKeyAliases(context.Background(), db, keys)
	if err != nil {
		t.Fatalf("list key aliases: %v", err)
	}
	if len(aliases) != keyCount {
		t.Fatalf("expected %d aliases, got %d", keyCount, len(aliases))
	}
	if queryCount > 20 {
		t.Fatalf("expected bounded alias lookup queries, got %d for %d distinct identities", queryCount, keyCount)
	}
	t.Logf("loaded %d distinct aliases with %d queries", keyCount, queryCount)
}
