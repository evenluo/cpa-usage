package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"cpa-usage/internal/cpa/dto/providerconfig"
	"cpa-usage/internal/cpa/dto/response"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"
	servicedto "cpa-usage/internal/service/dto"
	"gorm.io/gorm"
)

// providerMetadataFetchFunc 抓取单个 provider 的元数据并摊平为 usage identity 输入。
type providerMetadataFetchFunc func(ctx context.Context, fetcher MetadataFetcher) ([]servicedto.ProviderMetadataInput, error)

// ProviderMetadataAdapter 是一个 provider 的元数据接入适配器：providerType 决定身份类型，
// fetch 负责调用对应管理接口并把响应摊平成统一的 ProviderMetadataInput。
type ProviderMetadataAdapter struct {
	providerType string
	fetch        providerMetadataFetchFunc
}

// ProviderMetadataRegistry 按注册顺序执行各 provider 适配器，新增 provider 只需注册一个适配器。
type ProviderMetadataRegistry struct {
	adapters []ProviderMetadataAdapter
}

func NewDefaultProviderMetadataRegistry() ProviderMetadataRegistry {
	return NewProviderMetadataRegistry([]ProviderMetadataAdapter{
		newProviderKeyConfigAdapter("gemini", MetadataFetcher.FetchGeminiAPIKeys),
		newProviderKeyConfigAdapter("claude", MetadataFetcher.FetchClaudeAPIKeys),
		newProviderKeyConfigAdapter("codex", MetadataFetcher.FetchCodexAPIKeys),
		newProviderKeyConfigAdapter("vertex", MetadataFetcher.FetchVertexAPIKeys),
		newOpenAICompatibilityAdapter(),
	})
}

func NewProviderMetadataRegistry(adapters []ProviderMetadataAdapter) ProviderMetadataRegistry {
	registry := ProviderMetadataRegistry{adapters: make([]ProviderMetadataAdapter, 0, len(adapters))}
	for _, adapter := range adapters {
		if strings.TrimSpace(adapter.providerType) == "" || adapter.fetch == nil {
			continue
		}
		registry.adapters = append(registry.adapters, adapter)
	}
	return registry
}

// fetch 依次执行注册的适配器：失败的 provider 只记录错误，成功的 provider 类型仍然参与身份替换；
// 跨 provider 按 authIndex 去重，保留先注册适配器产出的身份。
func (r ProviderMetadataRegistry) fetch(ctx context.Context, fetcher MetadataFetcher) ([]servicedto.ProviderMetadataInput, []string, error) {
	inputs := make([]servicedto.ProviderMetadataInput, 0)
	fetchedProviderTypes := make([]string, 0, len(r.adapters))
	seen := make(map[string]struct{})
	var errs []error
	for _, adapter := range r.adapters {
		adapterInputs, err := adapter.fetch(ctx, fetcher)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		fetchedProviderTypes = append(fetchedProviderTypes, adapter.providerType)
		for _, input := range adapterInputs {
			if _, ok := seen[input.AuthIndex]; ok {
				continue
			}
			seen[input.AuthIndex] = struct{}{}
			inputs = append(inputs, input)
		}
	}
	return inputs, fetchedProviderTypes, joinErrors(errs...)
}

// newProviderKeyConfigAdapter 适配 gemini/claude/codex/vertex 这类返回 ProviderKeyConfig 列表的管理接口。
func newProviderKeyConfigAdapter(providerType string, fetch func(MetadataFetcher, context.Context) (*response.ProviderKeyConfigResult, error)) ProviderMetadataAdapter {
	return ProviderMetadataAdapter{
		providerType: providerType,
		fetch: func(ctx context.Context, fetcher MetadataFetcher) ([]servicedto.ProviderMetadataInput, error) {
			result, err := fetch(fetcher, ctx)
			if err != nil {
				return nil, fmt.Errorf("fetch %s api keys: %w", providerType, err)
			}
			if result == nil {
				return nil, fmt.Errorf("%s api keys response is nil", providerType)
			}
			return providerKeyConfigInputs(providerType, result.Payload), nil
		},
	}
}

