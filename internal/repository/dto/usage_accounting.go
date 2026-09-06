package dto

// UsageAttemptFacts is the repository-owned interpretation consumed by evidence,
// distributions, and accounting analytics. Nil flags mean unknown, never false.
type UsageAttemptFacts struct {
	Accounting          UsageAccountingRecord
	Generate            *bool
	Stream              *bool
	RequestServiceTier  *string
	ResponseServiceTier *string
	OutputTPS           *float64
}

// UsageAccountingRecord retains typed reported facts even when unavailable.
// Consumers may aggregate canonical buckets only when State == "valid", and
// must still qualify Quality. Structural validity does not imply completeness.
type UsageAccountingRecord struct {
	State              string
	AccountingVersion  *int64
	SchemaVersion      *int64
	Quality            *string
	TotalTokens        *int64
	Input              UsageTokenInput
	Output             UsageTokenOutput
	UnclassifiedTokens *int64
}

type UsageTokenInput struct {
	TotalTokens      *int64
	UncachedTokens   *int64
	CacheReadTokens  *int64
	CacheWriteTokens *int64
}

type UsageTokenOutput struct {
	TotalTokens        *int64
	NonReasoningTokens *int64
	ReasoningTokens    *int64
}
