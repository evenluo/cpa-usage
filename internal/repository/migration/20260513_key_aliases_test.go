package migration

import (
	"path/filepath"
	"testing"

	"cpa-usage/internal/entities"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateKeyAliasesMigrationCreatesDedicatedUniqueTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := createKeyAliasesMigration(db); err != nil {
		t.Fatalf("create key aliases: %v", err)
	}
	if err := createKeyAliasesMigration(db); err != nil {
		t.Fatalf("create key aliases should be idempotent: %v", err)
	}
	if !db.Migrator().HasTable(&entities.KeyAlias{}) {
		t.Fatal("expected key_aliases table to exist")
	}

	first := entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: "sk-one", Alias: "Shared Alias"}
	second := entities.KeyAlias{AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth-one", Alias: "Shared Alias"}
	if err := db.Create(&first).Error; err != nil {
		t.Fatalf("insert first alias: %v", err)
	}
	if err := db.Create(&second).Error; err != nil {
		t.Fatalf("expected duplicate alias values to be allowed: %v", err)
	}
	if err := db.Create(&entities.KeyAlias{AuthType: first.AuthType, Identity: first.Identity, Alias: "Other"}).Error; err == nil {
		t.Fatal("expected auth_type plus identity to be unique")
	}
}