// newOpenAICompatibilityAdapter 适配 OpenAI 兼容接口：每个 provider 配置携带多条 api key 条目，
// 条目缺失 authIndex 时回退用 api key 本身作为身份。
func newOpenAICompatibilityAdapter() ProviderMetadataAdapter {
	return ProviderMetadataAdapter{
		providerType: "openai",
		fetch: func(ctx context.Context, fetcher MetadataFetcher) ([]servicedto.ProviderMetadataInput, error) {
			result, err := fetcher.FetchOpenAICompatibility(ctx)
			if err != nil {
				return nil, fmt.Errorf("fetch openai compatibility: %w", err)
			}
			if result == nil {
				return nil, fmt.Errorf("openai compatibility response is nil")
			}
			inputs := make([]servicedto.ProviderMetadataInput, 0)
			for _, provider := range result.Payload {
				displayName := firstNonEmpty(provider.Name, "openai")
				for _, entry := range provider.APIKeyEntries {
					if input, ok := newProviderMetadataInput(entry.APIKey, provider.Prefix, "openai", displayName, firstNonEmpty(entry.AuthIndex, entry.APIKey), provider.BaseURL); ok {
						inputs = append(inputs, input)
					}
				}
			}
			return inputs, nil
		},
	}
}

func providerKeyConfigInputs(providerType string, configs []providerconfig.ProviderKeyConfig) []servicedto.ProviderMetadataInput {
	inputs := make([]servicedto.ProviderMetadataInput, 0, len(configs))
	for _, cfg := range configs {
		displayName := firstNonEmpty(cfg.Name, providerType)
		if input, ok := newProviderMetadataInput(cfg.APIKey, cfg.Prefix, providerType, displayName, cfg.AuthIndex, cfg.BaseURL); ok {
			inputs = append(inputs, input)
		}
	}
	return inputs
}

// Provider metadata 只生成 auth-index 身份；prefix 作为同一身份的附加字段保存，不再生成独立行。
func newProviderMetadataInput(lookupKey, prefix, providerType, displayName, authIndex, baseURL string) (servicedto.ProviderMetadataInput, bool) {
	input := servicedto.ProviderMetadataInput{
		LookupKey:    strings.TrimSpace(lookupKey),
		Prefix:       strings.TrimSpace(prefix),
		ProviderType: strings.TrimSpace(providerType),
		DisplayName:  strings.TrimSpace(displayName),
		AuthIndex:    strings.TrimSpace(authIndex),
		BaseURL:      strings.TrimSpace(baseURL),
	}
	if input.LookupKey == "" || input.ProviderType == "" || input.DisplayName == "" || input.AuthIndex == "" {
		return servicedto.ProviderMetadataInput{}, false
	}
	return input, true
}

func syncProviderMetadata(ctx context.Context, db *gorm.DB, inputs []servicedto.ProviderMetadataInput, fetchedProviderTypes []string, fetchErr error, now time.Time) (error, error) {
	if db == nil {
		return fmt.Errorf("database is nil"), nil
	}

	identities := providerMetadataUsageIdentities(inputs)
	if err := repository.ReplaceUsageIdentitiesForProviderTypes(ctx, db, identities, fetchedProviderTypes, now); err != nil {
		return fmt.Errorf("sync provider usage identities: %w", err), nil
	}
	if fetchErr != nil {
		return nil, fmt.Errorf("fetch provider metadata: %w", fetchErr)
	}
	return nil, nil
}

func providerMetadataUsageIdentities(inputs []servicedto.ProviderMetadataInput) []entities.UsageIdentity {
	identities := make([]entities.UsageIdentity, 0, len(inputs))
	for _, input := range inputs {
		identities = append(identities, entities.UsageIdentity{
			Name:         input.DisplayName,
			AuthType:     entities.UsageIdentityAuthTypeAIProvider,
			AuthTypeName: "apikey",
			Identity:     input.AuthIndex,
			Type:         input.ProviderType,
			Provider:     input.DisplayName,
			LookupKey:    input.LookupKey,
			Prefix:       input.Prefix,
			BaseURL:      input.BaseURL,
		})
	}
	return identities
}
