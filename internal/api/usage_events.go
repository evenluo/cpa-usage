package api

import (
	"net/http"
	"strings"
	"time"

	"cpa-usage/internal/entities"
	"cpa-usage/internal/redact"
	repodto "cpa-usage/internal/repository/dto"
	"cpa-usage/internal/service"

	"github.com/gin-gonic/gin"
)

type usageEventsResponse struct {
	Events     []usageEventPayload `json:"events"`
	WindowEnd  string              `json:"window_end,omitempty"`
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
	ID              uint                     `json:"id,omitempty"`
	Timestamp       string                   `json:"timestamp"`
	Model           string                   `json:"model"`
	ModelAlias      string                   `json:"model_alias,omitempty"`
	Endpoint        string                   `json:"endpoint,omitempty"`
	RequestID       string                   `json:"request_id,omitempty"`
	Source          string                   `json:"source"`
	SourceRaw       string                   `json:"source_raw,omitempty"`
	SourceType      string                   `json:"source_type,omitempty"`
	AuthIndex       string                   `json:"auth_index,omitempty"`
	APIKeyAlias     string                   `json:"api_key_alias,omitempty"`
	APIKeyDisplay   string                   `json:"api_key_display,omitempty"`
	IsDelete        bool                     `json:"isDelete,omitempty"`
	Failed          bool                     `json:"failed"`
	StatusCode      *int                     `json:"status_code,omitempty"`
	ExecutorType    string                   `json:"executor_type,omitempty"`
	ReasoningEffort string                   `json:"reasoning_effort,omitempty"`
	ServiceTier     string                   `json:"service_tier,omitempty"`
	LatencyMS       int64                    `json:"latency_ms"`
	TTFTMS          *int64                   `json:"ttft_ms"`
	OutputTPS       *float64                 `json:"output_tps"`
	AttemptFacts    usageAttemptFactsPayload `json:"attempt_facts"`
	Tokens          usageEventTokenPayload   `json:"tokens"`
}

type usageAttemptFactsPayload struct {
	Generate            *bool                         `json:"generate"`
	Stream              *bool                         `json:"stream"`
	RequestServiceTier  *string                       `json:"request_service_tier"`
	ResponseServiceTier *string                       `json:"response_service_tier"`
	OutputTPS           *float64                      `json:"output_tps"`
	Accounting          usageAttemptAccountingPayload `json:"accounting"`
}

type usageAttemptAccountingPayload struct {
	State              string                    `json:"state"`
	AccountingVersion  *int64                    `json:"accounting_version"`
	SchemaVersion      *int64                    `json:"schema_version"`
	Quality            *string                   `json:"quality"`
	TotalTokens        *int64                    `json:"total_tokens"`
	Input              usageAttemptInputPayload  `json:"input"`
	Output             usageAttemptOutputPayload `json:"output"`
	UnclassifiedTokens *int64                    `json:"unclassified_tokens"`
}

type usageAttemptInputPayload struct {
	TotalTokens      *int64 `json:"total_tokens"`
	UncachedTokens   *int64 `json:"uncached_tokens"`
	CacheReadTokens  *int64 `json:"cache_read_tokens"`
	CacheWriteTokens *int64 `json:"cache_write_tokens"`
}

type usageAttemptOutputPayload struct {
	TotalTokens        *int64 `json:"total_tokens"`
	NonReasoningTokens *int64 `json:"non_reasoning_tokens"`
	ReasoningTokens    *int64 `json:"reasoning_tokens"`
}

type usageEventTokenPayload struct {
	InputTokens         int64  `json:"input_tokens"`
	OutputTokens        int64  `json:"output_tokens"`
	ReasoningTokens     int64  `json:"reasoning_tokens"`
	CachedTokens        int64  `json:"cached_tokens"`
	CacheReadTokens     *int64 `json:"cache_read_tokens,omitempty"`
	CacheCreationTokens *int64 `json:"cache_creation_tokens,omitempty"`
	TotalTokens         int64  `json:"total_tokens"`
}

