package migration

import (
	"database/sql"
	"path/filepath"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageEventTTFTMigrationAddsNullableColumnWithoutBackfill(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := db.Exec(`CREATE TABLE usage_events (
		id integer PRIMARY KEY AUTOINCREMENT,
		event_key text,
		latency_ms integer,
		output_tokens integer,
		total_tokens integer
	)`).Error; err != nil {
		t.Fatalf("create legacy usage_events table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_events (event_key, latency_ms, output_tokens, total_tokens)
		VALUES (?, ?, ?, ?)`, "event-1", 21245, 976, 105091).Error; err != nil {
		t.Fatalf("seed legacy usage event: %v", err)
	}

	if err := addUsageEventTTFTMigration(db); err != nil {
		t.Fatalf("add usage event ttft: %v", err)
	}

	if !db.Migrator().HasColumn("usage_events", "ttft_ms") {
		t.Fatal("expected usage_events.ttft_ms column to exist")
	}

	var ttft sql.NullInt64
	if err := db.Raw(`SELECT ttft_ms FROM usage_events WHERE event_key = ?`, "event-1").Row().Scan(&ttft); err != nil {
		t.Fatalf("scan ttft_ms: %v", err)
	}
	if ttft.Valid {
		t.Fatalf("expected existing usage event ttft_ms to remain NULL, got %+v", ttft)
	}
}
