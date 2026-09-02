package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/cpa"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"cpa-usage/internal/cpa/dto/authfiles"
	"cpa-usage/internal/cpa/dto/response"
	servicedto "cpa-usage/internal/service/dto"
	"gorm.io/gorm"
)

type MetadataFetcher interface {
	FetchAuthFiles(ctx context.Context) (*response.AuthFilesResult, error)
	FetchGeminiAPIKeys(ctx context.Context) (*response.ProviderKeyConfigResult, error)
	FetchClaudeAPIKeys(ctx context.Context) (*response.ProviderKeyConfigResult, error)
	FetchCodexAPIKeys(ctx context.Context) (*response.ProviderKeyConfigResult, error)
	FetchVertexAPIKeys(ctx context.Context) (*response.ProviderKeyConfigResult, error)
	FetchOpenAICompatibility(ctx context.Context) (*response.OpenAICompatibilityResult, error)
}

type CPAClientFetcher interface {
	MetadataFetcher
}

const (
	syncMetadataOptional = false
	syncMetadataRequired = true
)

type SyncService struct {
	db               *gorm.DB
	client           CPAClientFetcher
	redisQueue       RedisQueue
	redisQueueKey    string
	metadataFetcher  MetadataFetcher
	providerMetadata ProviderMetadataRegistry
	intake           redisUsageIntake
	baseURL          string
	now              func() time.Time
}

func NewSyncService(db *gorm.DB, cfg config.Config) *SyncService {
	return NewSyncServiceWithOptions(db, SyncServiceOptions{
		BaseURL: cfg.CPABaseURL,
		Client:  cpa.NewClient(cfg.CPABaseURL, cfg.CPAManagementKey, cfg.RequestTimeout, cfg.TLSSkipVerify),
		RedisQueue: cpa.NewRedisQueueClientWithOptions(cpa.RedisQueueOptions{
			BaseURL:       cfg.CPABaseURL,
			RedisAddr:     cfg.RedisQueueAddr,
			ManagementKey: cfg.CPAManagementKey,
			Timeout:       cfg.RequestTimeout,
			QueueKey:      cfg.RedisQueueKey,
			BatchSize:     cfg.RedisQueueBatchSize,
			TLS:           cfg.RedisQueueTLS,
			TLSSkipVerify: cfg.TLSSkipVerify,
		}),
		RedisQueueKey: cfg.RedisQueueKey,
	})
}

type SyncServiceOptions struct {
	BaseURL          string
	Client           CPAClientFetcher
	MetadataFetcher  MetadataFetcher
	ProviderMetadata ProviderMetadataRegistry
	RedisQueue       RedisQueue
	RedisQueueKey    string
	Now              func() time.Time
}

func NewSyncServiceWithOptions(db *gorm.DB, opts SyncServiceOptions) *SyncService {
	now := opts.Now
	if now == nil {
		now = time.Now
	}
	metadataFetcher := opts.MetadataFetcher
	if metadataFetcher == nil {
		metadataFetcher = opts.Client
	}
	providerMetadata := opts.ProviderMetadata
	if len(providerMetadata.adapters) == 0 {
		providerMetadata = NewDefaultProviderMetadataRegistry()
	}
	redisQueueKey := redisQueueKey(opts.RedisQueueKey)
	return &SyncService{
		db:               db,
		client:           opts.Client,
		redisQueue:       opts.RedisQueue,
		redisQueueKey:    redisQueueKey,
		metadataFetcher:  metadataFetcher,
		providerMetadata: providerMetadata,
		intake:           newRedisUsageIntake(db, opts.RedisQueue, redisQueueKey, now),
		baseURL:          strings.TrimSpace(opts.BaseURL),
		now:              now,
	}
}

func (s *SyncService) SyncMetadata(ctx context.Context) error {
	if err := s.validate(syncMetadataRequired); err != nil {
		return err
	}
	slog.Debug("metadata sync started")
	fetchedAt := s.now().UTC()
	authFilesResult, authFilesErr := s.metadataFetcher.FetchAuthFiles(ctx)
	providerInputs, fetchedProviderTypes, providerMetadataErr := s.providerMetadata.fetch(ctx, s.metadataFetcher)
	authSyncErr := syncAuthFiles(ctx, s.db, authFilesResult, authFilesErr, fetchedAt)
	providerSyncErr, providerWarningErr := syncProviderMetadata(ctx, s.db, providerInputs, fetchedProviderTypes, providerMetadataErr, fetchedAt)
	upsertErr := joinErrors(authSyncErr, providerSyncErr)
	var aggregateErr error
	if upsertErr == nil {
		aggregateErr = repository.AggregateUsageIdentityStats(ctx, s.db, fetchedAt)
		if aggregateErr != nil {
			aggregateErr = fmt.Errorf("aggregate usage identity stats: %w", aggregateErr)
		}
	}
	err := joinErrors(upsertErr, aggregateErr, providerWarningErr)
	status := "completed"
	if err != nil {
		status = "completed_with_warnings"
		slog.Debug("metadata sync finished", "status", status, "error", err.Error())
		return err
	}
	slog.Debug("metadata sync finished", "status", status)
	return nil
}

