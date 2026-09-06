package service

import (
	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
)

// Preserve typed producer facts only; repository owns validity and quality.
func queuedAccountingFacts(d cpa.UsageAccountingFields) entities.UsageAccounting {
	facts := entities.UsageAccounting{
		AccountingVersion:     d.AccountingVersion.Value,
		TokenBreakdownPresent: d.TokenBreakdown.Value != nil || d.TokenBreakdown.Malformed,
		AccountingMalformed:   d.AccountingVersion.Malformed || d.TokenBreakdown.Malformed,
	}
	b := d.TokenBreakdown.Value
	if b == nil {
		return facts
	}
	facts.TokenSchemaVersion = b.SchemaVersion.Value
	if b.Quality.Value != nil {
		quality := string(*b.Quality.Value)
		facts.TokenQuality = &quality
	}
	facts.CanonicalTotalTokens = b.TotalTokens.Value
	facts.CanonicalUnclassifiedTokens = b.UnclassifiedTokens.Value
	facts.AccountingMalformed = facts.AccountingMalformed || b.SchemaVersion.Malformed || b.Quality.Malformed || b.TotalTokens.Malformed || b.UnclassifiedTokens.Malformed || b.Input.Malformed || b.Output.Malformed
	if input := b.Input.Value; input != nil {
		facts.CanonicalInputTokens = input.TotalTokens.Value
		facts.CanonicalUncachedTokens = input.UncachedTokens.Value
		facts.CanonicalCacheReadTokens = input.CacheReadTokens.Value
		facts.CanonicalCacheWriteTokens = input.CacheWriteTokens.Value
		facts.AccountingMalformed = facts.AccountingMalformed || input.TotalTokens.Malformed || input.UncachedTokens.Malformed || input.CacheReadTokens.Malformed || input.CacheWriteTokens.Malformed
	}
	if output := b.Output.Value; output != nil {
		facts.CanonicalOutputTokens = output.TotalTokens.Value
		facts.CanonicalNonReasoningTokens = output.NonReasoningTokens.Value
		facts.CanonicalReasoningTokens = output.ReasoningTokens.Value
		facts.AccountingMalformed = facts.AccountingMalformed || output.TotalTokens.Malformed || output.NonReasoningTokens.Malformed || output.ReasoningTokens.Malformed
	}
	return facts
}
