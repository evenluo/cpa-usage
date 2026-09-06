package quota

import (
	"testing"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
)

func TestNormalizePassiveQuotaSnapshotClaudeUsesFractionAndEpochUnits(t *testing.T) {
	raw := &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00.123Z",
		Signals: map[string]any{
			"Anthropic-Ratelimit-Unified-5h-Utilization": "0.25",
			"Anthropic-Ratelimit-Unified-5h-Status":      "allowed",
			"Anthropic-Ratelimit-Unified-5h-Reset":       "1787296800",
			"Anthropic-Ratelimit-Unified-7d-Utilization": "0.53",
			"Anthropic-Ratelimit-Unified-7d-Status":      "rejected",
			"Anthropic-Ratelimit-Unified-Status":         "allowed_warning",
			"Anthropic-Ratelimit-Unified-Reset":          "2026-09-08T08:00:00Z",
			"Anthropic-Ratelimit-Unified-7d_oi-Status":   "rejected",
			"Anthropic-Ratelimit-Unified-7d_oi-Reset":    "Mon, 08 Sep 2026 09:00:00 GMT",
			"Retry-After": "120",
		},
	}

	got := NormalizePassiveQuotaSnapshot(" Claude ", raw, nil)
	if got.Account == nil || len(got.Account.Quota) != 5 {
		t.Fatalf("expected Claude windows, generic/model state and retry hint, got %+v", got)
	}
	if want := time.Date(2026, 9, 7, 8, 0, 0, 123_000_000, time.UTC); !got.Account.ObservedAt.Equal(want) {
		t.Fatalf("observed_at = %s, want %s", got.Account.ObservedAt, want)
	}
	fiveHour := got.Account.Quota[0]
	if fiveHour.UsedPercent == nil || *fiveHour.UsedPercent != 25 || fiveHour.Window == nil || fiveHour.Window.Seconds != 18_000 || fiveHour.Allowed == nil || !*fiveHour.Allowed || fiveHour.LimitReached == nil || *fiveHour.LimitReached {
		t.Fatalf("unexpected 5h normalization: %+v", fiveHour)
	}
	weekly := got.Account.Quota[1]
	if weekly.UsedPercent == nil || *weekly.UsedPercent != 53 || weekly.Window == nil || weekly.Window.Seconds != 604_800 || weekly.Allowed == nil || *weekly.Allowed || weekly.LimitReached == nil || !*weekly.LimitReached {
		t.Fatalf("unexpected weekly normalization: %+v", weekly)
	}
	if got.Account.Quota[2].Allowed == nil || !*got.Account.Quota[2].Allowed || got.Account.Quota[2].ResetAt != "2026-09-08T08:00:00Z" {
		t.Fatalf("unexpected unified state: %+v", got.Account.Quota[2])
	}
	if got.Account.Quota[3].Scope != "model" || got.Account.Quota[3].Metric != "Fable" || got.Account.Quota[3].LimitReached == nil || !*got.Account.Quota[3].LimitReached {
		t.Fatalf("unexpected Fable state: %+v", got.Account.Quota[3])
	}
	if got.Account.Quota[4].ResetAfterSeconds == nil || *got.Account.Quota[4].ResetAfterSeconds != 120 {
		t.Fatalf("unexpected retry hint: %+v", got.Account.Quota[4])
	}
}

func TestNormalizePassiveQuotaSnapshotCodexPreservesAllRelevantWindowsAndModels(t *testing.T) {
	account := &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00Z",
		Signals: map[string]any{
			"X-Codex-Plan-Type":                                               "pro",
			"X-Codex-Allowed":                                                 "true",
			"X-Codex-Limit-Reached":                                           "false",
			"X-Codex-Primary-Used-Percent":                                    "51",
			"X-Codex-Primary-Window-Minutes":                                  "300",
			"X-Codex-Primary-Reset-After-Seconds":                             "60",
			"X-Codex-Secondary-Used-Percent":                                  "35",
			"X-Codex-Secondary-Window-Minutes":                                "10080",
			"X-Codex-Additional-GPT-5.3-Codex-Spark-Limit-Name":               "GPT-5.3-Codex-Spark",
			"X-Codex-Additional-GPT-5.3-Codex-Spark-Secondary-Used-Percent":   "12.5",
			"X-Codex-Additional-GPT-5.3-Codex-Spark-Secondary-Window-Minutes": "10080",
			"X-Codex-Active-Limit":                                            "codex_bengalfox",
			"X-Codex-Credits-Has-Credits":                                     "false",
			"X-Codex-Credits-Unlimited":                                       "false",
			"X-Codex-Credits-Balance":                                         "4.5",
			"Authorization":                                                   "secret",
		},
	}
	models := map[string]authfiles.QuotaObservation{
		"gpt-5.3-codex": {
			ObservedAt: "2026-09-07T07:30:00Z",
			Signals: map[string]any{
				"X-Codex-Primary-Used-Percent":   "80",
				"X-Codex-Primary-Window-Minutes": "300",
			},
		},
		"malformed": {ObservedAt: "not-a-time", Signals: map[string]any{"X-Codex-Allowed": "true"}},
	}

	got := NormalizePassiveQuotaSnapshot("codex", account, models)
	if got.Account == nil || len(got.Account.Quota) != 4 {
		t.Fatalf("expected primary, secondary, named secondary and credits rows, got %+v", got.Account)
	}
	if got.Account.ActiveLimit != "codex_bengalfox" {
		t.Fatalf("active limit = %q, want bounded producer identifier", got.Account.ActiveLimit)
	}
	primary := got.Account.Quota[0]
	if primary.Key != "codex.rate_limit.primary" || primary.UsedPercent == nil || *primary.UsedPercent != 51 || primary.Window == nil || primary.Window.Seconds != 18_000 || primary.Allowed == nil || !*primary.Allowed || primary.PlanType != "pro" {
		t.Fatalf("unexpected primary row: %+v", primary)
	}
	named := got.Account.Quota[2]
	if named.Metric != "GPT-5.3-Codex-Spark" || named.Label != "GPT-5.3-Codex-Spark Weekly" {
		t.Fatalf("unexpected named limit row: %+v", named)
	}
	credits := got.Account.Quota[3]
	if credits.Remaining == nil || *credits.Remaining != 4.5 || credits.Unit != "credits" || credits.Allowed == nil || *credits.Allowed || credits.Unlimited == nil || *credits.Unlimited {
		t.Fatalf("unexpected credits row: %+v", credits)
	}
	if len(got.Models) != 1 || got.Models[0].Model != "gpt-5.3-codex" || len(got.Models[0].Quota) != 1 || got.Models[0].Quota[0].Scope != "model" {
		t.Fatalf("unexpected model observations: %+v", got.Models)
	}
}

