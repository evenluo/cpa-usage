package repository

import (
	"fmt"

	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// analyticsAggregateSource 描述一个分析聚合数据源（raw usage_events 或 usage_rollups_hourly）
// 的表列映射与查询构造，使同一份聚合定义可以按源渲染，避免 raw/rollup 双份 SELECT 漂移。
type analyticsAggregateSource struct {
	name string
	// requestCountExpr 是单行代表的请求数：raw 源每行一条请求，rollup 源每行聚合 request_count 条。
	requestCountExpr string
	successSumExpr   string
	failureSumExpr   string
	inputTokens      string
	outputTokens     string
	reasoningTokens  string
	cachedTokens     string
	totalTokens      string
	// promptTokensExpr 是 Cost 计算里的可计费 prompt tokens：raw 源由 input-cached 推导，rollup 源已物化。
	promptTokensExpr string
	providerExpr     string
	modelExpr        string
	latencySumExpr   string
	latencyCountExpr string
	lastUsedAtExpr   string
	// identityAuthTypeExpr/identityExpr 是 Key Alias 维度的身份列；apiKeyIdentityExpr 是 API Key 维度的身份列。
	identityAuthTypeExpr string
	identityExpr         string
	apiKeyIdentityExpr   string
	bucketExpr           func(bucketByDay bool) string
	query                func(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB
	// identityQuery/apiKeyQuery 在 query 基础上附加身份/别名 join 与非空身份过滤。
	identityQuery func(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB
	apiKeyQuery   func(db *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB
}

func analyticsEventsAggregateSource() analyticsAggregateSource {
	inputTokens := analyticsPositiveTokenSQLExpression("usage_events.input_tokens")
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	return analyticsAggregateSource{
		name:             "events",
		requestCountExpr: "1",
		successSumExpr:   "CASE WHEN usage_events.failed THEN 0 ELSE 1 END",
		failureSumExpr:   "CASE WHEN usage_events.failed THEN 1 ELSE 0 END",
		inputTokens:      "usage_events.input_tokens",
		outputTokens:     "usage_events.output_tokens",
		reasoningTokens:  "usage_events.reasoning_tokens",
		cachedTokens:     "usage_events.cached_tokens",
		totalTokens:      "usage_events.total_tokens",
		promptTokensExpr: "(CASE WHEN " + inputTokens + " - " + cachedTokens + " > 0 THEN " + inputTokens + " - " + cachedTokens + " ELSE 0 END)",
		providerExpr:     "TRIM(usage_events.provider)",
		modelExpr:        "TRIM(usage_events.model)",
		latencySumExpr:   "CASE WHEN usage_events.latency_ms > 0 THEN usage_events.latency_ms ELSE 0 END",
		latencyCountExpr: "CASE WHEN usage_events.latency_ms > 0 THEN 1 ELSE 0 END",
		lastUsedAtExpr:   "usage_events.timestamp",
		identityAuthTypeExpr: analyticsUsageIdentityAuthTypeSQLExpression(),
		identityExpr:         analyticsUsageIdentitySQLExpression(),
		apiKeyIdentityExpr:   analyticsAPIKeyIdentitySQLExpression(),
		bucketExpr:           analyticsBucketSQLExpression,
		query:                analyticsEventsWithPricingQuery,
		identityQuery:        analyticsIdentityEventsWithPricingQuery,
		apiKeyQuery:          analyticsAPIKeyEventsWithPricingQuery,
	}
}

func analyticsRollupsAggregateSource() analyticsAggregateSource {
	return analyticsAggregateSource{
		name:             "rollup",
		requestCountExpr: "usage_rollups_hourly.request_count",
		successSumExpr:   "usage_rollups_hourly.success_count",
		failureSumExpr:   "usage_rollups_hourly.failure_count",
		inputTokens:      "usage_rollups_hourly.input_tokens",
		outputTokens:     "usage_rollups_hourly.output_tokens",
		reasoningTokens:  "usage_rollups_hourly.reasoning_tokens",
		cachedTokens:     "usage_rollups_hourly.cached_tokens",
		totalTokens:      "usage_rollups_hourly.total_tokens",
		promptTokensExpr: "usage_rollups_hourly.billable_prompt_tokens",
		providerExpr:     "TRIM(usage_rollups_hourly.provider)",
		modelExpr:        "TRIM(usage_rollups_hourly.model)",
		latencySumExpr:   "usage_rollups_hourly.total_latency_ms",
		latencyCountExpr: "usage_rollups_hourly.latency_sample_count",
		lastUsedAtExpr:   "usage_rollups_hourly.last_event_at",
		identityAuthTypeExpr: analyticsRollupUsageIdentityAuthTypeSQLExpression(),
		identityExpr:         analyticsRollupUsageIdentitySQLExpression(),
		apiKeyIdentityExpr:   analyticsRollupAPIKeyIdentitySQLExpression(),
		bucketExpr:           analyticsRollupBucketSQLExpression,
		query:                analyticsRollupsWithPricingQuery,
		identityQuery:        analyticsRollupIdentityWithPricingQuery,
		apiKeyQuery:          analyticsRollupAPIKeyWithPricingQuery,
	}
}

// analyticsSourceCostSQLExpression 渲染该源的 Cost 表达式。
func analyticsSourceCostSQLExpression(source analyticsAggregateSource) string {
	return analyticsCostSQLExpressionWithPromptTokens(source.promptTokensExpr, analyticsPositiveTokenSQLExpression(source.outputTokens), analyticsPositiveTokenSQLExpression(source.cachedTokens))
}

func analyticsSourceMissingPricingSQLExpression(source analyticsAggregateSource) string {
	return analyticsMissingPricingSQLExpressionFor(source.inputTokens, source.outputTokens, source.cachedTokens, source.requestCountExpr)
}

func analyticsSourcePricedBillableSQLExpression(source analyticsAggregateSource) string {
	return analyticsPricedBillableSQLExpressionFor(source.inputTokens, source.outputTokens, source.cachedTokens, source.requestCountExpr)
}

// analyticsAggregateSelect 渲染 summary/trend 共用的聚合列，是 raw 与 rollup 唯一的聚合定义处。
func analyticsAggregateSelect(source analyticsAggregateSource) string {
	return `
			COALESCE(SUM(` + source.requestCountExpr + `), 0) AS request_count,
			COALESCE(SUM(` + source.successSumExpr + `), 0) AS success_count,
			COALESCE(SUM(` + source.failureSumExpr + `), 0) AS failure_count,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.inputTokens) + `), 0) AS input_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.outputTokens) + `), 0) AS output_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.reasoningTokens) + `), 0) AS reasoning_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.cachedTokens) + `), 0) AS cached_tokens,
			COALESCE(SUM(` + source.totalTokens + `), 0) AS total_tokens,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpressionFor(source.cachedTokens) + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpressionFor(source.cachedTokens, source.requestCountExpr) + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpressionFor(source.cachedTokens, source.requestCountExpr) + `), 0) AS cache_savings_ineligible_rows,
			COALESCE(SUM(` + analyticsSourceCostSQLExpression(source) + `), 0) AS total_cost,
			COALESCE(SUM(` + analyticsSourceMissingPricingSQLExpression(source) + `), 0) AS missing_pricing_events,
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`
}

func buildAnalyticsAggregateRow(db *gorm.DB, filter dto.UsageQueryFilter, source analyticsAggregateSource) (analyticsAggregateRow, error) {
	var row analyticsAggregateRow
	if err := source.query(db, filter).
		Select(analyticsAggregateSelect(source)).
		Scan(&row).Error; err != nil {
		return analyticsAggregateRow{}, fmt.Errorf("build analytics %s summary: %w", source.name, err)
	}
	return row, nil
}

func buildAnalyticsAggregateRowsByBucket(db *gorm.DB, filter dto.UsageQueryFilter, source analyticsAggregateSource) ([]analyticsAggregateRow, error) {
	bucketExpr := source.bucketExpr(analyticsTrendBucketsByDay(filter))
	var rows []analyticsAggregateRow
	if err := source.query(db, filter).
		Select(bucketExpr + " AS bucket,\n" + analyticsAggregateSelect(source)).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s trend: %w", source.name, err)
	}
	return rows, nil
}
