package repository

import (
	"context"

	repodto "cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// RollupBackfillReader 把数据库句柄绑定到 rollup backfill 状态读模型上，
// 作为 HTTP 层 RollupBackfillStatusProvider seam 的持久化 adapter。
type RollupBackfillReader struct {
	db *gorm.DB
}

func NewRollupBackfillReader(db *gorm.DB) RollupBackfillReader {
	return RollupBackfillReader{db: db}
}

func (r RollupBackfillReader) GetRollupBackfillStatus(ctx context.Context) (repodto.RollupBackfillStatus, error) {
	return GetUsageRollupBackfillStatus(ctx, r.db)
}
