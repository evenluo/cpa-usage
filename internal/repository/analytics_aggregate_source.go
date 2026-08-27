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
	requestCountExpr    string
	successSumExpr      string
	failureSumExpr      string
	inputTokensExpr     string
	outputTokensExpr    string
	reasoningTokensExpr string
	cachedTokensExpr    string
	totalTokensExpr     string
	// promptTokensExpr 是 Cost 计算里的可计费 prompt tokens：raw 源由 input-cached 推导，rollup 源已物化。
	promptTokensExpr string
	providerExpr     string
	modelExpr        string
	latencySumExpr   string
	latencyCountExpr string
	lastUsedAtExpr   string
	// firstUsedAtExpr 仅 raw events 有精确事实；rollup 不伪造 first-used。
	firstUsedAtExpr string
	// identityAuthTypeExpr/identityExpr 是 Key Alias 维度的身份列；apiKeyIdentityExpr 是 API Key 维度的身份列。
	identityAuthTypeExpr string
	identityExpr         string
	apiKeyIdentityExpr   string
	bucketExpr           func(bucketByDay bool) string
	query                func(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB
	// identityQuery/apiKeyQuery 在 query 基础上附加身份/别名 join 与非空身份过滤。
	identityQuery func(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB
	apiKeyQuery   func(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB
}

func analyticsEventsAggregateSource() analyticsAggregateSource {
	inputTokens := analyticsPositiveTokenSQLExpression("usage_events.input_tokens")
	cachedTokens := analyticsPositiveTokenSQLExpression("usage_events.cached_tokens")
	return analyticsAggregateSource{
		name:                 "events",
		requestCountExpr:     "1",
		successSumExpr:       "CASE WHEN usage_events.failed THEN 0 ELSE 1 END",
		failureSumExpr:       "CASE WHEN usage_events.failed THEN 1 ELSE 0 END",
		inputTokensExpr:      "usage_events.input_tokens",
		outputTokensExpr:     "usage_events.output_tokens",
		reasoningTokensExpr:  "usage_events.reasoning_tokens",
		cachedTokensExpr:     "usage_events.cached_tokens",
		totalTokensExpr:      "usage_events.total_tokens",
		promptTokensExpr:     "(CASE WHEN " + inputTokens + " - " + cachedTokens + " > 0 THEN " + inputTokens + " - " + cachedTokens + " ELSE 0 END)",
		providerExpr:         "TRIM(usage_events.provider)",
		modelExpr:            "TRIM(usage_events.model)",
		latencySumExpr:       "CASE WHEN usage_events.latency_ms > 0 THEN usage_events.latency_ms ELSE 0 END",
		latencyCountExpr:     "CASE WHEN usage_events.latency_ms > 0 THEN 1 ELSE 0 END",
		lastUsedAtExpr:       "usage_events.timestamp",
		firstUsedAtExpr:      "usage_events.timestamp",
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
		name:                 "rollup",
		requestCountExpr:     "usage_rollups_hourly.request_count",
		successSumExpr:       "usage_rollups_hourly.success_count",
		failureSumExpr:       "usage_rollups_hourly.failure_count",
		inputTokensExpr:      "usage_rollups_hourly.input_tokens",
		outputTokensExpr:     "usage_rollups_hourly.output_tokens",
		reasoningTokensExpr:  "usage_rollups_hourly.reasoning_tokens",
		cachedTokensExpr:     "usage_rollups_hourly.cached_tokens",
		totalTokensExpr:      "usage_rollups_hourly.total_tokens",
		promptTokensExpr:     "usage_rollups_hourly.billable_prompt_tokens",
		providerExpr:         "TRIM(usage_rollups_hourly.provider)",
		modelExpr:            "TRIM(usage_rollups_hourly.model)",
		latencySumExpr:       "usage_rollups_hourly.total_latency_ms",
		latencyCountExpr:     "usage_rollups_hourly.latency_sample_count",
		lastUsedAtExpr:       "usage_rollups_hourly.last_event_at",
		firstUsedAtExpr:      "",
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
	return analyticsCostSQLExpressionWithPromptTokens(source.promptTokensExpr, analyticsPositiveTokenSQLExpression(source.outputTokensExpr), analyticsPositiveTokenSQLExpression(source.cachedTokensExpr))
}

func analyticsSourceMissingPricingSQLExpression(source analyticsAggregateSource) string {
	return analyticsMissingPricingSQLExpressionFor(source.inputTokensExpr, source.outputTokensExpr, source.cachedTokensExpr, source.requestCountExpr)
}

func analyticsSourcePricedBillableSQLExpression(source analyticsAggregateSource) string {
	return analyticsPricedBillableSQLExpressionFor(source.inputTokensExpr, source.outputTokensExpr, source.cachedTokensExpr, source.requestCountExpr)
}

// analyticsTotalTokensDescOrder 渲染按总 token 量降序的排序片段，供各维度 breakdown 查询共用。
func analyticsTotalTokensDescOrder(source analyticsAggregateSource) string {
	return "COALESCE(SUM(" + source.totalTokensExpr + "), 0) DESC"
}

// analyticsSummaryTrendSelect 渲染 summary 与 trend 共用的基础聚合列，raw 与 rollup 两种源都由这一定义渲染；
// 各维度 breakdown 查询独立渲染各自的聚合列。trend 不消费 cache savings，逐桶渲染时跳过那三个 CASE 列。
func analyticsSummaryTrendSelect(source analyticsAggregateSource) string {
	return `
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
			COALESCE(SUM(` + analyticsSourcePricedBillableSQLExpression(source) + `), 0) AS priced_billable_events`
}

// analyticsSummarySelect 在 summary/trend 共用列基础上补 summary 专有的 cache savings 列。
func analyticsSummarySelect(source analyticsAggregateSource) string {
	return analyticsSummaryTrendSelect(source) + `,
			COALESCE(SUM(` + analyticsCacheSavingsSQLExpressionFor(source.cachedTokensExpr) + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsCacheSavingsEligibleSQLExpressionFor(source.cachedTokensExpr, source.requestCountExpr) + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsCacheSavingsIneligibleSQLExpressionFor(source.cachedTokensExpr, source.requestCountExpr) + `), 0) AS cache_savings_ineligible_rows`
}

func buildAnalyticsAggregateRow(db *gorm.DB, filter dto.AnalyticsFilter, source analyticsAggregateSource) (analyticsAggregateRow, error) {
	var row analyticsAggregateRow
	if err := source.query(db, filter).
		Select(analyticsSummarySelect(source)).
		Scan(&row).Error; err != nil {
		return analyticsAggregateRow{}, fmt.Errorf("build analytics %s summary: %w", source.name, err)
	}
	return row, nil
}

func buildAnalyticsAggregateRowsByBucket(db *gorm.DB, filter dto.AnalyticsFilter, source analyticsAggregateSource) ([]analyticsAggregateRow, error) {
	bucketExpr := source.bucketExpr(analyticsTrendBucketsByDay(filter))
	var rows []analyticsAggregateRow
	if err := source.query(db, filter).
		Select(bucketExpr + " AS bucket,\n" + analyticsSummaryTrendSelect(source)).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows).Error; err != nil {
		return nil, fmt.Errorf("build analytics %s trend: %w", source.name, err)
	}
	return rows, nil
}
