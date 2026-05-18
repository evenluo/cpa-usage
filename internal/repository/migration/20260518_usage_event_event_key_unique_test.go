package migration

import (
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestOpenDatabaseEnsuresUsageEventEventKeyUniqueIndex(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open legacy database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text
	)`).Error; err != nil {
		t.Fatalf("create usage_events table: %v", err)
	}
	if err := db.Exec(`CREATE INDEX idx_usage_events_event_key ON usage_events(event_key)`).Error; err != nil {
		t.Fatalf("create legacy event_key index: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_events (event_key) VALUES ('event-a'), ('event-b')`).Error; err != nil {
		t.Fatalf("seed usage_events: %v", err)
	}
	seedPriorMigrationsAsApplied(t, db, migrationEnsureUsageEventEventKeyUnique)

	if err := Run(db); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if sqliteIndexExists(t, db, "idx_usage_events_event_key") {
		t.Fatal("expected legacy non-unique event_key index to be removed")
	}
	if !sqliteIndexExists(t, db, "uniq_usage_events_event_key") {
		t.Fatal("expected unique event_key index to exist")
	}
	if !sqliteIndexIsUnique(t, db, "uniq_usage_events_event_key") {
		t.Fatal("expected event_key index to be unique")
	}
}

func seedPriorMigrationsAsApplied(t *testing.T, db *gorm.DB, skipVersion string) {
	t.Helper()
	if err := createSchemaMigrationsTable(db); err != nil {
		t.Fatalf("create schema migrations table: %v", err)
	}
	for _, migration := range orderedMigrations() {
		if migration.version == skipVersion {
			continue
		}
		if err := db.Exec("INSERT OR IGNORE INTO schema_migrations (version, applied_at) VALUES (?, CURRENT_TIMESTAMP)", migration.version).Error; err != nil {
			t.Fatalf("mark migration %s applied: %v", migration.version, err)
		}
	}
}

func sqliteIndexIsUnique(t *testing.T, db *gorm.DB, indexName string) bool {
	t.Helper()
	var rows []struct {
		Name   string
		Unique int
	}
	if err := db.Raw("PRAGMA index_list(usage_events)").Scan(&rows).Error; err != nil {
		t.Fatalf("load usage_events indexes: %v", err)
	}
	for _, row := range rows {
		if row.Name == indexName {
			return row.Unique == 1
		}
	}
	return false
}
