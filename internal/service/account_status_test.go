package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"cpa-usage/internal/cpa/dto/authfiles"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/repository"

	"gorm.io/gorm"
)

type stubAuthFileStatusClient struct {
	file            authfiles.AuthFile
	found           bool
	fetchErr        error
	setErr          error
	gotName         string
	gotDisabled     bool
	setCallCount    int
	afterSetFile    *authfiles.AuthFile
	afterSetErr     error
	afterSetMissing bool
}

func (s *stubAuthFileStatusClient) FetchAuthFileByAuthIndex(_ context.Context, _ string) (authfiles.AuthFile, bool, error) {
	if s.setCallCount > 0 {
		if s.afterSetErr != nil || s.afterSetMissing {
			return authfiles.AuthFile{}, false, s.afterSetErr
		}
		if s.afterSetFile != nil {
			return *s.afterSetFile, true, nil
		}
	}
	return s.file, s.found, s.fetchErr
}

func (s *stubAuthFileStatusClient) SetAuthFileDisabled(_ context.Context, name string, disabled bool) error {
	s.gotName = name
	s.gotDisabled = disabled
	s.setCallCount++
	if s.setErr == nil {
		s.file.Disabled = disabled
	}
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

func TestSetIdentityDisabledReadsBackCPAState(t *testing.T) {
	for _, target := range []bool{false, true} {
		for _, reported := range []string{"active", "disabled", "error", ""} {
			t.Run(fmt.Sprintf("disabled=%v/status=%s", target, reported), func(t *testing.T) {
				db := openSyncTestDatabase(t)
				before := "disabled"
				unavailable := true
				previousTime := time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)
				identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{Name: "account", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth", Type: "codex", Disabled: !target, AuthFileStatus: &before, Unavailable: &unavailable, LastRefresh: &previousTime, NextRetryAfter: &previousTime, TotalRequests: 42})
				after := authfiles.AuthFile{Name: "account.json", AuthIndex: "auth", Disabled: target}
				if reported != "" {
					after.Status = &reported
				}
				client := &stubAuthFileStatusClient{file: authfiles.AuthFile{Name: "account.json", AuthIndex: "auth"}, found: true, afterSetFile: &after}
				svc := NewAccountStatusService(db, client)
				now := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
				svc.now = func() time.Time { return now }
				if err := svc.SetIdentityDisabled(context.Background(), identity.ID, target); err != nil {
					t.Fatal(err)
				}
				got, err := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
				if err != nil {
					t.Fatal(err)
				}
				if got.Disabled != target || (reported == "" && got.AuthFileStatus != nil) || (reported != "" && (got.AuthFileStatus == nil || *got.AuthFileStatus != reported)) {
					t.Fatalf("stale state: disabled=%v status=%v; want %v/%q", got.Disabled, got.AuthFileStatus, target, reported)
				}
				if got.Unavailable != nil || got.LastRefresh != nil || got.NextRetryAfter != nil || got.MetadataObservedAt == nil || !got.MetadataObservedAt.Equal(now) || got.TotalRequests != 42 {
					t.Fatalf("readback did not replace observations or preserve usage: %+v", got)
				}
			})
		}
	}
}

func TestSetIdentityDisabledReportsReadbackFailureAfterAcceptedChange(t *testing.T) {
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing=%v", missing), func(t *testing.T) {
			db := openSyncTestDatabase(t)
			status := "disabled"
			identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{Name: "account", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth", Disabled: true, AuthFileStatus: &status})
			client := &stubAuthFileStatusClient{file: authfiles.AuthFile{Name: "account.json", AuthIndex: "auth"}, found: true, afterSetMissing: missing}
			if !missing {
				client.afterSetErr = errors.New("read failed")
			}
			err := NewAccountStatusService(db, client).SetIdentityDisabled(context.Background(), identity.ID, false)
			if !errors.Is(err, ErrAccountStatusRefresh) {
				t.Fatalf("expected explicit partial-success error, got %v", err)
			}
			got, err := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
			if err != nil {
				t.Fatal(err)
			}
			if got.Disabled || got.AuthFileStatus == nil || *got.AuthFileStatus != "disabled" || got.MetadataObservedAt != nil || client.setCallCount != 1 {
				t.Fatalf("must preserve accepted flag and prior observations without retry: %+v", got)
			}
		})
	}
}

func TestSetIdentityDisabledPreservesConflictingReadback(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{Name: "account", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth", Disabled: true})
	after := authfiles.AuthFile{Name: "account.json", AuthIndex: "auth", Disabled: true}
	client := &stubAuthFileStatusClient{file: after, found: true, afterSetFile: &after}
	err := NewAccountStatusService(db, client).SetIdentityDisabled(context.Background(), identity.ID, false)
	if !errors.Is(err, ErrAccountStatusRefresh) {
		t.Fatalf("expected unconfirmed result, got %v", err)
	}
	got, err := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Disabled || client.setCallCount != 1 {
		t.Fatal("must keep readback authority without replaying mutation")
	}
}

func TestSetIdentityDisabledPersistsReportedAvailability(t *testing.T) {
	db := openSyncTestDatabase(t)
	identity := seedAuthFileIdentity(t, db, entities.UsageIdentity{Name: "account", AuthType: entities.UsageIdentityAuthTypeAuthFile, Identity: "auth", Disabled: true})
	status := " ERROR "
	unavailable := true
	refreshed := time.Date(2026, 9, 8, 10, 0, 0, 0, time.UTC)
	retry := refreshed.Add(time.Hour)
	after := authfiles.AuthFile{Name: "account.json", AuthIndex: "auth", Status: &status, Unavailable: &unavailable, LastRefresh: &refreshed, NextRetryAfter: &retry}
	client := &stubAuthFileStatusClient{file: after, found: true, afterSetFile: &after}
	if err := NewAccountStatusService(db, client).SetIdentityDisabled(context.Background(), identity.ID, false); err != nil {
		t.Fatal(err)
	}
	got, err := repository.GetUsageIdentityByID(context.Background(), db, identity.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Disabled || got.AuthFileStatus == nil || *got.AuthFileStatus != "error" || got.Unavailable == nil || !*got.Unavailable || got.LastRefresh == nil || !got.LastRefresh.Equal(refreshed) || got.NextRetryAfter == nil || !got.NextRetryAfter.Equal(retry) {
		t.Fatalf("must preserve actual CPA observations: %+v", got)
	}
}
