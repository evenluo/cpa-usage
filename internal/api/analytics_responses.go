package api

import (
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
	servicedto "cpa-usage/internal/service/dto"
)

type analyticsSummaryResponse struct {
	Range              string                     `json:"range"`
	Granularity        string                     `json:"granularity"`
	RangeStart         *time.Time                 `json:"range_start,omitempty"`
	RangeEnd           *time.Time                 `json:"range_end,omitempty"`
	PreviousRangeStart *time.Time                 `json:"previous_range_start,omitempty"`
	PreviousRangeEnd   *time.Time                 `json:"previous_range_end,omitempty"`
	Provider           string                     `json:"provider,omitempty"`
	Timezone           string                     `json:"timezone"`
	Summary            analyticsSummaryPayload    `json:"summary"`
	Comparison         analyticsComparisonPayload `json:"comparison"`
	Heatmap            analyticsHeatmapPayload    `json:"heatmap"`
	Trend              []analyticsTrendPoint      `json:"trend"`
	KeyAliases         []analyticsKeyAliasRow     `json:"key_alias_breakdown"`
	APIKeys            []analyticsKeyAliasRow     `json:"api_key_breakdown"`
	Models             []analyticsModelRow        `json:"model_distribution"`
	Time               []analyticsTrendPoint      `json:"time_breakdown"`
	Insights           []analyticsInsight         `json:"insights"`
	Providers          []analyticsProviderOption  `json:"provider_options"`
}

type analyticsCoreResponse struct {
	Range       string                    `json:"range"`
	Granularity string                    `json:"granularity"`
	RangeStart  *time.Time                `json:"range_start,omitempty"`
	RangeEnd    *time.Time                `json:"range_end,omitempty"`
	Provider    string                    `json:"provider,omitempty"`
	Timezone    string                    `json:"timezone"`
	Summary     analyticsSummaryPayload   `json:"summary"`
	Trend       []analyticsTrendPoint     `json:"trend"`
	KeyAliases  []analyticsKeyAliasRow    `json:"key_alias_breakdown"`
	APIKeys     []analyticsKeyAliasRow    `json:"api_key_breakdown"`
	Models      []analyticsModelRow       `json:"model_distribution"`
	Insights    []analyticsInsight        `json:"insights"`
	Providers   []analyticsProviderOption `json:"provider_options"`
}

type analyticsHeatmapResponse struct {
	Range       string                  `json:"range"`
	Granularity string                  `json:"granularity"`
	RangeStart  *time.Time              `json:"range_start,omitempty"`
	RangeEnd    *time.Time              `json:"range_end,omitempty"`
	Provider    string                  `json:"provider,omitempty"`
	Timezone    string                  `json:"timezone"`
	Heatmap     analyticsHeatmapPayload `json:"heatmap"`
}

type analyticsSummaryPayload struct {
	TotalCost             float64  `json:"total_cost"`
	TotalTokens           int64    `json:"total_tokens"`
	RequestCount          int64    `json:"request_count"`
	SuccessCount          int64    `json:"success_count"`
	FailureCount          int64    `json:"failure_count"`
	InputTokens           int64    `json:"input_tokens"`
	OutputTokens          int64    `json:"output_tokens"`
	ReasoningTokens       int64    `json:"reasoning_tokens"`
	CachedTokens          int64    `json:"cached_tokens"`
	SuccessRate           float64  `json:"success_rate"`
	CostAvailable         bool     `json:"cost_available"`
	CostStatus            string   `json:"cost_status"`
	CacheReadShare        float64  `json:"cache_read_share"`
	CacheReadShareState   string   `json:"cache_read_share_state"`
	EstimatedCacheSavings *float64 `json:"estimated_cache_savings,omitempty"`
}

