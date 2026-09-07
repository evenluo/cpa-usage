package api

import (
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestParseUsageEventListFilterQueryPresetRange(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rangeVal string
		duration time.Duration
	}{
		{name: "24h", rangeVal: "24h", duration: 24 * time.Hour},
		{name: "30d", rangeVal: "30d", duration: 30 * 24 * time.Hour},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/api/v1/usage/overview?range="+tc.rangeVal, nil)
			anchor := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

			filter, err := parseUsageEventListFilterQuery(req, anchor)
			if err != nil {
				t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
			}
			if filter.Range != tc.rangeVal {
				t.Fatalf("expected range to be preserved, got %+v", filter)
			}
			if filter.StartTime == nil || filter.EndTime == nil {
				t.Fatalf("expected preset range to resolve concrete times, got %+v", filter)
			}
			if !filter.EndTime.Equal(anchor) {
				t.Fatalf("expected preset range end to use anchor time, got %+v", filter)
			}
			if !filter.StartTime.Equal(anchor.Add(-tc.duration)) {
				t.Fatalf("expected preset range start to subtract %s, got %+v", tc.duration, filter)
			}
		})
	}
}

func TestParseUsageEventListFilterQueryTodayRangeUsesLocalDayBoundary(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=today", nil)
	anchor := time.Date(2026, 4, 22, 12, 34, 56, 0, time.UTC)

	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.Range != "today" {
		t.Fatalf("expected today range to be preserved, got %+v", filter)
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected today range to resolve concrete times, got %+v", filter)
	}
	expectedStart := time.Date(2026, 4, 22, 0, 0, 0, 0, location).UTC()
	expectedEnd := time.Date(2026, 4, 23, 0, 0, 0, 0, location).Add(-time.Nanosecond).UTC()
	if !filter.StartTime.Equal(expectedStart) {
		t.Fatalf("expected today start %s, got %s", expectedStart, *filter.StartTime)
	}
	if !filter.EndTime.Equal(expectedEnd) {
		t.Fatalf("expected today end %s, got %s", expectedEnd, *filter.EndTime)
	}
}

func TestParseUsageEventListFilterQueryYesterdayRangeUsesPreviousLocalDayBoundary(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=yesterday", nil)
	anchor := time.Date(2026, 4, 22, 12, 34, 56, 0, time.UTC)

	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.Range != "yesterday" {
		t.Fatalf("expected yesterday range to be preserved, got %+v", filter)
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected yesterday range to resolve concrete times, got %+v", filter)
	}
	expectedStart := time.Date(2026, 4, 21, 0, 0, 0, 0, location).UTC()
	expectedEnd := time.Date(2026, 4, 22, 0, 0, 0, 0, location).Add(-time.Nanosecond).UTC()
	if !filter.StartTime.Equal(expectedStart) {
		t.Fatalf("expected yesterday start %s, got %s", expectedStart, *filter.StartTime)
	}
	if !filter.EndTime.Equal(expectedEnd) {
		t.Fatalf("expected yesterday end %s, got %s", expectedEnd, *filter.EndTime)
	}
}

func TestParseUsageEventListFilterQueryTodayRangeUsesLocalDSTBoundary(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=today", nil)
	anchor := time.Date(2026, 3, 8, 12, 0, 0, 0, location)

	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected today range to resolve concrete times, got %+v", filter)
	}
	expectedStart := time.Date(2026, 3, 8, 0, 0, 0, 0, location).UTC()
	expectedEnd := time.Date(2026, 3, 9, 0, 0, 0, 0, location).Add(-time.Nanosecond).UTC()
	if !filter.StartTime.Equal(expectedStart) {
		t.Fatalf("expected DST today start %s, got %s", expectedStart, *filter.StartTime)
	}
	if !filter.EndTime.Equal(expectedEnd) {
		t.Fatalf("expected DST today end %s, got %s", expectedEnd, *filter.EndTime)
	}
}

func TestParseUsageEventListFilterQueryCustomRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=custom&start=2026-04-20T00:00:00Z&end=2026-04-21T23:59:59Z", nil)

	filter, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected custom range bounds, got %+v", filter)
	}
	if !filter.StartTime.Equal(time.Date(2026, 4, 20, 0, 0, 0, 0, time.UTC)) {
		t.Fatalf("unexpected custom start: %+v", filter)
	}
	if !filter.EndTime.Equal(time.Date(2026, 4, 21, 23, 59, 59, 0, time.UTC)) {
		t.Fatalf("unexpected custom end: %+v", filter)
	}
}

