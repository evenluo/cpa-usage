package quota

import (
	"context"
	"errors"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

// AuthFileIdentityLookup 是 quota 包对 usage identity 存储的窄查询口：
// provider dispatch 与 refresh 校验只需要这两种读取，不依赖具体数据库实现。
type AuthFileIdentityLookup interface {
	// FindActiveAuthFileIdentity 返回 auth_index 对应的活跃 auth-file 身份；found=false 表示不存在。
	FindActiveAuthFileIdentity(ctx context.Context, authIndex string) (identity entities.UsageIdentity, found bool, err error)
	// HasActiveIdentity 报告 auth_index 是否存在任意类型的活跃身份，用于区分“非 auth file”和“不存在”。
	HasActiveIdentity(ctx context.Context, authIndex string) (bool, error)
}

type repositoryAuthFileIdentityLookup struct {
	db *gorm.DB
}

func NewRepositoryAuthFileIdentityLookup(db *gorm.DB) AuthFileIdentityLookup {
	return repositoryAuthFileIdentityLookup{db: db}
}

func (l repositoryAuthFileIdentityLookup) FindActiveAuthFileIdentity(ctx context.Context, authIndex string) (entities.UsageIdentity, bool, error) {
	identity, err := repository.GetActiveAuthFileUsageIdentityByAuthIndex(ctx, l.db, authIndex)
	if err == nil {
		return identity, true, nil
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return entities.UsageIdentity{}, false, nil
	}
	return entities.UsageIdentity{}, false, err
}

func (l repositoryAuthFileIdentityLookup) HasActiveIdentity(ctx context.Context, authIndex string) (bool, error) {
	return repository.HasActiveUsageIdentityByAuthIndex(ctx, l.db, authIndex)
}
