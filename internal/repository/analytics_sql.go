package repository

import (
	"cpa-usage/internal/repository/dto"
	"strings"
)

func analyticsCostSQLExpression() string {
	return analyticsCostSQLExpressionFor("usage_events.input_tokens", "usage_events.output_tokens", "usage_events.cached_tokens")
}

func analyticsCostSQLExpressionFor(inputColumn string, outputColumn string, cachedColumn string) string {
	inputTokens := analyticsPositiveTokenSQLExpression(inputColumn)
	outputTokens := analyticsPositiveTokenSQLExpression(outputColumn)
	cachedTokens := analyticsPositiveTokenSQLExpression(cachedColumn)
	promptTokens := "(CASE WHEN " + inputTokens + " - " + cachedTokens + " > 0 THEN " + inputTokens + " - " + cachedTokens + " ELSE 0 END)"
	return analyticsCostSQLExpressionWithPromptTokens(promptTokens, outputTokens, cachedTokens)
}

func analyticsCostSQLExpressionWithPromptTokens(promptTokens string, outputTokens string, cachedTokens string) string {
	return `CASE
		WHEN model_price_settings.id IS NULL THEN 0
		ELSE
			(` + promptTokens + ` / 1000000.0) * model_price_settings.prompt_price_per1_m +
			(` + outputTokens + ` / 1000000.0) * model_price_settings.completion_price_per1_m +
			(` + cachedTokens + ` / 1000000.0) * model_price_settings.cache_price_per1_m
	END`
}

func analyticsCacheSavingsSQLExpression() string {
	return analyticsCacheSavingsSQLExpressionFor("usage_events.cached_tokens")
}

func analyticsCacheSavingsSQLExpressionFor(cachedColumn string) string {
	cachedTokens := analyticsPositiveTokenSQLExpression(cachedColumn)
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND model_price_settings.id IS NOT NULL
			AND model_price_settings.prompt_price_per1_m >= model_price_settings.cache_price_per1_m
		THEN (` + cachedTokens + ` / 1000000.0) * (model_price_settings.prompt_price_per1_m - model_price_settings.cache_price_per1_m)
		ELSE 0
	END`
}

func analyticsCacheSavingsEligibleSQLExpression() string {
	return analyticsCacheSavingsEligibleSQLExpressionFor("usage_events.cached_tokens", "1")
}

func analyticsCacheSavingsEligibleSQLExpressionFor(cachedColumn string, countExpression string) string {
	cachedTokens := analyticsPositiveTokenSQLExpression(cachedColumn)
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND model_price_settings.id IS NOT NULL
			AND model_price_settings.prompt_price_per1_m >= model_price_settings.cache_price_per1_m
		THEN ` + countExpression + `
		ELSE 0
	END`
}

func analyticsCacheSavingsIneligibleSQLExpression() string {
	return analyticsCacheSavingsIneligibleSQLExpressionFor("usage_events.cached_tokens", "1")
}

func analyticsCacheSavingsIneligibleSQLExpressionFor(cachedColumn string, countExpression string) string {
	cachedTokens := analyticsPositiveTokenSQLExpression(cachedColumn)
	return `CASE
		WHEN ` + cachedTokens + ` > 0
			AND (
				model_price_settings.id IS NULL
				OR model_price_settings.prompt_price_per1_m < model_price_settings.cache_price_per1_m
			)
		THEN ` + countExpression + `
		ELSE 0
	END`
}

func analyticsPositiveTokenSQLExpression(column string) string {
	return "(CASE WHEN " + column + " > 0 THEN " + column + " ELSE 0 END)"
}

func analyticsMissingPricingSQLExpression() string {
	return analyticsMissingPricingSQLExpressionFor("usage_events.input_tokens", "usage_events.output_tokens", "usage_events.cached_tokens", "1")
}

func analyticsMissingPricingSQLExpressionFor(inputColumn string, outputColumn string, cachedColumn string, countExpression string) string {
	return `CASE
		WHEN model_price_settings.id IS NULL
			AND (` + inputColumn + ` > 0 OR ` + outputColumn + ` > 0 OR ` + cachedColumn + ` > 0)
		THEN ` + countExpression + `
		ELSE 0
	END`
}

func analyticsPricedBillableSQLExpression() string {
	return analyticsPricedBillableSQLExpressionFor("usage_events.input_tokens", "usage_events.output_tokens", "usage_events.cached_tokens", "1")
}

func analyticsPricedBillableSQLExpressionFor(inputColumn string, outputColumn string, cachedColumn string, countExpression string) string {
	return `CASE
		WHEN model_price_settings.id IS NOT NULL
			AND (` + inputColumn + ` > 0 OR ` + outputColumn + ` > 0 OR ` + cachedColumn + ` > 0)
		THEN ` + countExpression + `
		ELSE 0
	END`
}

func analyticsBucketSQLExpression(bucketByDay bool) string {
	if bucketByDay {
		return "strftime('%Y-%m-%d', usage_events.timestamp, 'localtime')"
	}
	return "strftime('%Y-%m-%dT%H:00:00Z', usage_events.timestamp)"
}

func analyticsRollupBucketSQLExpression(bucketByDay bool) string {
	if bucketByDay {
		return "strftime('%Y-%m-%d', usage_rollups_hourly.bucket_start, 'localtime')"
	}
	return "strftime('%Y-%m-%dT%H:00:00Z', usage_rollups_hourly.bucket_start)"
}

func analyticsTrendBucketsByDay(filter dto.UsageQueryFilter) bool {
	return strings.TrimSpace(filter.Granularity) == "day"
}

func analyticsUsageIdentityAuthTypeSQLExpression() string {
	return `(CASE
		WHEN TRIM(usage_events.auth_type) = 'oauth' THEN 1
		WHEN TRIM(usage_events.auth_type) = 'apikey' THEN 2
		ELSE 0
	END)`
}

func analyticsUsageIdentitySQLExpression() string {
	return "TRIM(usage_events.auth_index)"
}

func analyticsAPIKeyAuthTypeSQLExpression() string {
	return "2"
}

func analyticsAPIKeyIdentitySQLExpression() string {
	return `(CASE
		WHEN TRIM(usage_events.api_group_key) LIKE 'sk-%' THEN TRIM(usage_events.api_group_key)
		WHEN TRIM(usage_events.auth_type) = 'apikey' AND TRIM(usage_events.source) LIKE 'sk-%' THEN TRIM(usage_events.source)
		ELSE ''
	END)`
}
func analyticsCostAvailability(missingPricingEvents int64, pricedBillableEvents int64) (bool, string) {
	if missingPricingEvents == 0 {
		return true, dto.AnalyticsCostStatusAvailable
	}
	if pricedBillableEvents > 0 {
		return false, dto.AnalyticsCostStatusPartial
	}
	return false, dto.AnalyticsCostStatusUnavailable
}
