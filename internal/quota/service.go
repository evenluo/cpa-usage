package quota

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	defaultRefreshWorkerLimit = 5
	defaultRefreshTaskTimeout = 20 * time.Second
)

// Service 只做两件事：按身份分发 provider quota 调用（Check），
// 以及把异步刷新编排委托给 refreshTaskStore（Refresh/GetRefreshTask/GetCachedQuota）。
type Service struct {
	identityLookup AuthFileIdentityLookup
	registry       ProviderRegistry
	refreshTasks   *refreshTaskStore

	refreshWorkerMu     sync.Mutex
	refreshWorkerCtx    context.Context
	refreshWorkerCancel context.CancelFunc
	refreshWorkersClose bool
	refreshWorkerWG     sync.WaitGroup
	refreshWorkerTokens chan struct{}
}

type CheckRequest struct {
	AuthIndex string `json:"auth_index"`
}

type CheckResponse struct {
	ID    string     `json:"id"`
	Quota []QuotaRow `json:"quota"`
}

func NewService(identityLookup AuthFileIdentityLookup, caller ManagementAPICaller) *Service {
	return NewServiceWithRegistry(identityLookup, NewDefaultProviderRegistry(caller, DefaultProviderConfigs()))
}

func NewServiceWithRegistry(identityLookup AuthFileIdentityLookup, registry ProviderRegistry) *Service {
	return &Service{
		identityLookup: identityLookup,
		registry:       registry,
		refreshTasks:   newRefreshTaskStore(defaultRefreshTaskTTL),
		// worker 默认脱离应用生命周期；应用启动时通过 AttachRefreshWorkerLifecycle 绑定。
		refreshWorkerCtx:    context.Background(),
		refreshWorkerTokens: make(chan struct{}, defaultRefreshWorkerLimit),
	}
}

func (s *Service) Check(ctx context.Context, request CheckRequest) (CheckResponse, error) {
	// 单条查询以 auth_index 为唯一入口，前端不需要知道具体 provider 的 API 细节。
	authIndex := strings.TrimSpace(request.AuthIndex)
	if authIndex == "" {
		return CheckResponse{}, fmt.Errorf("%w: auth_index is required", ErrValidation)
	}
	// 只允许 auth files 身份查询限额，AI provider 身份不进入 provider 调用链路。
	identity, found, err := s.identityLookup.FindActiveAuthFileIdentity(ctx, authIndex)
	if err != nil {
		return CheckResponse{}, err
	}
	if !found {
		return CheckResponse{}, fmt.Errorf("%w: %s", ErrNotFound, authIndex)
	}
	// 按相邻项目规则先匹配 provider 再匹配 type，解析出实际要调用的 quota handler。
	_, handler, ok := s.resolveQuotaHandler(identity.Provider, identity.Type)
	if !ok {
		return CheckResponse{}, fmt.Errorf("%w: %s", ErrUnsupportedType, normalizeIdentityType(identity.Provider))
	}
	// provider 返回各自原始结构后，再统一转换为前端可复用的 quota rows。
	providerOutput, err := handler.Check(ctx, ProviderInput{Identity: identity})
	if err != nil {
		return CheckResponse{}, err
	}
	return CheckResponse{
		ID:    authIndex,
		Quota: NormalizeQuotaRows(providerOutput),
	}, nil
}

func (s *Service) resolveQuotaHandler(provider string, identityType string) (string, ProviderHandler, bool) {
	for _, candidate := range resolveQuotaIdentityTypes(provider, identityType) {
		if handler, ok := s.registry.Provider(candidate); ok {
			return candidate, handler, true
		}
	}
	return "", nil, false
}

func resolveQuotaIdentityTypes(provider string, identityType string) []string {
	candidates := make([]string, 0, 2)
	for _, value := range []string{provider, identityType} {
		normalized := normalizeIdentityType(value)
		if normalized == "" || slices.Contains(candidates, normalized) {
			continue
		}
		candidates = append(candidates, normalized)
	}
	return candidates
}