func TestNormalizePassiveQuotaSnapshotTreatsConflictingLimitStateAsUnknown(t *testing.T) {
	got := NormalizePassiveQuotaSnapshot("codex", &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00Z",
		Signals: map[string]any{
			"X-Codex-Allowed":                     "true",
			"X-Codex-Limit-Reached":               "true",
			"X-Codex-Primary-Used-Percent":        "90",
			"X-Codex-Primary-Window-Minutes":      "300",
			"X-Codex-Primary-Reset-After-Seconds": "60",
		},
	}, nil)
	if got.Account == nil || len(got.Account.Quota) != 1 {
		t.Fatalf("expected percent evidence despite conflicting state, got %+v", got.Account)
	}
	if got.Account.Quota[0].Allowed != nil || got.Account.Quota[0].LimitReached != nil {
		t.Fatalf("conflicting booleans must remain unknown, got %+v", got.Account.Quota[0])
	}
}

func TestNormalizePassiveQuotaSnapshotCodexResetAtRequiresProducerUnixSeconds(t *testing.T) {
	got := NormalizePassiveQuotaSnapshot("codex", &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00Z",
		Signals: map[string]any{
			"X-Codex-Primary-Used-Percent":   "90",
			"X-Codex-Primary-Window-Minutes": "300",
			"X-Codex-Primary-Reset-At":       "2026-09-07T09:00:00Z",
		},
	}, nil)
	if got.Account == nil || len(got.Account.Quota) != 1 {
		t.Fatalf("expected independent percent/window evidence, got %+v", got.Account)
	}
	if got.Account.Quota[0].ResetAt != "" {
		t.Fatalf("Codex reset-at outside the pinned Unix-seconds contract must stay unknown, got %+v", got.Account.Quota[0])
	}
}

func TestNormalizePassiveQuotaSnapshotPreservesActiveLimitOnlyObservation(t *testing.T) {
	got := NormalizePassiveQuotaSnapshot("codex", &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00Z",
		Signals:    map[string]any{"X-Codex-Active-Limit": "codex_bengalfox"},
	}, nil)
	if got.Account == nil || got.Account.ActiveLimit != "codex_bengalfox" || len(got.Account.Quota) != 0 {
		t.Fatalf("expected active-limit-only partial observation, got %+v", got.Account)
	}
}

func TestNormalizePassiveQuotaSnapshotAcceptsAbsoluteRetryAfterWithoutSchedulerInference(t *testing.T) {
	got := NormalizePassiveQuotaSnapshot("claude", &authfiles.QuotaObservation{
		ObservedAt: "2026-09-07T08:00:00Z",
		Signals:    map[string]any{"Retry-After": "2026-09-07T09:00:00Z"},
	}, nil)
	if got.Account == nil || len(got.Account.Quota) != 1 || got.Account.Quota[0].ResetAt != "2026-09-07T09:00:00Z" || got.Account.Quota[0].ResetAfterSeconds != nil {
		t.Fatalf("unexpected absolute retry hint: %+v", got.Account)
	}
}

func TestNormalizePassiveQuotaSnapshotRejectsUnsupportedMalformedAndEmptyWithoutZeroes(t *testing.T) {
	validShape := &authfiles.QuotaObservation{ObservedAt: "2026-09-07T08:00:00Z", Signals: map[string]any{
		"X-Codex-Primary-Used-Percent":   "bad",
		"X-Codex-Primary-Window-Minutes": float64(300),
		"X-Codex-Unknown":                "0",
	}}
	for _, testCase := range []struct {
		name, provider string
		raw            *authfiles.QuotaObservation
	}{
		{name: "unsupported", provider: "gemini", raw: validShape},
		{name: "missing", provider: "codex", raw: nil},
		{name: "malformed time", provider: "codex", raw: &authfiles.QuotaObservation{ObservedAt: "bad", Signals: map[string]any{"X-Codex-Allowed": "true"}}},
		{name: "empty signals", provider: "codex", raw: &authfiles.QuotaObservation{ObservedAt: "2026-09-07T08:00:00Z"}},
		{name: "malformed values", provider: "codex", raw: validShape},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			got := NormalizePassiveQuotaSnapshot(testCase.provider, testCase.raw, nil)
			if got.Account != nil || len(got.Models) != 0 {
				t.Fatalf("expected no passive quota fact, got %+v", got)
			}
		})
	}
}
