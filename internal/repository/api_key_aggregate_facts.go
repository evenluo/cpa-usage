package repository

import (
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"

	"gorm.io/gorm"
)

// apiKeyAggregateFactRow is the repository-owned aggregate fact shared by
// analytics ranking and Reference Data pagination. Caller policies stay outside
// this row and its grouped query.
type apiKeyAggregateFactRow struct {
	Identity             string
	Alias                string
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

func apiKeyAggregateFactsQuery(db *gorm.DB, scope dto.UsageTimeScope, source analyticsAggregateSource) *gorm.DB {
	return source.apiKeyQuery(db, scope).
		Select(apiKeyAggregateFactSelect(source)).
		Group(source.apiKeyIdentityExpr)
}

func apiKeyAggregateFactSelect(source analyticsAggregateSource) string {
	firstUsedAt := `'' AS first_used_at`
	if source.firstUsedAtExpr != "" {
		firstUsedAt = `MIN(strftime('%Y-%m-%dT%H:%M:%SZ', ` + source.firstUsedAtExpr + `)) AS first_used_at`
	}
	return `
			` + source.apiKeyIdentityExpr + ` AS identity,
			COALESCE(MAX(key_aliases.alias), '') AS alias,
			COALESCE(MIN(NULLIF(` + source.providerExpr + `, '')), '') AS provider,
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.inputTokensExpr) + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.outputTokensExpr) + `), 0) AS output_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.reasoningTokensExpr) + `), 0) AS reasoning_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.cachedTokensExpr) + `), 0) AS cached_tokens,
			COALESCE(SUM(` + source.totalTokensExpr + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events,
			` + firstUsedAt + `,
			MAX(strftime('%Y-%m-%dT%H:%M:%SZ', ` + source.lastUsedAtExpr + `)) AS last_used_at`
}

func analyticsIdentityAggregateRowFromAPIKeyFact(row apiKeyAggregateFactRow) analyticsIdentityAggregateRow {
	authTypeName, _ := entities.UsageIdentityAuthTypeAIProvider.CanonicalName()
	return analyticsIdentityAggregateRow{
		AuthType:             int(entities.UsageIdentityAuthTypeAIProvider),
		Identity:             row.Identity,
		Alias:                row.Alias,
		AuthTypeName:         authTypeName,
		Provider:             row.Provider,
		RequestCount:         row.RequestCount,
		SuccessCount:         row.SuccessCount,
		FailureCount:         row.FailureCount,
		TotalTokens:          row.TotalTokens,
		TotalCost:            row.TotalCost,
		MissingPricingEvents: row.MissingPricingEvents,
		PricedBillableEvents: row.PricedBillableEvents,
		LastUsedAt:           row.LastUsedAt,
	}
}
