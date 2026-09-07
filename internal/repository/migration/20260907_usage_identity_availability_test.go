package migration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"cpa-usage/internal/entities"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageIdentityAvailabilityMigrationAddsNullableColumns(t *testing.T) {
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
		is_deleted numeric
	)`).Error; err != nil {
		t.Fatalf("create legacy usage_identities table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_identities (name, auth_type, auth_type_name, identity, type, provider, is_deleted)
		VALUES (?, ?, ?, ?, ?, ?, ?)`, "Codex", entities.UsageIdentityAuthTypeAuthFile, "oauth", "codex-auth", "codex", "Codex", false).Error; err != nil {
		t.Fatalf("seed legacy usage identity: %v", err)
	}

	if err := addUsageIdentityAvailabilityMigration(db); err != nil {
		t.Fatalf("add usage identity availability: %v", err)
	}
	if err := addUsageIdentityAvailabilityMigration(db); err != nil {
		t.Fatalf("add usage identity availability should be idempotent: %v", err)
	}
	for _, column := range []string{"auth_file_status", "unavailable", "last_refresh", "next_retry_after", "metadata_observed_at"} {
		if !db.Migrator().HasColumn(&entities.UsageIdentity{}, column) {
			t.Fatalf("expected usage_identities.%s column to exist", column)
		}
	}

	var status sql.NullString
	var unavailable sql.NullBool
	var lastRefresh, nextRetryAfter, metadataObservedAt sql.NullTime
	if err := db.Raw(`SELECT auth_file_status, unavailable, last_refresh, next_retry_after, metadata_observed_at
		FROM usage_identities WHERE identity = ?`, "codex-auth").Row().Scan(
		&status, &unavailable, &lastRefresh, &nextRetryAfter, &metadataObservedAt,
	); err != nil {
		t.Fatalf("scan availability fields: %v", err)
	}
	if status.Valid || unavailable.Valid || lastRefresh.Valid || nextRetryAfter.Valid || metadataObservedAt.Valid {
		t.Fatalf("expected legacy availability fields to remain absent, got status=%v unavailable=%v last_refresh=%v next_retry_after=%v metadata_observed_at=%v", status, unavailable, lastRefresh, nextRetryAfter, metadataObservedAt)
	}
}
