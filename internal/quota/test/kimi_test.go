package test

import (
	"context"
	"encoding/json"
	"testing"

	"cpa-usage/internal/cpa/dto/apicall"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/quota"
)

func TestKimiProviderCallsUsageRequest(t *testing.T) {
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   `{"usage":{"used":3,"limit":10,"remaining":7,"reset_at":"2026-05-09T12:00:00Z"},"limits":[{"name":"daily","title":"Daily","scope":"request","used":3,"limit":10,"remaining":7,"window":{"duration":1,"timeUnit":"day"},"detail":{"used":3,"limit":10,"remaining":7,"resetIn":3600,"ttl":7200}}]}`,
		Body:       json.RawMessage(`{"usage":{"used":3,"limit":10,"remaining":7,"reset_at":"2026-05-09T12:00:00Z"},"limits":[{"name":"daily","title":"Daily","scope":"request","used":3,"limit":10,"remaining":7,"window":{"duration":1,"timeUnit":"day"},"detail":{"used":3,"limit":10,"remaining":7,"resetIn":3600,"ttl":7200}}]}`),
	}}}
	provider := quota.NewKimiProvider(caller, quota.DefaultProviderConfigs().Kimi)

	output, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{Identity: "kimi-auth"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if output.Provider != "kimi" {
		t.Fatalf("expected kimi output provider, got %q", output.Provider)
	}
	result, ok := output.Result.(quota.KimiResult)
	if !ok {
		t.Fatalf("expected kimi result type, got %T", output.Result)
	}
	if result.Usage == nil || result.Usage.Usage == nil || result.Usage.Usage.Limit == nil || *result.Usage.Usage.Limit != 10 || len(result.Usage.Limits) != 1 || result.Usage.Limits[0].Window.Duration != 1 || result.Usage.Limits[0].Window.TimeUnit != "day" || result.Usage.Limits[0].Title != "Daily" || result.Usage.Limits[0].Detail == nil || result.Usage.Limits[0].Detail.ResetIn != 3600 || result.Usage.Limits[0].TTL != 7200 {
		t.Fatalf("expected parsed kimi usage payload, got %#v", result.Usage)
	}
	encoded, err := json.Marshal(output.Result)
	if err != nil {
		t.Fatalf("marshal kimi result: %v", err)
	}
	body := string(encoded)
	if !contains(body, `"usage":{"usage"`) || contains(body, "bodyText") || contains(body, "statusCode") {
		t.Fatalf("unexpected kimi result JSON: %s", body)
	}
	if len(caller.requests) != 1 {
		t.Fatalf("expected one api-call request, got %d", len(caller.requests))
	}
	request := caller.requests[0]
	if request.AuthIndex != "kimi-auth" || request.Method != "GET" || request.URL != "https://api.kimi.com/coding/v1/usages" {
		t.Fatalf("unexpected api-call request: %+v", request)
	}
	if request.Header["Authorization"] != "Bearer $TOKEN$" {
		t.Fatalf("unexpected api-call headers: %+v", request.Header)
	}
	if request.Data != nil {
		t.Fatalf("expected no data body, got %#v", request.Data)
	}
}

// 真实上游形状：limits 顶层没有数字，数字以字符串形式放在 detail；timeUnit 是 TIME_UNIT_ 枚举。
func TestKimiProviderReadsNumbersFromDetailAndNormalizesTimeUnit(t *testing.T) {
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   `{"usage":{"limit":"2048","used":"214","remaining":"1834","resetTime":"2026-01-09T15:23:13Z"},"limits":[{"window":{"duration":300,"timeUnit":"TIME_UNIT_MINUTE"},"detail":{"limit":"200","used":"139","remaining":"61","resetTime":"2026-01-09T10:00:00Z"}}]}`,
	}}}
	provider := quota.NewKimiProvider(caller, quota.DefaultProviderConfigs().Kimi)

	output, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{Identity: "kimi-auth"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	result := output.Result.(quota.KimiResult)
	if len(result.Usage.Limits) != 1 {
		t.Fatalf("expected 1 limit, got %#v", result.Usage.Limits)
	}
	limit := result.Usage.Limits[0]
	if limit.Used == nil || *limit.Used != 139 || limit.Limit == nil || *limit.Limit != 200 || limit.Remaining == nil || *limit.Remaining != 61 {
		t.Fatalf("expected numbers read from detail, got %#v", limit)
	}
	if limit.ResetAt != "2026-01-09T10:00:00Z" {
		t.Fatalf("expected resetAt from detail, got %q", limit.ResetAt)
	}
	if limit.Window == nil || limit.Window.TimeUnit != "minute" || limit.Window.Duration != 300 {
		t.Fatalf("expected normalized time unit, got %#v", limit.Window)
	}
	if result.Usage.Usage.Used == nil || *result.Usage.Usage.Used != 214 {
		t.Fatalf("expected string numbers parsed, got %#v", result.Usage.Usage)
	}

	rows := quota.NormalizeQuotaRows(output)
	if len(rows) != 2 {
		t.Fatalf("expected 2 quota rows, got %#v", rows)
	}
	var limitRow *quota.QuotaRow
	for index := range rows {
		if rows[index].Key == "limits.0" {
			limitRow = &rows[index]
		}
	}
	if limitRow == nil {
		t.Fatalf("missing limits.0 row in %#v", rows)
	}
	if limitRow.Label != "5h" || limitRow.UsedPercent == nil || *limitRow.UsedPercent != 69.5 {
		t.Fatalf("unexpected limits row: %#v", limitRow)
	}
	if limitRow.Window == nil || limitRow.Window.Seconds == nil || *limitRow.Window.Seconds != 18_000 {
		t.Fatalf("expected 5h window seconds, got %#v", limitRow.Window)
	}
}
