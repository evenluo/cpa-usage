package migration

import (
	"testing"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageIdentityPassiveQuotaMigrationAddsNullableJSONColumns(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(t.TempDir()+"/passive-quota.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_identities (id integer primary key, identity text)`).Error; err != nil {
		t.Fatalf("create legacy usage identities: %v", err)
	}
	if err := addUsageIdentityPassiveQuotaMigration(db); err != nil {
		t.Fatalf("add passive quota fields: %v", err)
	}
	if err := addUsageIdentityPassiveQuotaMigration(db); err != nil {
		t.Fatalf("add passive quota fields idempotently: %v", err)
	}
	for _, column := range []string{"passive_quota", "passive_model_quotas"} {
		if !db.Migrator().HasColumn(&entities.UsageIdentity{}, column) {
			t.Fatalf("expected usage_identities.%s", column)
		}
	}
	var row struct {
		PassiveQuota       *string
		PassiveModelQuotas *string
	}
	if err := db.Table("usage_identities").Create(map[string]any{"identity": "legacy"}).Scan(&row).Error; err != nil {
		t.Fatalf("insert legacy row: %v", err)
	}
	if row.PassiveQuota != nil || row.PassiveModelQuotas != nil {
		t.Fatalf("expected migrated fields to stay null, got %+v", row)
	}
}
