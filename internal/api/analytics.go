package api

import (
	"net/http"
	"time"

	"cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/gin-gonic/gin"
)

type analyticsSummaryResponse struct {
	Range      string                  `json:"range"`
	RangeStart *time.Time              `json:"range_start,omitempty"`
	RangeEnd   *time.Time              `json:"range_end,omitempty"`
	Timezone   string                  `json:"timezone"`
	Summary    analyticsSummaryPayload `json:"summary"`
	Trend      []analyticsTrendPoint   `json:"trend"`
}

type analyticsSummaryPayload struct {
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int64   `json:"total_tokens"`
	RequestCount  int64   `json:"request_count"`
	SuccessCount  int64   `json:"success_count"`
	FailureCount  int64   `json:"failure_count"`
	SuccessRate   float64 `json:"success_rate"`
	CostAvailable bool    `json:"cost_available"`
	CostStatus    string  `json:"cost_status"`
}

type analyticsTrendPoint struct {
	Label         string    `json:"label"`
	BucketStart   time.Time `json:"bucket_start"`
	BucketEnd     time.Time `json:"bucket_end"`
	TotalCost     float64   `json:"total_cost"`
	TotalTokens   int64     `json:"total_tokens"`
	RequestCount  int64     `json:"request_count"`
	SuccessCount  int64     `json:"success_count"`
	FailureCount  int64     `json:"failure_count"`
	CostAvailable bool      `json:"cost_available"`
	CostStatus    string    `json:"cost_status"`
}

func registerAnalyticsRoutes(router gin.IRoutes, analyticsProvider service.AnalyticsProvider) {
	router.GET("/analytics/summary", func(c *gin.Context) {
		filter, err := parseUsageTimeFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if analyticsProvider == nil {
			c.JSON(http.StatusOK, buildAnalyticsSummaryResponse(filter, nil))
			return
		}

		snapshot, err := analyticsProvider.GetAnalyticsSummary(c.Request.Context(), filter)
		if err != nil {
			writeInternalError(c, "get analytics summary failed", err)
			return
		}
		c.JSON(http.StatusOK, buildAnalyticsSummaryResponse(filter, snapshot))
	})
}

func buildAnalyticsSummaryResponse(filter servicedto.UsageFilter, snapshot *servicedto.AnalyticsSummarySnapshot) analyticsSummaryResponse {
	response := analyticsSummaryResponse{
		Range:      filter.Range,
		RangeStart: filter.StartTime,
		RangeEnd:   filter.EndTime,
		Timezone:   time.Local.String(),
		Summary: analyticsSummaryPayload{
			CostAvailable: true,
			CostStatus:    dto.AnalyticsCostStatusAvailable,
		},
		Trend: []analyticsTrendPoint{},
	}
	if snapshot == nil {
		return response
	}
	response.Summary = analyticsSummaryPayload{
		TotalCost:     snapshot.Summary.TotalCost,
		TotalTokens:   snapshot.Summary.TotalTokens,
		RequestCount:  snapshot.Summary.RequestCount,
		SuccessCount:  snapshot.Summary.SuccessCount,
		FailureCount:  snapshot.Summary.FailureCount,
		SuccessRate:   snapshot.Summary.SuccessRate,
		CostAvailable: snapshot.Summary.CostAvailable,
		CostStatus:    snapshot.Summary.CostStatus,
	}
	response.Trend = make([]analyticsTrendPoint, 0, len(snapshot.Trend))
	for _, point := range snapshot.Trend {
		response.Trend = append(response.Trend, analyticsTrendPoint{
			Label:         point.Label,
			BucketStart:   point.BucketStart,
			BucketEnd:     point.BucketEnd,
			TotalCost:     point.TotalCost,
			TotalTokens:   point.TotalTokens,
			RequestCount:  point.RequestCount,
			SuccessCount:  point.SuccessCount,
			FailureCount:  point.FailureCount,
			CostAvailable: point.CostAvailable,
			CostStatus:    point.CostStatus,
		})
	}
	return response
}
