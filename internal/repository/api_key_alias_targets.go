package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"

	"gorm.io/gorm"
)

type ListAPIKeyAliasTargetsPageRequest struct {
	Page     int
	PageSize int
}

func ListAPIKeyAliasTargetsPage(ctx context.Context, db *gorm.DB, request ListAPIKeyAliasTargetsPageRequest) ([]dto.APIKeyAliasTargetRecord, int64, error) {
	if db == nil {
		return nil, 0, fmt.Errorf("database is nil")
	}

	identityExpr := analyticsAPIKeyIdentitySQLExpression()
	base := db.WithContext(ctx).Model(&entities.UsageEvent{}).
		Where(identityExpr + " <> ''")

	var total int64
	if err := db.WithContext(ctx).Table("(?) AS api_keys", base.Select(identityExpr+" AS identity").Group(identityExpr)).
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

	var rows []apiKeyAggregateFactRow
	source := analyticsEventsAggregateSource()
	if err := apiKeyAggregateFactsQuery(db.WithContext(ctx), dto.UsageTimeScope{}, source).
		Order("total_cost DESC").
		Order(analyticsTotalTokensDescOrder(source)).
		Order("last_used_at DESC").
		Limit(pageSize).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, fmt.Errorf("list api key alias targets: %w", err)
	}

	result := make([]dto.APIKeyAliasTargetRecord, 0, len(rows))
	for _, row := range rows {
		cost := assessCostCompleteness(row.MissingPricingEvents, row.PricedBillableEvents)
		result = append(result, dto.APIKeyAliasTargetRecord{
			Identity:        row.Identity,
			Alias:           row.Alias,
			Provider:        row.Provider,
			RequestCount:    row.RequestCount,
			SuccessCount:    row.SuccessCount,
			FailureCount:    row.FailureCount,
			InputTokens:     row.InputTokens,
			OutputTokens:    row.OutputTokens,
			ReasoningTokens: row.ReasoningTokens,
			CachedTokens:    row.CachedTokens,
			TotalTokens:     row.TotalTokens,
			TotalCost:       row.TotalCost,
			CostAvailable:   cost.Available,
			CostStatus:      cost.Status,
			FirstUsedAt:     parseAnalyticsTimestamp(row.FirstUsedAt),
			LastUsedAt:      parseAnalyticsTimestamp(row.LastUsedAt),
		})
	}

	return result, total, nil
}

func ListAPIKeySources(ctx context.Context, db *gorm.DB) ([]string, error) {
	if db == nil {
		return nil, fmt.Errorf("database is nil")
	}
	var values []string
	identityExpr := analyticsAPIKeyIdentitySQLExpression()
	var rows []struct {
		Identity string
	}
	if err := db.WithContext(ctx).Model(&entities.UsageEvent{}).
		Select("DISTINCT " + identityExpr + " AS identity").
		Where(identityExpr + " <> ''").
		Order("identity ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("list api key sources: %w", err)
	}
	values = make([]string, 0, len(rows))
	for _, row := range rows {
		values = append(values, row.Identity)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return result, nil
}
