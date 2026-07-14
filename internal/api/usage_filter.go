package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	servicedto "cpa-usage/internal/service/dto"
)

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

func parseUsageTimeFilterQuery(req *http.Request, anchor time.Time) (servicedto.UsageFilter, error) {
	window, err := parseUsageWindowQuery(req, anchor)
	if err != nil {
		return servicedto.UsageFilter{}, err
	}
	filter := window.UsageFilter()
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

func parseUsageFilterQuery(req *http.Request, anchor time.Time) (servicedto.UsageFilter, error) {
	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		return servicedto.UsageFilter{}, err
	}
	return filter.UsageFilter(), nil
}

func parseUsageEventListFilterQuery(req *http.Request, anchor time.Time) (servicedto.UsageEventListFilter, error) {
	window, err := parseUsageWindowQuery(req, anchor)
	if err != nil {
		return servicedto.UsageEventListFilter{}, err
	}
	filter := servicedto.UsageEventListFilter{
		Window:   window,
		Limit:    servicedto.DefaultUsageEventsLimit,
		Page:     1,
		PageSize: servicedto.DefaultUsageEventsLimit,
	}
	if req == nil {
		return filter, nil
	}

	query := req.URL.Query()
	if pageValue := strings.TrimSpace(query.Get("page")); pageValue != "" {
		page, err := strconv.Atoi(pageValue)
		if err != nil || page < 1 {
			return servicedto.UsageEventListFilter{}, fmt.Errorf("invalid page %q", pageValue)
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
			return servicedto.UsageEventListFilter{}, fmt.Errorf("invalid page_size %q", pageSizeValue)
		}
		if _, ok := allowedUsageEventsPageSizes[pageSize]; !ok {
			return servicedto.UsageEventListFilter{}, fmt.Errorf("invalid page_size %q", pageSizeValue)
		}
		filter.PageSize = pageSize
		filter.Limit = pageSize
	}
	filter.Offset = (filter.Page - 1) * filter.PageSize
	filter.Model = strings.TrimSpace(query.Get("model"))
	filter.Provider = strings.TrimSpace(query.Get("provider"))
	filter.Source = strings.TrimSpace(query.Get("source"))
	filter.AuthIndex = strings.TrimSpace(query.Get("auth_index"))
	filter.Result = strings.TrimSpace(query.Get("result"))
	if filter.Result != "" && filter.Result != "success" && filter.Result != "failed" {
		return servicedto.UsageEventListFilter{}, fmt.Errorf("invalid result %q", filter.Result)
	}
	return filter, nil
}

func parseUsageWindowQuery(req *http.Request, anchor time.Time) (servicedto.UsageWindow, error) {
	if req == nil {
		return servicedto.UsageWindow{}, nil
	}

	rangeValue := strings.TrimSpace(req.URL.Query().Get("range"))
	if rangeValue == "" {
		rangeValue = "all"
	}

	fixedWindowEnd := anchor.UTC()
	window := servicedto.UsageWindow{Range: rangeValue, FixedWindowEnd: &fixedWindowEnd}
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
			return servicedto.UsageWindow{}, fmt.Errorf("custom range requires start and end")
		}
		startTime, err := parseCustomUsageRangeBoundary(startValue, false)
		if err != nil {
			return servicedto.UsageWindow{}, fmt.Errorf("invalid start: %w", err)
		}
		endTime, err := parseCustomUsageRangeBoundary(endValue, true)
		if err != nil {
			return servicedto.UsageWindow{}, fmt.Errorf("invalid end: %w", err)
		}
		startTime = startTime.UTC()
		endTime = endTime.UTC()
		if startTime.After(endTime) {
			return servicedto.UsageWindow{}, fmt.Errorf("custom range start must be before end")
		}
		window.StartTime = &startTime
		window.EndTime = &endTime
		return window, nil
	default:
		duration, ok := presetUsageRangeDurations[rangeValue]
		if !ok {
			return servicedto.UsageWindow{}, fmt.Errorf("unsupported usage range %q", rangeValue)
		}
		endTime := anchor.UTC()
		startTime := endTime.Add(-duration)
		window.StartTime = &startTime
		window.EndTime = &endTime
		return window, nil
	}
}
