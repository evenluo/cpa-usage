package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

type stubAuthFileStatusClient struct {
	file         authfiles.AuthFile
	found        bool
	fetchErr     error
	setErr       error
	gotName      string
	gotDisabled  bool
	setCallCount int
}

func (s *stubAuthFileStatusClient) FetchAuthFileByAuthIndex(_ context.Context, _ string) (authfiles.AuthFile, bool, error) {
	return s.file, s.found, s.fetchErr
}

func (s *stubAuthFileStatusClient) SetAuthFileDisabled(_ context.Context, name string, disabled bool) error {
	s.gotName = name
	s.gotDisabled = disabled
	s.setCallCount++
	return s.setErr
}

func seedAuthFileIdentity(t *testing.T, db *gorm.DB, identity entities.UsageIdentity) entities.UsageIdentity {
	t.Helper()
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("seed usage identity: %v", err)
	}
	return identity
}

func TestSetIdentityDisabledUpdatesCPAAndLocalRow(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{
		Name:         "Codex Account",
		AuthType:     entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName: "oauth",
		Identity:     "auth-codex",
		Type:         "codex",
		Provider:     "Codex",
	})
	client := &stubAuthFileStatusClient{
		file:  authfiles.AuthFile{AuthIndex: "auth-codex", Name: "codex-user.json"},
		found: true,
	}
	service := NewAccountStatusService(db, client)
	service.now = func() time.Time { return time.Date(2026, 9, 2, 10, 0, 0, 0, time.UTC) }

	if err := service.SetIdentityDisabled(context.Background(), identity.ID, true); err != nil {
		t.Fatalf("SetIdentityDisabled returned error: %v", err)
	}
	if client.setCallCount != 1 || client.gotName != "codex-user.json" || !client.gotDisabled {
		t.Fatalf("unexpected CPA call: count=%d name=%q disabled=%v", client.setCallCount, client.gotName, client.gotDisabled)
	}

	stored, err := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
	if err != nil {
		t.Fatalf("reload usage identity: %v", err)
	}
	if !stored.Disabled {
		t.Fatalf("expected local identity to be marked disabled, got %+v", stored)
	}
}

func TestSetIdentityDisabledRejectsMissingIdentity(t *testing.T) {
	db := openSyncTestDatabase(t)
	service := NewAccountStatusService(db, &stubAuthFileStatusClient{})

	err := service.SetIdentityDisabled(context.Background(), 999, true)
	if !errors.Is(err, ErrUsageIdentityMissing) {
		t.Fatalf("expected ErrUsageIdentityMissing, got %v", err)
	}
}

func TestSetIdentityDisabledRejectsAPIKeyIdentity(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{
		Name:         "API Key",
		AuthType:     entities.UsageIdentityAuthTypeAIProvider,
		AuthTypeName: "apikey",
		Identity:     "api-key-1",
		Type:         "openai",
		Provider:     "openai",
	})
	service := NewAccountStatusService(db, &stubAuthFileStatusClient{})

	err := service.SetIdentityDisabled(context.Background(), identity.ID, true)
	if !errors.Is(err, ErrIdentityNotAuthFile) {
		t.Fatalf("expected ErrIdentityNotAuthFile, got %v", err)
	}
}

func TestSetIdentityDisabledReportsAuthFileMissingInCPA(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{
		Name:         "Codex Account",
		AuthType:     entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName: "oauth",
		Identity:     "auth-codex",
		Type:         "codex",
		Provider:     "Codex",
	})
	service := NewAccountStatusService(db, &stubAuthFileStatusClient{found: false})

	err := service.SetIdentityDisabled(context.Background(), identity.ID, true)
	if !errors.Is(err, ErrAuthFileNotFoundInCPA) {
		t.Fatalf("expected ErrAuthFileNotFoundInCPA, got %v", err)
	}
}

func TestSetIdentityDisabledKeepsLocalStateWhenCPAFails(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{
		Name:         "Codex Account",
		AuthType:     entities.UsageIdentityAuthTypeAuthFile,
		AuthTypeName: "oauth",
		Identity:     "auth-codex",
		Type:         "codex",
		Provider:     "Codex",
	})
	client := &stubAuthFileStatusClient{
		file:   authfiles.AuthFile{AuthIndex: "auth-codex", Name: "codex-user.json"},
		found:  true,
		setErr: errors.New("cpa unavailable"),
	}
	service := NewAccountStatusService(db, client)

	err := service.SetIdentityDisabled(context.Background(), identity.ID, true)
	if err == nil {
		t.Fatal("expected CPA failure error, got nil")
	}
	stored, loadErr := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
	if loadErr != nil {
		t.Fatalf("reload usage identity: %v", loadErr)
	}
	if stored.Disabled {
		t.Fatalf("expected local identity to stay enabled after CPA failure, got %+v", stored)
	}
}