func TestParseUsageEventListFilterQueryCustomDateRangeUsesLocalDayBoundary(t *testing.T) {
	previousLocal := time.Local
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatalf("load location: %v", err)
	}
	t.Cleanup(func() { time.Local = previousLocal })
	time.Local = location

	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=custom&start=2026-04-20&end=2026-04-21", nil)

	filter, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.StartTime == nil || filter.EndTime == nil {
		t.Fatalf("expected custom date range bounds, got %+v", filter)
	}
	expectedStart := time.Date(2026, 4, 20, 0, 0, 0, 0, location).UTC()
	expectedEnd := time.Date(2026, 4, 22, 0, 0, 0, 0, location).Add(-time.Nanosecond).UTC()
	if !filter.StartTime.Equal(expectedStart) {
		t.Fatalf("expected custom date start %s, got %s", expectedStart, *filter.StartTime)
	}
	if !filter.EndTime.Equal(expectedEnd) {
		t.Fatalf("expected custom date end %s, got %s", expectedEnd, *filter.EndTime)
	}
}

func TestParseUsageEventListFilterQueryRejectsInvalidCustomRange(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=custom&start=2026-04-21T00:00:00Z&end=2026-04-20T23:59:59Z", nil)

	_, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err == nil {
		t.Fatal("expected invalid custom range error")
	}
}

func TestParseUsageTimeFilterQueryIgnoresEventListFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/overview?range=24h&page=3&page_size=10&model=ignored&source=ignored&auth_index=ignored&result=failed&provider=OpenAI", nil)
	anchor := time.Date(2026, 4, 22, 12, 0, 0, 0, time.UTC)

	filter, err := parseUsageTimeFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseUsageTimeFilterQuery returned error: %v", err)
	}
	if filter.Range != "24h" || filter.Provider != "OpenAI" {
		t.Fatalf("expected selected analysis window with provider scope, got %+v", filter)
	}
	if filter.repositoryScope().Provider != "OpenAI" {
		t.Fatalf("expected repository scope to contain only the normalized provider and bounds, got %+v", filter.repositoryScope())
	}
}

func TestParseUsageEventListFilterQueryDefaultsEventsPagination(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/events?range=all", nil)

	filter, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.Page != 1 || filter.PageSize != 100 || filter.Offset != 0 {
		t.Fatalf("expected default pagination, got %+v", filter)
	}
}

func TestParseUsageEventListFilterQueryAcceptsEventsPaginationAndFilters(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/events?page=3&page_size=100&model=%20claude-sonnet%20&provider=%20codex%20&source=%20source-a%20&auth_index=%202%20", nil)

	filter, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.Page != 3 || filter.PageSize != 100 || filter.Offset != 200 {
		t.Fatalf("expected page 3/page size 100 offset 200, got %+v", filter)
	}
	if filter.Model != "claude-sonnet" || filter.Provider != "codex" || filter.Source != "source-a" || filter.AuthIndex != "2" {
		t.Fatalf("expected trimmed server-side filters, got %+v", filter)
	}
}

func TestParseUsageEventListFilterQueryAcceptsCompactEvidencePageSize(t *testing.T) {
	req := httptest.NewRequest("GET", "/api/v1/usage/events?range=24h&page_size=10", nil)

	filter, err := parseUsageEventListFilterQuery(req, time.Time{})
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	if filter.Page != 1 || filter.PageSize != 10 || filter.Offset != 0 {
		t.Fatalf("expected compact evidence pagination, got %+v", filter)
	}
}

func TestParseUsageEventListFilterQueryRejectsRemovedLimitAlias(t *testing.T) {
	for _, query := range []string{"limit=20", "page_size=50&limit=20"} {
		req := httptest.NewRequest("GET", "/api/v1/usage/events?"+query, nil)
		if _, err := parseUsageEventListFilterQuery(req, time.Time{}); err == nil {
			t.Fatal("removed limit alias must not silently change selection")
		}
	}
}

func TestParseUsageEventListFilterQueryRejectsInvalidEventsPagination(t *testing.T) {
	tests := []string{
		"/api/v1/usage/events?page=0",
		"/api/v1/usage/events?page_size=25",
	}
	for _, path := range tests {
		req := httptest.NewRequest("GET", path, nil)
		if _, err := parseUsageEventListFilterQuery(req, time.Time{}); err == nil {
			t.Fatalf("expected pagination error for %s", path)
		}
	}
}

