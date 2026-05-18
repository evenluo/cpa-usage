package repository

import (
	"cpa-usage/internal/repository/dto"
	"fmt"
	"strings"
)

func buildAnalyticsInsights(
	summary dto.AnalyticsSummary,
	trend []dto.AnalyticsTrendPoint,
	keyAliases []dto.AnalyticsKeyAliasBreakdown,
	models []dto.AnalyticsModelBreakdown,
) []dto.AnalyticsInsight {
	if summary.RequestCount == 0 {
		return []dto.AnalyticsInsight{}
	}

	insights := make([]dto.AnalyticsInsight, 0, 6)
	insights = append(insights, metricCompletenessInsight(summary, models))
	insights = append(insights, cacheEfficiencyInsight(summary))
	if topCost, ok := topCostKeyAlias(keyAliases); ok {
		insights = append(insights, dto.AnalyticsInsight{
			Type:        "top_cost_key",
			Severity:    "green",
			Title:       "Top Cost Key",
			Detail:      "Highest configured Cost contributor in this range.",
			Subject:     analyticsInsightKeyLabel(topCost),
			MetricLabel: "Cost",
			MetricValue: topCost.TotalCost,
			Count:       topCost.RequestCount,
			CostStatus:  topCost.CostStatus,
		})
	}
	if spike, ok := topTokenBucket(trend); ok {
		insights = append(insights, dto.AnalyticsInsight{
			Type:        "token_spike",
			Severity:    "violet",
			Title:       "Token Spike",
			Detail:      "Highest token bucket in the selected range.",
			Subject:     spike.Label,
			MetricLabel: "Tokens",
			MetricValue: float64(spike.TotalTokens),
			Count:       spike.RequestCount,
			CostStatus:  spike.CostStatus,
		})
	}
	if failure, ok := failureConcentration(keyAliases); ok {
		insights = append(insights, dto.AnalyticsInsight{
			Type:        "failure_concentration",
			Severity:    "amber",
			Title:       "Failure Cluster",
			Detail:      "Largest failure concentration by Key Alias.",
			Subject:     analyticsInsightKeyLabel(failure),
			MetricLabel: "Failures",
			MetricValue: float64(failure.FailureCount),
			Count:       failure.FailureCount,
			CostStatus:  failure.CostStatus,
		})
	}
	if summary.ReasoningTokens > 0 {
		insights = append(insights, dto.AnalyticsInsight{
			Type:        "reasoning_tokens",
			Severity:    "blue",
			Title:       "Reasoning Tokens",
			Detail:      "Reasoning token volume is tracked separately from prompt cache reads.",
			Subject:     "Reasoning behavior",
			MetricLabel: "Tokens",
			MetricValue: float64(summary.ReasoningTokens),
			Count:       summary.ReasoningTokens,
			CostStatus:  summary.CostStatus,
		})
	}
	return insights
}

func metricCompletenessInsight(summary dto.AnalyticsSummary, models []dto.AnalyticsModelBreakdown) dto.AnalyticsInsight {
	incompleteModels := countModelsWithIncompletePricing(models)
	cacheComplete := summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateAvailable
	costComplete := summary.CostStatus == dto.AnalyticsCostStatusAvailable
	insight := dto.AnalyticsInsight{
		Type:        "metric_completeness",
		Severity:    "green",
		Title:       "Metric Completeness",
		Detail:      "Cost and cache efficiency have the supporting data needed for complete interpretation.",
		Subject:     "Complete",
		MetricLabel: "Metric Completeness",
		MetricValue: 0,
		Count:       incompleteModels,
		CostStatus:  summary.CostStatus,
	}
	if costComplete && cacheComplete {
		return insight
	}
	insight.Severity = "amber"
	insight.Subject = metricCompletenessSubject(summary, incompleteModels)
	insight.Detail = "Some derived metrics are incomplete, but the underlying usage events remain valid."
	insight.MetricValue = float64(incompleteModels)
	return insight
}

func metricCompletenessSubject(summary dto.AnalyticsSummary, incompleteModels int64) string {
	if summary.CostStatus != dto.AnalyticsCostStatusAvailable {
		return "Cost " + summary.CostStatus
	}
	if summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateNoCacheData {
		return "No cache data"
	}
	if summary.CacheReadShareState == dto.AnalyticsCacheReadShareStateNoPromptInput {
		return "No prompt input"
	}
	if incompleteModels == 1 {
		return "1 model"
	}
	if incompleteModels > 1 {
		return fmt.Sprintf("%d models", incompleteModels)
	}
	return "Incomplete"
}

func cacheEfficiencyInsight(summary dto.AnalyticsSummary) dto.AnalyticsInsight {
	insight := dto.AnalyticsInsight{
		Type:        "cache_efficiency",
		Severity:    "green",
		Title:       "Cache Read Share",
		Detail:      "Prompt cache reads are measured against prompt input tokens, separately from reasoning tokens.",
		Subject:     "Prompt input cache",
		MetricLabel: "Cache Read Share",
		MetricValue: summary.CacheReadShare,
		Count:       summary.CachedTokens,
		CostStatus:  summary.CostStatus,
	}
	switch summary.CacheReadShareState {
	case dto.AnalyticsCacheReadShareStateNoCacheData:
		insight.Severity = "amber"
		insight.Subject = "No cache data"
		insight.MetricLabel = "Cache state"
		insight.Detail = "Cached-token evidence is unavailable for this range; reasoning tokens are not counted as cache reads."
	case dto.AnalyticsCacheReadShareStateNoPromptInput:
		insight.Severity = "amber"
		insight.Subject = "No prompt input"
		insight.MetricLabel = "Cache state"
		insight.Detail = "Prompt input is zero for this range, so Cache Read Share has no denominator."
	}
	return insight
}

func topCostKeyAlias(rows []dto.AnalyticsKeyAliasBreakdown) (dto.AnalyticsKeyAliasBreakdown, bool) {
	var best dto.AnalyticsKeyAliasBreakdown
	found := false
	for _, row := range rows {
		if row.CostAvailable == false || row.CostStatus == dto.AnalyticsCostStatusUnavailable || row.TotalCost <= 0 {
			continue
		}
		if !found || row.TotalCost > best.TotalCost {
			best = row
			found = true
		}
	}
	return best, found
}

func topTokenBucket(points []dto.AnalyticsTrendPoint) (dto.AnalyticsTrendPoint, bool) {
	var best dto.AnalyticsTrendPoint
	found := false
	for _, point := range points {
		if !found || point.TotalTokens > best.TotalTokens {
			best = point
			found = true
		}
	}
	return best, found && best.TotalTokens > 0
}

func failureConcentration(rows []dto.AnalyticsKeyAliasBreakdown) (dto.AnalyticsKeyAliasBreakdown, bool) {
	var best dto.AnalyticsKeyAliasBreakdown
	found := false
	for _, row := range rows {
		if row.FailureCount == 0 {
			continue
		}
		if !found || row.FailureCount > best.FailureCount {
			best = row
			found = true
		}
	}
	return best, found
}

func analyticsInsightKeyLabel(row dto.AnalyticsKeyAliasBreakdown) string {
	for _, value := range []string{row.Alias, row.Name, row.Identity} {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return "Unknown key"
}

func countModelsWithIncompletePricing(models []dto.AnalyticsModelBreakdown) int64 {
	var count int64
	for _, model := range models {
		if model.CostStatus != dto.AnalyticsCostStatusAvailable {
			count++
		}
	}
	return count
}
