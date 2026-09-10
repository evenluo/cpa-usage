package quota

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	dbrepository "cpa-usage/internal/repository"
	"gorm.io/gorm"
)

func TestQuotaObservationSurvivesDatabaseReopen(t *testing.T) {
	ctx := context.Background()
	dbPath := filepath.Join(t.TempDir(), "quota-observation.db")
	db := openQuotaTestDatabase(t, dbPath)
	identity := createQuotaTestIdentity(t, db, "auth-1")
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := NewServiceWithRegistry(NewRepository(db), NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))
	observedAt := time.Date(2026, 9, 8, 1, 2, 3, 456, time.UTC)
	service.now = func() time.Time { return observedAt }

	response, err := service.Check(ctx, CheckRequest{AuthIndex: identity.Identity})
	if err != nil || !response.ObservedAt.Equal(observedAt) {
		t.Fatalf("persist quota observation: response=%+v err=%v", response, err)
	}
	closeQuotaTestDatabase(t, db)

	db = openQuotaTestDatabase(t, dbPath)
	defer closeQuotaTestDatabase(t, db)
	restarted := NewServiceWithRegistry(NewRepository(db), NewProviderRegistry(nil))
	observations, err := restarted.GetQuotaObservations(ctx, ObservationsRequest{AuthIndexes: []string{identity.Identity}, Limit: 1})
	if err != nil || len(observations.Items) != 1 {
		t.Fatalf("read observation after reopen: observations=%+v err=%v", observations, err)
	}
	got := observations.Items[0]
	if got.ID != identity.Identity || !got.ObservedAt.Equal(observedAt) || len(got.Quota) != 1 || got.Quota[0].UsedPercent == nil || *got.Quota[0].UsedPercent != 25 {
		t.Fatalf("unexpected persisted observation: %+v", got)
	}
}

func TestDisabledIdentityQuotaQueriesPersistObservationWithoutEnablingIdentity(t *testing.T) {
	for _, mode := range []string{"check", "refresh"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			db := openQuotaTestDatabase(t, filepath.Join(t.TempDir(), "disabled-quota.db"))
			defer closeQuotaTestDatabase(t, db)
			identity := createQuotaTestIdentity(t, db, "auth-disabled")
			observedAt := time.Date(2026, 9, 10, 1, 2, 3, 0, time.UTC)
			if err := dbrepository.SetUsageIdentityDisabled(ctx, db, identity.ID, true, observedAt.Add(-time.Minute)); err != nil {
				t.Fatalf("disable identity: %v", err)
			}
			repository := NewRepository(db)
			handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
			service := NewServiceWithRegistry(repository, NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))
			defer service.StopRefreshWorkers()
			service.now = func() time.Time { return observedAt }

			var response CheckResponse
			if mode == "check" {
				var err error
				response, err = service.Check(ctx, CheckRequest{AuthIndex: identity.Identity})
				if err != nil {
					t.Fatalf("check disabled identity: %v", err)
				}
			} else {
				taskID := refreshAuthIndex(t, service, identity.Identity)
				task := waitForRefreshTask(t, service, taskID, RefreshTaskStatusCompleted)
				if task.Quota == nil {
					t.Fatalf("completed refresh must return quota: %+v", task)
				}
				response = *task.Quota
			}
			if response.ID != identity.Identity || !response.ObservedAt.Equal(observedAt) {
				t.Fatalf("unexpected quota response: %+v", response)
			}
			if handler.callCount() != 1 {
				t.Fatalf("expected one provider call, got %d", handler.callCount())
			}
			assertRepositoryObservation(t, repository, identity.Identity, observedAt, 25)
			persisted, found, err := repository.FindActiveAuthFileIdentity(ctx, identity.Identity)
			if err != nil || !found || !persisted.Disabled {
				t.Fatalf("quota query must preserve disabled identity: identity=%+v found=%v err=%v", persisted, found, err)
			}
		})
	}
}

