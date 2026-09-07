package repository

import (
	"math"
	"testing"

	"cpa-usage/internal/entities"
)

func TestAccountingSumsRejectOverflowAndInvalidQuality(t *testing.T) {
	ptr := func(v int64) *int64 { return &v }
	quality := "complete"
	valid := entities.UsageAccounting{
		AccountingVersion: ptr(2), TokenSchemaVersion: ptr(2), TokenQuality: &quality,
		CanonicalTotalTokens: ptr(130), CanonicalInputTokens: ptr(100), CanonicalUncachedTokens: ptr(50), CanonicalCacheReadTokens: ptr(40), CanonicalCacheWriteTokens: ptr(10),
		CanonicalOutputTokens: ptr(30), CanonicalNonReasoningTokens: ptr(18), CanonicalReasoningTokens: ptr(12), CanonicalUnclassifiedTokens: ptr(0),
	}
	for _, test := range []struct {
		name   string
		change func(*entities.UsageAccounting)
	}{
		{"input sum", func(f *entities.UsageAccounting) { f.CanonicalUncachedTokens = ptr(51) }},
		{"output sum", func(f *entities.UsageAccounting) { f.CanonicalNonReasoningTokens = ptr(19) }},
		{"input overflow", func(f *entities.UsageAccounting) {
			f.CanonicalUncachedTokens = ptr(math.MaxInt64)
			f.CanonicalCacheReadTokens = ptr(math.MaxInt64)
		}},
		{"output overflow", func(f *entities.UsageAccounting) {
			f.CanonicalNonReasoningTokens = ptr(math.MaxInt64)
			f.CanonicalReasoningTokens = ptr(math.MaxInt64)
		}},
		{"overall overflow", func(f *entities.UsageAccounting) {
			f.CanonicalInputTokens = ptr(math.MaxInt64)
			f.CanonicalUncachedTokens = ptr(math.MaxInt64 - 50)
			f.CanonicalTotalTokens = ptr(math.MaxInt64)
		}},
		{"negative bucket", func(f *entities.UsageAccounting) { f.CanonicalCacheReadTokens = ptr(-1) }},
		{"complete with remainder", func(f *entities.UsageAccounting) {
			f.CanonicalUnclassifiedTokens = ptr(1)
			f.CanonicalTotalTokens = ptr(131)
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			facts := valid
			test.change(&facts)
			if state := InterpretUsageAttempt(entities.UsageEvent{UsageAccounting: facts}).Accounting.State; state != AccountingInvalid {
				t.Fatalf("expected invalid, got %s", state)
			}
		})
	}
	for _, value := range []string{"inconsistent", "unclassified"} {
		facts := valid
		facts.TokenQuality = &value
		got := InterpretUsageAttempt(entities.UsageEvent{UsageAccounting: facts, OutputTokens: 30, LatencyMS: 1200, TTFTMS: ptr(200)})
		if got.Accounting.State != AccountingValid || got.OutputTPS != nil {
			t.Fatalf("structure must not upgrade quality: %+v", got)
		}
	}
	generate, stream := true, true
	if facts := InterpretUsageAttempt(entities.UsageEvent{UsageAccounting: valid, Generate: &generate, Stream: &stream, OutputTokens: 999, LatencyMS: 1200, TTFTMS: ptr(200)}); facts.OutputTPS == nil || *facts.OutputTPS != 30 {
		t.Fatalf("expected canonical output including reasoning to own TPS, got %+v", facts.OutputTPS)
	}
}

func TestAccountingDirectCutDistinguishesHistoricalAbsenceAndRequiresExplicitStreamingGeneration(t *testing.T) {
	ptr := func(v int64) *int64 { return &v }
	if state := UsageAccountingState(entities.UsageAccounting{}); state != AccountingAbsent {
		t.Fatalf("historical row state: got %s want %s", state, AccountingAbsent)
	}
	if state := UsageAccountingState(entities.UsageAccounting{AccountingVersion: ptr(2)}); state != AccountingInvalid {
		t.Fatalf("partial canonical row state: got %s want %s", state, AccountingInvalid)
	}

	quality := "complete"
	valid := entities.UsageAccounting{
		AccountingVersion: ptr(2), TokenSchemaVersion: ptr(2), TokenQuality: &quality,
		CanonicalTotalTokens: ptr(130), CanonicalInputTokens: ptr(100), CanonicalUncachedTokens: ptr(50), CanonicalCacheReadTokens: ptr(40), CanonicalCacheWriteTokens: ptr(10),
		CanonicalOutputTokens: ptr(30), CanonicalNonReasoningTokens: ptr(18), CanonicalReasoningTokens: ptr(12), CanonicalUnclassifiedTokens: ptr(0),
	}
	ttft := int64(200)
	trueValue, falseValue := true, false
	base := entities.UsageEvent{UsageAccounting: valid, LatencyMS: 1200, TTFTMS: &ttft, Generate: &trueValue, Stream: &trueValue}
	if got := InterpretUsageAttempt(base).OutputTPS; got == nil || *got != 30 {
		t.Fatalf("canonical output TPS: got %v want 30", got)
	}
	for _, event := range []entities.UsageEvent{
		{UsageAccounting: valid, LatencyMS: 1200, TTFTMS: &ttft, Stream: &trueValue},
		{UsageAccounting: valid, LatencyMS: 1200, TTFTMS: &ttft, Generate: &trueValue},
		{UsageAccounting: valid, LatencyMS: 1200, TTFTMS: &ttft, Generate: &falseValue, Stream: &trueValue},
		{UsageAccounting: valid, LatencyMS: 1200, TTFTMS: &ttft, Generate: &trueValue, Stream: &falseValue},
	} {
		if got := InterpretUsageAttempt(event).OutputTPS; got != nil {
			t.Fatalf("non-generating or non-streaming attempt produced TPS: %v", got)
		}
	}
}
