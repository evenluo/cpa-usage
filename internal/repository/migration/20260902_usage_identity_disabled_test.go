package migration

import (
	"path/filepath"
	"testing"

	"cpa-usage/internal/entities"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageIdentityDisabledMigrationAddsDefaultedColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_identities (
		id integer PRIMARY KEY AUTOINCREMENT,
		name text,
		auth_type integer,
		auth_type_name text,
		identity text,
		type text,
		provider text,
		lookup_key text,
		prefix text,
		is_deleted numeric
	)`).Error; err != nil {
		t.Fatalf("create legacy usage_identities table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_identities (name, auth_type, auth_type_name, identity, type, provider, lookup_key, prefix, is_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`, "Codex", entities.UsageIdentityAuthTypeAuthFile, "oauth", "codex-auth", "codex", "codex", "", "", false).Error; err != nil {
		t.Fatalf("seed legacy usage identity: %v", err)
	}

	if err := addUsageIdentityDisabledMigration(db); err != nil {
		t.Fatalf("add usage identity disabled: %v", err)
	}
	if err := addUsageIdentityDisabledMigration(db); err != nil {
		t.Fatalf("add usage identity disabled should be idempotent: %v", err)
	}
	if !db.Migrator().HasColumn(&entities.UsageIdentity{}, "disabled") {
		t.Fatal("expected usage_identities.disabled column to exist")
	}

	var disabled bool
	if err := db.Raw(`SELECT disabled FROM usage_identities WHERE identity = ?`, "codex-auth").Row().Scan(&disabled); err != nil {
		t.Fatalf("scan disabled: %v", err)
	}
	if disabled {
		t.Fatal("expected legacy rows to default disabled=false")
	}
}
