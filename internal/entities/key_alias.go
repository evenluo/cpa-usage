package entities

import "time"

// KeyAlias 是 CPA Usage 本地维护的 Key 可读别名，不写回 CPA。
type KeyAlias struct {
	ID        uint                  `gorm:"primaryKey"`
	AuthType  UsageIdentityAuthType `gorm:"uniqueIndex:uniq_key_aliases_auth_type_identity;index:idx_key_aliases_auth_type"`
	Identity  string                `gorm:"uniqueIndex:uniq_key_aliases_auth_type_identity;not null"`
	Alias     string                `gorm:"not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
