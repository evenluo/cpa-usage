package repository

import (
	"context"

	"cpa-usage/internal/entities"
	"gorm.io/gorm"
)

// UsageIdentityReader 把数据库句柄绑定到 usage identities 读模型函数上，
// 作为 HTTP 层 UsageIdentityProvider seam 的持久化 adapter；Reference Data 的
// CPA Key 读模型与 Request Evidence source 解析共用这一入口。
type UsageIdentityReader struct {
	db *gorm.DB
}

func NewUsageIdentityReader(db *gorm.DB) UsageIdentityReader {
	return UsageIdentityReader{db: db}
}

func (r UsageIdentityReader) ListActiveUsageIdentities(ctx context.Context) ([]entities.UsageIdentity, error) {
	return ListActiveUsageIdentities(ctx, r.db)
}

func (r UsageIdentityReader) ListActiveUsageIdentitiesPage(ctx context.Context, request ListUsageIdentitiesPageRequest) ([]entities.UsageIdentity, int64, error) {
	return ListActiveUsageIdentitiesPage(ctx, r.db, request)
}
