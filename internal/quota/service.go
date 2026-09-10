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

// Service dispatches provider quota checks, persists successful observations,
// and delegates asynchronous refresh task state to refreshTaskStore.
type Service struct {
	repository   Repository
	registry     ProviderRegistry
	refreshTasks *refreshTaskStore
	now          func() time.Time

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
	ID         string     `json:"id"`
	ObservedAt time.Time  `json:"observedAt"`
	Quota      []QuotaRow `json:"quota"`
}

func NewService(repository Repository, caller ManagementAPICaller) *Service {
	return NewServiceWithRegistry(repository, NewDefaultProviderRegistry(caller, DefaultProviderConfigs()))
}

func NewServiceWithRegistry(repository Repository, registry ProviderRegistry) *Service {
	return &Service{
		repository:   repository,
		registry:     registry,
		refreshTasks: newRefreshTaskStore(defaultRefreshTaskTTL),
		now:          time.Now,
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
	identity, found, err := s.repository.FindActiveAuthFileIdentity(ctx, authIndex)
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
	rows := NormalizeQuotaRows(providerOutput)
	if len(rows) == 0 {
		return CheckResponse{}, fmt.Errorf("quota response has no usable rows")
	}
	response := CheckResponse{
		ID:         authIndex,
		ObservedAt: s.now().UTC(),
		Quota:      rows,
	}
	if err := s.repository.SaveQuotaObservation(ctx, identity.ID, response); err != nil {
		return CheckResponse{}, fmt.Errorf("persist quota observation: %w", err)
	}
	return response, nil
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
