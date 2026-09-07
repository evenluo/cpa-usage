package models

// AuthFileModelsResponse is the narrow CPA registered-model response for one
// auth file. These rows describe registry membership, not live routability.
type AuthFileModelsResponse struct {
	Models []RegisteredModel `json:"models"`
}

// RegisteredModel is the allowlisted model identity returned by
// /management/auth-files/models.
type RegisteredModel struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name,omitempty"`
	Type        string `json:"type,omitempty"`
	OwnedBy     string `json:"owned_by,omitempty"`
}

// StaticModelDefinitionsResponse is the CPA static catalog for one channel.
type StaticModelDefinitionsResponse struct {
	Channel string                  `json:"channel"`
	Models  []StaticModelDefinition `json:"models"`
}

// StaticModelDefinition deliberately allowlists only user-visible capability
// facts needed by CPA Usage. Configuration and unknown upstream fields are not
// retained.
type StaticModelDefinition struct {
	ID                        string           `json:"id"`
	DisplayName               string           `json:"display_name,omitempty"`
	ContextLength             *int             `json:"context_length,omitempty"`
	InputTokenLimit           *int             `json:"inputTokenLimit,omitempty"`
	MaxCompletionTokens       *int             `json:"max_completion_tokens,omitempty"`
	OutputTokenLimit          *int             `json:"outputTokenLimit,omitempty"`
	SupportedInputModalities  []string         `json:"supportedInputModalities,omitempty"`
	SupportedOutputModalities []string         `json:"supportedOutputModalities,omitempty"`
	Thinking                  *ThinkingSupport `json:"thinking,omitempty"`
}

// ThinkingSupport preserves field presence so omitted false/zero values are
// not presented as upstream negative assertions.
type ThinkingSupport struct {
	Min            *int     `json:"min,omitempty"`
	Max            *int     `json:"max,omitempty"`
	ZeroAllowed    *bool    `json:"zero_allowed,omitempty"`
	DynamicAllowed *bool    `json:"dynamic_allowed,omitempty"`
	Levels         []string `json:"levels,omitempty"`
}

// ModelsResponse 是 CPA OpenAI-compatible /models 响应 DTO。
type ModelsResponse struct {
	Object string      `json:"object"`
	Data   []ModelInfo `json:"data"`
}

// ModelInfo 是 CPA OpenAI-compatible /models 响应中的单个模型 DTO。
type ModelInfo struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	Created int64  `json:"created,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}
