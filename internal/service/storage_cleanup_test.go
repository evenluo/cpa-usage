package service

import (
	"context"
	"errors"
	"testing"
)

func TestCleanupStorageHonorsCanceledContext(t *testing.T) {
	db := openSyncTestDatabase(t)
	service := NewSyncServiceWithOptions(db, SyncServiceOptions{})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := service.CleanupStorage(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled cleanup, got %v", err)
	}
}