// PullRedisUsageInbox 是 Redis 同步的拉取阶段：LPOP 队列消息并把 replay-safe 投影写入 redis_usage_inboxes。
// 这个阶段不构建或写入 usage_events，保证 Redis 消费和本地处理职责分离。
func (s *SyncService) PullRedisUsageInbox(ctx context.Context) (*servicedto.RedisInboxPullResult, error) {
	if err := s.validate(syncMetadataOptional); err != nil {
		return nil, err
	}
	return s.intake.pull(ctx)
}

// ProcessRedisUsageInbox 是 Redis 同步的本地处理阶段：只读取 pending/process_failed inbox 行并写入 usage_events。
// 成功处理后仅用 usage_event_key 记录 inbox 与最终事件的关联。
func (s *SyncService) ProcessRedisUsageInbox(ctx context.Context) (*servicedto.RedisBatchSyncResult, error) {
	if err := s.validate(syncMetadataOptional); err != nil {
		return nil, err
	}
	return s.intake.process(ctx)
}

// CleanupStorage 是每日 03:00 维护任务调用的统一入口：先清 Redis inbox，最后 VACUUM 收缩 SQLite。
func (s *SyncService) CleanupStorage(ctx context.Context) error {
	if err := s.validate(syncMetadataOptional); err != nil {
		return err
	}
	_, err := repository.CleanupStorage(s.db, s.now())
	return err
}

func (s *SyncService) validate(syncMetadata bool) error {
	if s == nil {
		return fmt.Errorf("sync service is nil")
	}
	if s.db == nil {
		return fmt.Errorf("sync service database is nil")
	}
	if syncMetadata && s.metadataFetcher == nil {
		return fmt.Errorf("sync service metadata fetcher is nil")
	}
	return nil
}

func redisQueueKey(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return cpa.ManagementUsageQueueKey
	}
	return trimmed
}

func syncAuthFiles(ctx context.Context, db *gorm.DB, result *response.AuthFilesResult, fetchErr error, now time.Time) error {
	if fetchErr != nil {
		return fmt.Errorf("fetch auth files: %w", fetchErr)
	}
	if db == nil {
		return fmt.Errorf("database is nil")
	}
	if result == nil {
		return fmt.Errorf("fetch auth files: empty response")
	}

	identities := make([]entities.UsageIdentity, 0, len(result.Payload.Files))
	for _, file := range result.Payload.Files {
		if authFileInactive(file) {
			continue
		}
		identities = append(identities, authFileUsageIdentity(file))
	}
	if err := repository.ReplaceUsageIdentitiesForAuthType(ctx, db, identities, entities.UsageIdentityAuthTypeAuthFile, now); err != nil {
		return fmt.Errorf("sync auth file usage identities: %w", err)
	}
	return nil
}

// authFileInactive 只丢弃 CPA 侧已不可恢复的凭据；disabled 账户仍同步为带 Disabled 标记的身份，
// 保证看板可以展示禁用状态并支持重新启用。
func authFileInactive(file authfiles.AuthFile) bool {
	if file.Unavailable {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(file.Status)) {
	case "deleted", "removed", "unavailable", "inactive", "revoked":
		return true
	default:
		return false
	}
}

type authFileUsageIdentityExtension func(authfiles.AuthFile, *entities.UsageIdentity)

var authFileUsageIdentityExtensions = map[string]authFileUsageIdentityExtension{
	"codex": extendCodexAuthFileUsageIdentity,
}

// auth_files 先走通用身份映射，再按 type 追加各来源特有字段，方便后续扩展新类型。
func authFileUsageIdentity(file authfiles.AuthFile) entities.UsageIdentity {
	identity := baseAuthFileUsageIdentity(file)
	if extend, ok := authFileUsageIdentityExtensions[strings.ToLower(strings.TrimSpace(file.Type))]; ok {
		extend(file, &identity)
	}
	identity.ProjectID = resolveAuthFileProjectID(file)
	return identity
}

func baseAuthFileUsageIdentity(file authfiles.AuthFile) entities.UsageIdentity {
	authTypeName, _ := entities.UsageIdentityAuthTypeAuthFile.CanonicalName()
	return entities.UsageIdentity{
		Name:         firstNonEmpty(file.Email, file.Label, file.Name, file.AuthIndex),
		AuthType:     entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName: authTypeName,
		Identity:     file.AuthIndex,
		Type:         file.Type,
		Provider:     file.Provider,
		Disabled:     file.Disabled,
	}
}

// Codex 的 ChatGPT id_token 字段只在 type=codex 且字段存在时写入；缺失字段保持 nil，入库后就是 NULL。
func extendCodexAuthFileUsageIdentity(file authfiles.AuthFile, identity *entities.UsageIdentity) {
	identity.AccountID = resolveCodexAccountID(file)
	identity.ActiveStart = resolveCodexActiveStart(file)
	identity.ActiveUntil = resolveCodexActiveUntil(file)
	identity.PlanType = resolveCodexPlanType(file)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func joinErrors(errs ...error) error {
	messages := make([]string, 0, len(errs))
	for _, err := range errs {
		if err == nil {
			continue
		}
		messages = append(messages, strings.TrimSpace(err.Error()))
	}
	if len(messages) == 0 {
		return nil
	}
	return fmt.Errorf("%s", strings.Join(messages, "; "))
}
