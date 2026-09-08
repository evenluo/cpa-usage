package repository

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type QuotaObservationRecord struct {
	AuthIndex  string
	ObservedAt time.Time
	QuotaJSON  string
}

func SaveQuotaObservation(ctx context.Context, db *gorm.DB, identityID uint, observedAt time.Time, quotaJSON string) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if identityID == 0 || observedAt.IsZero() || strings.TrimSpace(quotaJSON) == "" {
		return fmt.Errorf("invalid quota observation")
	}
	return db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var activeIdentityCount int64
		if err := tx.Model(&entities.UsageIdentity{}).
			Where("id = ? AND auth_type = ? AND is_deleted = ?", identityID, entities.UsageIdentityAuthTypeAuthFile, false).
			Count(&activeIdentityCount).Error; err != nil {
			return fmt.Errorf("check quota observation identity: %w", err)
		}
		if activeIdentityCount == 0 {
			return gorm.ErrRecordNotFound
		}
		observation := entities.QuotaObservation{
			IdentityID: identityID,
			ObservedAt: observedAt.UTC(),
			QuotaJSON:  quotaJSON,
		}
		if err := tx.Omit("Identity").Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "identity_id"}},
			DoUpdates: clause.Assignments(map[string]any{
				"observed_at": gorm.Expr("excluded.observed_at"),
				"quota":       gorm.Expr("excluded.quota"),
			}),
			Where: clause.Where{Exprs: []clause.Expression{clause.Expr{SQL: "excluded.observed_at > quota_observations.observed_at"}}},
		}).Create(&observation).Error; err != nil {
			return fmt.Errorf("save quota observation: %w", err)
		}
		return nil
	})
}

func ListActiveAuthFileQuotaObservations(ctx context.Context, db *gorm.DB, authIndexes []string) ([]QuotaObservationRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if len(authIndexes) == 0 {
		return []QuotaObservationRecord{}, nil
	}
	var rows []QuotaObservationRecord
	if err := db.WithContext(ctx).Table("quota_observations").
		Select("usage_identities.identity AS auth_index, quota_observations.observed_at, quota_observations.quota AS quota_json").
		Joins("JOIN usage_identities ON usage_identities.id = quota_observations.identity_id").
		Where("usage_identities.auth_type = ? AND usage_identities.is_deleted = ?", entities.UsageIdentityAuthTypeAuthFile, false).
		Where("usage_identities.identity IN ?", authIndexes).
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("list quota observations: %w", err)
	}
	return rows, nil
}
