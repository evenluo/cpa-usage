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

func (r UsageReader) GetUsageOverview(_ context.Context, filter dto.UsageQueryFilter) (*dto.UsageOverviewRecord, error) {
	return BuildUsageOverviewWithFilter(r.db, filter)
}

func (r UsageReader) GetRequestHealth(_ context.Context, filter dto.UsageQueryFilter) (*dto.UsageOverviewHealthRecord, error) {
	return BuildUsageRequestHealthWithFilter(r.db, filter)
}

func (r UsageReader) ListUsageEvents(_ context.Context, filter dto.UsageQueryFilter) (*dto.UsageEventsPageRecord, error) {
	return ListUsageEventsWithFilter(r.db, filter)
}

func (r UsageReader) ListUsageEventFilterOptions(_ context.Context, filter dto.UsageQueryFilter) (*dto.UsageEventFilterOptionsRecord, error) {
	return ListUsageEventFilterOptionsWithFilter(r.db, filter)
}

func (r UsageReader) GetUsageAnalysis(_ context.Context, filter dto.UsageQueryFilter) ([]dto.UsageAnalysisAPIStatRecord, []dto.UsageAnalysisModelStatRecord, error) {
	return ListUsageAnalysisWithFilter(r.db, filter)
}
