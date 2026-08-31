package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository/dto"
)

type RedisQueue interface {
	PopUsage(ctx context.Context) ([]string, error)
}

func DecodeRedisUsageMessage(message string, fallbackTimestamp time.Time) (entities.UsageEvent, json.RawMessage, error) {
	raw := json.RawMessage(message)
	var payload queuedUsageDetail
	if err := json.Unmarshal(raw, &payload); err != nil {
		return entities.UsageEvent{}, nil, fmt.Errorf("decode redis usage message: %w", err)
	}
	return payload.toUsageEvent(fallbackTimestamp), raw, nil
}

// replaySafeRedisUsageMessage projects a popped CPA record through the same
// schema used by the decoder before it can enter SQLite. This retains every
// field required for deterministic replay while excluding fail.body,
// response_headers, and arbitrary unknown payload fields.
func replaySafeRedisUsageMessage(message string) (string, error) {
	var payload queuedUsageDetail
	if err := json.Unmarshal([]byte(message), &payload); err != nil {
		return redactedInvalidRedisUsageMessage(message), nil
	}
	compactQueuedUsageDetail(&payload)
	projected, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("encode replay-safe redis usage message: %w", err)
	}
	return string(projected), nil
}

func redactedInvalidRedisUsageMessage(message string) string {
	digest := sha256.Sum256([]byte(message))
	// A JSON string is intentionally not decodable as queuedUsageDetail, so the
	// existing decode_failed lifecycle remains the single invalid-row owner.
	marker := fmt.Sprintf("redacted invalid CPA usage message sha256=%x bytes=%d", digest, len(message))
	encoded, _ := json.Marshal(marker)
	return string(encoded)
}

type queuedUsageDetail struct {
	Timestamp       *time.Time             `json:"timestamp,omitempty"`
	LatencyMS       int64                  `json:"latency_ms,omitempty"`
	TTFTMS          *int64                 `json:"ttft_ms,omitempty"`
	Source          string                 `json:"source,omitempty"`
	AuthIndex       string                 `json:"auth_index,omitempty"`
	Tokens          *queuedUsageTokenStats `json:"tokens,omitempty"`
	Failed          bool                   `json:"failed,omitempty"`
	Fail            *queuedUsageFail       `json:"fail,omitempty"`
	Provider        string                 `json:"provider,omitempty"`
	ExecutorType    string                 `json:"executor_type,omitempty"`
	Model           string                 `json:"model,omitempty"`
	Alias           *string                `json:"alias,omitempty"`
	Endpoint        string                 `json:"endpoint,omitempty"`
	AuthType        string                 `json:"auth_type,omitempty"`
	APIKey          string                 `json:"api_key,omitempty"`
	RequestID       string                 `json:"request_id,omitempty"`
	ReasoningEffort string                 `json:"reasoning_effort,omitempty"`
	ServiceTier     string                 `json:"service_tier,omitempty"`
}

type queuedUsageFail struct {
	StatusCode int `json:"status_code,omitempty"`
}

