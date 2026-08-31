package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"cpa-usage/internal/repository/dto"
)

func TestDecodeRedisUsageMessageMapsPayloadToUsageEvent(t *testing.T) {
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)

	event, raw, err := DecodeRedisUsageMessage(`{
		"timestamp":"2026-04-27T07:59:00Z",
		"latency_ms":1234,
		"ttft_ms":234,
		"source":"sk-test",
		"auth_index":"auth-1",
		"tokens":{"input_tokens":10,"output_tokens":20,"reasoning_tokens":3,"cached_tokens":7,"cache_read_tokens":4,"cache_creation_tokens":3,"total_tokens":0},
		"failed":true,
		"fail":{"status_code":429,"body":"must not persist"},
		"provider":"claude",
		"executor_type":"claude",
		"model":"claude-sonnet-4-6",
		"alias":"claude-sonnet-alias",
		"endpoint":"/v1/messages",
		"auth_type":"api_key",
		"api_key":"raw-key",
		"request_id":"req-123",
		"reasoning_effort":"high",
		"service_tier":"priority",
		"unknown":"ignored"
	}`, fetchedAt)
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.EventKey != "req-123" || event.APIGroupKey != "raw-key" || event.Model != "claude-sonnet-4-6" || event.Source != "sk-test" || event.AuthIndex != "auth-1" || !event.Failed || event.LatencyMS != 1234 {
		t.Fatalf("unexpected event: %+v", event)
	}
	if event.TTFTMS == nil || *event.TTFTMS != 234 {
		t.Fatalf("expected ttft_ms 234, got %+v", event.TTFTMS)
	}
	if event.Provider != "claude" || event.Endpoint != "/v1/messages" || event.AuthType != "apikey" || event.RequestID != "req-123" {
		t.Fatalf("unexpected redis identity fields: %+v", event)
	}
	if event.ModelAlias == nil || *event.ModelAlias != "claude-sonnet-alias" {
		t.Fatalf("expected model alias to decode, got %+v", event.ModelAlias)
	}
	if event.InputTokens != 10 || event.OutputTokens != 20 || event.ReasoningTokens != 3 || event.CachedTokens != 7 || event.TotalTokens != 33 {
		t.Fatalf("unexpected tokens: %+v", event)
	}
	if event.CacheReadTokens == nil || *event.CacheReadTokens != 4 || event.CacheCreationTokens == nil || *event.CacheCreationTokens != 3 {
		t.Fatalf("expected exact cache token fields to be preserved, got %+v", event)
	}
	if event.StatusCode != 429 || event.ExecutorType != "claude" || event.ReasoningEffort != "high" || event.ServiceTier != "priority" {
		t.Fatalf("expected safe attempt metadata to decode, got %+v", event)
	}
	if !event.Timestamp.Equal(time.Date(2026, 4, 27, 7, 59, 0, 0, time.UTC)) {
		t.Fatalf("unexpected timestamp: %s", event.Timestamp)
	}
	if !strings.Contains(string(raw), `"unknown":"ignored"`) {
		t.Fatalf("expected raw message to be preserved, got %s", string(raw))
	}
}

