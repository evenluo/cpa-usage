package authfiles

import "time"

// AuthFilesResponse 是 CPA /v8/management/credentials 响应 DTO。
type AuthFilesResponse struct {
	Files []AuthFile `json:"files"`
}

// AuthFileStatusRequest 是 CPA PATCH /v8/management/credentials/status 的请求 DTO。
// Name 匹配 CPA 侧的 auth ID 或文件名，不是 auth_index。
type AuthFileStatusRequest struct {
	Name     string `json:"name"`
	Disabled bool   `json:"disabled"`
}

// AuthFile 是 CPA /v8/management/credentials 中单个凭据的原始响应 DTO。
type AuthFile struct {
	ID                   string                      `json:"id"`
	AuthIndex            string                      `json:"auth_index"`
	Name                 string                      `json:"name"`
	Email                string                      `json:"email"`
	Type                 string                      `json:"type"`
	Provider             string                      `json:"provider"`
	Label                string                      `json:"label"`
	Status               *string                     `json:"status"`
	Source               string                      `json:"source"`
	Disabled             bool                        `json:"disabled"`
	Unavailable          *bool                       `json:"unavailable"`
	LastRefresh          *time.Time                  `json:"last_refresh"`
	NextRetryAfter       *time.Time                  `json:"next_retry_after"`
	RequiresSignIn       *bool                       `json:"requires_sign_in"`
	RenewalState         *string                     `json:"renewal_state"`
	RecoveryStage        *string                     `json:"recovery_stage"`
	AccessTokenExpiresAt *time.Time                  `json:"access_token_expires_at"`
	NextRefreshAfter     *time.Time                  `json:"next_refresh_after"`
	NextAction           *string                     `json:"next_action"`
	RecoveryHint         *string                     `json:"recovery_hint"`
	RefreshError         *AuthFileRefreshError       `json:"refresh_error"`
	Quota                *QuotaObservation           `json:"quota,omitempty"`
	ModelQuotas          map[string]QuotaObservation `json:"model_quotas,omitempty"`
	RuntimeOnly          bool                        `json:"runtime_only"`
	Account              string                      `json:"account,omitempty"`
	Metadata             map[string]any              `json:"metadata,omitempty"`
	Attributes           map[string]any              `json:"attributes,omitempty"`
	ProjectID            string                      `json:"project_id,omitempty"`
	ProjectIDCamel       string                      `json:"projectId,omitempty"`
	IDToken              *AuthFileIDToken            `json:"id_token"`
}

// AuthFileRefreshError preserves structured failure facts without retaining the upstream message body.
type AuthFileRefreshError struct {
	Code       string `json:"code"`
	HTTPStatus int    `json:"http_status"`
}

// QuotaObservation preserves the bounded passive signal map returned by CPA.
// Values stay untyped at this transport seam so one malformed signal can be
// rejected by the quota normalizer without failing the complete auth snapshot.
type QuotaObservation struct {
	ObservedAt any            `json:"observed_at,omitempty"`
	Signals    map[string]any `json:"signals,omitempty"`
}

// AuthFileIDToken 是 Codex auth file 的 id_token 订阅元数据 DTO。
type AuthFileIDToken struct {
	AccountID        *string    `json:"chatgpt_account_id,omitempty"`
	AccountIDCamel   *string    `json:"chatgptAccountId,omitempty"`
	ActiveStart      *time.Time `json:"chatgpt_subscription_active_start,omitempty"`
	ActiveStartCamel *time.Time `json:"chatgptSubscriptionActiveStart,omitempty"`
	ActiveUntil      *time.Time `json:"chatgpt_subscription_active_until,omitempty"`
	ActiveUntilCamel *time.Time `json:"chatgptSubscriptionActiveUntil,omitempty"`
	PlanType         *string    `json:"plan_type,omitempty"`
	PlanTypeCamel    *string    `json:"planType,omitempty"`
}
