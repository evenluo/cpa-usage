package quota

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

// Repository is quota's persistence boundary. It owns identity visibility and
// the latest successful manual observation for each auth-file identity.
type Repository interface {
	// FindActiveAuthFileIdentity 返回 auth_index 对应的活跃 auth-file 身份；found=false 表示不存在。
	FindActiveAuthFileIdentity(ctx context.Context, authIndex string) (identity entities.UsageIdentity, found bool, err error)
	// HasActiveIdentity 报告 auth_index 是否存在任意类型的活跃身份，用于区分“非 auth file”和“不存在”。
	HasActiveIdentity(ctx context.Context, authIndex string) (bool, error)
	SaveQuotaObservation(ctx context.Context, identityID uint, response CheckResponse) error
	ListQuotaObservations(ctx context.Context, authIndexes []string, limit int) ([]CheckResponse, error)
}

type repositoryAdapter struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) Repository {
	return repositoryAdapter{db: db}
}

func (r repositoryAdapter) FindActiveAuthFileIdentity(ctx context.Context, authIndex string) (entities.UsageIdentity, bool, error) {
	identity, err := repository.GetActiveAuthFileUsageIdentityByAuthIndex(ctx, r.db, authIndex)
	if err == nil {
		return identity, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entities.UsageIdentity{}, false, nil
	}
	return entities.UsageIdentity{}, false, err
}

func (r repositoryAdapter) HasActiveIdentity(ctx context.Context, authIndex string) (bool, error) {
	return repository.HasActiveUsageIdentityByAuthIndex(ctx, r.db, authIndex)
}

func (r repositoryAdapter) SaveQuotaObservation(ctx context.Context, identityID uint, response CheckResponse) error {
	if identityID == 0 || response.ObservedAt.IsZero() {
		return fmt.Errorf("invalid quota observation")
	}
	quotaJSON, err := json.Marshal(response.Quota)
	if err != nil {
		return fmt.Errorf("marshal quota observation: %w", err)
	}
	if err := repository.SaveQuotaObservation(ctx, r.db, identityID, response.ObservedAt, string(quotaJSON)); err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return fmt.Errorf("%w: identity %d", ErrNotFound, identityID)
		}
		return err
	}
	return nil
}

func (r repositoryAdapter) ListQuotaObservations(ctx context.Context, authIndexes []string, limit int) ([]CheckResponse, error) {
	if limit <= 0 {
		return nil, fmt.Errorf("%w: limit is required", ErrValidation)
	}
	requested := make([]string, 0, min(limit, len(authIndexes)))
	seen := make(map[string]struct{}, len(authIndexes))
	for _, raw := range authIndexes {
		if len(requested) >= limit {
			break
		}
		authIndex := strings.TrimSpace(raw)
		if authIndex == "" {
			continue
		}
		if _, exists := seen[authIndex]; exists {
			continue
		}
		seen[authIndex] = struct{}{}
		requested = append(requested, authIndex)
	}
	if len(requested) == 0 {
		return []CheckResponse{}, nil
	}
	rows, err := repository.ListActiveAuthFileQuotaObservations(ctx, r.db, requested)
	if err != nil {
		return nil, err
	}
	byAuthIndex := make(map[string]CheckResponse, len(rows))
	for _, row := range rows {
		var quotaRows []QuotaRow
		if err := json.Unmarshal([]byte(row.QuotaJSON), &quotaRows); err != nil {
			return nil, fmt.Errorf("decode quota observation for %q: %w", row.AuthIndex, err)
		}
		if quotaRows == nil {
			quotaRows = []QuotaRow{}
		}
		byAuthIndex[row.AuthIndex] = CheckResponse{ID: row.AuthIndex, ObservedAt: row.ObservedAt.UTC(), Quota: quotaRows}
	}
	responses := make([]CheckResponse, 0, len(rows))
	for _, authIndex := range requested {
		if response, ok := byAuthIndex[authIndex]; ok {
			responses = append(responses, response)
		}
	}
	return responses, nil
}
