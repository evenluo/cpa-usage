package migration

import (
	"testing"

	"cpa-usage/internal/entities"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestAddUsageIdentityAuthRenewalMigrationAddsNullableJSONColumn(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(testSQLiteDSN(t.TempDir()+"/auth-renewal.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	if err := db.Exec(`CREATE TABLE usage_identities (id integer primary key, identity text)`).Error; err != nil {
		t.Fatalf("create usage identities: %v", err)
	}
	if err := addUsageIdentityAuthRenewalMigration(db); err != nil {
		t.Fatalf("add auth renewal field: %v", err)
	}
	if err := addUsageIdentityAuthRenewalMigration(db); err != nil {
		t.Fatalf("add auth renewal field idempotently: %v", err)
	}
	if !db.Migrator().HasColumn(&entities.UsageIdentity{}, "auth_renewal") {
		t.Fatal("expected usage_identities.auth_renewal")
	}
	var row struct {
		AuthRenewal *string
	}
	if err := db.Table("usage_identities").Create(map[string]any{"identity": "existing"}).Scan(&row).Error; err != nil {
		t.Fatalf("insert existing row: %v", err)
	}
	if row.AuthRenewal != nil {
		t.Fatalf("expected migrated auth renewal to stay null, got %+v", row)
	}
}
