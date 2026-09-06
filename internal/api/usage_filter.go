package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"

	repodto "cpa-usage/internal/repository/dto"
)

type usageWindow struct {
	Range          string
	StartTime      *time.Time
	EndTime        *time.Time
	FixedWindowEnd *time.Time
}

type usageTimeFilter struct {
	usageWindow
	Provider string
}

type usageEventListFilter struct {
	usageDiagnosticFilter
	Page      int
	PageSize  int
	Offset    int
	Source    string
	AuthIndex string
	Result    string
}

type usageDiagnosticFilter struct {
	usageTimeFilter
	Model     string
	Account   string
	Endpoint  string
	Status    string
	RequestID string
}

type analyticsFilter struct {
	usageTimeFilter
	Granularity string
}

func (f usageTimeFilter) repositoryScope() repodto.UsageTimeScope {
	return repodto.UsageTimeScope{
		StartTime: f.StartTime,
		EndTime:   f.EndTime,
		Provider:  f.Provider,
	}
}

func (f usageTimeFilter) repositoryOverviewFilter() repodto.UsageOverviewFilter {
	return repodto.UsageOverviewFilter{
		UsageTimeScope: f.repositoryScope(),
		Range:          f.Range,
	}
}

func (f usageEventListFilter) repositoryFilter() repodto.UsageEventListFilter {
	return repodto.UsageEventListFilter{
		UsageTimeScope: f.repositoryScope(),
		Page:           f.Page,
		PageSize:       f.PageSize,
		Offset:         f.Offset,
		Model:          f.Model,
		Account:        f.Account,
		Endpoint:       f.Endpoint,
		Status:         f.Status,
		RequestID:      f.RequestID,
		Source:         f.Source,
		AuthIndex:      f.AuthIndex,
		Result:         f.Result,
	}
}

func (f usageDiagnosticFilter) repositoryFilter() repodto.UsageDiagnosticFilter {
	return repodto.UsageDiagnosticFilter{
		UsageTimeScope: f.repositoryScope(),
		Model:          f.Model,
		Account:        f.Account,
		Endpoint:       f.Endpoint,
		Status:         f.Status,
		RequestID:      f.RequestID,
	}
}

func (f analyticsFilter) repositoryFilter() repodto.AnalyticsFilter {
	return repodto.AnalyticsFilter{
		UsageTimeScope: f.repositoryScope(),
		Range:          f.Range,
		FixedWindowEnd: f.FixedWindowEnd,
		Granularity:    f.Granularity,
	}
}

var presetUsageRangeDurations = map[string]time.Duration{
	"4h":  4 * time.Hour,
	"8h":  8 * time.Hour,
	"12h": 12 * time.Hour,
	"24h": 24 * time.Hour,
	"7d":  7 * 24 * time.Hour,
	"30d": 30 * 24 * time.Hour,
}

var allowedUsageEventsPageSizes = map[int]struct{}{
	1:    {},
	10:   {},
	20:   {},
	50:   {},
	100:  {},
	500:  {},
	1000: {},
}

func parseUsageTimeFilterQuery(req *http.Request, anchor time.Time) (usageTimeFilter, error) {
	window, err := parseUsageWindowQuery(req, anchor)
	if err != nil {
		return usageTimeFilter{}, err
	}
	filter := usageTimeFilter{usageWindow: window}
	if req != nil {
		filter.Provider = strings.TrimSpace(req.URL.Query().Get("provider"))
	}
	return filter, nil
}

func parseCustomUsageRangeBoundary(value string, endOfDay bool) (time.Time, error) {
	if date, err := time.ParseInLocation(time.DateOnly, value, time.Local); err == nil {
		if endOfDay {
			return date.AddDate(0, 0, 1).Add(-time.Nanosecond), nil
		}
		return date, nil
	}
	return time.Parse(time.RFC3339, value)
}

