package cpa

// UsageAccountingFields is the allowlisted queue contract from CPA v7.2.152.
type UsageAccountingFields struct {
	AccountingVersion   *int64               `json:"accounting_version,omitempty"`
	TokenBreakdown      *UsageTokenBreakdown `json:"token_breakdown,omitempty"`
	Generate            *bool                `json:"generate,omitempty"`
	Stream              *bool                `json:"stream,omitempty"`
	ResponseServiceTier *string              `json:"response_service_tier,omitempty"`
}

type UsageTokenBreakdown struct {
	SchemaVersion      *int64            `json:"schema_version,omitempty"`
	Quality            *string           `json:"quality,omitempty"`
	TotalTokens        *int64            `json:"total_tokens,omitempty"`
	Input              *UsageTokenInput  `json:"input,omitempty"`
	Output             *UsageTokenOutput `json:"output,omitempty"`
	UnclassifiedTokens *int64            `json:"unclassified_tokens,omitempty"`
}

type UsageTokenInput struct {
	TotalTokens      *int64 `json:"total_tokens,omitempty"`
	UncachedTokens   *int64 `json:"uncached_tokens,omitempty"`
	CacheReadTokens  *int64 `json:"cache_read_tokens,omitempty"`
	CacheWriteTokens *int64 `json:"cache_write_tokens,omitempty"`
}

type UsageTokenOutput struct {
	TotalTokens        *int64 `json:"total_tokens,omitempty"`
	NonReasoningTokens *int64 `json:"non_reasoning_tokens,omitempty"`
	ReasoningTokens    *int64 `json:"reasoning_tokens,omitempty"`
}
