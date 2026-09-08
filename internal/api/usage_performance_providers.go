package api

import (
	"net/http"
	"time"

	repodto "cpa-usage/internal/repository/dto"
	"github.com/gin-gonic/gin"
)

type usagePerformanceProvidersResponse struct {
	WindowStart     string                                   `json:"window_start"`
	WindowEnd       string                                   `json:"window_end"`
	ProviderOptions []usagePerformanceProviderOptionResponse `json:"provider_options"`
}

type usagePerformanceProviderOptionResponse struct {
	Provider     string `json:"provider"`
	RequestCount int64  `json:"request_count"`
}

func registerUsagePerformanceProvidersRoute(router gin.IRoutes, usageProvider UsageProvider) {
	router.GET("/usage/performance/providers", func(c *gin.Context) {
		window, err := parseFixedUsageDiagnosticWindow(c.Request.URL.Query(), time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		filter := usageTimeFilter{usageWindow: window}
		if usageProvider == nil {
			c.JSON(http.StatusOK, buildUsagePerformanceProvidersPayload(filter, nil))
			return
		}
		record, err := usageProvider.ListUsagePerformanceProviders(c.Request.Context(), filter.repositoryScope())
		if err != nil {
			writeInternalError(c, "list usage performance providers failed", err)
			return
		}
		c.JSON(http.StatusOK, buildUsagePerformanceProvidersPayload(filter, record))
	})
}

func buildUsagePerformanceProvidersPayload(filter usageTimeFilter, record *repodto.UsagePerformanceProviderOptionsRecord) usagePerformanceProvidersResponse {
	options := []usagePerformanceProviderOptionResponse{}
	if record != nil {
		options = make([]usagePerformanceProviderOptionResponse, 0, len(record.ProviderOptions))
		for _, option := range record.ProviderOptions {
			options = append(options, usagePerformanceProviderOptionResponse{
				Provider: option.Provider, RequestCount: option.RequestCount,
			})
		}
	}
	return usagePerformanceProvidersResponse{
		WindowStart:     filter.StartTime.UTC().Format(time.RFC3339Nano),
		WindowEnd:       filter.EndTime.UTC().Format(time.RFC3339Nano),
		ProviderOptions: options,
	}
}
