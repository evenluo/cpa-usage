package repository

import (
	"context"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// AnalyticsReader 把数据库句柄绑定到 analytics 读模型函数上，
// 作为 HTTP 层 AnalyticsProvider seam 的持久化 adapter；raw/rollup 选择保持在 repository 内部。
type AnalyticsReader struct {
	db *gorm.DB
}

func NewAnalyticsReader(db *gorm.DB) AnalyticsReader {
	return AnalyticsReader{db: db}
}

func (r AnalyticsReader) GetAnalyticsSummary(_ context.Context, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	return BuildAnalyticsSummaryWithFilter(r.db, filter)
}

func (r AnalyticsReader) GetAnalyticsCore(_ context.Context, filter dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error) {
	return BuildAnalyticsCoreWithFilter(r.db, filter)
}

func (r AnalyticsReader) GetAnalyticsHeatmap(_ context.Context, filter dto.UsageQueryFilter) (dto.AnalyticsHeatmap, error) {
	return BuildAnalyticsHeatmapWithFilter(r.db, filter)
}
