package repository

import (
	"context"
	"fmt"
	"sort"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

type usagePerformanceProviderCountRow struct {
	Provider     string
	RequestCount int64
}

// ListUsagePerformanceProvidersWithFilter returns every non-blank provider in
// one bounded window. It reuses the analytics raw/hourly source plan while
// selecting only provider and request count.
func ListUsagePerformanceProvidersWithFilter(ctx context.Context, db *gorm.DB, scope dto.UsageTimeScope) (*dto.UsagePerformanceProviderOptionsRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if scope.StartTime == nil || scope.EndTime == nil {
		return nil, fmt.Errorf("performance providers require bounded start and end times")
	}

	filter := dto.AnalyticsFilter{
		UsageTimeScope: scope,
		Range:          "24h",
		FixedWindowEnd: scope.EndTime,
		Granularity:    "hour",
	}
	plan := analyticsCoreRollupWindowPlan(filter)
	if plan.rollupFilter != nil {
		allowed, detail, err := analyticsRollupReadAllowed(ctx, db.WithContext(ctx), *plan.rollupFilter)
		if err != nil {
			return nil, err
		}
		if !allowed {
			logAnalyticsRawFallback("performance providers raw fallback", filter, detail)
			plan = analyticsCoreRawWindowPlan(filter)
		}
	}

	combined := map[string]int64{}
	if err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, rawFilter := range plan.rawFilters {
			rows, err := loadUsagePerformanceProviderCounts(tx, rawFilter, analyticsEventsAggregateSource())
			if err != nil {
				return err
			}
			addUsagePerformanceProviderCounts(combined, rows)
		}
		if plan.rollupFilter != nil {
			rows, err := loadUsagePerformanceProviderCounts(tx, *plan.rollupFilter, analyticsRollupsAggregateSource())
			if err != nil {
				return err
			}
			addUsagePerformanceProviderCounts(combined, rows)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	options := make([]dto.UsagePerformanceProviderOptionRecord, 0, len(combined))
	for provider, requestCount := range combined {
		options = append(options, dto.UsagePerformanceProviderOptionRecord{Provider: provider, RequestCount: requestCount})
	}
	sort.Slice(options, func(i, j int) bool {
		if options[i].RequestCount != options[j].RequestCount {
			return options[i].RequestCount > options[j].RequestCount
		}
		return options[i].Provider < options[j].Provider
	})
	return &dto.UsagePerformanceProviderOptionsRecord{ProviderOptions: options}, nil
}

func loadUsagePerformanceProviderCounts(db *gorm.DB, filter dto.AnalyticsFilter, source analyticsAggregateSource) ([]usagePerformanceProviderCountRow, error) {
	var rows []usagePerformanceProviderCountRow
	if err := source.baseQuery(db, filter).
		Select(source.providerExpr + " AS provider, COALESCE(SUM(" + source.requestCountExpr + "), 0) AS request_count").
		Where(source.providerExpr + " <> ''").
		Group(source.providerExpr).
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("load %s performance provider counts: %w", source.name, err)
	}
	return rows, nil
}

func addUsagePerformanceProviderCounts(dst map[string]int64, rows []usagePerformanceProviderCountRow) {
	for _, row := range rows {
		dst[row.Provider] += row.RequestCount
	}
}
