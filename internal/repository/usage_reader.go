package repository

import (
	"context"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// UsageReader 把数据库句柄绑定到 usage 读模型函数上，
// 作为 HTTP 层 UsageProvider seam 的持久化 adapter；Overview、Request Evidence 与 Analysis 共用这一入口。
type UsageReader struct {
	db *gorm.DB
}

func NewUsageReader(db *gorm.DB) UsageReader {
	return UsageReader{db: db}
}

func (r UsageReader) GetUsageOverview(ctx context.Context, filter dto.UsageOverviewFilter) (*dto.UsageOverviewRecord, error) {
	return BuildUsageOverviewWithFilter(ctx, r.db, filter)
}

func (r UsageReader) GetRequestHealth(ctx context.Context, filter dto.UsageOverviewFilter) (*dto.UsageOverviewHealthRecord, error) {
	return BuildUsageRequestHealthWithFilter(ctx, r.db, filter)
}

func (r UsageReader) ListUsageEvents(ctx context.Context, filter dto.UsageEventListFilter) (*dto.UsageEventsPageRecord, error) {
	return ListUsageEventsWithFilter(ctx, r.db, filter)
}

func (r UsageReader) ListUsageEventFilterOptions(ctx context.Context, filter dto.UsageTimeScope) (*dto.UsageEventFilterOptionsRecord, error) {
	return ListUsageEventFilterOptionsWithFilter(ctx, r.db, filter)
}

func (r UsageReader) GetUsageAnalysis(ctx context.Context, filter dto.UsageTimeScope) ([]dto.UsageAnalysisAPIStatRecord, []dto.UsageAnalysisModelStatRecord, error) {
	return ListUsageAnalysisWithFilter(ctx, r.db, filter)
}
