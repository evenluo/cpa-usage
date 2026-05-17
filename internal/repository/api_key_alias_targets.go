package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"

	"gorm.io/gorm"
)

type apiKeyAliasTargetScanRow struct {
	Identity             string
	Provider             string
	RequestCount         int64
	SuccessCount         int64
	FailureCount         int64
	InputTokens          int64
	OutputTokens         int64
	ReasoningTokens      int64
	CachedTokens         int64
	TotalTokens          int64
	TotalCost            float64
	MissingPricingEvents int64
	PricedBillableEvents int64
	FirstUsedAt          string
	LastUsedAt           string
}

type ListAPIKeyAliasTargetsPageRequest struct {
	Page     int
	PageSize int
}

func ListAPIKeyAliasTargetsPage(ctx context.Context, db *gorm.DB, request ListAPIKeyAliasTargetsPageRequest) ([]dto.APIKeyAliasTargetRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database is nil")
	}

	base := db.WithContext(ctx).Model(&entities.UsageEvent{}).
		Where("TRIM(auth_type) = ?", "apikey").
		Where("TRIM(source) <> ''")

	var total int64
	if err := db.WithContext(ctx).Table("(?) AS api_keys", base.Select("TRIM(source) AS identity").Group("TRIM(source)")).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("count api key alias targets: %w", err)
	}

	page := request.Page
	if page <= 0 {
		page = 1
	}
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = 100
	}
	offset := (page - 1) * pageSize

	var rows []apiKeyAliasTargetScanRow
	if err := analyticsEventsWithPricingQuery(db.WithContext(ctx), dto.UsageQueryFilter{}).
		Select(`
			TRIM(usage_events.source) AS identity,
			COALESCE(MIN(NULLIF(TRIM(usage_events.provider), '')), '') AS provider,
			COUNT(*) AS request_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 0 ELSE 1 END), 0) AS success_count,
			COALESCE(SUM(CASE WHEN usage_events.failed THEN 1 ELSE 0 END), 0) AS failure_count,
			COALESCE(SUM(`+analyticsPositiveTokenSQLExpression("usage_events.input_tokens")+`), 0) AS input_tokens,
			COALESCE(SUM(`+analyticsPositiveTokenSQLExpression("usage_events.output_tokens")+`), 0) AS output_tokens,
			COALESCE(SUM(`+analyticsPositiveTokenSQLExpression("usage_events.reasoning_tokens")+`), 0) AS reasoning_tokens,
			COALESCE(SUM(`+analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")+`), 0) AS cached_tokens,
			COALESCE(SUM(usage_events.total_tokens), 0) AS total_tokens,
			COALESCE(SUM(`+analyticsCostSQLExpression()+`), 0) AS total_cost,
			COALESCE(SUM(`+analyticsMissingPricingSQLExpression()+`), 0) AS missing_pricing_events,
			COALESCE(SUM(`+analyticsPricedBillableSQLExpression()+`), 0) AS priced_billable_events,
			MIN(strftime('%Y-%m-%dT%H:%M:%SZ', usage_events.timestamp)) AS first_used_at,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', usage_events.timestamp)) AS last_used_at`).
		Where("TRIM(usage_events.auth_type) = ?", "apikey").
		Where("TRIM(usage_events.source) <> ''").
		Group("TRIM(usage_events.source)").
		Order("total_cost DESC").
		Order("COALESCE(SUM(usage_events.total_tokens), 0) DESC").
		Order("last_used_at DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list api key alias targets: %w", err)
	}

	result := make([]dto.APIKeyAliasTargetRecord, 0, len(rows))
	for _, row := range rows {
		result = append(result, dto.APIKeyAliasTargetRecord{
			Identity:             row.Identity,
			Provider:             row.Provider,
			RequestCount:         row.RequestCount,
			SuccessCount:         row.SuccessCount,
			FailureCount:         row.FailureCount,
			InputTokens:          row.InputTokens,
			OutputTokens:         row.OutputTokens,
			ReasoningTokens:      row.ReasoningTokens,
			CachedTokens:         row.CachedTokens,
			TotalTokens:          row.TotalTokens,
			TotalCost:            row.TotalCost,
			MissingPricingEvents: row.MissingPricingEvents,
			PricedBillableEvents: row.PricedBillableEvents,
			FirstUsedAt:          parseAnalyticsTimestamp(row.FirstUsedAt),
			LastUsedAt:           parseAnalyticsTimestamp(row.LastUsedAt),
		})
	}

	return result, total, nil
}

func ListAPIKeySources(ctx context.Context, db *gorm.DB) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	var values []string
	if err := db.WithContext(ctx).Model(&entities.UsageEvent{}).
		Select("DISTINCT TRIM(source)").
		Where("TRIM(auth_type) = ?", "apikey").
		Where("TRIM(source) <> ''").
		Order("TRIM(source) ASC").
		Pluck("source", &values).Error; err != nil {
		return nil, fmt.Errorf("list api key sources: %w", err)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}
