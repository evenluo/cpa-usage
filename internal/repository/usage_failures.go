package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

const failureStatusCategorySQL = `CASE
	WHEN status_code = 0 THEN 'unknown'
	WHEN status_code BETWEEN 100 AND 199 THEN '1xx'
	WHEN status_code BETWEEN 200 AND 299 THEN '2xx'
	WHEN status_code BETWEEN 300 AND 399 THEN '3xx'
	WHEN status_code BETWEEN 400 AND 499 THEN '4xx'
	WHEN status_code BETWEEN 500 AND 599 THEN '5xx'
	ELSE 'other' END`

// BuildUsageFailureDistributionWithFilter returns independent, bounded
// breakdowns for failed attempts selected by the shared diagnostic filter.
func BuildUsageFailureDistributionWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageDiagnosticFilter) (*dto.UsageFailureDistributionRecord, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		return nil, fmt.Errorf("failure distribution requires bounded start and end times")
	}
	base := func() *gorm.DB {
		return applyUsageDiagnosticQuery(
			db.WithContext(ctx).Table("usage_events INDEXED BY idx_usage_events_timestamp_id"),
			filter,
		).Where("failed = ?", true)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, fmt.Errorf("count failed usage attempts: %w", err)
	}

	categories, err := loadUsageFailureBreakdown(base(), failureStatusCategorySQL, "")
	if err != nil {
		return nil, fmt.Errorf("load failure categories: %w", err)
	}
	statuses, err := loadUsageFailureBreakdown(base(), "CAST(status_code AS TEXT)", "status_code = 0 OR status_code BETWEEN 100 AND 599")
	if err != nil {
		return nil, fmt.Errorf("load failure statuses: %w", err)
	}
	providers, err := loadUsageFailureBreakdown(base(), "TRIM(provider)", "TRIM(provider) <> ''")
	if err != nil {
		return nil, fmt.Errorf("load failure providers: %w", err)
	}
	accounts, err := loadUsageFailureBreakdown(base(), "TRIM(auth_index)", "TRIM(auth_index) <> ''")
	if err != nil {
		return nil, fmt.Errorf("load failure accounts: %w", err)
	}
	models, err := loadUsageFailureBreakdown(base(), "TRIM(model)", "TRIM(model) <> ''")
	if err != nil {
		return nil, fmt.Errorf("load failure models: %w", err)
	}
	endpoints, err := loadUsageFailureBreakdown(base(), publicUsageEndpointSQL, publicUsageEndpointSQL+" <> ''")
	if err != nil {
		return nil, fmt.Errorf("load failure endpoints: %w", err)
	}

	return &dto.UsageFailureDistributionRecord{
		TotalFailures: total,
		Categories:    withUsageFailureOtherCount(categories, total),
		Statuses:      withUsageFailureOtherCount(statuses, total),
		Providers:     withUsageFailureOtherCount(providers, total),
		Accounts:      withUsageFailureOtherCount(accounts, total),
		Models:        withUsageFailureOtherCount(models, total),
		Endpoints:     withUsageFailureOtherCount(endpoints, total),
	}, nil
}

func loadUsageFailureBreakdown(base *gorm.DB, expression, predicate string) ([]dto.UsageFailureBreakdownItemRecord, error) {
	query := base.Select(expression + " AS value, COUNT(*) AS count")
	if predicate != "" {
		query = query.Where(predicate)
	}
	var rows []dto.UsageFailureBreakdownItemRecord
	if err := query.Group(expression).
		Order("count DESC, value ASC").
		Limit(dto.UsageFailureBreakdownLimit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	for index := range rows {
		rows[index].Value = strings.TrimSpace(rows[index].Value)
		if rows[index].Value == "0" && expression == "CAST(status_code AS TEXT)" {
			rows[index].Value = "unknown"
		}
	}
	return rows, nil
}

func withUsageFailureOtherCount(items []dto.UsageFailureBreakdownItemRecord, total int64) dto.UsageFailureBreakdownRecord {
	visible := int64(0)
	for _, item := range items {
		visible += item.Count
	}
	return dto.UsageFailureBreakdownRecord{Items: items, OtherCount: max(total-visible, 0)}
}
