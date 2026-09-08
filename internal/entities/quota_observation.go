package entities

import "time"

// QuotaObservation persists the latest successful manual quota observation for
// one usage identity. Provider credentials and auth-file payloads never cross
// this storage boundary.
type QuotaObservation struct {
	IdentityID uint          `gorm:"primaryKey"`
	Identity   UsageIdentity `gorm:"constraint:OnUpdate:CASCADE,OnDelete:CASCADE;foreignKey:IdentityID;references:ID"`
	ObservedAt time.Time     `gorm:"not null"`
	QuotaJSON  string        `gorm:"column:quota;type:text;not null"`
}
