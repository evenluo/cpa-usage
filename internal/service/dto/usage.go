package dto

import (
	"time"

	repodto "cpa-usage/internal/repository/dto"
)

const DefaultUsageEventsLimit = 100

// UsageWindow 是用户在 HTTP query 中选择的分析时间窗口。
type UsageWindow struct {
	Range          string
	StartTime      *time.Time
	EndTime        *time.Time
	FixedWindowEnd *time.Time
}

// UsageEventListFilter 是 Request Event Log 专属筛选条件。
type UsageEventListFilter struct {
	Window    UsageWindow
	Limit     int
	Page      int
	PageSize  int
	Offset    int
	Model     string
	Provider  string
	Source    string
	AuthIndex string
	Result    string
}

func (w UsageWindow) UsageFilter() UsageFilter {
	return UsageFilter{
		Range:          w.Range,
		StartTime:      w.StartTime,
		EndTime:        w.EndTime,
		FixedWindowEnd: w.FixedWindowEnd,
	}
}

func (f UsageEventListFilter) UsageFilter() UsageFilter {
	filter := f.Window.UsageFilter()
	filter.Limit = f.Limit
	filter.Page = f.Page
	filter.PageSize = f.PageSize
	filter.Offset = f.Offset
	filter.Model = f.Model
	filter.Provider = f.Provider
	filter.Source = f.Source
	filter.AuthIndex = f.AuthIndex
	filter.Result = f.Result
	return filter
}

func (f UsageFilter) SelectedWindowQueryFilter() repodto.UsageQueryFilter {
	return repodto.UsageQueryFilter{
		Range:          f.Range,
		StartTime:      f.StartTime,
		EndTime:        f.EndTime,
		FixedWindowEnd: f.FixedWindowEnd,
		Granularity:    f.Granularity,
		Provider:       f.Provider,
	}
}

func (f UsageFilter) EventListQueryFilter() repodto.UsageQueryFilter {
	return repodto.UsageQueryFilter{
		StartTime: f.StartTime,
		EndTime:   f.EndTime,
		Limit:     f.Limit,
		Page:      f.Page,
		PageSize:  f.PageSize,
		Offset:    f.Offset,
		Model:     f.Model,
		Provider:  f.Provider,
		Source:    f.Source,
		AuthIndex: f.AuthIndex,
		Result:    f.Result,
	}
}

// UsageFilter 是服务层的 usage 查询条件。
type UsageFilter struct {
	Range          string
	StartTime      *time.Time
	EndTime        *time.Time
	FixedWindowEnd *time.Time
	Limit          int
	Page           int
	PageSize       int
	Offset         int
	Model          string
	Granularity    string
	Provider       string
	Source         string
	AuthIndex      string
	Result         string
}

// UsageEventsPage 是 usage events 列表的服务层结果。
type UsageEventsPage struct {
	Events     []UsageEventRecord
	Models     []string
	TotalCount int64
	Page       int
	PageSize   int
	TotalPages int
}

// UsageEventFilterOptions 是 usage events 筛选项的服务层结果。
type UsageEventFilterOptions struct {
	Models []string
}

// UsageEventRecord 是单条 usage event 的服务层结果。
type UsageEventRecord struct {
	ID              uint
	Timestamp       time.Time
	APIGroupKey     string
	Model           string
	AuthType        string
	Provider        string
	Source          string
	AuthIndex       string
	Failed          bool
	LatencyMS       int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64
}

// UsageAnalysisModelStat 是按模型聚合的分析结果。
type UsageAnalysisModelStat struct {
	Model              string
	TotalRequests      int64
	SuccessCount       int64
	FailureCount       int64
	TotalTokens        int64
	InputTokens        int64
	OutputTokens       int64
	ReasoningTokens    int64
	CachedTokens       int64
	TotalLatencyMS     int64
	LatencySampleCount int64
}

// UsageAnalysisAPIStat 是按 API 聚合的分析结果。
type UsageAnalysisAPIStat struct {
	APIKey          string
	DisplayName     string
	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	TotalTokens     int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	Models          []UsageAnalysisModelStat
}

// UsageAnalysisSnapshot 是 analysis 的服务层结果。
type UsageAnalysisSnapshot struct {
	APIs   []UsageAnalysisAPIStat
	Models []UsageAnalysisModelStat
}

// UsageOverviewSummary 是 overview summary 的服务层结果。
type UsageOverviewSummary struct {
	RequestCount    int64
	TokenCount      int64
	WindowMinutes   int64
	RPM             float64
	TPM             float64
	TotalCost       float64
	CostAvailable   bool
	CachedTokens    int64
	ReasoningTokens int64
}

// UsageOverviewSeries 是 overview series 的服务层结果。
type UsageOverviewSeries struct {
	Requests        map[string]int64
	Tokens          map[string]int64
	RPM             map[string]float64
	TPM             map[string]float64
	Cost            map[string]float64
	InputTokens     map[string]int64
	OutputTokens    map[string]int64
	CachedTokens    map[string]int64
	ReasoningTokens map[string]int64
	Models          map[string]UsageOverviewSeries
}

// UsageOverviewHealthBlock 是 overview health 的单个时间块。
type UsageOverviewHealthBlock struct {
	StartTime time.Time
	EndTime   time.Time
	Success   int64
	Failure   int64
	Rate      float64
}

// UsageOverviewHealth 是 overview health 的聚合结果。
type UsageOverviewHealth struct {
	TotalSuccess  int64
	TotalFailure  int64
	SuccessRate   float64
	Rows          int
	Columns       int
	BucketSeconds int64
	WindowStart   time.Time
	WindowEnd     time.Time
	BlockDetails  []UsageOverviewHealthBlock
}

// UsageOverviewSnapshot 是 overview 的服务层结果。
type UsageOverviewSnapshot struct {
	Usage        *repodto.StatisticsSnapshot
	Summary      UsageOverviewSummary
	Series       UsageOverviewSeries
	HourlySeries UsageOverviewSeries
	DailySeries  UsageOverviewSeries
	Health       UsageOverviewHealth
}
