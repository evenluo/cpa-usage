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
	if d.AccountingVersion == nil || *d.AccountingVersion != 2 || d.TokenBreakdown.SchemaVersion == nil || *d.TokenBreakdown.SchemaVersion != 2 {
		return fmt.Errorf("Accounting and token schema versions must both be 2")
	}
	if err := repository.ValidateUsageAccounting(queuedAccountingFacts(d)); err != nil {
		return fmt.Errorf("canonical Accounting v2 facts are invalid")
	}
	return nil
}

// queuedAccountingFacts projects a structurally complete Accounting v2 envelope
// for canonical validation or persistence.
func queuedAccountingFacts(d cpa.UsageAccountingFields) entities.UsageAccounting {
	b := d.TokenBreakdown
	return entities.UsageAccounting{
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
