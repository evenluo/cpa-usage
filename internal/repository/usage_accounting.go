package repository

import (
	"math"
	"strings"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

const (
	AccountingAbsent             = "absent"
	AccountingMalformed          = "malformed"
	AccountingUnsupportedVersion = "unsupported_accounting_version"
	AccountingUnsupportedSchema  = "unsupported_schema_version"
	AccountingMissing            = "missing"
	AccountingUnknownQuality     = "unknown_quality"
	AccountingInvalid            = "invalid"
	AccountingValid              = "valid"
)

// InterpretUsageAttempt is the single per-attempt accounting/throughput seam.
// It never reconstructs canonical facts from legacy tokens or provider names.
func InterpretUsageAttempt(event entities.UsageEvent) dto.UsageAttemptFacts {
	facts := dto.UsageAttemptFacts{
		Accounting: dto.UsageAccountingRecord{
			State:             usageAccountingState(event.UsageAccounting),
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
	output := event.OutputTokens
	switch facts.Accounting.State {
	case AccountingAbsent:
		// Keep the existing historical Output TPS interpretation, visibly without
		// canonical evidence. Distribution population selection belongs to C.
	case AccountingValid:
		if *event.TokenQuality != "complete" || event.OutputTokens > *event.CanonicalOutputTokens {
			return facts
		}
		// Preserve the existing provider-normalized OutputTokens numerator.
		// Canonical total/non-reasoning buckets would each change units for
		// some providers; they are separate accounting facts.
	default:
		return facts
	}
	if (event.Generate != nil && !*event.Generate) || (event.Stream != nil && !*event.Stream) {
		return facts
	}
	facts.OutputTPS = usageEventOutputTPS(output, event.LatencyMS, event.TTFTMS)
	return facts
}

func usageAccountingState(f entities.UsageAccounting) string {
	if f.AccountingMalformed {
		return AccountingMalformed
	}
	if f.AccountingVersion != nil && *f.AccountingVersion != 2 {
		return AccountingUnsupportedVersion
	}
	if f.TokenSchemaVersion != nil && *f.TokenSchemaVersion != 2 {
		return AccountingUnsupportedSchema
	}
	if f.AccountingVersion == nil && !f.TokenBreakdownPresent {
		return AccountingAbsent
	}
	if f.AccountingVersion == nil || !f.TokenBreakdownPresent || f.TokenSchemaVersion == nil || f.TokenQuality == nil {
		return AccountingMissing
	}
	switch *f.TokenQuality {
	case "complete", "inconsistent", "unclassified":
	default:
		return AccountingUnknownQuality
	}
	values := []*int64{f.CanonicalTotalTokens, f.CanonicalInputTokens, f.CanonicalUncachedTokens, f.CanonicalCacheReadTokens, f.CanonicalCacheWriteTokens, f.CanonicalOutputTokens, f.CanonicalNonReasoningTokens, f.CanonicalReasoningTokens, f.CanonicalUnclassifiedTokens}
	for _, value := range values {
		if value == nil {
			return AccountingMissing
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
