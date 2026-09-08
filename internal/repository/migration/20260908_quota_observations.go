package migration

import (
	"fmt"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

func createQuotaObservationsMigration(tx *gorm.DB) error {
	if !tx.Migrator().HasTable(&entities.UsageIdentity{}) {
		return fmt.Errorf("missing usage_identities table")
	}
	if tx.Migrator().HasTable(&entities.QuotaObservation{}) {
		return nil
	}
	if err := tx.Exec(`CREATE TABLE quota_observations (
		identity_id INTEGER PRIMARY KEY,
		observed_at DATETIME NOT NULL,
		quota TEXT NOT NULL,
		CONSTRAINT fk_quota_observations_identity
			FOREIGN KEY (identity_id) REFERENCES usage_identities(id)
			ON UPDATE CASCADE ON DELETE CASCADE
	)`).Error; err != nil {
		return fmt.Errorf("create quota observations: %w", err)
	}
	return nil
}
