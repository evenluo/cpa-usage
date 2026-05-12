package api

import (
	"net/http"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
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
	KeyAliases []analyticsKeyAliasRow  `json:"key_alias_breakdown"`
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

type analyticsKeyAliasTrendPoint struct {
	Label         string  `json:"label"`
	TotalCost     float64 `json:"total_cost"`
	TotalTokens   int64   `json:"total_tokens"`
	CostAvailable bool    `json:"cost_available"`
	CostStatus    string  `json:"cost_status"`
}

type analyticsKeyAliasRow struct {
	Label         string                         `json:"label"`
	Alias         string                         `json:"alias"`
	Traceability  string                         `json:"traceability"`
	Identity      string                         `json:"identity"`
	AuthType      entities.UsageIdentityAuthType `json:"auth_type"`
	AuthTypeName  string                         `json:"auth_type_name"`
	Type          string                         `json:"type"`
	Provider      string                         `json:"provider"`
	IsDeleted     bool                           `json:"is_deleted"`
	TotalCost     float64                        `json:"total_cost"`
	TotalTokens   int64                          `json:"total_tokens"`
	RequestCount  int64                          `json:"request_count"`
	SuccessCount  int64                          `json:"success_count"`
	FailureCount  int64                          `json:"failure_count"`
	SuccessRate   float64                        `json:"success_rate"`
	LastUsedAt    *time.Time                     `json:"last_used_at,omitempty"`
	CostAvailable bool                           `json:"cost_available"`
	CostStatus    string                         `json:"cost_status"`
	Trend         []analyticsKeyAliasTrendPoint  `json:"trend"`
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
		Trend:      []analyticsTrendPoint{},
		KeyAliases: []analyticsKeyAliasRow{},
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
	response.KeyAliases = make([]analyticsKeyAliasRow, 0, len(snapshot.KeyAliasBreakdown))
	for _, row := range snapshot.KeyAliasBreakdown {
		response.KeyAliases = append(response.KeyAliases, mapAnalyticsKeyAliasRow(row))
	}
	return response
}

func mapAnalyticsKeyAliasRow(row servicedto.AnalyticsKeyAliasBreakdown) analyticsKeyAliasRow {
	authType := entities.UsageIdentityAuthType(row.AuthType)
	identity := analyticsMaskedIdentity(authType, row.Identity)
	label := strings.TrimSpace(row.Alias)
	if label == "" {
		label = usageIdentityDisplayName(entities.UsageIdentity{
			Name:     row.Name,
			AuthType: authType,
			Type:     row.Type,
			Provider: row.Provider,
			Prefix:   row.Prefix,
			BaseURL:  row.BaseURL,
		})
	}
	if strings.TrimSpace(label) == "" {
		label = identity
	}
	traceability := identity
	if strings.TrimSpace(row.Provider) != "" {
		traceability += " · " + strings.TrimSpace(row.Provider)
	}
	trend := make([]analyticsKeyAliasTrendPoint, 0, len(row.Trend))
	for _, point := range row.Trend {
		trend = append(trend, analyticsKeyAliasTrendPoint{
			Label:         point.Label,
			TotalCost:     point.TotalCost,
			TotalTokens:   point.TotalTokens,
			CostAvailable: point.CostAvailable,
			CostStatus:    point.CostStatus,
		})
	}
	return analyticsKeyAliasRow{
		Label:         label,
		Alias:         row.Alias,
		Traceability:  traceability,
		Identity:      identity,
		AuthType:      authType,
		AuthTypeName:  row.AuthTypeName,
		Type:          row.Type,
		Provider:      row.Provider,
		IsDeleted:     row.IsDeleted,
		TotalCost:     row.TotalCost,
		TotalTokens:   row.TotalTokens,
		RequestCount:  row.RequestCount,
		SuccessCount:  row.SuccessCount,
		FailureCount:  row.FailureCount,
		SuccessRate:   row.SuccessRate,
		LastUsedAt:    row.LastUsedAt,
		CostAvailable: row.CostAvailable,
		CostStatus:    row.CostStatus,
		Trend:         trend,
	}
}

func analyticsMaskedIdentity(authType entities.UsageIdentityAuthType, identity string) string {
	if authType == entities.UsageIdentityAuthTypeAIProvider {
		return redact.APIKeyDisplayName(identity)
	}
	return strings.TrimSpace(identity)
}
