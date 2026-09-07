package api

import (
	"context"
	"errors"
	"net/http"

	"cpa-usage/internal/service"

	"github.com/gin-gonic/gin"
)

type ModelSupportProvider interface {
	Load(context.Context, service.ModelSupportRequest) (service.ModelSupportResult, error)
}

type modelSupportRequest struct {
	IdentityIDs []uint `json:"identity_ids"`
}

type modelSupportResponse struct {
	ScopeComplete bool                           `json:"scope_complete"`
	SelectedCount int                            `json:"selected_count"`
	LoadedCount   int                            `json:"loaded_count"`
	Accounts      []accountModelSupportResponse  `json:"accounts"`
	Models        []modelSupportCoverageResponse `json:"models"`
	Limits        modelSupportLimitsResponse     `json:"limits"`
}

type modelSupportLimitsResponse struct {
	MaxAccounts         int `json:"max_accounts"`
	MaxConcurrency      int `json:"max_concurrency"`
	TimeoutSeconds      int `json:"timeout_seconds"`
	MaxUpstreamRequests int `json:"max_upstream_requests"`
}

type accountModelSupportResponse struct {
	IdentityID       uint                             `json:"identity_id"`
	AuthIndex        string                           `json:"auth_index"`
	DisplayName      string                           `json:"display_name"`
	Provider         string                           `json:"provider"`
	Channel          string                           `json:"channel,omitempty"`
	Disabled         bool                             `json:"disabled"`
	Unavailable      *bool                            `json:"unavailable"`
	Status           string                           `json:"status"`
	ErrorCode        string                           `json:"error_code,omitempty"`
	CatalogStatus    string                           `json:"catalog_status"`
	RegisteredModels []registeredModelSupportResponse `json:"registered_models"`
}

type registeredModelSupportResponse struct {
	ID               string                   `json:"id"`
	DisplayName      string                   `json:"display_name,omitempty"`
	Type             string                   `json:"type,omitempty"`
	OwnedBy          string                   `json:"owned_by,omitempty"`
	DefinitionStatus string                   `json:"definition_status"`
	Capability       *modelCapabilityResponse `json:"capability,omitempty"`
}

type modelCapabilityResponse struct {
	ContextLength             *int                          `json:"context_length,omitempty"`
	InputTokenLimit           *int                          `json:"input_token_limit,omitempty"`
	MaxCompletionTokens       *int                          `json:"max_completion_tokens,omitempty"`
	OutputTokenLimit          *int                          `json:"output_token_limit,omitempty"`
	SupportedInputModalities  []string                      `json:"supported_input_modalities,omitempty"`
	SupportedOutputModalities []string                      `json:"supported_output_modalities,omitempty"`
	Thinking                  *modelThinkingSupportResponse `json:"thinking,omitempty"`
}

type modelThinkingSupportResponse struct {
	Min            *int     `json:"min,omitempty"`
	Max            *int     `json:"max,omitempty"`
	ZeroAllowed    *bool    `json:"zero_allowed,omitempty"`
	DynamicAllowed *bool    `json:"dynamic_allowed,omitempty"`
	Levels         []string `json:"levels,omitempty"`
}

type modelSupportCoverageResponse struct {
	ModelID                        string `json:"model_id"`
	DisplayName                    string `json:"display_name,omitempty"`
	ObservedSupportingAccounts     int    `json:"observed_supporting_accounts"`
	SelectedAccounts               int    `json:"selected_accounts"`
	SingleRegisteredAccountInScope *bool  `json:"single_registered_account_in_scope"`
}

func registerModelSupportRoute(router gin.IRoutes, provider ModelSupportProvider) {
	router.POST("/usage/identities/model-support", func(c *gin.Context) {
		if provider == nil {
			writeInternalError(c, "model support provider is not configured", nil)
			return
		}
		var request modelSupportRequest
		if err := c.ShouldBindJSON(&request); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid model support payload"})
			return
		}
		result, err := provider.Load(c.Request.Context(), service.ModelSupportRequest{IdentityIDs: request.IdentityIDs})
		if err != nil {
			if errors.Is(err, service.ErrModelSupportScopeRequired) || errors.Is(err, service.ErrModelSupportScopeTooLarge) || errors.Is(err, service.ErrModelSupportScopeInvalid) {
				c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
				return
			}
			writeInternalError(c, "load model support failed", err)
			return
		}
		c.JSON(http.StatusOK, mapModelSupportResponse(result))
	})
}

func mapModelSupportResponse(result service.ModelSupportResult) modelSupportResponse {
	accounts := make([]accountModelSupportResponse, 0, len(result.Accounts))
	for _, account := range result.Accounts {
		models := make([]registeredModelSupportResponse, 0, len(account.RegisteredModels))
		for _, model := range account.RegisteredModels {
			models = append(models, registeredModelSupportResponse{
				ID: model.ID, DisplayName: model.DisplayName, Type: model.Type, OwnedBy: model.OwnedBy,
				DefinitionStatus: model.DefinitionStatus, Capability: mapModelCapabilityResponse(model.Capability),
			})
		}
		accounts = append(accounts, accountModelSupportResponse{
			IdentityID: account.IdentityID, AuthIndex: account.AuthIndex, DisplayName: account.DisplayName,
			Provider: account.Provider, Channel: account.Channel, Disabled: account.Disabled, Unavailable: account.Unavailable,
			Status: account.Status, ErrorCode: account.ErrorCode, CatalogStatus: account.CatalogStatus, RegisteredModels: models,
		})
	}
	models := make([]modelSupportCoverageResponse, 0, len(result.Models))
	for _, model := range result.Models {
		models = append(models, modelSupportCoverageResponse{
			ModelID: model.ModelID, DisplayName: model.DisplayName,
			ObservedSupportingAccounts: model.ObservedSupportingAccounts, SelectedAccounts: model.SelectedAccounts,
			SingleRegisteredAccountInScope: model.SingleRegisteredAccountInScope,
		})
	}
	return modelSupportResponse{
		ScopeComplete: result.Complete, SelectedCount: result.SelectedCount, LoadedCount: result.LoadedCount,
		Accounts: accounts, Models: models,
		Limits: modelSupportLimitsResponse{
			MaxAccounts: result.Limits.MaxAccounts, MaxConcurrency: result.Limits.MaxConcurrency,
			TimeoutSeconds: result.Limits.TimeoutSeconds, MaxUpstreamRequests: result.Limits.MaxUpstreamRequests,
		},
	}
}

func mapModelCapabilityResponse(capability *service.ModelCapability) *modelCapabilityResponse {
	if capability == nil {
		return nil
	}
	response := &modelCapabilityResponse{
		ContextLength: capability.ContextLength, InputTokenLimit: capability.InputTokenLimit,
		MaxCompletionTokens: capability.MaxCompletionTokens, OutputTokenLimit: capability.OutputTokenLimit,
		SupportedInputModalities:  capability.SupportedInputModalities,
		SupportedOutputModalities: capability.SupportedOutputModalities,
	}
	if capability.Thinking != nil {
		response.Thinking = &modelThinkingSupportResponse{
			Min: capability.Thinking.Min, Max: capability.Thinking.Max,
			ZeroAllowed: capability.Thinking.ZeroAllowed, DynamicAllowed: capability.Thinking.DynamicAllowed,
			Levels: capability.Thinking.Levels,
		}
	}
	return response
}
