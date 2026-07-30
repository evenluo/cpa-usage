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
