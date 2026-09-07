package api

import (
	"net/http"
	"time"

	repodto "cpa-usage/internal/repository/dto"
	"github.com/gin-gonic/gin"
)

type usageAttemptPerformanceResponse struct {
	WindowStart         string                           `json:"window_start"`
	WindowEnd           string                           `json:"window_end"`
	TotalAttempts       int64                            `json:"total_attempts"`
	SuccessfulAttempts  int64                            `json:"successful_attempts"`
	FailedAttempts      int64                            `json:"failed_attempts"`
	SuccessfulExecution usageExecutionPopulationPayload  `json:"successful_execution"`
	LatencyMS           usageResultPercentilesPayload    `json:"latency_ms"`
	TTFTMS              usageTTFTPercentilesPayload      `json:"ttft_ms"`
	OutputTPS           usageExecutionPercentilesPayload `json:"output_tps"`
	Providers           usagePerformanceBreakdownPayload `json:"providers"`
	Models              usagePerformanceBreakdownPayload `json:"models"`
	Accounts            usagePerformanceBreakdownPayload `json:"accounts"`
}

type usageExecutionPopulationPayload struct {
	GeneratingStreaming int64 `json:"generating_streaming"`
	NonGenerating       int64 `json:"non_generating"`
	NonStreaming        int64 `json:"non_streaming"`
	Unknown             int64 `json:"unknown"`
}

type usageResultPercentilesPayload struct {
	Successful usagePercentilePayload `json:"successful"`
	Failed     usagePercentilePayload `json:"failed"`
}

type usageExecutionPercentilesPayload struct {
	GeneratingStreaming usagePercentilePayload `json:"generating_streaming"`
}

type usageTTFTPercentilesPayload struct {
	GeneratingStreaming usagePercentilePayload `json:"generating_streaming"`
	UnknownExecution    usagePercentilePayload `json:"unknown_execution"`
}

type usagePercentilePayload struct {
	PopulationCount int64    `json:"population_count"`
	SampleCount     int64    `json:"sample_count"`
	Coverage        *float64 `json:"coverage"`
	P50             *float64 `json:"p50"`
	P95             *float64 `json:"p95"`
}

type usagePerformanceBreakdownPayload struct {
	Items      []usagePerformanceBreakdownItemPayload `json:"items"`
	OtherCount int64                                  `json:"other_count"`
}

type usagePerformanceBreakdownItemPayload struct {
	Value               string                           `json:"value"`
	Label               string                           `json:"label"`
	AttemptCount        int64                            `json:"attempt_count"`
	SuccessfulAttempts  int64                            `json:"successful_attempts"`
	FailedAttempts      int64                            `json:"failed_attempts"`
	SuccessfulExecution usageExecutionPopulationPayload  `json:"successful_execution"`
	LatencyMS           usageResultPercentilesPayload    `json:"latency_ms"`
	TTFTMS              usageTTFTPercentilesPayload      `json:"ttft_ms"`
	OutputTPS           usageExecutionPercentilesPayload `json:"output_tps"`
}

func registerUsagePerformanceRoute(router gin.IRoutes, usageProvider UsageProvider, usageIdentityProvider UsageIdentityProvider) {
	router.GET("/usage/performance", func(c *gin.Context) {
		filter, err := parseFixedUsageDiagnosticFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if usageProvider == nil {
			c.JSON(http.StatusOK, buildUsageAttemptPerformancePayload(filter, &repodto.UsageAttemptPerformanceRecord{}))
			return
		}
		record, err := usageProvider.GetUsageAttemptPerformance(c.Request.Context(), filter.repositoryFilter())
		if err != nil {
			writeInternalError(c, "get usage attempt performance failed", err)
			return
		}
		identities, err := loadUsageResolutionData(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "load usage performance account labels failed", err)
			return
		}
		c.JSON(http.StatusOK, buildUsageAttemptPerformancePayload(filter, record, newUsageIdentityResolver(identities)))
	})
}

