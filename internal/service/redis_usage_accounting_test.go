package service

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/repository/dto"
)

func accountingFixture(t testing.TB, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "cpa", "testdata", "usage", name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func mutateAccountingFixture(t testing.TB, mutate func(map[string]any)) string {
	t.Helper()
	var data map[string]any
	if err := json.Unmarshal([]byte(accountingFixture(t, "v7.2.152-complete")), &data); err != nil {
		t.Fatal(err)
	}
	mutate(data)
	encoded, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	return string(encoded)
}

func TestAccountingProducerFixturesSurviveProjection(t *testing.T) {
	for _, test := range []struct {
		name, state, quality string
		output               int64
		tps                  bool
	}{
		{"v7.2.62-legacy", repository.AccountingAbsent, "", 0, true},
		{"v7.2.152-complete", repository.AccountingValid, "complete", 30, true},
		{"v7.2.152-separate-reasoning", repository.AccountingValid, "complete", 42, true},
		{"v7.2.152-independent", repository.AccountingValid, "complete", 30, true},
		{"v7.2.152-inconsistent", repository.AccountingValid, "inconsistent", 0, false},
		{"v7.2.152-unclassified", repository.AccountingValid, "unclassified", 30, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			message := accountingFixture(t, test.name)
			event := assertAccountingProjection(t, message)
			facts := repository.InterpretUsageAttempt(event)
			if facts.Accounting.State != test.state {
				t.Fatalf("accounting state: %+v", facts.Accounting)
			}
			if test.quality == "" {
				if facts.Accounting.Quality != nil || facts.Generate != nil || facts.Stream != nil || facts.ResponseServiceTier != nil {
					t.Fatalf("fabricated historical facts: %+v", facts)
				}
			} else {
				if facts.Accounting.Quality == nil || *facts.Accounting.Quality != test.quality || *facts.Accounting.Output.TotalTokens != test.output {
					t.Fatalf("lost canonical facts: %+v", facts.Accounting)
				}
				if facts.RequestServiceTier == nil || *facts.RequestServiceTier != "auto" || facts.ResponseServiceTier == nil || *facts.ResponseServiceTier != "default" {
					t.Fatalf("tiers conflated: %+v", facts)
				}
				if facts.Generate == nil || facts.Stream == nil {
					t.Fatal("explicit flags were lost")
				}
			}
			if (facts.OutputTPS != nil) != test.tps {
				t.Fatalf("TPS availability: %+v", facts)
			}
			if test.tps && test.quality != "" && *facts.OutputTPS != float64(event.OutputTokens) {
				t.Fatalf("TPS must retain the existing provider-normalized output scalar, got %v", *facts.OutputTPS)
			}
			if event.InputTokens != 100 || event.OutputTokens != 30 || event.ReasoningTokens != 12 || event.CachedTokens != 40 {
				t.Fatalf("legacy scalar meanings changed: %+v", event)
			}
		})
	}
}

func assertAccountingProjection(t *testing.T, message string) entities.UsageEvent {
	t.Helper()
	projected, err := replaySafeRedisUsageMessage(message)
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"SENSITIVE", "DEFERRED", "response_headers", "access_token_sha256", "client_ip", "x_forwarded_for", "user_agent", "session_id", `"body"`} {
		if strings.Contains(projected, forbidden) {
			t.Fatalf("projection leaked %s", forbidden)
		}
	}
	projectedAgain, err := replaySafeRedisUsageMessage(projected)
	if err != nil || projectedAgain != projected {
		t.Fatalf("projection not stable: %v\n%s\n%s", err, projected, projectedAgain)
	}
	now := time.Date(2026, 9, 7, 9, 0, 0, 0, time.UTC)
	original, _, err := DecodeRedisUsageMessage(message, now)
	if err != nil {
		t.Fatal(err)
	}
	replayed, _, err := DecodeRedisUsageMessage(projected, now)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(original, replayed) {
		t.Fatalf("replay facts changed: original=%+v replayed=%+v", original, replayed)
	}
	return replayed
}

