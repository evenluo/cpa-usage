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
	requestCountExpr                 string
	successSumExpr                   string
	failureSumExpr                   string
	inputTokensExpr                  string
	outputTokensExpr                 string
	reasoningTokensExpr              string
	cachedTokensExpr                 string
	cacheReadTokensExpr              string
	cacheReadObservedInputTokensExpr string
	totalTokensExpr                  string
	completeAttemptsExpr             string
	completeZeroAttemptsExpr         string
	completePromptTokensExpr         string
	completeCacheReadTokensExpr      string
	completeOutputTokensExpr         string
	providerExpr                     string
	modelExpr                        string
	latencySumExpr                   string
	latencyCountExpr                 string
	lastUsedAtExpr                   string
	// firstUsedAtExpr 仅 raw events 有精确事实；rollup 不伪造 first-used。
	firstUsedAtExpr string
	// identityAuthTypeExpr/identityExpr 是 Key Alias 维度的身份列；apiKeyIdentityExpr 是 API Key 维度的身份列。
	identityAuthTypeExpr string
	identityExpr         string
	apiKeyIdentityExpr   string
	accounting           analyticsAccountingAggregateSource
	bucketExpr           func(bucketByDay bool) string
	query                func(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB
	// identityQuery/apiKeyQuery 在 query 基础上附加身份/别名 join 与非空身份过滤。
	identityQuery func(db *gorm.DB, filter dto.AnalyticsFilter) *gorm.DB
	apiKeyQuery   func(db *gorm.DB, scope dto.UsageTimeScope) *gorm.DB
}

type analyticsAccountingAggregateSource struct {
	stateAttemptsExpr        func(string) string
	validQualityAttemptsExpr func(string) string
	validTokenExpr           func(string) string
}

func analyticsEventsAggregateSource() analyticsAggregateSource {
	accounting := analyticsEventsAccountingAggregateSource()
	validToken := accounting.validTokenExpr
	completeToken := func(column string) string {
		return "CASE WHEN usage_events.accounting_state = 'valid' AND usage_events.token_quality = 'complete' THEN COALESCE(usage_events." + column + ", 0) ELSE 0 END"
	}
	return analyticsAggregateSource{
		name:                             "events",
		requestCountExpr:                 "1",
		successSumExpr:                   "CASE WHEN usage_events.failed THEN 0 ELSE 1 END",
		failureSumExpr:                   "CASE WHEN usage_events.failed THEN 1 ELSE 0 END",
		inputTokensExpr:                  validToken("canonical_input_tokens"),
		outputTokensExpr:                 validToken("canonical_output_tokens"),
		reasoningTokensExpr:              validToken("canonical_reasoning_tokens"),
		cachedTokensExpr:                 validToken("canonical_cache_read_tokens"),
		cacheReadTokensExpr:              validToken("canonical_cache_read_tokens"),
		cacheReadObservedInputTokensExpr: validToken("canonical_input_tokens"),
		totalTokensExpr:                  validToken("canonical_total_tokens"),
		completeAttemptsExpr:             accounting.validQualityAttemptsExpr("complete"),
		completeZeroAttemptsExpr: `CASE WHEN usage_events.accounting_state = 'valid'
			AND usage_events.token_quality = 'complete'
			AND COALESCE(usage_events.canonical_uncached_tokens, 0) + COALESCE(usage_events.canonical_cache_write_tokens, 0) = 0
			AND COALESCE(usage_events.canonical_cache_read_tokens, 0) = 0
			AND COALESCE(usage_events.canonical_output_tokens, 0) = 0
			THEN 1 ELSE 0 END`,
		completePromptTokensExpr:    "(" + completeToken("canonical_uncached_tokens") + " + " + completeToken("canonical_cache_write_tokens") + ")",
		completeCacheReadTokensExpr: completeToken("canonical_cache_read_tokens"),
		completeOutputTokensExpr:    completeToken("canonical_output_tokens"),
		providerExpr:                "TRIM(usage_events.provider)",
		modelExpr:                   "TRIM(usage_events.model)",
		latencySumExpr:              "CASE WHEN usage_events.latency_ms > 0 THEN usage_events.latency_ms ELSE 0 END",
		latencyCountExpr:            "CASE WHEN usage_events.latency_ms > 0 THEN 1 ELSE 0 END",
		lastUsedAtExpr:              "usage_events.timestamp",
		firstUsedAtExpr:             "usage_events.timestamp",
		identityAuthTypeExpr:        analyticsUsageIdentityAuthTypeSQLExpression(),
		identityExpr:                analyticsUsageIdentitySQLExpression(),
		apiKeyIdentityExpr:          analyticsAPIKeyIdentitySQLExpression(),
		accounting:                  accounting,
		bucketExpr:                  analyticsBucketSQLExpression,
		query:                       analyticsEventsWithPricingQuery,
		identityQuery:               analyticsIdentityEventsWithPricingQuery,
		apiKeyQuery:                 apiKeyEventsWithPricingQuery,
	}
}

