package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

var (
	ErrInvalidKeyAlias      = repository.ErrInvalidKeyAlias
	ErrUsageIdentityMissing = errors.New("usage identity not found")
)

type UsageIdentityAliasKey struct {
	AuthType entities.UsageIdentityAuthType
	Identity string
}

type KeyAliasProvider interface {
	ListAliasesForUsageIdentities(context.Context, []entities.UsageIdentity) (map[UsageIdentityAliasKey]string, error)
	GetUsageIdentityAlias(context.Context, uint) (string, error)
	SetUsageIdentityAlias(context.Context, uint, string) (string, error)
	ClearUsageIdentityAlias(context.Context, uint) error
}

type keyAliasService struct {
	db *gorm.DB
}

func NewKeyAliasService(db *gorm.DB) KeyAliasProvider {
	return &keyAliasService{db: db}
}

func (s *keyAliasService) ListAliasesForUsageIdentities(ctx context.Context, identities []entities.UsageIdentity) (map[UsageIdentityAliasKey]string, error) {
	result := make(map[UsageIdentityAliasKey]string)
	keys := make([]repository.KeyAliasKey, 0, len(identities))
	for _, identity := range identities {
		keys = append(keys, repository.KeyAliasKey{AuthType: identity.AuthType, Identity: identity.Identity})
	}
	aliases, err := repository.ListKeyAliases(ctx, s.db, keys)
	if err != nil {
		return result, err
	}
	for key, row := range aliases {
		result[UsageIdentityAliasKey{AuthType: key.AuthType, Identity: key.Identity}] = row.Alias
	}
	return result, nil
}

func (s *keyAliasService) GetUsageIdentityAlias(ctx context.Context, id uint) (string, error) {
	identity, err := repository.GetUsageIdentityByID(ctx, s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUsageIdentityMissing
		}
		return "", err
	}
	alias, err := repository.GetKeyAlias(ctx, s.db, identity.AuthType, identity.Identity)
	if err != nil {
		if errors.Is(err, repository.ErrKeyAliasNotFound) {
			return "", nil
		}
		return "", err
	}
	return alias.Alias, nil
}

func (s *keyAliasService) SetUsageIdentityAlias(ctx context.Context, id uint, alias string) (string, error) {
	identity, err := repository.GetUsageIdentityByID(ctx, s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrUsageIdentityMissing
		}
		return "", err
	}
	normalized, err := repository.NormalizeKeyAlias(alias)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		if err := repository.ClearKeyAlias(ctx, s.db, identity.AuthType, identity.Identity); err != nil {
			return "", err
		}
		return "", nil
	}
	row, err := repository.SetKeyAlias(ctx, s.db, identity.AuthType, identity.Identity, normalized, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("set usage identity alias: %w", err)
	}
	return row.Alias, nil
}

func (s *keyAliasService) ClearUsageIdentityAlias(ctx context.Context, id uint) error {
	identity, err := repository.GetUsageIdentityByID(ctx, s.db, id)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUsageIdentityMissing
		}
		return err
	}
	return repository.ClearKeyAlias(ctx, s.db, identity.AuthType, identity.Identity)
}