func TestDecodeRedisUsageMessagePreservesGenericProjectionAndExactCacheFields(t *testing.T) {
	fetchedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	tests := []struct {
		name                 string
		message              string
		expectedCachedTokens int64
		expectedRead         *int64
		expectedCreation     *int64
	}{
		{
			name:                 "explicit split does not change generic compatibility projection",
			message:              `{"tokens":{"cached_tokens":9,"cache_read_tokens":4,"cache_creation_tokens":5}}`,
			expectedCachedTokens: 9,
			expectedRead:         int64Pointer(4),
			expectedCreation:     int64Pointer(5),
		},
		{
			name:                 "legacy generic remains compatible",
			message:              `{"tokens":{"cached_tokens":6}}`,
			expectedCachedTokens: 6,
		},
		{
			name:                 "creation split remains lossless without changing generic projection",
			message:              `{"tokens":{"cached_tokens":5,"cache_read_tokens":0,"cache_creation_tokens":5}}`,
			expectedCachedTokens: 5,
			expectedRead:         int64Pointer(0),
			expectedCreation:     int64Pointer(5),
		},
		{
			name:    "missing fields remain distinguishable from zero",
			message: `{"tokens":{}}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, _, err := DecodeRedisUsageMessage(tt.message, fetchedAt)
			if err != nil {
				t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
			}
			if event.CachedTokens != tt.expectedCachedTokens || !equalOptionalInt64(event.CacheReadTokens, tt.expectedRead) || !equalOptionalInt64(event.CacheCreationTokens, tt.expectedCreation) {
				t.Fatalf("unexpected cache projection: %+v", event)
			}
		})
	}
}

func TestReplaySafeRedisUsageMessageCompactsEmptyFieldsAndPreservesExplicitZeroCacheFacts(t *testing.T) {
	fetchedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	projected, err := replaySafeRedisUsageMessage(`{
		"timestamp":"0001-01-01T00:00:00Z",
		"latency_ms":0,
		"ttft_ms":null,
		"source":"   ",
		"auth_index":"",
		"tokens":{"input_tokens":0,"output_tokens":0,"reasoning_tokens":0,"cached_tokens":0,"cache_read_tokens":0,"cache_creation_tokens":0,"total_tokens":0},
		"failed":false,
		"fail":{"status_code":0,"body":"PRIVATE_FAIL_BODY"},
		"provider":"",
		"executor_type":"",
		"model":"",
		"alias":" ",
		"endpoint":"",
		"auth_type":"",
		"api_key":"",
		"request_id":"",
		"reasoning_effort":"",
		"service_tier":"",
		"response_headers":{"set-cookie":"PRIVATE_COOKIE"},
		"unknown":"PRIVATE_UNKNOWN"
	}`)
	if err != nil {
		t.Fatalf("project replay-safe message: %v", err)
	}

	for _, omitted := range []string{
		`"timestamp"`, `"latency_ms"`, `"ttft_ms"`, `"source"`, `"auth_index"`, `"input_tokens"`, `"output_tokens"`, `"reasoning_tokens"`, `"total_tokens"`, `"failed"`, `"fail"`, `"provider"`, `"executor_type"`, `"model"`, `"alias"`, `"endpoint"`, `"auth_type"`, `"api_key"`, `"request_id"`, `"reasoning_effort"`, `"service_tier"`,
	} {
		if strings.Contains(projected, omitted) {
			t.Fatalf("replay-safe projection retained empty field %s: %s", omitted, projected)
		}
	}
	for _, forbidden := range []string{"PRIVATE_FAIL_BODY", "PRIVATE_COOKIE", "PRIVATE_UNKNOWN", "response_headers", `"body"`, `"unknown"`} {
		if strings.Contains(projected, forbidden) {
			t.Fatalf("replay-safe projection retained excluded field %s: %s", forbidden, projected)
		}
	}
	for _, explicitZero := range []string{`"cached_tokens":0`, `"cache_read_tokens":0`, `"cache_creation_tokens":0`} {
		if !strings.Contains(projected, explicitZero) {
			t.Fatalf("replay-safe projection lost explicit zero cache fact %s: %s", explicitZero, projected)
		}
	}

	event, _, err := DecodeRedisUsageMessage(projected, fetchedAt)
	if err != nil {
		t.Fatalf("decode compact replay projection: %v", err)
	}
	if !event.Timestamp.Equal(fetchedAt) || event.CachedTokens != 0 || !equalOptionalInt64(event.CacheReadTokens, int64Pointer(0)) || !equalOptionalInt64(event.CacheCreationTokens, int64Pointer(0)) {
		t.Fatalf("compact replay projection changed decode semantics: %+v", event)
	}
}

func int64Pointer(value int64) *int64 { return &value }

func equalOptionalInt64(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func TestDecodeRedisUsageMessageFallsBackFieldsAndEventKey(t *testing.T) {
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)

	event, _, err := DecodeRedisUsageMessage(`{"latency_ms":-5,"tokens":{"input_tokens":1,"output_tokens":2},"endpoint":"/fallback"}`, fetchedAt)
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.APIGroupKey != "/fallback" || event.Model != "unknown" || event.LatencyMS != 0 {
		t.Fatalf("unexpected fallback event: %+v", event)
	}
	if event.Provider != "" || event.Endpoint != "/fallback" || event.AuthType != "" || event.RequestID != "" {
		t.Fatalf("unexpected fallback redis identity fields: %+v", event)
	}
	if event.ModelAlias != nil {
		t.Fatalf("expected missing alias to stay nil, got %+v", event.ModelAlias)
	}
	if !event.Timestamp.Equal(fetchedAt) {
		t.Fatalf("expected fetchedAt timestamp, got %s", event.Timestamp)
	}
	expectedKey := BuildEventKey("/fallback", "unknown", fetchedAt, "", "", false, dto.TokenStats{InputTokens: 1, OutputTokens: 2})
	if event.EventKey != expectedKey {
		t.Fatalf("expected fallback event key %s, got %s", expectedKey, event.EventKey)
	}
}

func TestDecodeRedisUsageMessageFallsBackToProviderWhenAPIKeyIsBlank(t *testing.T) {
	event, _, err := DecodeRedisUsageMessage(`{"api_key":"   ","provider":"claude","endpoint":"/v1/messages","request_id":"req-blank-key"}`, time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.EventKey != "req-blank-key" || event.APIGroupKey != "claude" {
		t.Fatalf("unexpected fallback event: %+v", event)
	}
}

func TestDecodeRedisUsageMessagePreservesUnknownAuthType(t *testing.T) {
	event, _, err := DecodeRedisUsageMessage(`{"auth_type":"  Future_Auth  ","request_id":"req-future"}`, time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.AuthType != "future_auth" {
		t.Fatalf("expected unknown CPA auth type to remain lossless after transport normalization, got %q", event.AuthType)
	}
}

func TestDecodeRedisUsageMessageReportsOnlyMessageError(t *testing.T) {
	_, _, err := DecodeRedisUsageMessage(`{bad-json}`, time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC))
	if err == nil || !strings.Contains(err.Error(), "decode redis usage message") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

type staticRedisQueue struct {
	messages []string
	err      error
}

func (q staticRedisQueue) PopUsage(context.Context) ([]string, error) {
	return q.messages, q.err
}