func analyticsEventsAccountingAggregateSource() analyticsAccountingAggregateSource {
	return analyticsAccountingAggregateSource{
		stateAttemptsExpr: func(state string) string {
			return "CASE WHEN usage_events.accounting_state = '" + state + "' THEN 1 ELSE 0 END"
		},
		validQualityAttemptsExpr: func(quality string) string {
			return "CASE WHEN usage_events.accounting_state = 'valid' AND usage_events.token_quality = '" + quality + "' THEN 1 ELSE 0 END"
		},
		validTokenExpr: func(column string) string {
			return "CASE WHEN usage_events.accounting_state = 'valid' THEN COALESCE(usage_events." + column + ", 0) ELSE 0 END"
		},
	}
}

func analyticsRollupsAggregateSource() analyticsAggregateSource {
	return analyticsAggregateSource{
		name:                             "rollup",
		requestCountExpr:                 "usage_rollups_hourly.request_count",
		successSumExpr:                   "usage_rollups_hourly.success_count",
		failureSumExpr:                   "usage_rollups_hourly.failure_count",
		inputTokensExpr:                  "usage_rollups_hourly.canonical_input_tokens",
		outputTokensExpr:                 "usage_rollups_hourly.canonical_output_tokens",
		reasoningTokensExpr:              "usage_rollups_hourly.canonical_reasoning_tokens",
		cachedTokensExpr:                 "usage_rollups_hourly.canonical_cache_read_tokens",
		cacheReadTokensExpr:              "usage_rollups_hourly.canonical_cache_read_tokens",
		cacheReadObservedInputTokensExpr: "usage_rollups_hourly.canonical_input_tokens",
		totalTokensExpr:                  "usage_rollups_hourly.canonical_total_tokens",
		completeAttemptsExpr:             "usage_rollups_hourly.accounting_valid_complete_attempts",
		completeZeroAttemptsExpr:         "usage_rollups_hourly.canonical_complete_zero_attempts",
		completePromptTokensExpr:         "usage_rollups_hourly.canonical_complete_prompt_tokens",
		completeCacheReadTokensExpr:      "usage_rollups_hourly.canonical_complete_cache_read_tokens",
		completeOutputTokensExpr:         "usage_rollups_hourly.canonical_complete_output_tokens",
		providerExpr:                     "TRIM(usage_rollups_hourly.provider)",
		modelExpr:                        "TRIM(usage_rollups_hourly.model)",
		latencySumExpr:                   "usage_rollups_hourly.total_latency_ms",
		latencyCountExpr:                 "usage_rollups_hourly.latency_sample_count",
		lastUsedAtExpr:                   "usage_rollups_hourly.last_event_at",
		firstUsedAtExpr:                  "",
		identityAuthTypeExpr:             analyticsRollupUsageIdentityAuthTypeSQLExpression(),
		identityExpr:                     analyticsRollupUsageIdentitySQLExpression(),
		apiKeyIdentityExpr:               analyticsRollupAPIKeyIdentitySQLExpression(),
		accounting:                       analyticsRollupsAccountingAggregateSource(),
		bucketExpr:                       analyticsRollupBucketSQLExpression,
		query:                            analyticsRollupsWithPricingQuery,
		identityQuery:                    analyticsRollupIdentityWithPricingQuery,
		apiKeyQuery:                      rollupAPIKeyWithPricingQuery,
	}
}

func analyticsRollupsAccountingAggregateSource() analyticsAccountingAggregateSource {
	return analyticsAccountingAggregateSource{
		stateAttemptsExpr: func(state string) string {
			columns := map[string]string{
				AccountingAbsent:  "accounting_absent_attempts",
				AccountingInvalid: "accounting_invalid_attempts",
				AccountingValid:   "accounting_valid_attempts",
			}
			return "usage_rollups_hourly." + columns[state]
		},
		validQualityAttemptsExpr: func(quality string) string {
			columns := map[string]string{
				"complete":     "accounting_valid_complete_attempts",
				"inconsistent": "accounting_valid_inconsistent_attempts",
				"unclassified": "accounting_valid_unclassified_attempts",
			}
			return "usage_rollups_hourly." + columns[quality]
		},
		validTokenExpr: func(column string) string {
			return "usage_rollups_hourly." + column
		},
	}
}

