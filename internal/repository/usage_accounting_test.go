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
		AccountingVersion: ptr(2), TokenBreakdownPresent: true, TokenSchemaVersion: ptr(2), TokenQuality: &quality,
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
}
