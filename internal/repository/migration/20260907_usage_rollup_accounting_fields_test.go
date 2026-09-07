package migration

import (
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAccountingUpgradeInitializesHistoryWithoutResettingCoverage(t *testing.T) {
	for _, status := range []string{"completed", "pending", "failed"} {
		t.Run(status, func(t *testing.T) {
			db, err := gorm.Open(sqlite.Open(testSQLiteDSN(filepath.Join(t.TempDir(), "accounting-rollup.db"))), &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			defer closeOpenedDatabase(t, db)
			if err := db.Exec(`CREATE TABLE usage_rollups_hourly (id integer PRIMARY KEY, request_count integer NOT NULL, total_tokens integer NOT NULL)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.Exec(`INSERT INTO usage_rollups_hourly VALUES (1, 7, 1200), (2, 3, 0)`).Error; err != nil {
				t.Fatal(err)
			}
			if err := db.AutoMigrate(&entities.UsageRollupBackfillState{}); err != nil {
				t.Fatal(err)
			}
			at := time.Date(2026, 9, 7, 8, 0, 0, 0, time.UTC)
			state := entities.UsageRollupBackfillState{
				Name: entities.UsageRollupBackfillStateName, Status: status,
				TargetBucketStart: &at, CoveredBucketStart: &at,
				StartedAt: &at, CompletedAt: &at, FailedAt: &at, LastError: "existing observation",
			}
			if err := db.Create(&state).Error; err != nil {
				t.Fatal(err)
			}
			var before entities.UsageRollupBackfillState
			if err := db.First(&before).Error; err != nil {
				t.Fatal(err)
			}
			if err := createSchemaMigrationsTable(db); err != nil {
				t.Fatal(err)
			}
			migration := databaseMigration{version: migrationAddUsageRollupAccountingFields, run: addUsageRollupAccountingFieldsMigration}
			if err := runSchemaMigration(db, migration); err != nil {
				t.Fatal(err)
			}
			var after entities.UsageRollupBackfillState
			if err := db.First(&after).Error; err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, after) {
				t.Fatalf("coverage changed: before=%+v after=%+v", before, after)
			}
			var rows []entities.UsageRollupHourly
			if err := db.Order("id").Find(&rows).Error; err != nil {
				t.Fatal(err)
			}
			if len(rows) != 2 || rows[0].AccountingAbsentAttempts != 7 || rows[1].AccountingAbsentAttempts != 3 || rows[0].TotalTokens != 1200 {
				t.Fatalf("history initialization changed existing facts: %+v", rows)
			}
			for _, row := range rows {
				if row.AccountingValidAttempts != 0 || row.AccountingInvalidAttempts != 0 || row.CanonicalTotalTokens != 0 || row.CanonicalCompletePromptTokens != 0 || row.CanonicalCompleteCacheReadTokens != 0 || row.CanonicalCompleteOutputTokens != 0 {
					t.Fatalf("fabricated canonical observation: %+v", row)
				}
			}
			// A subsequent boot must leave newly ingested canonical data untouched.
			if err := db.Exec(`UPDATE usage_rollups_hourly SET accounting_absent_attempts = 6, accounting_valid_attempts = 1, canonical_total_tokens = 130 WHERE id = 1`).Error; err != nil {
				t.Fatal(err)
			}
			if err := runSchemaMigration(db, migration); err != nil {
				t.Fatal(err)
			}
			var row entities.UsageRollupHourly
			if err := db.First(&row, 1).Error; err != nil {
				t.Fatal(err)
			}
			if row.AccountingAbsentAttempts != 6 || row.AccountingValidAttempts != 1 || row.CanonicalTotalTokens != 130 {
				t.Fatalf("second boot reset canonical data: %+v", row)
			}
		})
	}
}
