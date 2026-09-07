package api

import (
	"net/http"
	"time"

	repodto "cpa-usage/internal/repository/dto"
	"github.com/gin-gonic/gin"
)

type usageModelMappingDistributionResponse struct {
	WindowStart            string                     `json:"window_start"`
	WindowEnd              string                     `json:"window_end"`
	TotalAttempts          int64                      `json:"total_attempts"`
	ObservedAliasAttempts  int64                      `json:"observed_alias_attempts"`
	CanonicalValidAttempts int64                      `json:"canonical_valid_attempts"`
	MissingAliasAttempts   int64                      `json:"missing_alias_attempts"`
	AliasCoverage          float64                    `json:"alias_coverage"`
	ObservedTotalCost      float64                    `json:"observed_total_cost"`
	ObservedCostAvailable  bool                       `json:"observed_cost_available"`
	ObservedCostStatus     string                     `json:"observed_cost_status"`
	Mappings               []usageModelMappingPayload `json:"mappings"`
	OtherAttempts          int64                      `json:"other_attempts"`
}

type usageModelMappingPayload struct {
	ModelAlias             string  `json:"model_alias"`
	Model                  string  `json:"model"`
	Provider               string  `json:"provider"`
	AttemptCount           int64   `json:"attempt_count"`
	CanonicalValidAttempts int64   `json:"canonical_valid_attempts"`
	FailureCount           int64   `json:"failure_count"`
	FailureShare           float64 `json:"failure_share"`
	LatencySampleCount     int64   `json:"latency_sample_count"`
	MeanLatencyMS          float64 `json:"mean_latency_ms"`
	TotalCost              float64 `json:"total_cost"`
	CostAvailable          bool    `json:"cost_available"`
	CostStatus             string  `json:"cost_status"`
}

func registerUsageModelMappingsRoute(router gin.IRoutes, usageProvider UsageProvider) {
	router.GET("/usage/model-mappings", func(c *gin.Context) {
		filter, err := parseFixedUsageDiagnosticFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if usageProvider == nil {
			c.JSON(http.StatusOK, buildUsageModelMappingDistributionPayload(filter, nil))
			return
		}
		record, err := usageProvider.GetUsageModelMappings(c.Request.Context(), filter.repositoryFilter())
		if err != nil {
			writeInternalError(c, "get usage model mappings failed", err)
			return
		}
		c.JSON(http.StatusOK, buildUsageModelMappingDistributionPayload(filter, record))
	})
}

func buildUsageModelMappingDistributionPayload(filter usageDiagnosticFilter, record *repodto.UsageModelMappingDistributionRecord) usageModelMappingDistributionResponse {
	if record == nil {
		record = &repodto.UsageModelMappingDistributionRecord{}
	}
	coverage := float64(0)
	if record.TotalAttempts > 0 {
		coverage = float64(record.ObservedAliasAttempts) / float64(record.TotalAttempts) * 100
	}
	mappings := make([]usageModelMappingPayload, 0, len(record.Mappings))
	for _, row := range record.Mappings {
		mappings = append(mappings, usageModelMappingPayload{
			ModelAlias: row.ModelAlias, Model: row.Model, Provider: row.Provider,
			AttemptCount: row.AttemptCount, FailureCount: row.FailureCount, FailureShare: row.FailureShare,
			CanonicalValidAttempts: row.CanonicalValidAttempts,
			LatencySampleCount:     row.LatencySampleCount, MeanLatencyMS: row.MeanLatencyMS,
			TotalCost: row.TotalCost, CostAvailable: row.CostAvailable, CostStatus: row.CostStatus,
		})
	}
	return usageModelMappingDistributionResponse{
		WindowStart: filter.StartTime.UTC().Format(time.RFC3339Nano), WindowEnd: filter.EndTime.UTC().Format(time.RFC3339Nano),
		TotalAttempts: record.TotalAttempts, ObservedAliasAttempts: record.ObservedAliasAttempts,
		CanonicalValidAttempts: record.CanonicalValidAttempts,
		MissingAliasAttempts:   record.MissingAliasAttempts, AliasCoverage: coverage,
		ObservedTotalCost: record.ObservedTotalCost, ObservedCostAvailable: record.ObservedCostAvailable,
		ObservedCostStatus: record.ObservedCostStatus, Mappings: mappings, OtherAttempts: record.OtherAttempts,
	}
}