type analyticsTrendPoint struct {
	Label           string    `json:"label"`
	BucketStart     time.Time `json:"bucket_start"`
	BucketEnd       time.Time `json:"bucket_end"`
	TotalCost       float64   `json:"total_cost"`
	TotalTokens     int64     `json:"total_tokens"`
	InputTokens     int64     `json:"input_tokens"`
	OutputTokens    int64     `json:"output_tokens"`
	ReasoningTokens int64     `json:"reasoning_tokens"`
	CachedTokens    int64     `json:"cached_tokens"`
	RequestCount    int64     `json:"request_count"`
	SuccessCount    int64     `json:"success_count"`
	FailureCount    int64     `json:"failure_count"`
	CostAvailable   bool      `json:"cost_available"`
	CostStatus      string    `json:"cost_status"`
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

type analyticsModelRow struct {
	Model                 string   `json:"model"`
	Provider              string   `json:"provider"`
	TotalCost             float64  `json:"total_cost"`
	TotalTokens           int64    `json:"total_tokens"`
	RequestCount          int64    `json:"request_count"`
	SuccessCount          int64    `json:"success_count"`
	FailureCount          int64    `json:"failure_count"`
	InputTokens           int64    `json:"input_tokens"`
	OutputTokens          int64    `json:"output_tokens"`
	ReasoningTokens       int64    `json:"reasoning_tokens"`
	CachedTokens          int64    `json:"cached_tokens"`
	SuccessRate           float64  `json:"success_rate"`
	TotalLatencyMS        int64    `json:"total_latency_ms"`
	LatencySampleCount    int64    `json:"latency_sample_count"`
	AverageLatencyMS      float64  `json:"average_latency_ms"`
	CostAvailable         bool     `json:"cost_available"`
	CostStatus            string   `json:"cost_status"`
	CacheReadShare        float64  `json:"cache_read_share"`
	CacheReadShareState   string   `json:"cache_read_share_state"`
	EstimatedCacheSavings *float64 `json:"estimated_cache_savings,omitempty"`
}

type analyticsInsight struct {
	Type        string  `json:"type"`
	Severity    string  `json:"severity"`
	Title       string  `json:"title"`
	Detail      string  `json:"detail"`
	Subject     string  `json:"subject"`
	MetricLabel string  `json:"metric_label"`
	MetricValue float64 `json:"metric_value"`
	Count       int64   `json:"count"`
	CostStatus  string  `json:"cost_status"`
}

type analyticsProviderOption struct {
	Provider      string  `json:"provider"`
	RequestCount  int64   `json:"request_count"`
	TotalTokens   int64   `json:"total_tokens"`
	TotalCost     float64 `json:"total_cost"`
	CostAvailable bool    `json:"cost_available"`
	CostStatus    string  `json:"cost_status"`
}

type analyticsComparisonPayload struct {
	HasPreviousPeriod     bool     `json:"has_previous_period"`
	TotalCostChangePct    *float64 `json:"total_cost_change_pct,omitempty"`
	TotalTokensChangePct  *float64 `json:"total_tokens_change_pct,omitempty"`
	RequestCountChangePct *float64 `json:"request_count_change_pct,omitempty"`
	SuccessRateChangePP   *float64 `json:"success_rate_change_pp,omitempty"`
}

type analyticsHeatmapPayload struct {
	Measure     string                `json:"measure"`
	MaxTokens   int64                 `json:"max_tokens"`
	MaxCost     float64               `json:"max_cost"`
	MaxRequests int64                 `json:"max_requests"`
	MaxFailures int64                 `json:"max_failures"`
	Rows        []analyticsHeatmapRow `json:"rows"`
}

type analyticsHeatmapRow struct {
	Date  string                 `json:"date"`
	Label string                 `json:"label"`
	Cells []analyticsHeatmapCell `json:"cells"`
}

type analyticsHeatmapCell struct {
	Hour          int       `json:"hour"`
	InRange       bool      `json:"in_range"`
	BucketStart   time.Time `json:"bucket_start"`
	BucketEnd     time.Time `json:"bucket_end"`
	TotalTokens   int64     `json:"total_tokens"`
	TotalCost     float64   `json:"total_cost"`
	RequestCount  int64     `json:"request_count"`
	FailureCount  int64     `json:"failure_count"`
	CostAvailable bool      `json:"cost_available"`
	CostStatus    string    `json:"cost_status"`
}

func buildAnalyticsCoreResponse(filter servicedto.UsageFilter, snapshot *dto.AnalyticsSummarySnapshot) analyticsCoreResponse {
	response := analyticsCoreResponse{
		Range:       filter.Range,
		Granularity: filter.Granularity,
		RangeStart:  filter.StartTime,
		RangeEnd:    filter.EndTime,
		Provider:    filter.Provider,
		Timezone:    time.Local.String(),
		Summary:     emptyAnalyticsSummaryPayload(),
		Trend:       []analyticsTrendPoint{},
		KeyAliases:  []analyticsKeyAliasRow{},
		APIKeys:     []analyticsKeyAliasRow{},
		Models:      []analyticsModelRow{},
		Insights:    []analyticsInsight{},
		Providers:   []analyticsProviderOption{},
	}
	if snapshot == nil {
		return response
	}
	response.Summary = mapAnalyticsSummaryPayload(snapshot.Summary)
	response.Trend = mapAnalyticsTrendPoints(snapshot.Trend)
	response.KeyAliases = mapAnalyticsKeyAliasRows(snapshot.KeyAliasBreakdown)
	response.APIKeys = mapAnalyticsKeyAliasRows(snapshot.APIKeyBreakdown)
	response.Models = mapAnalyticsModelRows(snapshot.ModelBreakdown)
	response.Insights = mapAnalyticsInsights(snapshot.Insights)
	response.Providers = mapAnalyticsProviderOptions(snapshot.ProviderOptions)
	return response
}

func buildAnalyticsSummaryResponse(filter servicedto.UsageFilter, snapshot *dto.AnalyticsSummarySnapshot) analyticsSummaryResponse {
	response := analyticsSummaryResponse{
		Range:       filter.Range,
		Granularity: filter.Granularity,
		RangeStart:  filter.StartTime,
		RangeEnd:    filter.EndTime,
		Provider:    filter.Provider,
		Timezone:    time.Local.String(),
		Summary:     emptyAnalyticsSummaryPayload(),
		Trend:       []analyticsTrendPoint{},
		Heatmap:     analyticsHeatmapPayload{Measure: "tokens", Rows: []analyticsHeatmapRow{}},
		KeyAliases:  []analyticsKeyAliasRow{},
		APIKeys:     []analyticsKeyAliasRow{},
		Models:      []analyticsModelRow{},
		Time:        []analyticsTrendPoint{},
		Insights:    []analyticsInsight{},
		Providers:   []analyticsProviderOption{},
	}
	if snapshot == nil {
		return response
	}
	response.PreviousRangeStart = snapshot.PreviousRangeStart
	response.PreviousRangeEnd = snapshot.PreviousRangeEnd
	response.Summary = mapAnalyticsSummaryPayload(snapshot.Summary)
	response.Comparison = analyticsComparisonPayload{
		HasPreviousPeriod:     snapshot.Comparison.HasPreviousPeriod,
		TotalCostChangePct:    snapshot.Comparison.TotalCostChangePct,
		TotalTokensChangePct:  snapshot.Comparison.TotalTokensChangePct,
		RequestCountChangePct: snapshot.Comparison.RequestCountChangePct,
		SuccessRateChangePP:   snapshot.Comparison.SuccessRateChangePP,
	}
	response.Heatmap = mapAnalyticsHeatmap(snapshot.Heatmap)
	response.Trend = mapAnalyticsTrendPoints(snapshot.Trend)
	response.KeyAliases = mapAnalyticsKeyAliasRows(snapshot.KeyAliasBreakdown)
	response.APIKeys = mapAnalyticsKeyAliasRows(snapshot.APIKeyBreakdown)
	response.Models = mapAnalyticsModelRows(snapshot.ModelBreakdown)
	response.Time = mapAnalyticsTrendPoints(snapshot.TimeBreakdown)
	response.Insights = mapAnalyticsInsights(snapshot.Insights)
	response.Providers = mapAnalyticsProviderOptions(snapshot.ProviderOptions)
	return response
}

// emptyAnalyticsSummaryPayload 是无数据时的响应默认值，保持既有 HTTP 契约不变。
func emptyAnalyticsSummaryPayload() analyticsSummaryPayload {
	return analyticsSummaryPayload{
		CostAvailable:       true,
		CostStatus:          dto.AnalyticsCostStatusAvailable,
		CacheReadShareState: dto.AnalyticsCacheReadShareStateNoPromptInput,
	}
}

func mapAnalyticsSummaryPayload(summary dto.AnalyticsSummary) analyticsSummaryPayload {
	return analyticsSummaryPayload{
		TotalCost:             summary.TotalCost,
		TotalTokens:           summary.TotalTokens,
		RequestCount:          summary.RequestCount,
		SuccessCount:          summary.SuccessCount,
		FailureCount:          summary.FailureCount,
		InputTokens:           summary.InputTokens,
		OutputTokens:          summary.OutputTokens,
		ReasoningTokens:       summary.ReasoningTokens,
		CachedTokens:          summary.CachedTokens,
		SuccessRate:           summary.SuccessRate,
		CostAvailable:         summary.CostAvailable,
		CostStatus:            summary.CostStatus,
		CacheReadShare:        summary.CacheReadShare,
		CacheReadShareState:   summary.CacheReadShareState,
		EstimatedCacheSavings: summary.EstimatedCacheSavings,
	}
}

func mapAnalyticsTrendPoints(points []dto.AnalyticsTrendPoint) []analyticsTrendPoint {
	result := make([]analyticsTrendPoint, 0, len(points))
	for _, point := range points {
		result = append(result, analyticsTrendPoint{
			Label:           point.Label,
			BucketStart:     point.BucketStart,
			BucketEnd:       point.BucketEnd,
			TotalCost:       point.TotalCost,
			TotalTokens:     point.TotalTokens,
			InputTokens:     point.InputTokens,
			OutputTokens:    point.OutputTokens,
			ReasoningTokens: point.ReasoningTokens,
			CachedTokens:    point.CachedTokens,
			RequestCount:    point.RequestCount,
			SuccessCount:    point.SuccessCount,
			FailureCount:    point.FailureCount,
			CostAvailable:   point.CostAvailable,
			CostStatus:      point.CostStatus,
		})
	}
	return result
}

func mapAnalyticsKeyAliasRows(rows []dto.AnalyticsKeyAliasBreakdown) []analyticsKeyAliasRow {
	result := make([]analyticsKeyAliasRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, mapAnalyticsKeyAliasRow(row))
	}
	return result
}

