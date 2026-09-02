package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

var (
	// ErrIdentityNotAuthFile 表示目标身份不是 OAuth auth file，账户开关只适用于 auth file 身份。
	ErrIdentityNotAuthFile = errors.New("identity is not an auth-file account")
	// ErrAuthFileNotFoundInCPA 表示 CPA 侧已经不存在该 auth_index 对应的凭据。
	ErrAuthFileNotFoundInCPA = errors.New("auth file not found in CPA")
)

// AuthFileStatusClient 是账户启停对 CPA management API 的窄调用口：
// 先按 auth_index 解析出 CPA 要求的凭据 name，再提交 disabled 变更。
type AuthFileStatusClient interface {
	FetchAuthFileByAuthIndex(ctx context.Context, authIndex string) (authfiles.AuthFile, bool, error)
	SetAuthFileDisabled(ctx context.Context, name string, disabled bool) error
}

type AccountStatusService struct {
	db     *gorm.DB
	client AuthFileStatusClient
	now    func() time.Time
}

func NewAccountStatusService(db *gorm.DB, client AuthFileStatusClient) *AccountStatusService {
	return &AccountStatusService{db: db, client: client, now: time.Now}
}

// SetIdentityDisabled 启停 auth-file 账户：先校验本地身份，再向 CPA 提交变更，
// 成功后立即落库 disabled 标记；CPA 失败时本地状态保持不变。
func (s *AccountStatusService) SetIdentityDisabled(ctx context.Context, identityID uint, disabled bool) error {
	if s == nil || s.db == nil || s.client == nil {
		return fmt.Errorf("account status service is not configured")
	}
	identity, err := repository.GetUsageIdentityByID(ctx, s.db, identityID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrUsageIdentityMissing
		}
		return fmt.Errorf("load usage identity: %w", err)
	}
	if identity.AuthType != entities.UsageIdentityAuthTypeAuthFile {
		return ErrIdentityNotAuthFile
	}
	file, found, err := s.client.FetchAuthFileByAuthIndex(ctx, identity.Identity)
	if err != nil {
		return fmt.Errorf("resolve auth file name: %w", err)
	}
	if !found {
		return ErrAuthFileNotFoundInCPA
	}
	if err := s.client.SetAuthFileDisabled(ctx, file.Name, disabled); err != nil {
		return fmt.Errorf("set auth file disabled: %w", err)
	}
	if err := repository.SetUsageIdentityDisabled(ctx, s.db, identity.ID, disabled, s.now().UTC()); err != nil {
		return err
	}
	return nil
}
