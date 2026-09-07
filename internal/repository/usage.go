package repository

import (
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func applyUsageQueryWindow(query *gorm.DB, filter dto.UsageTimeScope) *gorm.DB {
	if filter.StartTime != nil {
		query = query.Where("timestamp >= ?", filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime.UTC())
	}
	return query
}

func applyUsageProviderFilter(query *gorm.DB, filter dto.UsageTimeScope) *gorm.DB {
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("TRIM(provider) = ?", provider)
	}
	return query
}

// Overview Tab 第一步：应用时间窗口和 provider scope，后续 Overview 专属条件也从这里加。
func applyUsageOverviewQuery(query *gorm.DB, filter dto.UsageTimeScope) *gorm.DB {
	return applyUsageProviderFilter(applyUsageQueryWindow(query, filter), filter)
}

func loadPriceSettingsByModel(db *gorm.DB) (map[string]entities.ModelPriceSetting, error) {
	settings, err := ListModelPriceSettings(db)
	if err != nil {
		return nil, err
	}
	result := make(map[string]entities.ModelPriceSetting, len(settings))
	for _, setting := range settings {
		result[strings.TrimSpace(setting.Model)] = setting
	}
	return result, nil
}

func calculateUsageEventCost(event entities.UsageEvent, pricing entities.ModelPriceSetting) float64 {
	facts := InterpretUsageAttempt(event).Accounting
	if facts.State != AccountingValid || facts.Quality == nil || *facts.Quality != "complete" {
		return 0
	}
	promptTokens := optionalInt64Value(facts.Input.UncachedTokens) + optionalInt64Value(facts.Input.CacheWriteTokens)
	completionTokens := optionalInt64Value(facts.Output.TotalTokens)
	cachedTokens := optionalInt64Value(facts.Input.CacheReadTokens)
	return (float64(promptTokens)/1_000_000.0)*pricing.PromptPricePer1M +
		(float64(completionTokens)/1_000_000.0)*pricing.CompletionPricePer1M +
		(float64(cachedTokens)/1_000_000.0)*pricing.CachePricePer1M
}

func usageEventHasCompleteAccounting(event entities.UsageEvent) bool {
	facts := InterpretUsageAttempt(event).Accounting
	return facts.State == AccountingValid && facts.Quality != nil && *facts.Quality == "complete"
}

func usageEventCanonicalTokenStats(event entities.UsageEvent) (dto.TokenStats, bool) {
	facts := InterpretUsageAttempt(event).Accounting
	if facts.State != AccountingValid {
		return dto.TokenStats{}, false
	}
	return dto.TokenStats{
		InputTokens:     optionalInt64Value(facts.Input.TotalTokens),
		OutputTokens:    optionalInt64Value(facts.Output.TotalTokens),
		ReasoningTokens: optionalInt64Value(facts.Output.ReasoningTokens),
		CachedTokens:    optionalInt64Value(facts.Input.CacheReadTokens),
		TotalTokens:     optionalInt64Value(facts.TotalTokens),
	}, true
}