func mapAnalyticsModelRows(rows []dto.AnalyticsModelBreakdown) []analyticsModelRow {
	result := make([]analyticsModelRow, 0, len(rows))
	for _, row := range rows {
		result = append(result, analyticsModelRow{
			Model:                 row.Model,
			Provider:              row.Provider,
			TotalCost:             row.TotalCost,
			TotalTokens:           row.TotalTokens,
			RequestCount:          row.RequestCount,
			SuccessCount:          row.SuccessCount,
			FailureCount:          row.FailureCount,
			InputTokens:           row.InputTokens,
			OutputTokens:          row.OutputTokens,
			ReasoningTokens:       row.ReasoningTokens,
			CachedTokens:          row.CachedTokens,
			SuccessRate:           row.SuccessRate,
			TotalLatencyMS:        row.TotalLatencyMS,
			LatencySampleCount:    row.LatencySampleCount,
			AverageLatencyMS:      row.AverageLatencyMS,
			CostAvailable:         row.CostAvailable,
			CostStatus:            row.CostStatus,
			CacheReadShare:        row.CacheReadShare,
			CacheReadShareState:   row.CacheReadShareState,
			EstimatedCacheSavings: row.EstimatedCacheSavings,
		})
	}
	return result
}

func mapAnalyticsInsights(insights []dto.AnalyticsInsight) []analyticsInsight {
	result := make([]analyticsInsight, 0, len(insights))
	for _, insight := range insights {
		result = append(result, analyticsInsight{
			Type:        insight.Type,
			Severity:    insight.Severity,
			Title:       insight.Title,
			Detail:      insight.Detail,
			Subject:     insight.Subject,
			MetricLabel: insight.MetricLabel,
			MetricValue: insight.MetricValue,
			Count:       insight.Count,
			CostStatus:  insight.CostStatus,
		})
	}
	return result
}

