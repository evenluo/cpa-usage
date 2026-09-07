package service

import (
	"encoding/json"
	"testing"
)

func withAccountingV2(t testing.TB, message string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		t.Fatalf("decode test usage message: %v", err)
	}
	payload["accounting_version"] = float64(2)
	payload["generate"] = true
	payload["stream"] = true
	payload["token_breakdown"] = map[string]any{
		"schema_version":      float64(2),
		"quality":             "complete",
		"total_tokens":        float64(0),
		"input":               map[string]any{"total_tokens": float64(0), "uncached_tokens": float64(0), "cache_read_tokens": float64(0), "cache_write_tokens": float64(0)},
		"output":              map[string]any{"total_tokens": float64(0), "non_reasoning_tokens": float64(0), "reasoning_tokens": float64(0)},
		"unclassified_tokens": float64(0),
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("encode test usage message: %v", err)
	}
	return string(encoded)
}
