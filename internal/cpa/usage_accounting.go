package cpa

import (
	"bytes"
	"encoding/json"
)

// UsageAccountingFields is the allowlisted queue extension from CPA v7.2.152
// (c76dfd4e0edabab9000628b1560ab8ab379eadb8). Interpretation belongs to repository.
type UsageAccountingFields struct {
	AccountingVersion   usageField[int64]               `json:"accounting_version,omitzero"`
	TokenBreakdown      usageField[UsageTokenBreakdown] `json:"token_breakdown,omitzero"`
	Generate            usageField[bool]                `json:"generate,omitzero"`
	Stream              usageField[bool]                `json:"stream,omitzero"`
	ResponseServiceTier usageField[string]              `json:"response_service_tier,omitzero"`
}

type UsageTokenBreakdown struct {
	SchemaVersion      usageField[int64]            `json:"schema_version,omitzero"`
	Quality            usageField[usageQuality]     `json:"quality,omitzero"`
	TotalTokens        usageField[int64]            `json:"total_tokens,omitzero"`
	Input              usageField[UsageTokenInput]  `json:"input,omitzero"`
	Output             usageField[UsageTokenOutput] `json:"output,omitzero"`
	UnclassifiedTokens usageField[int64]            `json:"unclassified_tokens,omitzero"`
}

type UsageTokenInput struct {
	TotalTokens      usageField[int64] `json:"total_tokens,omitzero"`
	UncachedTokens   usageField[int64] `json:"uncached_tokens,omitzero"`
	CacheReadTokens  usageField[int64] `json:"cache_read_tokens,omitzero"`
	CacheWriteTokens usageField[int64] `json:"cache_write_tokens,omitzero"`
}

type UsageTokenOutput struct {
	TotalTokens        usageField[int64] `json:"total_tokens,omitzero"`
	NonReasoningTokens usageField[int64] `json:"non_reasoning_tokens,omitzero"`
	ReasoningTokens    usageField[int64] `json:"reasoning_tokens,omitzero"`
}

// usageField isolates malformed extension fields without dropping an attempt.
// An invalid value is replayed as []: a fixed, non-sensitive invalid type for
// every field above. Missing, explicit zero/false, and malformed survive replay.
// This wrapper is private to this queue extension, not a compatibility layer.
type usageField[T any] struct {
	Value     *T
	Malformed bool
}

func (f *usageField[T]) UnmarshalJSON(data []byte) error {
	*f = usageField[T]{}
	var value T
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) || json.Unmarshal(data, &value) != nil {
		f.Malformed = true
		return nil
	}
	f.Value = &value
	return nil
}

func (f usageField[T]) MarshalJSON() ([]byte, error) {
	if f.Malformed {
		return []byte("[]"), nil
	}
	return json.Marshal(f.Value)
}

func (f usageField[T]) IsZero() bool { return f.Value == nil && !f.Malformed }

// Unknown quality is a bounded category; arbitrary upstream text never enters
// the inbox or the database as a quality explanation.
type usageQuality string

func (q *usageQuality) UnmarshalJSON(data []byte) error {
	var value string
	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}
	switch value {
	case "complete", "inconsistent", "unclassified":
		*q = usageQuality(value)
	default:
		*q = "unknown"
	}
	return nil
}