func parseUsageEventListFilterQuery(req *http.Request, anchor time.Time) (usageEventListFilter, error) {
	window, err := parseUsageWindowQuery(req, anchor)
	if err != nil {
		return usageEventListFilter{}, err
	}
	filter := usageEventListFilter{
		usageDiagnosticFilter: usageDiagnosticFilter{usageTimeFilter: usageTimeFilter{usageWindow: window}},
		Page:                  1,
		PageSize:              repodto.DefaultUsageEventsLimit,
	}
	if req == nil {
		return filter, nil
	}

	query := req.URL.Query()
	if hasUsageDiagnosticSelection(query) {
		fixedWindow, err := parseFixedUsageDiagnosticWindow(query, anchor)
		if err != nil {
			return usageEventListFilter{}, err
		}
		filter.usageWindow = fixedWindow
	}
	if pageValue := strings.TrimSpace(query.Get("page")); pageValue != "" {
		page, err := strconv.Atoi(pageValue)
		if err != nil || page < 1 {
			return usageEventListFilter{}, fmt.Errorf("invalid page %q", pageValue)
		}
		filter.Page = page
	}
	pageSizeValue := strings.TrimSpace(query.Get("page_size"))
	if pageSizeValue == "" {
		pageSizeValue = strings.TrimSpace(query.Get("limit"))
	}
	if pageSizeValue != "" {
		pageSize, err := strconv.Atoi(pageSizeValue)
		if err != nil {
			return usageEventListFilter{}, fmt.Errorf("invalid page_size %q", pageSizeValue)
		}
		if _, ok := allowedUsageEventsPageSizes[pageSize]; !ok {
			return usageEventListFilter{}, fmt.Errorf("invalid page_size %q", pageSizeValue)
		}
		filter.PageSize = pageSize
	}
	filter.Offset = (filter.Page - 1) * filter.PageSize
	if hasUsageDiagnosticSelection(query) {
		selection, err := parseUsageDiagnosticSelection(query)
		if err != nil {
			return usageEventListFilter{}, err
		}
		filter.Provider = selection.Provider
		filter.Model = selection.Model
		filter.Account = selection.Account
		filter.Endpoint = selection.Endpoint
		filter.Status = selection.Status
		filter.RequestID = selection.RequestID
	} else {
		// Preserve the existing event-list contract when no diagnostic-only
		// selection is present; model/provider historically only trim whitespace.
		filter.Model = strings.TrimSpace(query.Get("model"))
		filter.Provider = strings.TrimSpace(query.Get("provider"))
	}
	filter.Source = strings.TrimSpace(query.Get("source"))
	filter.AuthIndex = strings.TrimSpace(query.Get("auth_index"))
	filter.Result = strings.TrimSpace(query.Get("result"))
	if filter.Result != "" && filter.Result != "success" && filter.Result != "failed" {
		return usageEventListFilter{}, fmt.Errorf("invalid result %q", filter.Result)
	}
	return filter, nil
}

func hasUsageDiagnosticSelection(query mapQuery) bool {
	for _, name := range []string{"account", "endpoint", "status", "request_id", "window_end"} {
		if strings.TrimSpace(query.Get(name)) != "" {
			return true
		}
	}
	return false
}

func parseFixedUsageDiagnosticFilterQuery(req *http.Request, anchor time.Time) (usageDiagnosticFilter, error) {
	window, err := parseFixedUsageDiagnosticWindow(nil, anchor)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	filter := usageDiagnosticFilter{usageTimeFilter: usageTimeFilter{usageWindow: window}}
	if req == nil {
		return filter, nil
	}
	query := req.URL.Query()
	window, err = parseFixedUsageDiagnosticWindow(query, anchor)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	selection, err := parseUsageDiagnosticSelection(query)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	selection.usageWindow = window
	return selection, nil
}

func parseFixedUsageDiagnosticWindow(query mapQuery, anchor time.Time) (usageWindow, error) {
	windowEnd := anchor.UTC()
	if query != nil {
		if rangeValue := strings.TrimSpace(query.Get("range")); rangeValue != "" && rangeValue != "24h" {
			return usageWindow{}, fmt.Errorf("diagnostic selection requires range %q", "24h")
		}
		if value := strings.TrimSpace(query.Get("window_end")); value != "" {
			parsed, err := time.Parse(time.RFC3339Nano, value)
			if err != nil {
				return usageWindow{}, fmt.Errorf("invalid window_end: %w", err)
			}
			windowEnd = parsed.UTC()
		}
	}
	windowStart := windowEnd.Add(-24 * time.Hour)
	return usageWindow{Range: "24h", StartTime: &windowStart, EndTime: &windowEnd, FixedWindowEnd: &windowEnd}, nil
}

