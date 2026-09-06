package dto

import "time"

// UsageTimeScope 是 usage 读模型共享的时间与 provider 范围。
type UsageTimeScope struct {
	StartTime *time.Time
	EndTime   *time.Time
	Provider  string
}

// UsageOverviewFilter 是 overview 与 request health 共享的查询条件。
type UsageOverviewFilter struct {
	UsageTimeScope
	Range string
}

// AnalyticsFilter 是 analytics 读模型的查询条件。
type AnalyticsFilter struct {
	UsageTimeScope
	Range          string
	FixedWindowEnd *time.Time
	Granularity    string
}

// UsageDiagnosticFilter is the shared fixed-window selection consumed by
// failure aggregation and Request Evidence. Status is canonicalized by the API
// layer to an exact HTTP code, an HTTP family (1xx-5xx), "unknown", or empty.
type UsageDiagnosticFilter struct {
	UsageTimeScope
	Model    string
	Account  string
	Endpoint string
	Status   string
}

// UsageEventListFilter 是 Request Evidence 列表的查询条件。
type UsageEventListFilter struct {
	UsageTimeScope
	Page      int
	PageSize  int
	Offset    int
	Model     string
	Account   string
	Endpoint  string
	Status    string
	Source    string
	AuthIndex string
	Result    string
}

func (f UsageEventListFilter) DiagnosticFilter() UsageDiagnosticFilter {
	return UsageDiagnosticFilter{
		UsageTimeScope: f.UsageTimeScope,
		Model:          f.Model,
		Account:        f.Account,
		Endpoint:       f.Endpoint,
		Status:         f.Status,
	}
}

const DefaultUsageEventsLimit = 100
