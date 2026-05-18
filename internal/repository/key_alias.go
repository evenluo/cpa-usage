package repository

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"cpa-usage/internal/entities"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const KeyAliasMaxLength = 80

var (
	ErrInvalidKeyAlias  = errors.New("invalid key alias")
	ErrKeyAliasNotFound = errors.New("key alias not found")
)

type KeyAliasKey struct {
	AuthType entities.UsageIdentityAuthType
	Identity string
}

func SetKeyAlias(ctx context.Context, db *gorm.DB, authType entities.UsageIdentityAuthType, identity string, alias string, now time.Time) (entities.KeyAlias, error) {
	var result entities.KeyAlias
	if db == nil {
		return result, fmt.Errorf("database is nil")
	}
	key, err := normalizeKeyAliasKey(authType, identity)
	if err != nil {
		return result, err
	}
	normalizedAlias, err := NormalizeKeyAlias(alias)
	if err != nil {
		return result, err
	}

	row := entities.KeyAlias{
		AuthType:  key.AuthType,
		Identity:  key.Identity,
		Alias:     normalizedAlias,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "auth_type"}, {Name: "identity"}},
		DoUpdates: clause.Assignments(map[string]any{
			"alias":      normalizedAlias,
			"updated_at": now,
		}),
	}).Create(&row).Error; err != nil {
		return result, fmt.Errorf("set key alias: %w", err)
	}

	result, err = GetKeyAlias(ctx, db, key.AuthType, key.Identity)
	if err != nil {
		return entities.KeyAlias{}, err
	}
	return result, nil
}

func GetKeyAlias(ctx context.Context, db *gorm.DB, authType entities.UsageIdentityAuthType, identity string) (entities.KeyAlias, error) {
	var alias entities.KeyAlias
	if db == nil {
		return alias, fmt.Errorf("database is nil")
	}
	key, err := normalizeKeyAliasKey(authType, identity)
	if err != nil {
		return alias, err
	}
	if err := db.WithContext(ctx).Where("auth_type = ? AND identity = ?", key.AuthType, key.Identity).First(&alias).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return alias, ErrKeyAliasNotFound
		}
		return alias, fmt.Errorf("get key alias: %w", err)
	}
	return alias, nil
}

func ClearKeyAlias(ctx context.Context, db *gorm.DB, authType entities.UsageIdentityAuthType, identity string) error {
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	key, err := normalizeKeyAliasKey(authType, identity)
	if err != nil {
		return err
	}
	if err := db.WithContext(ctx).Where("auth_type = ? AND identity = ?", key.AuthType, key.Identity).Delete(&entities.KeyAlias{}).Error; err != nil {
		return fmt.Errorf("clear key alias: %w", err)
	}
	return nil
}

func ListKeyAliases(ctx context.Context, db *gorm.DB, keys []KeyAliasKey) (map[KeyAliasKey]entities.KeyAlias, error) {
	result := make(map[KeyAliasKey]entities.KeyAlias)
	if db == nil {
		return result, fmt.Errorf("database is nil")
	}
	normalized := make([]KeyAliasKey, 0, len(keys))
	seen := make(map[KeyAliasKey]struct{}, len(keys))
	for _, raw := range keys {
		key, err := normalizeKeyAliasKey(raw.AuthType, raw.Identity)
		if err != nil {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, key)
	}
	if len(normalized) == 0 {
		return result, nil
	}

	for _, key := range normalized {
		var row entities.KeyAlias
		err := db.WithContext(ctx).Where("auth_type = ? AND identity = ?", key.AuthType, key.Identity).First(&row).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			continue
		}
		if err != nil {
			return result, fmt.Errorf("list key aliases: %w", err)
		}
		result[key] = row
	}
	return result, nil
}

func NormalizeKeyAlias(alias string) (string, error) {
	trimmed := strings.TrimSpace(alias)
	if trimmed == "" {
		return "", nil
	}
	if utf8.RuneCountInString(trimmed) > KeyAliasMaxLength {
		return "", ErrInvalidKeyAlias
	}
	return trimmed, nil
}

func normalizeKeyAliasKey(authType entities.UsageIdentityAuthType, identity string) (KeyAliasKey, error) {
	if authType != entities.UsageIdentityAuthTypeAuthFile && authType != entities.UsageIdentityAuthTypeAIProvider {
		return KeyAliasKey{}, ErrInvalidKeyAlias
	}
	trimmedIdentity := strings.TrimSpace(identity)
	if trimmedIdentity == "" {
		return KeyAliasKey{}, ErrInvalidKeyAlias
	}
	return KeyAliasKey{AuthType: authType, Identity: trimmedIdentity}, nil
}
