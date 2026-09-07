package service

import (
	"fmt"

	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
)

func validateQueuedAccounting(d cpa.UsageAccountingFields) error {
	if d.Generate == nil || d.Stream == nil || d.TokenBreakdown == nil || d.TokenBreakdown.Input == nil || d.TokenBreakdown.Output == nil {
		return fmt.Errorf("complete Accounting v2 envelope is required")
	}
	if repository.UsageAccountingState(queuedAccountingFacts(d)) != repository.AccountingValid {
		return fmt.Errorf("canonical Accounting v2 facts are invalid")
	}
	return nil
}

// queuedAccountingFacts projects the already validated Accounting v2 record.
func queuedAccountingFacts(d cpa.UsageAccountingFields) entities.UsageAccounting {
	b := d.TokenBreakdown
	if b == nil {
		return entities.UsageAccounting{}
	}
	return entities.UsageAccounting{
		AccountingVersion:           d.AccountingVersion,
		TokenSchemaVersion:          b.SchemaVersion,
		TokenQuality:                b.Quality,
		CanonicalTotalTokens:        b.TotalTokens,
		CanonicalInputTokens:        b.Input.TotalTokens,
		CanonicalUncachedTokens:     b.Input.UncachedTokens,
		CanonicalCacheReadTokens:    b.Input.CacheReadTokens,
		CanonicalCacheWriteTokens:   b.Input.CacheWriteTokens,
		CanonicalOutputTokens:       b.Output.TotalTokens,
		CanonicalNonReasoningTokens: b.Output.NonReasoningTokens,
		CanonicalReasoningTokens:    b.Output.ReasoningTokens,
		CanonicalUnclassifiedTokens: b.UnclassifiedTokens,
	}
}
