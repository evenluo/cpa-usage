package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
	"github.com/gin-gonic/gin"
)

// AnalyticsProvider 是 Usage Intelligence 分析读模型的 HTTP 层入口 seam；
// 实现由 repository 的 AnalyticsReader 提供，raw/rollup 选择对 HTTP 层不可见。
type AnalyticsProvider interface {
	GetAnalyticsSummary(context.Context, dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error)
	GetAnalyticsCore(context.Context, dto.UsageQueryFilter) (*dto.AnalyticsSummarySnapshot, error)
	GetAnalyticsHeatmap(context.Context, dto.UsageQueryFilter) (dto.AnalyticsHeatmap, error)
}

func registerAnalyticsRoutes(router gin.IRoutes, analyticsProvider AnalyticsProvider) {
	router.GET("/analytics/core", func(c *gin.Context) {
		filter, err := parseAnalyticsSummaryFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if analyticsProvider == nil {
			c.JSON(http.StatusOK, buildAnalyticsCoreResponse(filter, nil))
			return
		}

		snapshot, err := analyticsProvider.GetAnalyticsCore(c.Request.Context(), filter.SelectedWindowQueryFilter())
		if err != nil {
			writeInternalError(c, "get analytics core failed", err)
			return
		}
		c.JSON(http.StatusOK, buildAnalyticsCoreResponse(filter, snapshot))
	})

	router.GET("/analytics/summary", func(c *gin.Context) {
		filter, err := parseAnalyticsSummaryFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if analyticsProvider == nil {
			c.JSON(http.StatusOK, buildAnalyticsSummaryResponse(filter, nil))
			return
		}

		snapshot, err := analyticsProvider.GetAnalyticsSummary(c.Request.Context(), filter.SelectedWindowQueryFilter())
		if err != nil {
			writeInternalError(c, "get analytics summary failed", err)
			return
		}
		c.JSON(http.StatusOK, buildAnalyticsSummaryResponse(filter, snapshot))
	})

	router.GET("/analytics/heatmap", func(c *gin.Context) {
		filter, err := parseAnalyticsSummaryFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if analyticsProvider == nil {
			c.JSON(http.StatusOK, buildAnalyticsHeatmapResponse(filter, dto.AnalyticsHeatmap{Measure: "tokens", Rows: []dto.AnalyticsHeatmapRow{}}))
			return
		}

		heatmap, err := analyticsProvider.GetAnalyticsHeatmap(c.Request.Context(), filter.SelectedWindowQueryFilter())
		if err != nil {
			writeInternalError(c, "get analytics heatmap failed", err)
			return
		}
		c.JSON(http.StatusOK, buildAnalyticsHeatmapResponse(filter, heatmap))
	})
}

func parseAnalyticsSummaryFilterQuery(req *http.Request, anchor time.Time) (servicedto.UsageFilter, error) {
	filter, err := parseUsageTimeFilterQuery(req, anchor)
	if err != nil {
		return servicedto.UsageFilter{}, err
	}
	filter.Granularity = "hour"
	if req != nil {
		filter.Provider = strings.TrimSpace(req.URL.Query().Get("provider"))
		if value := strings.TrimSpace(req.URL.Query().Get("granularity")); value != "" {
			switch value {
			case "hour", "day":
				filter.Granularity = value
			default:
				return servicedto.UsageFilter{}, fmt.Errorf("unsupported granularity %q", value)
			}
		}
	}
	return filter, nil
}