func buildUsageAttemptPerformancePayload(filter usageDiagnosticFilter, record *repodto.UsageAttemptPerformanceRecord, resolvers ...usageIdentityResolver) usageAttemptPerformanceResponse {
	if record == nil {
		record = &repodto.UsageAttemptPerformanceRecord{}
	}
	resolver := usageIdentityResolver{}
	if len(resolvers) > 0 {
		resolver = resolvers[0]
	}
	return usageAttemptPerformanceResponse{
		WindowStart:        filter.StartTime.UTC().Format(time.RFC3339Nano),
		WindowEnd:          filter.EndTime.UTC().Format(time.RFC3339Nano),
		TotalAttempts:      record.TotalAttempts,
		SuccessfulAttempts: record.SuccessfulAttempts,
		FailedAttempts:     record.FailedAttempts,
		SuccessfulExecution: usageExecutionPopulationPayload{
			GeneratingStreaming: record.SuccessfulExecution.GeneratingStreaming,
			NonGenerating:       record.SuccessfulExecution.NonGenerating,
			NonStreaming:        record.SuccessfulExecution.NonStreaming,
			Unknown:             record.SuccessfulExecution.Unknown,
		},
		LatencyMS: usageResultPercentilesPayload{
			Successful: buildUsagePercentilePayload(record.SuccessfulLatencyMS),
			Failed:     buildUsagePercentilePayload(record.FailedLatencyMS),
		},
		TTFTMS: usageTTFTPercentilesPayload{
			GeneratingStreaming: buildUsagePercentilePayload(record.StreamingTTFTMS),
			UnknownExecution:    buildUsagePercentilePayload(record.UnknownExecutionTTFTMS),
		},
		OutputTPS: usageExecutionPercentilesPayload{
			GeneratingStreaming: buildUsagePercentilePayload(record.StreamingOutputTPS),
		},
		Providers: buildUsagePerformanceBreakdownPayload(record.Providers, identityLabel),
		Models:    buildUsagePerformanceBreakdownPayload(record.Models, identityLabel),
		Accounts: buildUsagePerformanceBreakdownPayload(record.Accounts, func(value string) string {
			if identity, matched := resolver.resolveByAuthIndex(value); matched {
				return identity.DisplayName
			}
			return value
		}),
	}
}

func buildUsagePerformanceBreakdownPayload(record repodto.UsagePerformanceBreakdownRecord, label func(string) string) usagePerformanceBreakdownPayload {
	items := make([]usagePerformanceBreakdownItemPayload, 0, len(record.Items))
	for _, item := range record.Items {
		items = append(items, usagePerformanceBreakdownItemPayload{
			Value:              item.Value,
			Label:              label(item.Value),
			AttemptCount:       item.AttemptCount,
			SuccessfulAttempts: item.SuccessfulAttempts,
			FailedAttempts:     item.FailedAttempts,
			SuccessfulExecution: usageExecutionPopulationPayload{
				GeneratingStreaming: item.SuccessfulExecution.GeneratingStreaming,
				NonGenerating:       item.SuccessfulExecution.NonGenerating,
				NonStreaming:        item.SuccessfulExecution.NonStreaming,
				Unknown:             item.SuccessfulExecution.Unknown,
			},
			LatencyMS: usageResultPercentilesPayload{
				Successful: buildUsagePercentilePayload(item.SuccessfulLatencyMS),
				Failed:     buildUsagePercentilePayload(item.FailedLatencyMS),
			},
			TTFTMS: usageTTFTPercentilesPayload{
				GeneratingStreaming: buildUsagePercentilePayload(item.StreamingTTFTMS),
				UnknownExecution:    buildUsagePercentilePayload(item.UnknownExecutionTTFTMS),
			},
			OutputTPS: usageExecutionPercentilesPayload{
				GeneratingStreaming: buildUsagePercentilePayload(item.StreamingOutputTPS),
			},
		})
	}
	return usagePerformanceBreakdownPayload{Items: items, OtherCount: record.OtherCount}
}

func buildUsagePercentilePayload(record repodto.UsagePercentileRecord) usagePercentilePayload {
	return usagePercentilePayload{
		PopulationCount: record.PopulationCount,
		SampleCount:     record.SampleCount,
		Coverage:        record.Coverage,
		P50:             record.P50,
		P95:             record.P95,
	}
}
