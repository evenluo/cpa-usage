package repository

import (
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func analyticsCostSQLExpressionWithPromptTokens(promptTokens string, outputTokens string, cachedTokens string) string {
	return `CASE
		WHEN model_price_settings.id IS NULL THEN 0
		ELSE
			(` + promptTokens + ` / 1000000.0) * model_price_settings.prompt_price_per1_m +
			(` + outputTokens + ` / 1000000.0) * model_price_settings.completion_price_per1_m +
			(` + cachedTokens + ` / 1000000.0) * model_price_settings.cache_price_per1_m
	END`
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

func analyticsMissingPricingSQLExpressionFor(inputColumn string, outputColumn string, cachedColumn string, countExpression string) string {
	return `CASE
		WHEN model_price_settings.id IS NULL
			AND (` + inputColumn + ` > 0 OR ` + outputColumn + ` > 0 OR ` + cachedColumn + ` > 0)
		THEN ` + countExpression + `
		ELSE 0
	END`
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

func analyticsRollupUsageIdentityAuthTypeSQLExpression() string {
	return usageIdentityAuthTypeSQLExpression("usage_rollups_hourly.auth_type")
}

func analyticsRollupUsageIdentitySQLExpression() string {
	return "TRIM(usage_rollups_hourly.auth_index)"
}

func analyticsRollupAPIKeyIdentitySQLExpression() string {
	return "TRIM(usage_rollups_hourly.api_key_identity)"
}

func analyticsTrendBucketsByDay(filter dto.AnalyticsFilter) bool {
	return strings.TrimSpace(filter.Granularity) == "day"
}

func analyticsUsageIdentityAuthTypeSQLExpression() string {
	return usageIdentityAuthTypeSQLExpression("usage_events.auth_type")
}

func analyticsUsageIdentitySQLExpression() string {
	return "TRIM(usage_events.auth_index)"
}

func analyticsAPIKeyAuthTypeSQLExpression() string {
	return fmt.Sprintf("%d", entities.UsageIdentityAuthTypeAIProvider)
}

func analyticsAPIKeyIdentitySQLExpression() string {
	apiKeyName, _ := entities.UsageIdentityAuthTypeAIProvider.CanonicalName()
	return `(CASE
		WHEN TRIM(usage_events.api_group_key) LIKE 'sk-%' THEN TRIM(usage_events.api_group_key)
		WHEN TRIM(usage_events.auth_type) = '` + apiKeyName + `' AND TRIM(usage_events.source) LIKE 'sk-%' THEN TRIM(usage_events.source)
		ELSE ''
	END)`
}

func usageIdentityAuthTypeSQLExpression(column string) string {
	authFileName, _ := entities.UsageIdentityAuthTypeAuthFile.CanonicalName()
	apiKeyName, _ := entities.UsageIdentityAuthTypeAIProvider.CanonicalName()
	return fmt.Sprintf(`(CASE
		WHEN TRIM(%s) = '%s' THEN %d
		WHEN TRIM(%s) = '%s' THEN %d
		ELSE 0
	END)`,
		column, authFileName, entities.UsageIdentityAuthTypeAuthFile,
		column, apiKeyName, entities.UsageIdentityAuthTypeAIProvider,
	)
}
