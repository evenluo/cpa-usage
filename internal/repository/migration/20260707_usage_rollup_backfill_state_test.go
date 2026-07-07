package migration

import (
	"path/filepath"
	"testing"

	"cpa-usage/internal/entities"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateUsageRollupBackfillStateMigrationCreatesStatusTable(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "legacy.db"))), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	defer closeOpenedDatabase(t, db)

	if err := createUsageRollupBackfillStateMigration(db); err != nil {
		t.Fatalf("create usage rollup backfill state: %v", err)
	}
	if err := createUsageRollupBackfillStateMigration(db); err != nil {
		t.Fatalf("create usage rollup backfill state should be idempotent: %v", err)
	}
	if !db.Migrator().HasTable(&entities.UsageRollupBackfillState{}) {
		t.Fatal("expected usage_rollup_backfill_states table to exist")
	}
	for _, column := range []string{"status", "target_bucket_start", "covered_bucket_start", "started_at", "completed_at", "failed_at", "last_error"} {
		if !db.Migrator().HasColumn(&entities.UsageRollupBackfillState{}, column) {
			t.Fatalf("expected usage_rollup_backfill_states.%s column to exist", column)
		}
	}

	var state entities.UsageRollupBackfillState
	if err := db.Where("name = ?", entities.UsageRollupBackfillStateName).First(&state).Error; err != nil {
		t.Fatalf("expected migration to seed pending state row: %v", err)
	}
	if state.Status != entities.UsageRollupBackfillStateStatusPending {
		t.Fatalf("expected pending state row, got %+v", state)
	}
}
