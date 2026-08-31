package entities

import "time"

// UsageEvent 是落库后的单条 CPA upstream usage attempt 实体。
type UsageEvent struct {
	ID              uint      `gorm:"primaryKey;index:idx_usage_events_timestamp_id,sort:desc,priority:2;index:idx_usage_events_auth_type_auth_index_id,priority:3;index:idx_usage_events_auth_type_source_id,priority:3"`
	EventKey        string    `gorm:"uniqueIndex:uniq_usage_events_event_key"`
	APIGroupKey     string    `gorm:"index:idx_usage_events_trim_api_group_key,expression:TRIM(api_group_key)"`
	Provider        string    `gorm:"column:provider;index:idx_usage_events_trim_provider,expression:TRIM(provider)"`
	Endpoint        string    `gorm:"column:endpoint"`
	AuthType        string    `gorm:"column:auth_type;index:idx_usage_events_trim_auth_type,expression:TRIM(auth_type);index:idx_usage_events_auth_type_auth_index_id,priority:1;index:idx_usage_events_auth_type_source_id,priority:1"`
	RequestID       string    `gorm:"column:request_id"`
	Model           string    `gorm:"index:idx_usage_events_model;index:idx_usage_events_trim_model,expression:TRIM(model)"`
	ModelAlias      *string   `gorm:"column:model_alias"`
	Timestamp       time.Time `gorm:"index:idx_usage_events_timestamp_id,sort:desc,priority:1"`
	Source          string    `gorm:"index:idx_usage_events_trim_source,expression:TRIM(source);index:idx_usage_events_auth_type_source_id,priority:2"`
	AuthIndex       string    `gorm:"index:idx_usage_events_trim_auth_index,expression:TRIM(auth_index);index:idx_usage_events_auth_type_auth_index_id,priority:2"`
	Failed          bool      `gorm:"index:idx_usage_events_failed"`
	StatusCode      int       `gorm:"column:status_code;not null;default:0"`
	ExecutorType    string    `gorm:"column:executor_type;not null;default:''"`
	ReasoningEffort string    `gorm:"column:reasoning_effort;not null;default:''"`
	ServiceTier     string    `gorm:"column:service_tier;not null;default:''"`
	LatencyMS       int64
	TTFTMS          *int64 `gorm:"column:ttft_ms"`
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	// CachedTokens preserves CPA's generic cached_tokens fact consumed by existing
	// analytics and cost paths. It must not be added to the explicit read/create
	// fields or presented as an explicit cache-read aggregate.
	CachedTokens        int64
	CacheReadTokens     *int64 `gorm:"column:cache_read_tokens"`
	CacheCreationTokens *int64 `gorm:"column:cache_creation_tokens"`
	TotalTokens         int64
	CreatedAt           time.Time
}