func TestAccountingInvalidExtensionsPreserveAttemptAndQualifiedFacts(t *testing.T) {
	tests := []struct {
		name, state string
		mutate      func(map[string]any)
	}{
		{"unsupported top version", repository.AccountingUnsupportedVersion, func(d map[string]any) { d["accounting_version"] = 3 }},
		{"missing top version", repository.AccountingMissing, func(d map[string]any) { delete(d, "accounting_version") }},
		{"missing breakdown", repository.AccountingMissing, func(d map[string]any) { delete(d, "token_breakdown") }},
		{"malformed top version", repository.AccountingMalformed, func(d map[string]any) { d["accounting_version"] = "SENSITIVE" }},
		{"null breakdown", repository.AccountingMalformed, func(d map[string]any) { d["token_breakdown"] = nil }},
		{"malformed breakdown", repository.AccountingMalformed, func(d map[string]any) { d["token_breakdown"] = "SENSITIVE" }},
	}
	for _, test := range []struct {
		name, field, state string
		value              any
	}{
		{"unsupported nested version", "schema_version", repository.AccountingUnsupportedSchema, 3},
		{"malformed nested version", "schema_version", repository.AccountingMalformed, "SENSITIVE"},
		{"unknown quality", "quality", repository.AccountingUnknownQuality, "SENSITIVE"},
		{"malformed quality", "quality", repository.AccountingMalformed, []any{"SENSITIVE"}},
		{"malformed input", "input", repository.AccountingMalformed, "SENSITIVE"},
		{"malformed output", "output", repository.AccountingMalformed, []any{"SENSITIVE"}},
		{"empty input", "input", repository.AccountingMissing, map[string]any{}},
		{"negative", "total_tokens", repository.AccountingInvalid, -1},
		{"inconsistent sum", "total_tokens", repository.AccountingInvalid, 131},
		{"fractional count", "total_tokens", repository.AccountingMalformed, 130.5},
		{"overflow count", "total_tokens", repository.AccountingMalformed, json.Number("9223372036854775808")},
		{"complete with unclassified", "unclassified_tokens", repository.AccountingInvalid, 1},
	} {
		tests = append(tests, struct {
			name, state string
			mutate      func(map[string]any)
		}{test.name, test.state, func(d map[string]any) { d["token_breakdown"].(map[string]any)[test.field] = test.value }})
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			event := assertAccountingProjection(t, mutateAccountingFixture(t, test.mutate))
			facts := repository.InterpretUsageAttempt(event)
			if facts.Accounting.State != test.state || facts.OutputTPS != nil {
				t.Fatalf("expected %s without TPS, got %+v", test.state, facts)
			}
			if event.RequestID != "fixture-request" || event.InputTokens != 100 || *event.CacheReadTokens != 40 {
				t.Fatalf("allowlisted legacy attempt lost: %+v", event)
			}
		})
	}
}

func TestAccountingRequiredBucketsPreserveMissingVersusZero(t *testing.T) {
	for _, group := range []string{"", "input", "output"} {
		var original map[string]any
		if err := json.Unmarshal([]byte(accountingFixture(t, "v7.2.152-complete")), &original); err != nil {
			t.Fatal(err)
		}
		fields := original["token_breakdown"].(map[string]any)
		if group != "" {
			fields = fields[group].(map[string]any)
		}
		for key := range fields {
			t.Run(group+"/"+key, func(t *testing.T) {
				event := assertAccountingProjection(t, mutateAccountingFixture(t, func(d map[string]any) {
					fields := d["token_breakdown"].(map[string]any)
					if group != "" {
						fields = fields[group].(map[string]any)
					}
					delete(fields, key)
				}))
				if facts := repository.InterpretUsageAttempt(event); facts.Accounting.State != repository.AccountingMissing {
					t.Fatalf("missing %s/%s fabricated: %+v", group, key, facts)
				}
			})
		}
	}
	event := assertAccountingProjection(t, mutateAccountingFixture(t, func(d map[string]any) {
		b := d["token_breakdown"].(map[string]any)
		b["total_tokens"], b["unclassified_tokens"] = 0, 0
		for _, group := range []string{"input", "output"} {
			for key := range b[group].(map[string]any) {
				b[group].(map[string]any)[key] = 0
			}
		}
	}))
	if facts := repository.InterpretUsageAttempt(event); facts.Accounting.State != repository.AccountingValid || facts.OutputTPS != nil {
		t.Fatalf("explicit zero is valid but has no throughput: %+v", facts)
	}
}

