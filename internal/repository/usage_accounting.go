package repository

import (
	"math"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

const (
	AccountingAbsent  = "absent"
	AccountingInvalid = "invalid"
	AccountingValid   = "valid"
)

// InterpretUsageAttempt is the single per-attempt accounting/throughput seam.
// It never reconstructs canonical facts from legacy tokens or provider names.
func InterpretUsageAttempt(event entities.UsageEvent) dto.UsageAttemptFacts {
	facts := dto.UsageAttemptFacts{
		Accounting: dto.UsageAccountingRecord{
			State:             UsageAccountingState(event.UsageAccounting),
			AccountingVersion: event.AccountingVersion,
			SchemaVersion:     event.TokenSchemaVersion,
			Quality:           event.TokenQuality,
			TotalTokens:       event.CanonicalTotalTokens,
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

func UsageAccountingState(f entities.UsageAccounting) string {
	if accountingFactsAbsent(f) {
		return AccountingAbsent
	}
	if f.AccountingVersion == nil || *f.AccountingVersion != 2 || f.TokenSchemaVersion == nil || *f.TokenSchemaVersion != 2 || f.TokenQuality == nil {
		return AccountingInvalid
	}
	switch *f.TokenQuality {
	case "complete", "inconsistent", "unclassified":
	default:
		return AccountingInvalid
	}
	values := []*int64{f.CanonicalTotalTokens, f.CanonicalInputTokens, f.CanonicalUncachedTokens, f.CanonicalCacheReadTokens, f.CanonicalCacheWriteTokens, f.CanonicalOutputTokens, f.CanonicalNonReasoningTokens, f.CanonicalReasoningTokens, f.CanonicalUnclassifiedTokens}
	for _, value := range values {
		if value == nil {
			return AccountingInvalid
		}
	}
	for _, value := range values {
		if *value < 0 {
			return AccountingInvalid
		}
	}
	if !accountingSumEquals(*f.CanonicalInputTokens, *f.CanonicalUncachedTokens, *f.CanonicalCacheReadTokens, *f.CanonicalCacheWriteTokens) ||
		!accountingSumEquals(*f.CanonicalOutputTokens, *f.CanonicalNonReasoningTokens, *f.CanonicalReasoningTokens) ||
		!accountingSumEquals(*f.CanonicalTotalTokens, *f.CanonicalInputTokens, *f.CanonicalOutputTokens, *f.CanonicalUnclassifiedTokens) ||
		(*f.TokenQuality == "complete" && *f.CanonicalUnclassifiedTokens != 0) {
		return AccountingInvalid
	}
	return AccountingValid
}

func accountingFactsAbsent(f entities.UsageAccounting) bool {
	return f.AccountingVersion == nil && f.TokenSchemaVersion == nil && f.TokenQuality == nil &&
		f.CanonicalTotalTokens == nil && f.CanonicalInputTokens == nil && f.CanonicalUncachedTokens == nil &&
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
