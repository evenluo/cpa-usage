package repository

import (
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

func applyUsageQueryWindow(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	if filter.StartTime != nil {
		query = query.Where("timestamp >= ?", filter.StartTime.UTC())
	}
	if filter.EndTime != nil {
		query = query.Where("timestamp <= ?", filter.EndTime.UTC())
	}
	return query
}

func applyUsageProviderFilter(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("TRIM(provider) = ?", provider)
	}
	return query
}

// Overview Tab 第一步：应用时间窗口和 provider scope，后续 Overview 专属条件也从这里加。
func applyUsageOverviewQuery(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
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
	inputTokens := event.InputTokens
	if inputTokens < 0 {
		inputTokens = 0
	}
	completionTokens := event.OutputTokens
	if completionTokens < 0 {
		completionTokens = 0
	}
	cachedTokens := event.CachedTokens
	if cachedTokens < 0 {
		cachedTokens = 0
	}
	promptTokens := inputTokens - cachedTokens
	if promptTokens < 0 {
		promptTokens = 0
	}
	return (float64(promptTokens)/1_000_000.0)*pricing.PromptPricePer1M +
		(float64(completionTokens)/1_000_000.0)*pricing.CompletionPricePer1M +
		(float64(cachedTokens)/1_000_000.0)*pricing.CachePricePer1M
}
