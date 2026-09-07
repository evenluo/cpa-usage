package service

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestDecodeRedisUsageMessageMapsPayloadToUsageEvent(t *testing.T) {
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)

	event, raw, err := DecodeRedisUsageMessage(withAccountingV2(t, `{
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
		}`), fetchedAt)
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.EventKey != "" || event.APIGroupKey != "raw-key" || event.Model != "claude-sonnet-4-6" || event.Source != "sk-test" || event.AuthIndex != "auth-1" || !event.Failed || event.LatencyMS != 1234 {
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
	if event.InputTokens != 0 || event.OutputTokens != 0 || event.ReasoningTokens != 0 || event.CachedTokens != 0 || event.TotalTokens != 0 || event.CacheReadTokens != nil || event.CacheCreationTokens != nil {
		t.Fatalf("new events must not populate archival scalar tokens: %+v", event)
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

func TestReplaySafeRedisUsageMessageCompactsEmptyFieldsAndPreservesCanonicalZeroFacts(t *testing.T) {
	fetchedAt := time.Date(2026, 8, 31, 8, 0, 0, 0, time.UTC)
	projected, err := replaySafeRedisUsageMessage(withAccountingV2(t, `{
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
		}`))
	if err != nil {
		t.Fatalf("project replay-safe message: %v", err)
	}

	for _, omitted := range []string{
		`"timestamp"`, `"latency_ms"`, `"ttft_ms"`, `"source"`, `"auth_index"`, `"tokens"`, `"failed"`, `"fail"`, `"provider"`, `"executor_type"`, `"model"`, `"alias"`, `"endpoint"`, `"auth_type"`, `"api_key"`, `"request_id"`, `"reasoning_effort"`, `"service_tier"`,
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
	for _, explicitZero := range []string{`"uncached_tokens":0`, `"cache_read_tokens":0`, `"cache_write_tokens":0`, `"reasoning_tokens":0`} {
		if !strings.Contains(projected, explicitZero) {
			t.Fatalf("replay-safe projection lost explicit zero cache fact %s: %s", explicitZero, projected)
		}
	}

	event, _, err := DecodeRedisUsageMessage(projected, fetchedAt)
	if err != nil {
		t.Fatalf("decode compact replay projection: %v", err)
	}
	if !event.Timestamp.Equal(fetchedAt) || event.CachedTokens != 0 || event.CacheReadTokens != nil || event.CacheCreationTokens != nil {
		t.Fatalf("compact replay projection changed decode semantics: %+v", event)
	}
}

func TestDecodeRedisUsageMessageFallsBackFieldsAndEventKey(t *testing.T) {
	fetchedAt := time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC)

	event, _, err := DecodeRedisUsageMessage(withAccountingV2(t, `{"latency_ms":-5,"tokens":{"input_tokens":1,"output_tokens":2},"endpoint":"/fallback"}`), fetchedAt)
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
	if event.EventKey != "" {
		t.Fatalf("pure decoding must not assign attempt identity, got %s", event.EventKey)
	}
}

func TestDecodeRedisUsageMessageFallsBackToProviderWhenAPIKeyIsBlank(t *testing.T) {
	event, _, err := DecodeRedisUsageMessage(withAccountingV2(t, `{"api_key":"   ","provider":"claude","endpoint":"/v1/messages","request_id":"req-blank-key"}`), time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("DecodeRedisUsageMessage returned error: %v", err)
	}
	if event.EventKey != "" || event.APIGroupKey != "claude" {
		t.Fatalf("unexpected fallback event: %+v", event)
	}
}

func TestDecodeRedisUsageMessagePreservesUnknownAuthType(t *testing.T) {
	event, _, err := DecodeRedisUsageMessage(withAccountingV2(t, `{"auth_type":"  Future_Auth  ","request_id":"req-future"}`), time.Date(2026, 4, 27, 8, 0, 0, 0, time.UTC))
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
