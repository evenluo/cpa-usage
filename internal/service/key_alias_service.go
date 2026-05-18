package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/repository"
	repodto "cpa-usage/internal/repository/dto"

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

type ListAPIKeyAliasTargetsRequest struct {
	Page     int
	PageSize int
}

type APIKeyAliasTarget struct {
	ID              string
	Identity        string
	Alias           string
	Provider        string
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
	TotalCost       float64
	CostAvailable   bool
	CostStatus      string
	FirstUsedAt     *time.Time
	LastUsedAt      *time.Time
}

type ListAPIKeyAliasTargetsResponse struct {
	Items []APIKeyAliasTarget
	Total int64
}

type KeyAliasProvider interface {
	ListAliasesForUsageIdentities(context.Context, []entities.UsageIdentity) (map[UsageIdentityAliasKey]string, error)
	ListAPIKeyAliasTargetsPage(context.Context, ListAPIKeyAliasTargetsRequest) (ListAPIKeyAliasTargetsResponse, error)
	GetUsageIdentityAlias(context.Context, uint) (string, error)
	SetUsageIdentityAlias(context.Context, uint, string) (string, error)
	ClearUsageIdentityAlias(context.Context, uint) error
	SetAPIKeyAlias(context.Context, string, string) (string, error)
	ClearAPIKeyAlias(context.Context, string) error
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

func (s *keyAliasService) ListAPIKeyAliasTargetsPage(ctx context.Context, request ListAPIKeyAliasTargetsRequest) (ListAPIKeyAliasTargetsResponse, error) {
	rows, total, err := repository.ListAPIKeyAliasTargetsPage(ctx, s.db, repository.ListAPIKeyAliasTargetsPageRequest{
		Page:     request.Page,
		PageSize: request.PageSize,
	})
	if err != nil {
		return ListAPIKeyAliasTargetsResponse{}, err
	}
	keys := make([]repository.KeyAliasKey, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, repository.KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: row.Identity})
	}
	aliases, err := repository.ListKeyAliases(ctx, s.db, keys)
	if err != nil {
		return ListAPIKeyAliasTargetsResponse{}, err
	}
	items := make([]APIKeyAliasTarget, 0, len(rows))
	for _, row := range rows {
		alias := aliases[repository.KeyAliasKey{AuthType: entities.UsageIdentityAuthTypeAIProvider, Identity: row.Identity}].Alias
		items = append(items, mapAPIKeyAliasTarget(row, alias))
	}
	return ListAPIKeyAliasTargetsResponse{Items: items, Total: total}, nil
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

func (s *keyAliasService) SetAPIKeyAlias(ctx context.Context, apiKeyID string, alias string) (string, error) {
	identity, err := s.resolveAPIKeyAliasIdentity(ctx, apiKeyID)
	if err != nil {
		return "", err
	}
	normalized, err := repository.NormalizeKeyAlias(alias)
	if err != nil {
		return "", err
	}
	if normalized == "" {
		if err := repository.ClearKeyAlias(ctx, s.db, entities.UsageIdentityAuthTypeAIProvider, identity); err != nil {
			return "", err
		}
		return "", nil
	}
	row, err := repository.SetKeyAlias(ctx, s.db, entities.UsageIdentityAuthTypeAIProvider, identity, normalized, time.Now().UTC())
	if err != nil {
		return "", fmt.Errorf("set api key alias: %w", err)
	}
	return row.Alias, nil
}

func (s *keyAliasService) ClearAPIKeyAlias(ctx context.Context, apiKeyID string) error {
	identity, err := s.resolveAPIKeyAliasIdentity(ctx, apiKeyID)
	if err != nil {
		return err
	}
	return repository.ClearKeyAlias(ctx, s.db, entities.UsageIdentityAuthTypeAIProvider, identity)
}

func (s *keyAliasService) resolveAPIKeyAliasIdentity(ctx context.Context, apiKeyID string) (string, error) {
	sources, err := repository.ListAPIKeySources(ctx, s.db)
	if err != nil {
		return "", err
	}
	for _, source := range sources {
		if redact.APIAlias(source) == apiKeyID {
			return source, nil
		}
	}
	return "", ErrUsageIdentityMissing
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

func mapAPIKeyAliasTarget(row repodto.APIKeyAliasTargetRecord, alias string) APIKeyAliasTarget {
	costAvailable, costStatus := analyticsCostAvailability(row.MissingPricingEvents, row.PricedBillableEvents)
	return APIKeyAliasTarget{
		ID:              redact.APIAlias(row.Identity),
		Identity:        redact.APIKeyDisplayName(row.Identity),
		Alias:           alias,
		Provider:        row.Provider,
		TotalRequests:   row.RequestCount,
		SuccessCount:    row.SuccessCount,
		FailureCount:    row.FailureCount,
		InputTokens:     row.InputTokens,
		OutputTokens:    row.OutputTokens,
		ReasoningTokens: row.ReasoningTokens,
		CachedTokens:    row.CachedTokens,
		TotalTokens:     row.TotalTokens,
		TotalCost:       row.TotalCost,
		CostAvailable:   costAvailable,
		CostStatus:      costStatus,
		FirstUsedAt:     row.FirstUsedAt,
		LastUsedAt:      row.LastUsedAt,
	}
}

func analyticsCostAvailability(missingPricingEvents int64, pricedBillableEvents int64) (bool, string) {
	if missingPricingEvents == 0 {
		return true, "available"
	}
	if pricedBillableEvents > 0 {
		return false, "partial"
	}
	return false, "unavailable"
}
