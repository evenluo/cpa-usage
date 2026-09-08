package migration

import (
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestCreateQuotaObservationsMigrationPreservesIdentitySchemaAndSuccessfulObservation(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(t.TempDir()+"/quota-observations.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { closeOpenedDatabase(t, db) })
	// This migration owns only the observation table. Existing identities must
	// not be implicitly rebuilt from today's entity model through associations.
	if err := db.Exec(`CREATE TABLE usage_identities (id INTEGER PRIMARY KEY, identity TEXT NOT NULL, operator_note TEXT)`).Error; err != nil {
		t.Fatalf("create existing identity table: %v", err)
	}
	if err := db.Exec(`INSERT INTO usage_identities (id, identity, operator_note) VALUES (7, 'existing-auth', 'preserve me')`).Error; err != nil {
		t.Fatalf("insert existing identity: %v", err)
	}
	var schemaBefore string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'usage_identities'`).Scan(&schemaBefore).Error; err != nil {
		t.Fatalf("read identity schema: %v", err)
	}
	if err := createQuotaObservationsMigration(db); err != nil {
		t.Fatalf("create observation table: %v", err)
	}
	var schemaAfter string
	if err := db.Raw(`SELECT sql FROM sqlite_master WHERE type = 'table' AND name = 'usage_identities'`).Scan(&schemaAfter).Error; err != nil {
		t.Fatalf("read identity schema after migration: %v", err)
	}
	if schemaAfter != schemaBefore {
		t.Fatalf("observation migration changed unrelated identity schema: before=%s after=%s", schemaBefore, schemaAfter)
	}
	var note string
	if err := db.Raw(`SELECT operator_note FROM usage_identities WHERE id = 7`).Scan(&note).Error; err != nil || note != "preserve me" {
		t.Fatalf("identity data changed: note=%q err=%v", note, err)
	}
	var count int64
	if err := db.Model(&entities.QuotaObservation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("migration must not fabricate observations: count=%d err=%v", count, err)
	}
	observedAt := time.Date(2026, 9, 8, 1, 2, 3, 123456789, time.UTC)
	observation := entities.QuotaObservation{
		IdentityID: 7,
		ObservedAt: observedAt,
		QuotaJSON:  `[{"key":"5h","usedPercent":25}]`,
	}
	if err := db.Omit("Identity").Create(&observation).Error; err != nil {
		t.Fatalf("save successful observation: %v", err)
	}
	if err := createQuotaObservationsMigration(db); err != nil {
		t.Fatalf("repeat observation migration: %v", err)
	}
	var retained entities.QuotaObservation
	if err := db.First(&retained, "identity_id = ?", 7).Error; err != nil {
		t.Fatalf("read observation after repeated migration: %v", err)
	}
	if !retained.ObservedAt.Equal(observedAt) || retained.QuotaJSON != observation.QuotaJSON {
		t.Fatalf("repeat migration changed successful observation: %+v", retained)
	}
	if err := db.Exec(`DELETE FROM usage_identities WHERE id = 7`).Error; err != nil {
		t.Fatalf("delete identity: %v", err)
	}
	if err := db.Model(&entities.QuotaObservation{}).Count(&count).Error; err != nil || count != 0 {
		t.Fatalf("hard-deleted identity retained an orphan observation: count=%d err=%v", count, err)
	}
}