func registerUsageEventsRoute(
	router gin.IRoutes,
	usageProvider UsageProvider,
	usageIdentityProvider UsageIdentityProvider,
	keyAliasProvider service.KeyAliasProvider,
) {
	router.GET("/usage/events/filters/models", func(c *gin.Context) {
		filter, err := parseUsageTimeFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		models, err := loadUsageEventModelFilterOptions(c, usageProvider, filter.repositoryScope())
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
		filter, err := parseUsageEventListFilterQuery(c.Request, time.Now().UTC())
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		windowEnd := usageEventResponseWindowEnd(filter)
		if usageProvider == nil {
			page, totalPages := paginationMetadata(0, 1, repodto.DefaultUsageEventsLimit)
			c.JSON(http.StatusOK, usageEventsResponse{Events: []usageEventPayload{}, WindowEnd: windowEnd, Page: page, PageSize: repodto.DefaultUsageEventsLimit, TotalPages: totalPages})
			return
		}
		if err := applyUsageEventsSourceFilter(&filter); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		rows, err := usageProvider.ListUsageEvents(c.Request.Context(), filter.repositoryFilter())
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
		page, totalPages := paginationMetadata(rows.TotalCount, rows.Page, rows.PageSize)
		c.JSON(http.StatusOK, usageEventsResponse{
			Events:     buildUsageEventsPayload(rows.Events, resolver, apiKeyAliases),
			WindowEnd:  windowEnd,
			TotalCount: rows.TotalCount,
			Page:       page,
			PageSize:   rows.PageSize,
			TotalPages: totalPages,
		})
	})
}

func usageEventResponseWindowEnd(filter usageEventListFilter) string {
	if filter.EndTime == nil {
		return ""
	}
	return filter.EndTime.UTC().Format(time.RFC3339Nano)
}

