package service

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
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

func TestAccountingV2FixturesSurviveSafeProjection(t *testing.T) {
	for _, test := range []struct {
		name    string
		quality string
		tps     *float64
	}{
		{name: "v7.2.152-complete", quality: "complete", tps: float64Pointer(30)},
		{name: "v7.2.152-separate-reasoning", quality: "complete", tps: float64Pointer(42)},
		{name: "v7.2.152-independent", quality: "complete", tps: float64Pointer(30)},
		{name: "v7.2.152-inconsistent", quality: "inconsistent"},
		{name: "v7.2.152-unclassified", quality: "unclassified"},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := assertAccountingProjection(t, accountingFixture(t, test.name))
			facts := repository.InterpretUsageAttempt(event)
			if facts.Accounting.State != repository.AccountingValid || facts.Accounting.Quality == nil || *facts.Accounting.Quality != test.quality {
				t.Fatalf("unexpected accounting facts: %+v", facts)
			}
			if !equalOptionalFloat64(facts.OutputTPS, test.tps) {
				t.Fatalf("canonical Output TPS: got %v want %v", facts.OutputTPS, test.tps)
			}
		})
	}
}

func TestAccountingV2RejectsUnsupportedMalformedAndIncompleteMessages(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "old producer", message: accountingFixture(t, "v7.2.62-legacy")},
		{name: "unsupported version", message: mutateAccountingFixture(t, func(d map[string]any) { d["accounting_version"] = 3 })},
		{name: "missing version", message: mutateAccountingFixture(t, func(d map[string]any) { delete(d, "accounting_version") })},
		{name: "malformed version", message: mutateAccountingFixture(t, func(d map[string]any) { d["accounting_version"] = "PRIVATE_VALUE" })},
		{name: "missing execution flag", message: mutateAccountingFixture(t, func(d map[string]any) { delete(d, "stream") })},
		{name: "unknown quality", message: mutateAccountingFixture(t, func(d map[string]any) { d["token_breakdown"].(map[string]any)["quality"] = "future" })},
		{name: "invalid sum", message: mutateAccountingFixture(t, func(d map[string]any) { d["token_breakdown"].(map[string]any)["total_tokens"] = 131 })},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, _, err := DecodeRedisUsageMessage(test.message, time.Now()); err == nil {
				t.Fatal("expected strict Accounting v2 rejection")
			}
			projected, err := replaySafeRedisUsageMessage(test.message)
			if err != nil {
				t.Fatal(err)
			}
			digest := fmt.Sprintf("%x", sha256.Sum256([]byte(test.message)))
			if !strings.Contains(projected, digest) || strings.Contains(projected, "PRIVATE_VALUE") {
				t.Fatalf("expected safe invalid marker, got %s", projected)
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

func float64Pointer(value float64) *float64 { return &value }

func equalOptionalFloat64(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