type queuedUsageTokenStats struct {
	InputTokens         int64  `json:"input_tokens,omitempty"`
	OutputTokens        int64  `json:"output_tokens,omitempty"`
	ReasoningTokens     int64  `json:"reasoning_tokens,omitempty"`
	CachedTokens        *int64 `json:"cached_tokens,omitempty"`
	CacheReadTokens     *int64 `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens *int64 `json:"cache_creation_tokens,omitempty"`
	TotalTokens         int64  `json:"total_tokens,omitempty"`
}

func (t queuedUsageTokenStats) compatibilityProjection() dto.TokenStats {
	cachedTokens := int64(0)
	if t.CachedTokens != nil {
		cachedTokens = *t.CachedTokens
	}
	return dto.TokenStats{
		InputTokens:     t.InputTokens,
		OutputTokens:    t.OutputTokens,
		ReasoningTokens: t.ReasoningTokens,
		CachedTokens:    cachedTokens,
		TotalTokens:     t.TotalTokens,
	}
}

func compactQueuedUsageDetail(payload *queuedUsageDetail) {
	if payload.Timestamp != nil && payload.Timestamp.IsZero() {
		payload.Timestamp = nil
	}
	if payload.Fail != nil && payload.Fail.StatusCode == 0 {
		payload.Fail = nil
	}
	if payload.Tokens != nil && payload.Tokens.isZero() {
		payload.Tokens = nil
	}
	payload.Source = strings.TrimSpace(payload.Source)
	payload.AuthIndex = strings.TrimSpace(payload.AuthIndex)
	payload.Provider = strings.TrimSpace(payload.Provider)
	payload.ExecutorType = strings.TrimSpace(payload.ExecutorType)
	payload.Model = strings.TrimSpace(payload.Model)
	payload.Endpoint = strings.TrimSpace(payload.Endpoint)
	payload.AuthType = strings.TrimSpace(payload.AuthType)
	payload.APIKey = strings.TrimSpace(payload.APIKey)
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.ReasoningEffort = strings.TrimSpace(payload.ReasoningEffort)
	payload.ServiceTier = strings.TrimSpace(payload.ServiceTier)
	payload.Alias = trimRedisOptionalString(payload.Alias)
}

func (t queuedUsageTokenStats) isZero() bool {
	return t.InputTokens == 0 &&
		t.OutputTokens == 0 &&
		t.ReasoningTokens == 0 &&
		t.CachedTokens == nil &&
		t.CacheReadTokens == nil &&
		t.CacheCreationTokens == nil &&
		t.TotalTokens == 0
}

func normalizeRedisAuthType(value string) string {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	if trimmed == "api_key" {
		trimmed = entities.UsageIdentityAuthTypeNameAPIKey
	}
	authType, ok := entities.ParseUsageIdentityAuthType(trimmed)
	if !ok {
		// UsageEvent keeps unknown CPA transport values losslessly. Identity read
		// projections parse and explicitly exclude values they do not own.
		return trimmed
	}
	canonical, _ := authType.CanonicalName()
	return canonical
}

func trimRedisOptionalString(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (d queuedUsageDetail) toUsageEvent(fallbackTimestamp time.Time) entities.UsageEvent {
	queuedTokens := d.tokenStats()
	tokens := normalizeTokens(queuedTokens.compatibilityProjection())
	apiGroupKey := firstNonEmpty(d.APIKey, d.Provider, d.Endpoint, "unknown")
	model := firstNonEmpty(d.Model, "unknown")
	timestamp := fallbackTimestamp.UTC()
	if d.Timestamp != nil && !d.Timestamp.IsZero() {
		timestamp = d.Timestamp.UTC()
	}
	source := strings.TrimSpace(d.Source)
	authIndex := strings.TrimSpace(d.AuthIndex)
	eventKey := strings.TrimSpace(d.RequestID)
	if eventKey == "" {
		eventKey = BuildEventKey(apiGroupKey, model, timestamp, source, authIndex, d.Failed, tokens)
	}
	return entities.UsageEvent{
		EventKey:            eventKey,
		APIGroupKey:         apiGroupKey,
		Provider:            strings.TrimSpace(d.Provider),
		Endpoint:            strings.TrimSpace(d.Endpoint),
		AuthType:            normalizeRedisAuthType(d.AuthType),
		RequestID:           strings.TrimSpace(d.RequestID),
		Model:               model,
		ModelAlias:          trimRedisOptionalString(d.Alias),
		Timestamp:           timestamp,
		Source:              source,
		AuthIndex:           authIndex,
		Failed:              d.Failed,
		StatusCode:          d.statusCode(),
		ExecutorType:        strings.TrimSpace(d.ExecutorType),
		ReasoningEffort:     strings.TrimSpace(d.ReasoningEffort),
		ServiceTier:         strings.TrimSpace(d.ServiceTier),
		LatencyMS:           max(d.LatencyMS, 0),
		TTFTMS:              d.TTFTMS,
		InputTokens:         tokens.InputTokens,
		OutputTokens:        tokens.OutputTokens,
		ReasoningTokens:     tokens.ReasoningTokens,
		CachedTokens:        tokens.CachedTokens,
		CacheReadTokens:     queuedTokens.CacheReadTokens,
		CacheCreationTokens: queuedTokens.CacheCreationTokens,
		TotalTokens:         tokens.TotalTokens,
	}
}

func (d queuedUsageDetail) tokenStats() queuedUsageTokenStats {
	if d.Tokens == nil {
		return queuedUsageTokenStats{}
	}
	return *d.Tokens
}

func (d queuedUsageDetail) statusCode() int {
	if d.Fail == nil {
		return 0
	}
	return d.Fail.StatusCode
}