func TestAccountingExecutionFlagsRemainOptional(t *testing.T) {
	for _, value := range []any{nil, false, true, "SENSITIVE"} {
		event := assertAccountingProjection(t, mutateAccountingFixture(t, func(d map[string]any) {
			if value == nil {
				delete(d, "generate")
				delete(d, "stream")
				delete(d, "response_service_tier")
			} else {
				d["generate"], d["stream"] = value, value
			}
		}))
		want, known := value.(bool)
		if (event.Generate != nil) != known || (event.Stream != nil) != known {
			t.Fatalf("unknown flags fabricated for %v: %+v", value, event)
		}
		if known && !want && repository.InterpretUsageAttempt(event).OutputTPS != nil {
			t.Fatal("explicit false flags must suppress TPS")
		}
		if known && (*event.Generate != want || *event.Stream != want) {
			t.Fatalf("explicit flags changed for %v", value)
		}
	}
	for _, value := range []any{"", "   ", nil, []any{"SENSITIVE"}} {
		event := assertAccountingProjection(t, mutateAccountingFixture(t, func(d map[string]any) {
			d["response_service_tier"] = value
			d["token_breakdown"].(map[string]any)["unknown"] = "SENSITIVE"
			d["token_breakdown"].(map[string]any)["input"].(map[string]any)["unknown"] = "SENSITIVE"
		}))
		if event.ResponseServiceTier != nil || event.ServiceTier != "auto" {
			t.Fatalf("missing/malformed response tier must not fill from request: %+v", event)
		}
		if facts := repository.InterpretUsageAttempt(event); facts.Accounting.State != repository.AccountingValid {
			t.Fatalf("execution or unknown fields must not invalidate canonical accounting: %+v", facts)
		}
	}
}

func TestAccountingIntakeReplayAndRepositoryRead(t *testing.T) {
	db := openSyncTestDatabase(t)
	messages := []string{accountingFixture(t, "v7.2.62-legacy"), accountingFixture(t, "v7.2.152-complete"), accountingFixture(t, "v7.2.152-complete"), accountingFixture(t, "v7.2.152-inconsistent"), mutateAccountingFixture(t, func(d map[string]any) { d["accounting_version"] = "SENSITIVE" })}
	svc := NewSyncServiceWithOptions(db, SyncServiceOptions{BaseURL: "https://cpa.invalid", RedisQueue: staticRedisQueue{messages: messages}, RedisQueueKey: "usage"})
	if result, err := svc.PullRedisUsageInbox(context.Background()); err != nil || result.InsertedRows != len(messages) {
		t.Fatalf("intake: %+v %v", result, err)
	}
	var inbox []entities.RedisUsageInbox
	if err := db.Order("id").Find(&inbox).Error; err != nil {
		t.Fatal(err)
	}
	for _, row := range inbox {
		if strings.Contains(row.RawMessage, "SENSITIVE") {
			t.Fatal("sensitive field persisted")
		}
	}
	processor := newRedisUsageProcessor(db)
	for pass := 0; pass < 2; pass++ {
		result, err := processor.processRows(context.Background(), inbox, time.Now().UTC())
		if err != nil {
			t.Fatal(err)
		}
		if (pass == 0 && result.InsertedEvents != len(messages)) || (pass == 1 && result.DedupedEvents != len(messages)) {
			t.Fatalf("replay pass %d: %+v", pass, result)
		}
	}
	page, err := repository.ListUsageEventsWithFilter(context.Background(), db, dto.UsageEventListFilter{PageSize: 20})
	if err != nil || page.TotalCount != int64(len(messages)) {
		t.Fatalf("query: %+v %v", page, err)
	}
	states := map[string]int{}
	for _, row := range page.Events {
		states[row.AttemptFacts.Accounting.State]++
		var stored entities.UsageEvent
		if err := db.First(&stored, row.ID).Error; err != nil {
			t.Fatal(err)
		}
		if stored.AccountingState != row.AttemptFacts.Accounting.State {
			t.Fatalf("SQL state diverged from read interpretation: %+v", stored)
		}
		if row.RequestID != "fixture-request" || row.InputTokens != 100 {
			t.Fatalf("attempt facts lost: %+v", row)
		}
	}
	if !reflect.DeepEqual(states, map[string]int{repository.AccountingAbsent: 1, repository.AccountingValid: 3, repository.AccountingMalformed: 1}) {
		t.Fatalf("unexpected stored states: %v", states)
	}
}
