package entities

// UsageAccounting stores only typed allowlisted facts. Nullable numeric fields
// preserve absence versus explicit zero. Historical rows have no v2 evidence.
// AccountingState is repository's materialized interpretation for SQL readers;
// canonical facts are the only metric source; historical scalar columns are archival.
type UsageAccounting struct {
	AccountingVersion           *int64
	AccountingState             string `gorm:"not null;default:'absent'"`
	TokenSchemaVersion          *int64
	TokenQuality                *string
	CanonicalTotalTokens        *int64
	CanonicalInputTokens        *int64
	CanonicalUncachedTokens     *int64
	CanonicalCacheReadTokens    *int64
	CanonicalCacheWriteTokens   *int64
	CanonicalOutputTokens       *int64
	CanonicalNonReasoningTokens *int64
	CanonicalReasoningTokens    *int64
	CanonicalUnclassifiedTokens *int64
}