func TestParseFixedUsageDiagnosticFilterQueryBuildsBoundedSelection(t *testing.T) {
	anchor := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest("GET", "/api/v1/usage/failures?range=24h&provider=%20claude%20&model=%20sonnet%20&model_alias=%20sonnet-route%20&account=%20auth-1%20&endpoint=%20/v1/messages%20&status=4XX&min_latency_ms=500", nil)

	filter, err := parseFixedUsageDiagnosticFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseFixedUsageDiagnosticFilterQuery returned error: %v", err)
	}
	if !filter.StartTime.Equal(anchor.Add(-24*time.Hour)) || !filter.EndTime.Equal(anchor) {
		t.Fatalf("expected exact fixed 24h bounds, got %+v", filter)
	}
	if filter.Provider != "claude" || filter.Model != "sonnet" || filter.ModelAlias != "sonnet-route" || filter.Account != "auth-1" || filter.Endpoint != "/v1/messages" || filter.Status != "4xx" {
		t.Fatalf("unexpected normalized diagnostic selection: %+v", filter)
	}
	if filter.MinLatencyMS == nil || *filter.MinLatencyMS != 500 {
		t.Fatalf("expected normalized inclusive latency threshold, got %+v", filter)
	}
	if got := filter.repositoryFilter(); got.Provider != "claude" || got.ModelAlias != "sonnet-route" || got.Account != "auth-1" || got.Status != "4xx" || got.MinLatencyMS == nil || *got.MinLatencyMS != 500 {
		t.Fatalf("unexpected repository diagnostic filter: %+v", got)
	}
}

func TestDiagnosticSelectionIsSharedWithRequestEvidence(t *testing.T) {
	anchor := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest("GET", "/api/v1/usage/events?range=24h&page=3&page_size=10&provider=claude&model=sonnet&model_alias=sonnet-route&account=auth-1&endpoint=/v1/messages&status=429&request_id=request-42&result=failed", nil)

	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parseUsageEventListFilterQuery returned error: %v", err)
	}
	got := filter.repositoryFilter()
	if got.Page != 3 || got.Offset != 20 || got.Result != "failed" {
		t.Fatalf("unexpected evidence pagination/result: %+v", got)
	}
	if got.Provider != "claude" || got.Model != "sonnet" || got.ModelAlias != "sonnet-route" || got.Account != "auth-1" || got.Endpoint != "/v1/messages" || got.Status != "429" || got.RequestID != "request-42" {
		t.Fatalf("expected shared diagnostic selection, got %+v", got)
	}
}

func TestRequestEvidenceRejectsDiagnosticSelectionOutsideFixedWindow(t *testing.T) {
	for _, path := range []string{
		"/api/v1/usage/events?range=all&status=4xx",
		"/api/v1/usage/events?range=7d&account=auth-1",
		"/api/v1/usage/events?range=custom&start=2026-09-01&end=2026-09-07&endpoint=/v1/messages",
		"/api/v1/usage/events?range=all&request_id=request-42",
		"/api/v1/usage/events?range=7d&model_alias=sonnet-route",
	} {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			if _, err := parseUsageEventListFilterQuery(req, time.Now()); err == nil {
				t.Fatalf("expected diagnostic evidence outside fixed 24h window to be rejected: %s", path)
			}
		})
	}
}

func TestRequestIDEvidenceSelectionHasExactBoundsAndParameterValidation(t *testing.T) {
	anchor := time.Date(2026, 9, 7, 12, 0, 0, 123456789, time.UTC)
	requestID := strings.Repeat("r", 256)
	req := httptest.NewRequest("GET", "/api/v1/usage/events?range=24h&request_id="+requestID, nil)
	filter, err := parseUsageEventListFilterQuery(req, anchor)
	if err != nil {
		t.Fatalf("parse request-id evidence selection: %v", err)
	}
	if filter.RequestID != requestID || filter.StartTime == nil || filter.EndTime == nil || !filter.StartTime.Equal(anchor.Add(-24*time.Hour)) || !filter.EndTime.Equal(anchor) {
		t.Fatalf("expected normalized request ID with complete fixed bounds, got %+v", filter)
	}

	for _, path := range []string{
		"/api/v1/usage/events?range=24h&request_id=" + strings.Repeat("r", 257),
		"/api/v1/usage/events?range=24h&request_id=request%0Aid",
	} {
		if _, err := parseUsageEventListFilterQuery(httptest.NewRequest("GET", path, nil), anchor); err == nil {
			t.Fatalf("expected invalid request ID to be rejected: %s", path)
		}
	}
}

