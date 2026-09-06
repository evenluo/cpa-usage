package repository

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

func TestUsageFailureDistributionCountsAttemptsAndPreservesBreakdownParity(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "failure-distribution.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events := []entities.UsageEvent{
		{EventKey: "attempt-1", RequestID: "shared", Timestamp: start.Add(time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-b", Endpoint: "/v1/messages?private=1", Failed: true, StatusCode: 429},
		{EventKey: "attempt-2", RequestID: "shared", Timestamp: start.Add(2 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages#fragment", Failed: true, StatusCode: 429},
		{EventKey: "attempt-3", RequestID: "other", Timestamp: start.Add(3 * time.Hour), Provider: "claude", Model: "opus", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true, StatusCode: 0},
		{EventKey: "attempt-4", RequestID: "boundary", Timestamp: start.Add(4 * time.Hour), Provider: "openai", Model: "gpt-5", AuthIndex: "account-c", Endpoint: "/v1/responses", Failed: true, StatusCode: 500},
		{EventKey: "success", RequestID: "success", Timestamp: start.Add(5 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: false, StatusCode: 200},
		{EventKey: "outside", RequestID: "outside", Timestamp: start.Add(-time.Second), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true, StatusCode: 429},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	filter := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end}}
	result, err := BuildUsageFailureDistributionWithFilter(context.Background(), db, filter)
	if err != nil {
		t.Fatalf("build failure distribution: %v", err)
	}
	if result.TotalFailures != 4 {
		t.Fatalf("expected four failed attempts without request-id collapse, got %+v", result)
	}
	assertFailureBreakdownParity(t, result.TotalFailures, result.Categories)
	assertFailureBreakdownParity(t, result.TotalFailures, result.Statuses)
	assertFailureBreakdownParity(t, result.TotalFailures, result.Providers)
	assertFailureBreakdownParity(t, result.TotalFailures, result.Accounts)
	assertFailureBreakdownParity(t, result.TotalFailures, result.Models)
	assertFailureBreakdownParity(t, result.TotalFailures, result.Endpoints)
	if !reflect.DeepEqual(result.Statuses.Items, []dto.UsageFailureBreakdownItemRecord{{Value: "429", Count: 2}, {Value: "unknown", Count: 1}, {Value: "500", Count: 1}}) {
		t.Fatalf("unexpected stable status ranking: %+v", result.Statuses)
	}
	if !reflect.DeepEqual(result.Endpoints.Items, []dto.UsageFailureBreakdownItemRecord{{Value: "/v1/messages", Count: 3}, {Value: "/v1/responses", Count: 1}}) {
		t.Fatalf("expected public endpoint paths to aggregate without query/fragment, got %+v", result.Endpoints)
	}
	if !reflect.DeepEqual(result.Accounts.Items, []dto.UsageFailureBreakdownItemRecord{{Value: "account-a", Count: 2}, {Value: "account-b", Count: 1}, {Value: "account-c", Count: 1}}) {
		t.Fatalf("expected deterministic count/value ranking, got %+v", result.Accounts)
	}
}

func TestUsageFailureDistributionAndEvidenceShareDiagnosticSelection(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "failure-selection.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	start := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)
	end := start.Add(24 * time.Hour)
	events := []entities.UsageEvent{
		{EventKey: "399", Timestamp: start.Add(time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true, StatusCode: 399},
		{EventKey: "400", Timestamp: start.Add(2 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages?secret=1", Failed: true, StatusCode: 400},
		{EventKey: "499", Timestamp: start.Add(3 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true, StatusCode: 499},
		{EventKey: "500", Timestamp: start.Add(4 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true, StatusCode: 500},
		{EventKey: "unknown", Timestamp: start.Add(5 * time.Hour), Provider: "claude", Model: "sonnet", AuthIndex: "account-a", Endpoint: "/v1/messages", Failed: true},
	}
	if _, _, err := InsertUsageEvents(db, events); err != nil {
		t.Fatalf("insert usage events: %v", err)
	}

	for _, test := range []struct {
		status string
		want   int64
	}{
		{status: "3xx", want: 1},
		{status: "4xx", want: 2},
		{status: "5xx", want: 1},
		{status: "400", want: 1},
		{status: "unknown", want: 1},
	} {
		t.Run(test.status, func(t *testing.T) {
			selection := dto.UsageDiagnosticFilter{UsageTimeScope: dto.UsageTimeScope{StartTime: &start, EndTime: &end, Provider: "claude"}, Model: "sonnet", Account: "account-a", Endpoint: "/v1/messages", Status: test.status}
			distribution, err := BuildUsageFailureDistributionWithFilter(context.Background(), db, selection)
			if err != nil {
				t.Fatalf("build distribution: %v", err)
			}
			page, err := ListUsageEventsWithFilter(context.Background(), db, dto.UsageEventListFilter{
				UsageTimeScope: selection.UsageTimeScope, Model: selection.Model, Account: selection.Account,
				Endpoint: selection.Endpoint, Status: selection.Status, Result: "failed", Page: 1, PageSize: 10,
			})
			if err != nil {
				t.Fatalf("list evidence: %v", err)
			}
			if distribution.TotalFailures != test.want || page.TotalCount != test.want || int64(len(page.Events)) != test.want {
				t.Fatalf("expected aggregate/evidence parity %d, distribution=%+v page=%+v", test.want, distribution, page)
			}
		})
	}
}

func TestUsageFailureDistributionRequiresBoundedWindow(t *testing.T) {
	db, err := OpenDatabase(config.Config{SQLitePath: filepath.Join(t.TempDir(), "failure-unbounded.db")})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	closeTestDatabase(t, db)
	if _, err := BuildUsageFailureDistributionWithFilter(context.Background(), db, dto.UsageDiagnosticFilter{}); err == nil {
		t.Fatal("expected unbounded failure distribution to be rejected")
	}
}

func assertFailureBreakdownParity(t *testing.T, total int64, breakdown dto.UsageFailureBreakdownRecord) {
	t.Helper()
	count := breakdown.OtherCount
	for _, item := range breakdown.Items {
		count += item.Count
	}
	if count != total {
		t.Fatalf("breakdown does not preserve total %d: %+v", total, breakdown)
	}
}
