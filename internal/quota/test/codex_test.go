package test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cpa-usage/internal/cpa/dto/apicall"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/quota"
)

func TestCodexProviderUsesAccountIDForUsageRequest(t *testing.T) {
	codexUsageJSON := `{"user_id":"user-k7itHYqWm38P92JR13zywJOr","account_id":"user-k7itHYqWm38P92JR13zywJOr","email":"user@example.com","plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"used_percent":64,"limit_window_seconds":18000,"reset_after_seconds":11676,"reset_at":1778509871},"secondary_window":{"used_percent":10,"limit_window_seconds":604800,"reset_after_seconds":598476,"reset_at":1779096671}},"code_review_rate_limit":null,"additional_rate_limits":null,"credits":{"has_credits":false,"unlimited":false,"overage_limit_reached":false,"balance":"0","approx_local_messages":[0,0],"approx_cloud_messages":[0,0]},"spend_control":{"reached":false,"individual_limit":null},"rate_limit_reached_type":null,"promo":null,"referral_beacon":null}`
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   codexUsageJSON,
		Body:       json.RawMessage(codexUsageJSON),
	}}}
	provider := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex)

	output, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{
		Identity:  "codex-auth",
		AccountID: stringPtr("acct_123"),
	}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if output.Provider != "codex" {
		t.Fatalf("expected codex output provider, got %q", output.Provider)
	}
	result, ok := output.Result.(quota.CodexResult)
	if !ok {
		t.Fatalf("expected codex result type, got %T", output.Result)
	}
	if result.Usage == nil || result.Usage.PlanType != "plus" {
		t.Fatalf("expected parsed usage payload, got %#v", result.Usage)
	}
	if result.Usage.RateLimit == nil || result.Usage.RateLimit.PrimaryWindow == nil || result.Usage.RateLimit.PrimaryWindow.UsedPercent == nil || *result.Usage.RateLimit.PrimaryWindow.UsedPercent != 64 {
		t.Fatalf("expected parsed rate limit payload, got %#v", result.Usage.RateLimit)
	}
	if result.Usage.RateLimit.SecondaryWindow == nil || result.Usage.RateLimit.SecondaryWindow.UsedPercent == nil || *result.Usage.RateLimit.SecondaryWindow.UsedPercent != 10 {
		t.Fatalf("expected parsed secondary rate limit payload, got %#v", result.Usage.RateLimit)
	}
	if result.Usage.CodeReviewRateLimit != nil {
		t.Fatalf("expected nil code review rate limit payload, got %#v", result.Usage.CodeReviewRateLimit)
	}
	if result.Usage.AdditionalRateLimits != nil {
		t.Fatalf("expected nil additional rate limit payload, got %#v", result.Usage.AdditionalRateLimits)
	}
	rows := quota.NormalizeQuotaRows(output)
	if len(rows) != 3 || rows[0].PlanType != "plus" || rows[1].PlanType != "plus" || rows[2].PlanType != "plus" {
		t.Fatalf("expected normalized Codex rows to carry planType plus, got %#v", rows)
	}
	credits := rows[2]
	if credits.Key != "credits" || credits.Balance != "0" || credits.Remaining == nil || *credits.Remaining != 0 || credits.HasCredits == nil || *credits.HasCredits || credits.Unlimited == nil || *credits.Unlimited || credits.OverageLimitReached == nil || *credits.OverageLimitReached {
		t.Fatalf("expected zero balance and present false credit facts, got %+v", credits)
	}
	encoded, err := json.Marshal(output.Result)
	if err != nil {
		t.Fatalf("marshal codex result: %v", err)
	}
	body := string(encoded)
	if !contains(body, `"usage":{"planType":"plus"`) || contains(body, "bodyText") || contains(body, "statusCode") {
		t.Fatalf("unexpected codex result JSON: %s", body)
	}
	if len(caller.requests) != 1 {
		t.Fatalf("expected one api-call request, got %d", len(caller.requests))
	}
	request := caller.requests[0]
	if request.AuthIndex != "codex-auth" || request.Method != "GET" || request.URL != "https://chatgpt.com/backend-api/wham/usage" {
		t.Fatalf("unexpected api-call request: %+v", request)
	}
	if request.Header["Authorization"] != "Bearer $TOKEN$" || request.Header["Content-Type"] != "application/json" || request.Header["User-Agent"] != "codex_cli_rs/0.76.0 (Debian 13.0.0; x86_64) WindowsTerminal" || request.Header["Chatgpt-Account-Id"] != "acct_123" {
		t.Fatalf("unexpected api-call headers: %+v", request.Header)
	}
	if request.Data != nil {
		t.Fatalf("expected no data body, got %#v", request.Data)
	}
}