func loadUsageEventAPIKeyAliases(c *gin.Context, keyAliasProvider service.KeyAliasProvider, rows []repodto.UsageEventRecord) (map[string]string, error) {
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
func applyUsageEventsSourceFilter(filter *usageEventListFilter) error {
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
func buildUsageEventsPayload(rows []repodto.UsageEventRecord, resolver usageIdentityResolver, apiKeyAliases map[string]string) []usageEventPayload {
	if len(rows) == 0 {
		return []usageEventPayload{}
	}
	payload := make([]usageEventPayload, 0, len(rows))
	for _, row := range rows {
		identity, matched := resolver.resolveByAuthIndex(row.AuthIndex)
		source, isDelete := usageEventPublicSource(row, identity, matched)
		apiKeyIdentity := strings.TrimSpace(row.APIKeyIdentity)
		payload = append(payload, usageEventPayload{
			ID:              row.ID,
			Timestamp:       row.Timestamp.UTC().Format(time.RFC3339),
			Model:           row.Model,
			ModelAlias:      row.ModelAlias,
			Endpoint:        usageEventPublicEndpoint(row.Endpoint),
			RequestID:       row.RequestID,
			Source:          source,
			SourceType:      identity.Type,
			AuthIndex:       row.AuthIndex,
			APIKeyAlias:     strings.TrimSpace(apiKeyAliases[apiKeyIdentity]),
			APIKeyDisplay:   usageEventAPIKeyDisplay(apiKeyIdentity),
			IsDelete:        isDelete,
			Failed:          row.Failed,
			StatusCode:      row.StatusCode,
			ExecutorType:    row.ExecutorType,
			ReasoningEffort: row.ReasoningEffort,
			ServiceTier:     row.ServiceTier,
			LatencyMS:       row.LatencyMS,
			TTFTMS:          row.TTFTMS,
			OutputTPS:       row.AttemptFacts.OutputTPS,
			AttemptFacts:    mapUsageAttemptFactsPayload(row.AttemptFacts),
			Tokens: usageEventTokenPayload{
				InputTokens:         row.InputTokens,
				OutputTokens:        row.OutputTokens,
				ReasoningTokens:     row.ReasoningTokens,
				CachedTokens:        row.CachedTokens,
				CacheReadTokens:     row.CacheReadTokens,
				CacheCreationTokens: row.CacheCreationTokens,
				TotalTokens:         row.TotalTokens,
			},
		})
	}
	return payload
}

func mapUsageAttemptFactsPayload(facts repodto.UsageAttemptFacts) usageAttemptFactsPayload {
	return usageAttemptFactsPayload{
		Generate:            facts.Generate,
		Stream:              facts.Stream,
		RequestServiceTier:  facts.RequestServiceTier,
		ResponseServiceTier: facts.ResponseServiceTier,
		OutputTPS:           facts.OutputTPS,
		Accounting: usageAttemptAccountingPayload{
			State:             facts.Accounting.State,
			AccountingVersion: facts.Accounting.AccountingVersion,
			SchemaVersion:     facts.Accounting.SchemaVersion,
			Quality:           facts.Accounting.Quality,
			TotalTokens:       facts.Accounting.TotalTokens,
			Input: usageAttemptInputPayload{
				TotalTokens:      facts.Accounting.Input.TotalTokens,
				UncachedTokens:   facts.Accounting.Input.UncachedTokens,
				CacheReadTokens:  facts.Accounting.Input.CacheReadTokens,
				CacheWriteTokens: facts.Accounting.Input.CacheWriteTokens,
			},
			Output: usageAttemptOutputPayload{
				TotalTokens:        facts.Accounting.Output.TotalTokens,
				NonReasoningTokens: facts.Accounting.Output.NonReasoningTokens,
				ReasoningTokens:    facts.Accounting.Output.ReasoningTokens,
			},
			UnclassifiedTokens: facts.Accounting.UnclassifiedTokens,
		},
	}
}

func usageEventPublicEndpoint(endpoint string) string {
	endpoint, _, _ = strings.Cut(strings.TrimSpace(endpoint), "?")
	endpoint, _, _ = strings.Cut(endpoint, "#")
	return endpoint
}

func usageEventAPIKeyDisplay(identity string) string {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		return ""
	}
	return redact.APIKeyDisplayName(identity)
}

func usageEventPublicSource(row repodto.UsageEventRecord, identity resolvedUsageIdentity, matched bool) (string, bool) {
	if matched {
		return identity.DisplayName, false
	}
	isDelete := strings.TrimSpace(row.AuthIndex) != ""
	authType, ok := entities.ParseUsageIdentityAuthType(row.AuthType)
	if !ok {
		// Preserve the existing request-evidence projection for unknown CPA
		// transport values without guessing a UsageIdentityAuthType.
		return strings.TrimSpace(row.Provider), isDelete
	}
	switch authType {
	case entities.UsageIdentityAuthTypeAIProvider:
		return strings.TrimSpace(row.Provider), isDelete
	case entities.UsageIdentityAuthTypeAuthFile:
		return strings.TrimSpace(row.Source), isDelete
	default:
		return strings.TrimSpace(row.Provider), isDelete
	}
}

func loadUsageEventModelFilterOptions(c *gin.Context, usageProvider UsageProvider, scope repodto.UsageTimeScope) ([]string, error) {
	if usageProvider == nil {
		return []string{}, nil
	}
	options, err := usageProvider.ListUsageEventFilterOptions(c.Request.Context(), scope)
	if err != nil {
		return nil, err
	}
	return options.Models, nil
}

func loadUsageEventSourceFilterOptions(c *gin.Context, usageIdentityProvider UsageIdentityProvider) ([]usageSourceFilterOption, error) {
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
		displayName := identity.DisplayName()
		return usageSourceFilterOption{Value: value, Label: label, DisplayName: displayName}, true
	default:
		return usageSourceFilterOption{}, false
	}
}
