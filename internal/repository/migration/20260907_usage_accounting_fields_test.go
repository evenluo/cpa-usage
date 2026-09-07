package migration

import (
	"path/filepath"
	"reflect"
	"testing"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestUsageAccountingMigrationPreservesHistoricalFacts(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "accounting.db"))), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	defer closeOpenedDatabase(t, db)
	if err := db.Exec(`CREATE TABLE usage_events (id INTEGER PRIMARY KEY, event_key TEXT, input_tokens INTEGER, output_tokens INTEGER, cached_tokens INTEGER, total_tokens INTEGER, service_tier TEXT)`).Error; err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(`INSERT INTO usage_events VALUES (1, 'historical', 100, 30, 40, 142, 'priority')`).Error; err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := addUsageAccountingFieldsMigration(db); err != nil {
			t.Fatal(err)
		}
	}
	var event entities.UsageEvent
	if err := db.First(&event).Error; err != nil {
		t.Fatal(err)
	}
	if event.InputTokens != 100 || event.OutputTokens != 30 || event.CachedTokens != 40 || event.TotalTokens != 142 || event.ServiceTier != "priority" {
		t.Fatalf("historical facts rewritten: %+v", event)
	}
	if !reflect.DeepEqual(event.UsageAccounting, entities.UsageAccounting{AccountingState: "absent"}) || event.Generate != nil || event.Stream != nil || event.ResponseServiceTier != nil {
		t.Fatalf("historical evidence fabricated: %+v", event)
	}
	for _, column := range []string{"accounting_version", "token_schema_version"} {
		if db.Migrator().HasColumn("usage_events", column) {
			t.Fatalf("upgrade persisted fixed protocol field usage_events.%s", column)
		}
	}
	for _, column := range []string{"accounting_state", "token_quality", "canonical_total_tokens", "generate", "stream", "response_service_tier"} {
		if !db.Migrator().HasColumn("usage_events", column) {
			t.Fatalf("expected usage_events.%s", column)
		}
	}
}

func TestAccountingUpgradeRequiresPublishedConsumerToDrainInbox(t *testing.T) {
	for _, status := range []string{"pending", "process_failed", "processed", "decode_failed", "discarded"} {
		t.Run(status, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "inbox.db"))), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenedDatabase(t, db)
			if err := db.Exec(`CREATE TABLE usage_events (id INTEGER PRIMARY KEY)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`CREATE TABLE redis_usage_inboxes (id INTEGER PRIMARY KEY, status TEXT)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO redis_usage_inboxes VALUES (1, ?)`, status).Error; err != nil {
				t.Fatal(err)
			}
			err = db.Transaction(addUsageAccountingFieldsMigration)
			blocked := status == "pending" || status == "process_failed"
			if (err != nil) != blocked {
				t.Fatalf("status %s: err=%v", status, err)
			}
			if db.Migrator().HasColumn("usage_events", "accounting_state") == blocked {
				t.Fatalf("unexpected schema mutation for status %s", status)
			}
			var retained int64
			if err := db.Table("redis_usage_inboxes").Count(&retained).Error; err != nil || retained != 1 {
				t.Fatalf("inbox changed: %d %v", retained, err)
			}
		})
	}
}