func mapAnalyticsProviderOptions(options []dto.AnalyticsProviderOption) []analyticsProviderOption {
	result := make([]analyticsProviderOption, 0, len(options))
	for _, option := range options {
		result = append(result, analyticsProviderOption{
			Provider:      option.Provider,
			RequestCount:  option.RequestCount,
			TotalTokens:   option.TotalTokens,
			TotalCost:     option.TotalCost,
			CostAvailable: option.CostAvailable,
			CostStatus:    option.CostStatus,
		})
	}
	return result
}

func buildAnalyticsHeatmapResponse(filter servicedto.UsageFilter, heatmap dto.AnalyticsHeatmap) analyticsHeatmapResponse {
	return analyticsHeatmapResponse{
		Range:       filter.Range,
		Granularity: filter.Granularity,
		RangeStart:  filter.StartTime,
		RangeEnd:    filter.EndTime,
		Provider:    filter.Provider,
		Timezone:    time.Local.String(),
		Heatmap:     mapAnalyticsHeatmap(heatmap),
	}
}

func mapAnalyticsHeatmap(heatmap dto.AnalyticsHeatmap) analyticsHeatmapPayload {
	rows := make([]analyticsHeatmapRow, 0, len(heatmap.Rows))
	for _, row := range heatmap.Rows {
		cells := make([]analyticsHeatmapCell, 0, len(row.Cells))
		for _, cell := range row.Cells {
			cells = append(cells, analyticsHeatmapCell{
				Hour:          cell.Hour,
				InRange:       cell.InRange,
				BucketStart:   cell.BucketStart,
				BucketEnd:     cell.BucketEnd,
				TotalTokens:   cell.TotalTokens,
				TotalCost:     cell.TotalCost,
				RequestCount:  cell.RequestCount,
				FailureCount:  cell.FailureCount,
				CostAvailable: cell.CostAvailable,
				CostStatus:    cell.CostStatus,
			})
		}
		rows = append(rows, analyticsHeatmapRow{
			Date:  row.Date,
			Label: row.Label,
			Cells: cells,
		})
	}
	return analyticsHeatmapPayload{
		Measure:     heatmap.Measure,
		MaxTokens:   heatmap.MaxTokens,
		MaxCost:     heatmap.MaxCost,
		MaxRequests: heatmap.MaxRequests,
		MaxFailures: heatmap.MaxFailures,
		Rows:        rows,
	}
}

func mapAnalyticsKeyAliasRow(row dto.AnalyticsKeyAliasBreakdown) analyticsKeyAliasRow {
	authType := entities.UsageIdentityAuthType(row.AuthType)
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
		Label:         row.Label,
		Alias:         row.Alias,
		Traceability:  row.Traceability,
		Identity:      row.MaskedIdentity,
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