func TestQuotaObservationOnlyReplacesWithStrictlyNewerObservation(t *testing.T) {
	db := openQuotaTestDatabase(t, filepath.Join(t.TempDir(), "newer-wins.db"))
	defer closeQuotaTestDatabase(t, db)
	identity := createQuotaTestIdentity(t, db, "auth-1")
	repository := NewRepository(db)
	base := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)

	saveQuotaTestObservation(t, repository, identity.ID, identity.Identity, base.Add(time.Hour), 20)
	saveQuotaTestObservation(t, repository, identity.ID, identity.Identity, base, 10)
	saveQuotaTestObservation(t, repository, identity.ID, identity.Identity, base.Add(time.Hour), 30)
	assertRepositoryObservation(t, repository, identity.Identity, base.Add(time.Hour), 20)

	saveQuotaTestObservation(t, repository, identity.ID, identity.Identity, base.Add(2*time.Hour), 40)
	assertRepositoryObservation(t, repository, identity.Identity, base.Add(2*time.Hour), 40)
}

func TestQuotaObservationFollowsIdentityLifecycleVisibility(t *testing.T) {
	ctx := context.Background()
	db := openQuotaTestDatabase(t, filepath.Join(t.TempDir(), "lifecycle.db"))
	defer closeQuotaTestDatabase(t, db)
	identity := createQuotaTestIdentity(t, db, "auth-1")
	repository := NewRepository(db)
	observedAt := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	saveQuotaTestObservation(t, repository, identity.ID, identity.Identity, observedAt, 25)

	if err := dbrepository.SetUsageIdentityDisabled(ctx, db, identity.ID, true, observedAt.Add(time.Minute)); err != nil {
		t.Fatalf("disable identity: %v", err)
	}
	assertRepositoryObservation(t, repository, identity.Identity, observedAt, 25)

	if err := dbrepository.ReplaceUsageIdentitiesForAuthType(ctx, db, nil, entities.UsageIdentityAuthTypeAuthFile, observedAt.Add(2*time.Minute)); err != nil {
		t.Fatalf("soft delete identity: %v", err)
	}
	hidden, err := repository.ListQuotaObservations(ctx, []string{identity.Identity}, 1)
	if err != nil || len(hidden) != 0 {
		t.Fatalf("deleted identity must hide observation: items=%+v err=%v", hidden, err)
	}

	identity.Disabled = false
	if err := dbrepository.ReplaceUsageIdentitiesForAuthType(ctx, db, []entities.UsageIdentity{identity}, entities.UsageIdentityAuthTypeAuthFile, observedAt.Add(3*time.Minute)); err != nil {
		t.Fatalf("reactivate identity: %v", err)
	}
	assertRepositoryObservation(t, repository, identity.Identity, observedAt, 25)
}

func openQuotaTestDatabase(t *testing.T, path string) *gorm.DB {
	t.Helper()
	db, err := dbrepository.OpenDatabase(config.Config{SQLitePath: path})
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	return db
}

func closeQuotaTestDatabase(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("get sql database: %v", err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatalf("close database: %v", err)
	}
}

func createQuotaTestIdentity(t *testing.T, db *gorm.DB, authIndex string) entities.UsageIdentity {
	t.Helper()
	identity := claudeAuthFileIdentity(authIndex)
	identity.Name = authIndex
	if err := db.Create(&identity).Error; err != nil {
		t.Fatalf("create identity: %v", err)
	}
	return identity
}

func saveQuotaTestObservation(t *testing.T, repository Repository, identityID uint, authIndex string, observedAt time.Time, usedPercent float64) {
	t.Helper()
	rows := claudeUsageOutput(usedPercent).Result.(ClaudeResult).QuotaRows()
	if err := repository.SaveQuotaObservation(context.Background(), identityID, CheckResponse{ID: authIndex, ObservedAt: observedAt, Quota: rows}); err != nil {
		t.Fatalf("save quota observation: %v", err)
	}
}

func assertRepositoryObservation(t *testing.T, repository Repository, authIndex string, observedAt time.Time, usedPercent float64) {
	t.Helper()
	items, err := repository.ListQuotaObservations(context.Background(), []string{authIndex}, 1)
	if err != nil || len(items) != 1 || !items[0].ObservedAt.Equal(observedAt) || len(items[0].Quota) != 1 || items[0].Quota[0].UsedPercent == nil || *items[0].Quota[0].UsedPercent != usedPercent {
		t.Fatalf("unexpected observation: items=%+v err=%v", items, err)
	}
}
