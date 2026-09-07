package repository

import (
	"fmt"
	"math"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

const (
	AccountingAbsent = "absent"
	AccountingValid  = "valid"
)

// InterpretUsageAttempt is the single per-attempt accounting/throughput seam.
// It never reconstructs canonical facts from legacy tokens or provider names.
func InterpretUsageAttempt(event entities.UsageEvent) dto.UsageAttemptFacts {
	facts := dto.UsageAttemptFacts{
		Accounting: dto.UsageAccountingRecord{
			State:       usageAccountingState(event.UsageAccounting),
			Quality:     event.TokenQuality,
			TotalTokens: event.CanonicalTotalTokens,
			Input: dto.UsageTokenInput{
				TotalTokens:      event.CanonicalInputTokens,
				UncachedTokens:   event.CanonicalUncachedTokens,
				CacheReadTokens:  event.CanonicalCacheReadTokens,
				CacheWriteTokens: event.CanonicalCacheWriteTokens,
			},
			Output: dto.UsageTokenOutput{
				TotalTokens:        event.CanonicalOutputTokens,
				NonReasoningTokens: event.CanonicalNonReasoningTokens,
				ReasoningTokens:    event.CanonicalReasoningTokens,
			},
			UnclassifiedTokens: event.CanonicalUnclassifiedTokens,
		},
		Generate:            event.Generate,
		Stream:              event.Stream,
		ResponseServiceTier: event.ResponseServiceTier,
	}
	if tier := strings.TrimSpace(event.ServiceTier); tier != "" {
		facts.RequestServiceTier = &tier
	}
	if facts.Accounting.State != AccountingValid || event.TokenQuality == nil || *event.TokenQuality != "complete" {
		return facts
	}
	if event.Generate == nil || !*event.Generate || event.Stream == nil || !*event.Stream {
		return facts
	}
	facts.OutputTPS = usageEventOutputTPS(*event.CanonicalOutputTokens, event.LatencyMS, event.TTFTMS)
	return facts
}

func usageAccountingState(f entities.UsageAccounting) string {
	if accountingFactsAbsent(f) {
		return AccountingAbsent
	}
	return AccountingValid
}

// ValidateUsageAccounting is the single canonical bucket/quality validator.
// Historical all-nil facts are handled separately as absence by admission.
func ValidateUsageAccounting(f entities.UsageAccounting) error {
	if f.TokenQuality == nil {
		return fmt.Errorf("token quality is required")
	}
	switch *f.TokenQuality {
	case "complete", "inconsistent", "unclassified":
	default:
		return fmt.Errorf("unsupported token quality %q", *f.TokenQuality)
	}
	values := []*int64{f.CanonicalTotalTokens, f.CanonicalInputTokens, f.CanonicalUncachedTokens, f.CanonicalCacheReadTokens, f.CanonicalCacheWriteTokens, f.CanonicalOutputTokens, f.CanonicalNonReasoningTokens, f.CanonicalReasoningTokens, f.CanonicalUnclassifiedTokens}
	for _, value := range values {
		if value == nil {
			return fmt.Errorf("all canonical token buckets are required")
		}
	}
	for _, value := range values {
		if *value < 0 {
			return fmt.Errorf("canonical token buckets must be nonnegative")
		}
	}
	if !accountingSumEquals(*f.CanonicalInputTokens, *f.CanonicalUncachedTokens, *f.CanonicalCacheReadTokens, *f.CanonicalCacheWriteTokens) {
		return fmt.Errorf("canonical input buckets do not sum to input total")
	}
	if !accountingSumEquals(*f.CanonicalOutputTokens, *f.CanonicalNonReasoningTokens, *f.CanonicalReasoningTokens) {
		return fmt.Errorf("canonical output buckets do not sum to output total")
	}
	if !accountingSumEquals(*f.CanonicalTotalTokens, *f.CanonicalInputTokens, *f.CanonicalOutputTokens, *f.CanonicalUnclassifiedTokens) {
		return fmt.Errorf("canonical input, output, and unclassified buckets do not sum to total")
	}
	if *f.TokenQuality == "complete" && *f.CanonicalUnclassifiedTokens != 0 {
		return fmt.Errorf("complete accounting cannot contain unclassified tokens")
	}
	return nil
}

func accountingFactsAbsent(f entities.UsageAccounting) bool {
	return f.TokenQuality == nil && f.CanonicalTotalTokens == nil && f.CanonicalInputTokens == nil && f.CanonicalUncachedTokens == nil &&
		f.CanonicalCacheReadTokens == nil && f.CanonicalCacheWriteTokens == nil && f.CanonicalOutputTokens == nil &&
		f.CanonicalNonReasoningTokens == nil && f.CanonicalReasoningTokens == nil && f.CanonicalUnclassifiedTokens == nil
}

func accountingSumEquals(total int64, values ...int64) bool {
	var sum int64
	for _, value := range values {
		if value < 0 || sum > math.MaxInt64-value {
			return false
		}
		sum += value
	}
	return total == sum
}