func TestCodexCreditsKeepExactBalancesAndMissingFacts(t *testing.T) {
	for _, test := range []struct {
		name    string
		credits string
		balance string
		rows    int
	}{
		{name: "missing"},
		{name: "null", credits: `null`},
		{name: "empty", credits: `{}`},
		{name: "null facts", credits: `{"balance":null,"has_credits":null,"unlimited":null,"overage_limit_reached":null}`},
		{name: "string zero", credits: `{"balance":"0"}`, balance: "0", rows: 1},
		{name: "numeric zero", credits: `{"balance":0}`, balance: "0", rows: 1},
		{name: "exact string decimal", credits: `{"balance":"12345.67890123456789"}`, balance: "12345.67890123456789", rows: 1},
		{name: "exact numeric decimal", credits: `{"balance":12345.67890123456789}`, balance: "12345.67890123456789", rows: 1},
		{name: "false boolean", credits: `{"has_credits":false}`, rows: 1},
		{name: "negative", credits: `{"balance":"-1"}`},
		{name: "NaN", credits: `{"balance":"NaN"}`},
		{name: "infinity", credits: `{"balance":"Inf"}`},
		{name: "exponent", credits: `{"balance":"1e5"}`},
		{name: "nonnumeric", credits: `{"balance":"credits"}`},
		{name: "object", credits: `{"balance":{}}`},
		{name: "boolean balance", credits: `{"balance":false}`},
		{name: "unbounded", credits: `{"balance":"` + strings.Repeat("9", 65) + `"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			body := `{"plan_type":"pro"`
			if test.credits != "" {
				body += `,"credits":` + test.credits
			}
			body += `}`
			caller := &recordingManagementCaller{responses: []*apicall.Response{{StatusCode: 200, BodyText: body}}}
			output, err := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex).Check(context.Background(), quota.ProviderInput{})
			if err != nil {
				t.Fatalf("Check returned error: %v", err)
			}
			rows := quota.NormalizeQuotaRows(output)
			if len(rows) != test.rows {
				t.Fatalf("expected %d quota rows, got %+v", test.rows, rows)
			}
			if len(rows) == 0 {
				return
			}
			row := rows[0]
			if row.Key != "credits" || row.Balance != test.balance || row.Unit != "credits" {
				t.Fatalf("unexpected credit row: %+v", row)
			}
			if test.balance != "" {
				if row.Remaining == nil || row.HasCredits != nil || row.Unlimited != nil || row.OverageLimitReached != nil {
					t.Fatalf("balance must not invent boolean facts: %+v", row)
				}
			} else if row.Remaining != nil || row.HasCredits == nil || *row.HasCredits {
				t.Fatalf("false has_credits must not invent a zero balance: %+v", row)
			}
		})
	}
}

func TestCodexMissingUsedPercentStaysAbsentOnQuotaRow(t *testing.T) {
	usageJSON := `{"plan_type":"pro","rate_limit":{"allowed":true,"limit_reached":false,"primary_window":{"limit_window_seconds":604800,"reset_after_seconds":100}}}`
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   usageJSON,
		Body:       json.RawMessage(usageJSON),
	}}}
	provider := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex)

	output, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{Identity: "codex-auth"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	rows := quota.NormalizeQuotaRows(output)
	if len(rows) != 1 {
		t.Fatalf("expected one quota row, got %#v", rows)
	}
	if rows[0].UsedPercent != nil {
		t.Fatalf("missing used_percent must stay nil, got %#v", rows[0].UsedPercent)
	}
	if rows[0].ResetAfterSeconds == nil || *rows[0].ResetAfterSeconds != 100 {
		t.Fatalf("present reset_after_seconds must be kept, got %#v", rows[0].ResetAfterSeconds)
	}
}

func TestCodexWeeklyPrimaryFixtureKeepsSparkAndReserveOffRateLimit(t *testing.T) {
	usageJSON, err := os.ReadFile(filepath.Join("..", "testdata", "codex-wham-usage-weekly-primary.json"))
	if err != nil {
		t.Fatalf("read weekly-primary fixture: %v", err)
	}
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   string(usageJSON),
		Body:       json.RawMessage(usageJSON),
	}}}
	provider := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex)

	output, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{Identity: "codex-auth"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	rows := quota.NormalizeQuotaRows(output)
	if len(rows) != 4 {
		t.Fatalf("expected rate_limit weekly plus Spark 5h/Weekly and gpt-reserve Weekly, got %#v", rows)
	}
	primary := findQuotaRow(t, rows, "rate_limit.primary_window")
	assertQuotaText(t, primary, "Weekly", "window", "")
	assertFloatField(t, primary.UsedPercent, 57, "rate_limit usedPercent")
	assertIntField(t, primary.Window.Seconds, 604800, "rate_limit window seconds")
	if findQuotaRowOrZero(rows, "rate_limit.secondary_window").Key != "" {
		t.Fatalf("rate_limit must not invent a secondary window, got %#v", rows)
	}

	sparkPrimary := findQuotaRow(t, rows, "additional_rate_limits.GPT-5.3-Codex-Spark.primary_window")
	assertQuotaText(t, sparkPrimary, "GPT-5.3-Codex-Spark 5h", "additional", "codex_bengalfox")
	assertIntField(t, sparkPrimary.Window.Seconds, 18000, "spark primary window seconds")

	reserve := findQuotaRow(t, rows, "additional_rate_limits.gpt-reserve.primary_window")
	assertQuotaText(t, reserve, "gpt-reserve Weekly", "additional", "base_model_inference")
	assertFloatField(t, reserve.UsedPercent, 0, "gpt-reserve usedPercent")
	assertIntField(t, reserve.Window.Seconds, 604800, "gpt-reserve window seconds")
	if findQuotaRowOrZero(rows, "additional_rate_limits.gpt-reserve.secondary_window").Key != "" {
		t.Fatalf("gpt-reserve must stay primary-only, got %#v", rows)
	}
}

func findQuotaRowOrZero(rows []quota.QuotaRow, key string) quota.QuotaRow {
	for _, row := range rows {
		if row.Key == key {
			return row
		}
	}
	return quota.QuotaRow{}
}

func TestCodexProviderOmitsAccountIDHeaderWhenMissing(t *testing.T) {
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 200,
		BodyText:   `{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false}}`,
		Body:       json.RawMessage(`{"plan_type":"plus","rate_limit":{"allowed":true,"limit_reached":false}}`),
	}}}
	provider := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex)

	_, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{Identity: "codex-auth"}})
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if len(caller.requests) != 1 {
		t.Fatalf("expected one api-call request without account_id, got %d", len(caller.requests))
	}
	if _, ok := caller.requests[0].Header["Chatgpt-Account-Id"]; ok {
		t.Fatalf("expected account id header to be omitted, got headers: %+v", caller.requests[0].Header)
	}
}

func TestCodexProviderRejectsNonSuccessUsageResponse(t *testing.T) {
	caller := &recordingManagementCaller{responses: []*apicall.Response{{
		StatusCode: 429,
		BodyText:   `{"error":{"message":"rate limited"}}`,
		Body:       json.RawMessage(`{"error":{"message":"rate limited"}}`),
	}}}
	provider := quota.NewCodexProvider(caller, quota.DefaultProviderConfigs().Codex)

	_, err := provider.Check(context.Background(), quota.ProviderInput{Identity: entities.UsageIdentity{
		Identity:  "codex-auth",
		AccountID: stringPtr("acct_123"),
	}})
	if err == nil || err.Error() != "HTTP 429: rate limited" {
		t.Fatalf("expected target HTTP message, got %v", err)
	}
}
