package repository

import (
	"cpa-usage/internal/entities"

	"gorm.io/gorm"
)

func insertCanonicalUsageTestEvents(db *gorm.DB, events []entities.UsageEvent) (int, int, error) {
	return InsertUsageEvents(db, canonicalUsageTestEvents(events))
}

// canonicalUsageTestEvents upgrades synthetic pre-v2 fixtures at the test call
// site. Production code never infers canonical accounting from archival scalars.
func canonicalUsageTestEvents(events []entities.UsageEvent) []entities.UsageEvent {
	result := make([]entities.UsageEvent, len(events))
	for index, event := range events {
		result[index] = canonicalUsageTestEvent(event)
	}
	return result
}

func canonicalUsageTestEvent(event entities.UsageEvent) entities.UsageEvent {
	if usageAccountingState(event.UsageAccounting) != AccountingAbsent {
		return event
	}
	input := max(event.InputTokens, int64(0))
	output := max(event.OutputTokens, int64(0))
	reasoning := max(event.ReasoningTokens, int64(0))
	if reasoning > output {
		reasoning = output
	}
	nonReasoning := output - reasoning
	if input == 0 && output == 0 && event.TotalTokens > 0 {
		input = event.TotalTokens
	}
	cacheRead := int64(0)
	if event.CacheReadTokens != nil {
		cacheRead = *event.CacheReadTokens
	}
	if cacheRead < 0 || cacheRead > input {
		cacheRead = 0
	}
	uncached := input - cacheRead
	zero := int64(0)
	total := input + output
	quality := "complete"
	event.UsageAccounting = entities.UsageAccounting{
		TokenQuality:                &quality,
		CanonicalTotalTokens:        &total,
		CanonicalInputTokens:        &input,
		CanonicalUncachedTokens:     &uncached,
		CanonicalCacheReadTokens:    &cacheRead,
		CanonicalCacheWriteTokens:   &zero,
		CanonicalOutputTokens:       &output,
		CanonicalNonReasoningTokens: &nonReasoning,
		CanonicalReasoningTokens:    &reasoning,
		CanonicalUnclassifiedTokens: &zero,
	}
	return event
}
