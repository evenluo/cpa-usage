package entities

import (
	"strings"
	"time"
)

// UsageIdentityAuthType 表示 usage identity 的来源类型。
type UsageIdentityAuthType int

const (
	UsageIdentityAuthTypeAuthFile   UsageIdentityAuthType = 1
	UsageIdentityAuthTypeAIProvider UsageIdentityAuthType = 2
)

const (
	UsageIdentityAuthTypeNameOAuth  = "oauth"
	UsageIdentityAuthTypeNameAPIKey = "apikey"
)

// Valid reports whether the value is one of the persisted usage identity kinds.
func (authType UsageIdentityAuthType) Valid() bool {
	return authType == UsageIdentityAuthTypeAuthFile || authType == UsageIdentityAuthTypeAIProvider
}

// CanonicalName returns the canonical usage-event transport name for the identity kind.
func (authType UsageIdentityAuthType) CanonicalName() (string, bool) {
	switch authType {
	case UsageIdentityAuthTypeAuthFile:
		return UsageIdentityAuthTypeNameOAuth, true
	case UsageIdentityAuthTypeAIProvider:
		return UsageIdentityAuthTypeNameAPIKey, true
	default:
		return "", false
	}
}

// ParseUsageIdentityAuthType maps canonical usage-event transport names to identity kinds.
// Redis's historical "api_key" spelling is normalized at that transport boundary before parsing.
func ParseUsageIdentityAuthType(name string) (UsageIdentityAuthType, bool) {
	switch strings.TrimSpace(name) {
	case UsageIdentityAuthTypeNameOAuth:
		return UsageIdentityAuthTypeAuthFile, true
	case UsageIdentityAuthTypeNameAPIKey:
		return UsageIdentityAuthTypeAIProvider, true
	default:
		return 0, false
	}
}

// UsageIdentity 是从 CPA auth_files 和 provider config 同步出的 usage source 身份实体。
type UsageIdentity struct {
	ID           uint                  `gorm:"primaryKey;index:idx_usage_identities_auth_type_name_id,priority:3"`
	Name         string                `gorm:"index:idx_usage_identities_auth_type_name_id,priority:2"`
	AuthType     UsageIdentityAuthType `gorm:"uniqueIndex:uniq_usage_identities_type_identity;index:idx_usage_identities_auth_type_name_id,priority:1;index:idx_usage_identities_auth_type_type,priority:1"`
	AuthTypeName string
	Identity     string `gorm:"uniqueIndex:uniq_usage_identities_type_identity"`
	Type         string `gorm:"column:type;index:idx_usage_identities_auth_type_type,priority:2"`
	Provider     string
	LookupKey    string
	Prefix       string
	BaseURL      string
	AccountID    *string
	ProjectID    *string

	ActiveStart *time.Time
	ActiveUntil *time.Time
	PlanType    *string

	TotalRequests   int64
	SuccessCount    int64
	FailureCount    int64
	InputTokens     int64
	OutputTokens    int64
	ReasoningTokens int64
	CachedTokens    int64
	TotalTokens     int64

	LastAggregatedUsageEventID uint
	FirstUsedAt                *time.Time
	LastUsedAt                 *time.Time
	StatsUpdatedAt             *time.Time

	TotalCost     float64 `gorm:"-"`
	CostAvailable bool    `gorm:"-"`

	IsDeleted bool
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

// DisplayName 计算 usage identity 的人类可读展示名。
// 该规则同时服务 Request Evidence 解析和 Key Alias 分析，收拢在实体上以避免两层实现漂移。
func (item UsageIdentity) DisplayName() string {
	name := strings.TrimSpace(item.Name)
	provider := strings.TrimSpace(item.Provider)
	if item.AuthType != UsageIdentityAuthTypeAIProvider {
		if name != "" {
			return name
		}
		return provider
	}

	if strings.TrimSpace(item.Type) == "openai" && name != "" && name != "openai" && provider == name {
		return name
	}

	prefix := strings.TrimSpace(item.Prefix)
	baseURL := formatUsageIdentityBaseURLDisplay(item.BaseURL)
	qualifiers := usageIdentityDisplayQualifiers(prefix, baseURL)
	switch {
	case name != "" && len(qualifiers) > 0:
		return name + "(" + strings.Join(qualifiers, " @ ") + ")"
	case name != "":
		return name
	case prefix != "" && baseURL != "":
		return prefix + "(" + baseURL + ")"
	case prefix != "":
		return prefix
	case provider != "" && baseURL != "":
		return provider + "(" + baseURL + ")"
	case baseURL != "":
		return baseURL
	default:
		return provider
	}
}

func usageIdentityDisplayQualifiers(values ...string) []string {
	qualifiers := make([]string, 0, len(values))
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		qualifiers = append(qualifiers, value)
	}
	return qualifiers
}

func formatUsageIdentityBaseURLDisplay(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(lower, prefix) {
			trimmed = trimmed[len(prefix):]
			break
		}
	}
	return strings.TrimRight(trimmed, "/")
}