func parseUsageDiagnosticSelection(query mapQuery) (usageDiagnosticFilter, error) {
	provider, err := normalizeDiagnosticValue("provider", query.Get("provider"), 128)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	model, err := normalizeDiagnosticValue("model", query.Get("model"), 128)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	account, err := normalizeDiagnosticValue("account", query.Get("account"), 128)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	endpoint, err := normalizeDiagnosticValue("endpoint", query.Get("endpoint"), 256)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	if strings.ContainsAny(endpoint, "?#") {
		return usageDiagnosticFilter{}, fmt.Errorf("invalid endpoint %q", endpoint)
	}
	status, err := normalizeDiagnosticStatus(query.Get("status"))
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	requestID, err := normalizeDiagnosticValue("request_id", query.Get("request_id"), 256)
	if err != nil {
		return usageDiagnosticFilter{}, err
	}
	return usageDiagnosticFilter{
		usageTimeFilter: usageTimeFilter{Provider: provider},
		Model:           model, Account: account, Endpoint: endpoint, Status: status, RequestID: requestID,
	}, nil
}

// mapQuery is the narrow query-string Interface used by both diagnostic entry
// points. url.Values satisfies it without exposing HTTP request parsing below.
type mapQuery interface {
	Get(string) string
}

func normalizeDiagnosticValue(name, value string, maxLength int) (string, error) {
	value = strings.TrimSpace(value)
	if len(value) > maxLength {
		return "", fmt.Errorf("%s exceeds %d bytes", name, maxLength)
	}
	if strings.IndexFunc(value, unicode.IsControl) >= 0 {
		return "", fmt.Errorf("invalid %s", name)
	}
	return value, nil
}

func normalizeDiagnosticStatus(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" || value == "unknown" || value == "other" {
		return value, nil
	}
	if len(value) == 3 && value[1:] == "xx" && value[0] >= '1' && value[0] <= '5' {
		return value, nil
	}
	code, err := strconv.Atoi(value)
	if err != nil || code < 100 || code > 599 || strconv.Itoa(code) != value {
		return "", fmt.Errorf("invalid status %q", value)
	}
	return value, nil
}

func parseUsageWindowQuery(req *http.Request, anchor time.Time) (usageWindow, error) {
	if req == nil {
		return usageWindow{}, nil
	}

	rangeValue := strings.TrimSpace(req.URL.Query().Get("range"))
	if rangeValue == "" {
		rangeValue = "all"
	}

	fixedWindowEnd := anchor.UTC()
	window := usageWindow{Range: rangeValue, FixedWindowEnd: &fixedWindowEnd}
	switch rangeValue {
	case "all":
		return window, nil
	case "today", "yesterday":
		localAnchor := anchor.In(time.Local)
		localStart := time.Date(localAnchor.Year(), localAnchor.Month(), localAnchor.Day(), 0, 0, 0, 0, time.Local)
		if rangeValue == "yesterday" {
			localStart = localStart.AddDate(0, 0, -1)
		}
		startTime := localStart.UTC()
		endTime := localStart.AddDate(0, 0, 1).Add(-time.Nanosecond).UTC()
		window.StartTime = &startTime
		window.EndTime = &endTime
		return window, nil
	case "custom":
		startValue := strings.TrimSpace(req.URL.Query().Get("start"))
		endValue := strings.TrimSpace(req.URL.Query().Get("end"))
		if startValue == "" || endValue == "" {
			return usageWindow{}, fmt.Errorf("custom range requires start and end")
		}
		startTime, err := parseCustomUsageRangeBoundary(startValue, false)
		if err != nil {
			return usageWindow{}, fmt.Errorf("invalid start: %w", err)
		}
		endTime, err := parseCustomUsageRangeBoundary(endValue, true)
		if err != nil {
			return usageWindow{}, fmt.Errorf("invalid end: %w", err)
		}
		startTime = startTime.UTC()
		endTime = endTime.UTC()
		if startTime.After(endTime) {
			return usageWindow{}, fmt.Errorf("custom range start must be before end")
		}
		window.StartTime = &startTime
		window.EndTime = &endTime
		return window, nil
	default:
		duration, ok := presetUsageRangeDurations[rangeValue]
		if !ok {
			return usageWindow{}, fmt.Errorf("unsupported usage range %q", rangeValue)
		}
		endTime := anchor.UTC()
		startTime := endTime.Add(-duration)
		window.StartTime = &startTime
		window.EndTime = &endTime
		return window, nil
	}
}