// analyticsSourceCostSQLExpression 渲染该源的 Cost 表达式。
func analyticsSourceCostSQLExpression(source analyticsAggregateSource) string {
	return analyticsCostSQLExpressionWithPromptTokens(source.completePromptTokensExpr, source.completeOutputTokensExpr, source.completeCacheReadTokensExpr)
}

func analyticsSourceCacheSavingsSQLExpression(source analyticsAggregateSource) string {
	return analyticsCacheSavingsSQLExpressionFor(source.completeCacheReadTokensExpr)
}

func analyticsSourceCacheSavingsEligibleSQLExpression(source analyticsAggregateSource) string {
	return analyticsCacheSavingsEligibleSQLExpressionFor(source.completeCacheReadTokensExpr, source.completeAttemptsExpr)
}

func analyticsSourceCacheSavingsIneligibleSQLExpression(source analyticsAggregateSource) string {
	return analyticsCacheSavingsIneligibleSQLExpressionFor(source.completeCacheReadTokensExpr, source.completeAttemptsExpr)
}

func analyticsSourceMissingPricingSQLExpression(source analyticsAggregateSource) string {
	return analyticsMissingPricingSQLExpressionFor(source.requestCountExpr, source.completeAttemptsExpr, source.completeZeroAttemptsExpr)
}

// PricedBillable is the historical internal name for attempts whose Cost is
// known. Complete zero-token attempts are known even without a price row.
func analyticsSourcePricedBillableSQLExpression(source analyticsAggregateSource) string {
	return analyticsPricedBillableSQLExpressionFor(source.completeAttemptsExpr, source.completeZeroAttemptsExpr)
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
			COALESCE(SUM(` + source.accounting.stateAttemptsExpr(AccountingValid) + `), 0) AS accounting_valid_attempts,
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
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.cacheReadTokensExpr) + `), 0) AS cache_read_tokens,
			COALESCE(SUM(` + analyticsPositiveTokenSQLExpression(source.cacheReadObservedInputTokensExpr) + `), 0) AS cache_read_observed_input_tokens,
			COALESCE(SUM(` + analyticsSourceCacheSavingsSQLExpression(source) + `), 0) AS cache_savings,
			COALESCE(SUM(` + analyticsSourceCacheSavingsEligibleSQLExpression(source) + `), 0) AS cache_savings_eligible_rows,
			COALESCE(SUM(` + analyticsSourceCacheSavingsIneligibleSQLExpression(source) + `), 0) AS cache_savings_ineligible_rows,` +
		analyticsAccountingSummarySelect(source.accounting)
}

func analyticsAccountingSummarySelect(source analyticsAccountingAggregateSource) string {
	state := source.stateAttemptsExpr
	quality := source.validQualityAttemptsExpr
	token := source.validTokenExpr
	return `
			COALESCE(SUM(` + state(AccountingAbsent) + `), 0) AS accounting_absent_attempts,
			COALESCE(SUM(` + state(AccountingInvalid) + `), 0) AS accounting_invalid_attempts,
			COALESCE(SUM(` + quality("complete") + `), 0) AS accounting_valid_complete_attempts,
			COALESCE(SUM(` + quality("inconsistent") + `), 0) AS accounting_valid_inconsistent_attempts,
			COALESCE(SUM(` + quality("unclassified") + `), 0) AS accounting_valid_unclassified_attempts,
			COALESCE(SUM(` + token("canonical_total_tokens") + `), 0) AS canonical_total_tokens,
			COALESCE(SUM(` + token("canonical_input_tokens") + `), 0) AS canonical_input_tokens,
			COALESCE(SUM(` + token("canonical_uncached_tokens") + `), 0) AS canonical_uncached_tokens,
			COALESCE(SUM(` + token("canonical_cache_read_tokens") + `), 0) AS canonical_cache_read_tokens,
			COALESCE(SUM(` + token("canonical_cache_write_tokens") + `), 0) AS canonical_cache_write_tokens,
			COALESCE(SUM(` + token("canonical_output_tokens") + `), 0) AS canonical_output_tokens,
			COALESCE(SUM(` + token("canonical_non_reasoning_tokens") + `), 0) AS canonical_non_reasoning_tokens,
			COALESCE(SUM(` + token("canonical_reasoning_tokens") + `), 0) AS canonical_reasoning_tokens,
			COALESCE(SUM(` + token("canonical_unclassified_tokens") + `), 0) AS canonical_unclassified_tokens`
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
