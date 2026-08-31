package repository

import (
	"context"
	"errors"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

// Reader seam 必须把取消信号传到 SQL 层：已取消的 ctx 应让查询返回 context.Canceled，而不是照常执行。
func TestUsageReaderListUsageEventsHonorsCancelledContext(t *testing.T) {
	db := openTestDatabase(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewUsageReader(db).ListUsageEvents(ctx, dto.UsageEventListFilter{Page: 1, PageSize: 10})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestAnalyticsReaderGetAnalyticsSummaryHonorsCancelledContext(t *testing.T) {
	db := openTestDatabase(t)
	end := time.Date(2026, 7, 20, 12, 0, 0, 0, time.UTC)
	start := end.Add(-24 * time.Hour)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := NewAnalyticsReader(db).GetAnalyticsSummary(ctx, dto.AnalyticsFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}, Range: "24h", FixedWindowEnd: &end})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}