func TestParseFixedUsageDiagnosticFilterQueryRejectsUnboundedOrInvalidSelections(t *testing.T) {
	tests := []string{
		"/api/v1/usage/failures?range=7d",
		"/api/v1/usage/failures?status=99",
		"/api/v1/usage/failures?status=600",
		"/api/v1/usage/failures?status=04xx",
		"/api/v1/usage/failures?endpoint=/v1/messages%3Ftoken%3Dsecret",
		"/api/v1/usage/failures?endpoint=/v1/messages%23fragment",
		"/api/v1/usage/failures?endpoint=/v1/%0Amessages",
		"/api/v1/usage/failures?min_latency_ms=0",
		"/api/v1/usage/failures?min_latency_ms=-1",
		"/api/v1/usage/failures?min_latency_ms=0500",
		"/api/v1/usage/failures?min_latency_ms=1.5",
		"/api/v1/usage/failures?min_latency_ms=9223372036854775808",
	}
	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			req := httptest.NewRequest("GET", path, nil)
			if _, err := parseFixedUsageDiagnosticFilterQuery(req, time.Now()); err == nil {
				t.Fatalf("expected %s to be rejected", path)
			}
		})
	}
}

func TestSlowAttemptSelectionRoundTripsIntoEvidence(t *testing.T) {
	anchor := time.Date(2026, 9, 7, 12, 0, 0, 123456789, time.UTC)
	performanceRequest := httptest.NewRequest("GET", "/api/v1/usage/performance?range=24h&provider=claude&model=sonnet&account=auth-1&min_latency_ms=500", nil)
	performanceFilter, err := parseFixedUsageDiagnosticFilterQuery(performanceRequest, anchor)
	if err != nil {
		t.Fatalf("parse performance filter: %v", err)
	}
	evidenceRequest := httptest.NewRequest("GET", "/api/v1/usage/events?range=24h&provider=claude&model=sonnet&account=auth-1&min_latency_ms=500&window_end="+url.QueryEscape(performanceFilter.EndTime.Format(time.RFC3339Nano)), nil)
	evidenceFilter, err := parseUsageEventListFilterQuery(evidenceRequest, anchor.Add(time.Minute))
	if err != nil {
		t.Fatalf("parse evidence filter: %v", err)
	}
	if evidenceFilter.MinLatencyMS == nil || *evidenceFilter.MinLatencyMS != 500 || !evidenceFilter.EndTime.Equal(*performanceFilter.EndTime) || !evidenceFilter.StartTime.Equal(*performanceFilter.StartTime) {
		t.Fatalf("slow selection lost threshold or frozen window: performance=%+v evidence=%+v", performanceFilter, evidenceFilter)
	}
}

func TestNormalizeDiagnosticStatusAcceptsFamiliesBoundariesAndUnknown(t *testing.T) {
	for _, value := range []string{"unknown", "other", "1xx", "2xx", "3xx", "4xx", "5xx", "100", "399", "400", "499", "500", "599"} {
		if got, err := normalizeDiagnosticStatus(value); err != nil || got != value {
			t.Fatalf("expected %q to normalize unchanged, got %q err=%v", value, got, err)
		}
	}
}

func TestFrozenDiagnosticWindowRoundTripsNanosecondsIntoEvidence(t *testing.T) {
	anchor := time.Date(2026, 9, 7, 12, 0, 0, 123456789, time.UTC)
	failureFilter, err := parseFixedUsageDiagnosticFilterQuery(httptest.NewRequest("GET", "/api/v1/usage/failures", nil), anchor)
	if err != nil {
		t.Fatalf("parse failure filter: %v", err)
	}
	windowEnd := failureFilter.EndTime.Format(time.RFC3339Nano)
	evidenceRequest := httptest.NewRequest("GET", "/api/v1/usage/events?range=24h&window_end="+url.QueryEscape(windowEnd), nil)
	evidenceFilter, err := parseUsageEventListFilterQuery(evidenceRequest, anchor.Add(time.Minute))
	if err != nil {
		t.Fatalf("parse evidence filter: %v", err)
	}
	if !evidenceFilter.EndTime.Equal(*failureFilter.EndTime) || !evidenceFilter.StartTime.Equal(*failureFilter.StartTime) {
		t.Fatalf("frozen diagnostic window lost precision: failure=%+v evidence=%+v", failureFilter.usageWindow, evidenceFilter.usageWindow)
	}
}

func TestRequestEvidenceValidatesEverySelectionUniformly(t *testing.T) {
	for _, query := range []string{
		"range=all&model=" + strings.Repeat("m", 129),
		"range=24h&status=4xx&model=" + strings.Repeat("m", 129),
		"range=all&provider=" + strings.Repeat("p", 129),
		"range=24h&model=model%0Aname",
	} {
		req := httptest.NewRequest("GET", "/api/v1/usage/events?"+query, nil)
		if _, err := parseUsageEventListFilterQuery(req, time.Now()); err == nil {
			t.Fatalf("expected uniform selection validation for %s", query)
		}
	}
}
