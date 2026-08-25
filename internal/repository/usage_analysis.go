package repository

import (
	"context"
	"fmt"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	"gorm.io/gorm"
)

// Analysis Tab 第一步：应用时间窗口和 provider scope，避免 Request Event Log 的筛选污染聚合。
func applyUsageAnalysisTabQuery(query *gorm.DB, filter dto.UsageQueryFilter) *gorm.DB {
	return applyUsageProviderFilter(applyUsageQueryWindow(query, filter), filter)
}

// Analysis 第一步：按时间窗口做 API / model / API+model 聚合。
func ListUsageAnalysisWithFilter(ctx context.Context, db *gorm.DB, filter dto.UsageQueryFilter) ([]dto.UsageAnalysisAPIStatRecord, []dto.UsageAnalysisModelStatRecord, error) {
	if db == nil {
		return nil, nil, fmt.Errorf("database is nil")
	}
	db = db.WithContext(ctx)

	baseQuery := applyUsageAnalysisTabQuery(db.Model(&entities.UsageEvent{}), filter)

	apiQuery := baseQuery.Session(&gorm.Session{})
	apiQuery = apiQuery.Select(strings.Join([]string{
		"TRIM(api_group_key) AS api_group_key",
		"COUNT(*) AS total_requests",
		"SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_count",
		"SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failure_count",
		"SUM(input_tokens) AS input_tokens",
		"SUM(output_tokens) AS output_tokens",
		"SUM(reasoning_tokens) AS reasoning_tokens",
		"SUM(cached_tokens) AS cached_tokens",
		"SUM(total_tokens) AS total_tokens",
	}, ", "))
	apiQuery = apiQuery.Group("TRIM(api_group_key)")
	apiQuery = apiQuery.Order("total_requests DESC, api_group_key ASC")

	var apiRows []dto.UsageAnalysisAPIStatRecord
	if err := apiQuery.Scan(&apiRows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage analysis api stats: %w", err)
	}

	modelQuery := baseQuery.Session(&gorm.Session{})
	modelQuery = modelQuery.Select(strings.Join([]string{
		"TRIM(model) AS model",
		"COUNT(*) AS total_requests",
		"SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_count",
		"SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failure_count",
		"SUM(input_tokens) AS input_tokens",
		"SUM(output_tokens) AS output_tokens",
		"SUM(reasoning_tokens) AS reasoning_tokens",
		"SUM(cached_tokens) AS cached_tokens",
		"SUM(total_tokens) AS total_tokens",
		"SUM(latency_ms) AS total_latency_ms",
		"SUM(CASE WHEN latency_ms > 0 THEN 1 ELSE 0 END) AS latency_sample_count",
	}, ", "))
	modelQuery = modelQuery.Group("TRIM(model)")
	modelQuery = modelQuery.Order("total_requests DESC, model ASC")

	var modelRows []dto.UsageAnalysisModelStatRecord
	if err := modelQuery.Scan(&modelRows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage analysis model stats: %w", err)
	}

	apiModelQuery := baseQuery.Session(&gorm.Session{})
	apiModelQuery = apiModelQuery.Select(strings.Join([]string{
		"TRIM(api_group_key) AS api_group_key",
		"TRIM(model) AS model",
		"COUNT(*) AS total_requests",
		"SUM(CASE WHEN failed THEN 0 ELSE 1 END) AS success_count",
		"SUM(CASE WHEN failed THEN 1 ELSE 0 END) AS failure_count",
		"SUM(input_tokens) AS input_tokens",
		"SUM(output_tokens) AS output_tokens",
		"SUM(reasoning_tokens) AS reasoning_tokens",
		"SUM(cached_tokens) AS cached_tokens",
		"SUM(total_tokens) AS total_tokens",
		"SUM(latency_ms) AS total_latency_ms",
		"SUM(CASE WHEN latency_ms > 0 THEN 1 ELSE 0 END) AS latency_sample_count",
	}, ", "))
	apiModelQuery = apiModelQuery.Group("TRIM(api_group_key), TRIM(model)")
	apiModelQuery = apiModelQuery.Order("api_group_key ASC, total_requests DESC, model ASC")

	var apiModelRows []struct {
		APIGroupKey        string
		Model              string
		TotalRequests      int64
		SuccessCount       int64
		FailureCount       int64
		InputTokens        int64
		OutputTokens       int64
		ReasoningTokens    int64
		CachedTokens       int64
		TotalTokens        int64
		TotalLatencyMS     int64
		LatencySampleCount int64
	}
	if err := apiModelQuery.Scan(&apiModelRows).Error; err != nil {
		return nil, nil, fmt.Errorf("load usage analysis api model stats: %w", err)
	}

	normalize := func(value string) string {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			return "unknown"
		}
		return trimmed
	}
	modelsByAPI := make(map[string][]dto.UsageAnalysisModelStatRecord, len(apiRows))
	for _, row := range apiModelRows {
		apiKey := normalize(row.APIGroupKey)
		modelsByAPI[apiKey] = append(modelsByAPI[apiKey], dto.UsageAnalysisModelStatRecord{
			Model:              row.Model,
			TotalRequests:      row.TotalRequests,
			SuccessCount:       row.SuccessCount,
			FailureCount:       row.FailureCount,
			InputTokens:        row.InputTokens,
			OutputTokens:       row.OutputTokens,
			ReasoningTokens:    row.ReasoningTokens,
			CachedTokens:       row.CachedTokens,
			TotalTokens:        row.TotalTokens,
			TotalLatencyMS:     row.TotalLatencyMS,
			LatencySampleCount: row.LatencySampleCount,
		})
	}

	resultAPIs := make([]dto.UsageAnalysisAPIStatRecord, 0, len(apiRows))
	for _, row := range apiRows {
		row.APIGroupKey = normalize(row.APIGroupKey)
		row.DisplayName = row.APIGroupKey
		models := modelsByAPI[row.APIGroupKey]
		for index := range models {
			models[index].Model = normalize(models[index].Model)
		}
		row.Models = models
		resultAPIs = append(resultAPIs, row)
	}
	for index := range modelRows {
		modelRows[index].Model = normalize(modelRows[index].Model)
	}

	return resultAPIs, modelRows, nil
}
