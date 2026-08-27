package api

import (
	"net/http"
	"time"

	"cpa-usage/internal/redact"
	repodto "cpa-usage/internal/repository/dto"
	"github.com/gin-gonic/gin"
)

type usageAnalysisResponse struct {
	APIs   []usageAnalysisAPIPayload   `json:"apis"`
	Models []usageAnalysisModelPayload `json:"models"`
}

type usageAnalysisAPIPayload struct {
	APIKey          string                      `json:"api_key"`
	DisplayName     string                      `json:"display_name"`
	TotalRequests   int64                       `json:"total_requests"`
	SuccessCount    int64                       `json:"success_count"`
	FailureCount    int64                       `json:"failure_count"`
	InputTokens     int64                       `json:"input_tokens"`
	OutputTokens    int64                       `json:"output_tokens"`
	ReasoningTokens int64                       `json:"reasoning_tokens"`
	CachedTokens    int64                       `json:"cached_tokens"`
	TotalTokens     int64                       `json:"total_tokens"`
	Models          []usageAnalysisModelPayload `json:"models"`
}

type usageAnalysisModelPayload struct {
	Model              string `json:"model"`
	TotalRequests      int64  `json:"total_requests"`
	SuccessCount       int64  `json:"success_count"`
	FailureCount       int64  `json:"failure_count"`
	InputTokens        int64  `json:"input_tokens"`
	OutputTokens       int64  `json:"output_tokens"`
	ReasoningTokens    int64  `json:"reasoning_tokens"`
	CachedTokens       int64  `json:"cached_tokens"`
	TotalTokens        int64  `json:"total_tokens"`
	TotalLatencyMS     int64  `json:"total_latency_ms"`
	LatencySampleCount int64  `json:"latency_sample_count"`
}

func registerUsageAnalysisRoute(router gin.IRoutes, usageProvider UsageProvider) {
	router.GET("/usage/analysis", func(c *gin.Context) {
		if usageProvider == nil {
			c.JSON(http.StatusOK, usageAnalysisResponse{APIs: []usageAnalysisAPIPayload{}, Models: []usageAnalysisModelPayload{}})
			return
		}

		filter, err := parseUsageTimeFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		apiRows, modelRows, err := usageProvider.GetUsageAnalysis(c.Request.Context(), filter.repositoryScope())
		if err != nil {
			writeInternalError(c, "get usage analysis failed", err)
			return
		}

		c.JSON(http.StatusOK, buildUsageAnalysisPayload(apiRows, modelRows))
	})
}

func buildUsageAnalysisPayload(apiRows []repodto.UsageAnalysisAPIStatRecord, modelRows []repodto.UsageAnalysisModelStatRecord) usageAnalysisResponse {
	apis := make([]usageAnalysisAPIPayload, 0, len(apiRows))
	for _, api := range apiRows {
		models := make([]usageAnalysisModelPayload, 0, len(api.Models))
		for _, model := range api.Models {
			models = append(models, mapUsageAnalysisModelPayload(model))
		}
		apis = append(apis, usageAnalysisAPIPayload{
			APIKey:          redact.APIAlias(api.APIGroupKey),
			DisplayName:     redact.APIKeyDisplayName(api.APIGroupKey),
			TotalRequests:   api.TotalRequests,
			SuccessCount:    api.SuccessCount,
			FailureCount:    api.FailureCount,
			InputTokens:     api.InputTokens,
			OutputTokens:    api.OutputTokens,
			ReasoningTokens: api.ReasoningTokens,
			CachedTokens:    api.CachedTokens,
			TotalTokens:     api.TotalTokens,
			Models:          models,
		})
	}

	models := make([]usageAnalysisModelPayload, 0, len(modelRows))
	for _, model := range modelRows {
		models = append(models, mapUsageAnalysisModelPayload(model))
	}

	return usageAnalysisResponse{APIs: apis, Models: models}
}

func mapUsageAnalysisModelPayload(model repodto.UsageAnalysisModelStatRecord) usageAnalysisModelPayload {
	return usageAnalysisModelPayload{
		Model:              model.Model,
		TotalRequests:      model.TotalRequests,
		SuccessCount:       model.SuccessCount,
		FailureCount:       model.FailureCount,
		InputTokens:        model.InputTokens,
		OutputTokens:       model.OutputTokens,
		ReasoningTokens:    model.ReasoningTokens,
		CachedTokens:       model.CachedTokens,
		TotalTokens:        model.TotalTokens,
		TotalLatencyMS:     model.TotalLatencyMS,
		LatencySampleCount: model.LatencySampleCount,
	}
}
