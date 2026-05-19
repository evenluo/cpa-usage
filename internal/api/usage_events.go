package api

import (
	"net/http"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	"cpa-usage/internal/service"
	servicedto "cpa-usage/internal/service/dto"

	"github.com/gin-gonic/gin"
)

type usageEventsResponse struct {
	Events     []usageEventPayload `json:"events"`
	TotalCount int64               `json:"total_count"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}

type usageSourceFilterOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	DisplayName string `json:"displayName"`
}

type usageEventFilterOptionsResponse struct {
	Models  []string                  `json:"models"`
	Sources []usageSourceFilterOption `json:"sources"`
}

type usageEventPayload struct {
	ID            uint                   `json:"id,omitempty"`
	Timestamp     string                 `json:"timestamp"`
	Model         string                 `json:"model"`
	Source        string                 `json:"source"`
	SourceRaw     string                 `json:"source_raw,omitempty"`
	SourceType    string                 `json:"source_type,omitempty"`
	AuthIndex     string                 `json:"auth_index,omitempty"`
	APIKeyAlias   string                 `json:"api_key_alias,omitempty"`
	APIKeyDisplay string                 `json:"api_key_display,omitempty"`
	IsDelete      bool                   `json:"isDelete,omitempty"`
	Failed        bool                   `json:"failed"`
	LatencyMS     int64                  `json:"latency_ms"`
	Tokens        usageEventTokenPayload `json:"tokens"`
}

type usageEventTokenPayload struct {
	InputTokens     int64 `json:"input_tokens"`
	OutputTokens    int64 `json:"output_tokens"`
	ReasoningTokens int64 `json:"reasoning_tokens"`
	CachedTokens    int64 `json:"cached_tokens"`
	TotalTokens     int64 `json:"total_tokens"`
}

func registerUsageEventsRoute(
	router gin.IRoutes,
	usageProvider service.UsageProvider,
	usageIdentityProvider service.UsageIdentityProvider,
	keyAliasProvider service.KeyAliasProvider,
) {
	router.GET("/usage/events/filters/models", func(c *gin.Context) {
		models, err := loadUsageEventModelFilterOptions(c, usageProvider)
		if err != nil {
			writeInternalError(c, "list usage event model filter options failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"models": models})
	})

	router.GET("/usage/events/filters/sources", func(c *gin.Context) {
		sources, err := loadUsageEventSourceFilterOptions(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "list usage event source filter options failed", err)
			return
		}
		c.JSON(http.StatusOK, gin.H{"sources": sources})
	})

	router.GET("/usage/events", func(c *gin.Context) {
		if usageProvider == nil {
			c.JSON(http.StatusOK, usageEventsResponse{Events: []usageEventPayload{}, Page: 1, PageSize: servicedto.DefaultUsageEventsLimit})
			return
		}

		filter, err := parseUsageFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if err := applyUsageEventsSourceFilter(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		rows, err := usageProvider.ListUsageEvents(c.Request.Context(), filter)
		if err != nil {
			writeInternalError(c, "list usage events failed", err)
			return
		}

		identities, err := loadUsageResolutionData(c, usageIdentityProvider)
		if err != nil {
			writeInternalError(c, "load usage resolution data failed", err)
			return
		}
		resolver := newUsageIdentityResolver(identities)
		apiKeyAliases, err := loadUsageEventAPIKeyAliases(c, keyAliasProvider, rows.Events)
		if err != nil {
			writeInternalError(c, "load usage event api key aliases failed", err)
			return
		}
		c.JSON(http.StatusOK, usageEventsResponse{
			Events:     buildUsageEventsPayload(rows.Events, resolver, apiKeyAliases),
			TotalCount: rows.TotalCount,
			Page:       rows.Page,
			PageSize:   rows.PageSize,
			TotalPages: rows.TotalPages,
		})
	})
}

func loadUsageEventAPIKeyAliases(c *gin.Context, keyAliasProvider service.KeyAliasProvider, rows []servicedto.UsageEventRecord) (map[string]string, error) {
	result := map[string]string{}
	if keyAliasProvider == nil || len(rows) == 0 {
		return result, nil
	}
	identities := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		identity := strings.TrimSpace(row.APIKeyIdentity)
		if identity == "" {
			continue
		}
		if _, ok := seen[identity]; ok {
			continue
		}
		seen[identity] = struct{}{}
		identities = append(identities, identity)
	}
	if len(identities) == 0 {
		return result, nil
	}
	return keyAliasProvider.ListAliasesForAPIKeyIdentities(c.Request.Context(), identities)
}

// Source 下拉提交的是 usage identity，进入仓储前转换成 auth_index 查询。
func applyUsageEventsSourceFilter(filter *servicedto.UsageFilter) error {
	if filter == nil {
		return nil
	}
	source := strings.TrimSpace(filter.Source)
	if source == "" {
		return nil
	}
	filter.AuthIndex = source
	filter.Source = ""
	return nil
}

// 列表结果先按 auth_index 解析展示名，再组装前端需要的事件 payload。
func buildUsageEventsPayload(rows []servicedto.UsageEventRecord, resolver usageIdentityResolver, apiKeyAliases map[string]string) []usageEventPayload {
	if len(rows) == 0 {
		return []usageEventPayload{}
	}
	payload := make([]usageEventPayload, 0, len(rows))
	for _, row := range rows {
		identity, matched := resolver.resolveByAuthIndex(row.AuthIndex)
		source, isDelete := usageEventPublicSource(row, identity, matched)
		apiKeyIdentity := strings.TrimSpace(row.APIKeyIdentity)
		payload = append(payload, usageEventPayload{
			ID:            row.ID,
			Timestamp:     row.Timestamp.UTC().Format(time.RFC3339),
			Model:         row.Model,
			Source:        source,
			SourceType:    identity.Type,
			AuthIndex:     row.AuthIndex,
			APIKeyAlias:   strings.TrimSpace(apiKeyAliases[apiKeyIdentity]),
			APIKeyDisplay: usageEventAPIKeyDisplay(apiKeyIdentity),
			IsDelete:      isDelete,
			Failed:        row.Failed,
			LatencyMS:     row.LatencyMS,
			Tokens: usageEventTokenPayload{
				InputTokens:     row.InputTokens,
				OutputTokens:    row.OutputTokens,
				ReasoningTokens: row.ReasoningTokens,
				CachedTokens:    row.CachedTokens,
				TotalTokens:     row.TotalTokens,
			},
		})
	}
	return payload
}

func usageEventAPIKeyDisplay(identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	return redact.APIKeyDisplayName(identity)
}

func usageEventPublicSource(row servicedto.UsageEventRecord, identity resolvedUsageIdentity, matched bool) (string, bool) {
	if matched {
		return identity.DisplayName, false
	}
	isDelete := strings.TrimSpace(row.AuthIndex) != ""
	switch strings.TrimSpace(row.AuthType) {
	case "apikey":
		return strings.TrimSpace(row.Provider), isDelete
	case "oauth":
		return strings.TrimSpace(row.Source), isDelete
	default:
		return strings.TrimSpace(row.Provider), isDelete
	}
}

func loadUsageEventModelFilterOptions(c *gin.Context, usageProvider service.UsageProvider) ([]string, error) {
	if usageProvider == nil {
		return []string{}, nil
	}
	options, err := usageProvider.ListUsageEventFilterOptions(c.Request.Context(), servicedto.UsageFilter{})
	if err != nil {
		return nil, err
	}
	return options.Models, nil
}

func loadUsageEventSourceFilterOptions(c *gin.Context, usageIdentityProvider service.UsageIdentityProvider) ([]usageSourceFilterOption, error) {
	identities, err := loadUsageResolutionData(c, usageIdentityProvider)
	if err != nil {
		return nil, err
	}
	return buildUsageSourceFilterOptions(identities), nil
}

// Source 筛选项从活跃身份生成，避免把 usage_events.source 当成可选项暴露给页面。
func buildUsageSourceFilterOptions(identities []entities.UsageIdentity) []usageSourceFilterOption {
	if len(identities) == 0 {
		return []usageSourceFilterOption{}
	}
	options := make([]usageSourceFilterOption, 0, len(identities))
	seen := make(map[string]struct{}, len(identities))
	for _, identity := range identities {
		// Source 下拉只展示活跃且有流量的身份，避免已删除身份继续出现在筛选项里。
		if identity.IsDeleted || identity.TotalRequests == 0 {
			continue
		}
		option, ok := usageSourceFilterOptionFromIdentity(identity)
		if !ok {
			continue
		}
		if _, exists := seen[option.Value]; exists {
			continue
		}
		seen[option.Value] = struct{}{}
		options = append(options, option)
	}
	return options
}

func usageSourceFilterOptionFromIdentity(identity entities.UsageIdentity) (usageSourceFilterOption, bool) {
	switch identity.AuthType {
	case entities.UsageIdentityAuthTypeAuthFile, entities.UsageIdentityAuthTypeAIProvider:
		value := strings.TrimSpace(identity.Identity)
		if value == "" {
			return usageSourceFilterOption{}, false
		}
		label := strings.TrimSpace(identity.Name)
		displayName := usageIdentityDisplayName(identity)
		return usageSourceFilterOption{Value: value, Label: label, DisplayName: displayName}, true
	default:
		return usageSourceFilterOption{}, false
	}
}
