package app

import (
	"context"
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/poller"
)

type manualRedisSyncStub struct {
	status  poller.Status
	outcome poller.ManualSyncOutcome
	err     error
	calls   int
	order   *[]string
}

func (s *manualRedisSyncStub) Run(context.Context) error {
	return nil
}

func (s *manualRedisSyncStub) Status() poller.Status {
	return s.status
}

func (s *manualRedisSyncStub) SyncNow(context.Context) (poller.ManualSyncOutcome, error) {
	s.calls++
	if s.order != nil {
		*s.order = append(*s.order, "redis")
	}
	return s.outcome, s.err
}

type manualMetadataSyncStub struct {
	err   error
	calls int
	order *[]string
}

func (s *manualMetadataSyncStub) SyncMetadata(context.Context) error {
	s.calls++
	if s.order != nil {
		*s.order = append(*s.order, "metadata")
	}
	return s.err
}

func TestManualSyncRunnerRunsRedisThenMetadata(t *testing.T) {
	var order []string
	redis := &manualRedisSyncStub{
		status:  poller.Status{Running: true},
		outcome: poller.ManualSyncOutcome{Status: "empty"},
		order:   &order,
	}
	metadata := &manualMetadataSyncStub{order: &order}
	runner := newManualSyncRunner(redis, metadata)
	completedAt := time.Date(2026, 8, 27, 5, 0, 0, 0, time.UTC)
	runner.now = func() time.Time { return completedAt }

	if err := runner.SyncNow(context.Background()); err != nil {
		t.Fatalf("SyncNow returned error: %v", err)
	}
	if !reflect.DeepEqual(order, []string{"redis", "metadata"}) {
		t.Fatalf("expected redis then metadata sync, got %v", order)
	}
	status := runner.Status()
	if !status.Running || status.LastStatus != "empty" || !status.LastRunAt.Equal(completedAt) {
		t.Fatalf("expected manual owner to retain the command outcome and runner state, got %+v", status)
	}
}

func TestManualSyncRunnerReturnsRedisErrorWithoutMetadata(t *testing.T) {
	redisErr := errors.New("redis failed")
	redis := &manualRedisSyncStub{outcome: poller.ManualSyncOutcome{Status: "failed", Error: redisErr.Error()}, err: redisErr}
	metadata := &manualMetadataSyncStub{}
	runner := newManualSyncRunner(redis, metadata)

	err := runner.SyncNow(context.Background())
	if !errors.Is(err, redisErr) {
		t.Fatalf("expected redis error, got %v", err)
	}
	if err == nil || err.Error() != "redis sync failed: redis failed" {
		t.Fatalf("expected redis-specific sync error, got %v", err)
	}
	if metadata.calls != 0 {
		t.Fatalf("expected metadata sync not to run after redis failure, got %d calls", metadata.calls)
	}
	status := runner.Status()
	if status.LastStatus != "failed" || status.LastError != "redis failed" {
		t.Fatalf("expected failed manual status, got %+v", status)
	}
}

func TestManualSyncRunnerReturnsMetadataError(t *testing.T) {
	metadataErr := errors.New("metadata failed")
	redis := &manualRedisSyncStub{}
	metadata := &manualMetadataSyncStub{err: metadataErr}
	runner := newManualSyncRunner(redis, metadata)

	err := runner.SyncNow(context.Background())
	if !errors.Is(err, metadataErr) {
		t.Fatalf("expected metadata error, got %v", err)
	}
	if err == nil || err.Error() != "metadata sync failed: metadata failed" {
		t.Fatalf("expected metadata-specific sync error, got %v", err)
	}
	if redis.calls != 1 || metadata.calls != 1 {
		t.Fatalf("expected redis and metadata to run once, got redis=%d metadata=%d", redis.calls, metadata.calls)
	}
	status := runner.Status()
	if status.LastStatus != "completed_with_warnings" || status.LastWarning != "metadata failed" {
		t.Fatalf("expected metadata warning to remain visible, got %+v", status)
	}
}

func TestManualSyncRunnerRejectsNewCommandsAfterAdmissionCloses(t *testing.T) {
	redis := &manualRedisSyncStub{}
	metadata := &manualMetadataSyncStub{}
	runner := newManualSyncRunner(redis, metadata)
	runner.CloseAdmission()

	err := runner.SyncNow(context.Background())
	if !errors.Is(err, poller.ErrSyncUnavailable) {
		t.Fatalf("expected shutdown admission rejection, got %v", err)
	}
	if redis.calls != 0 || metadata.calls != 0 {
		t.Fatalf("expected no side effect after admission closed, got redis=%d metadata=%d", redis.calls, metadata.calls)
	}
}

func TestManualSyncRunnerDoesNotLetBackgroundStatusOverwriteLastManualResult(t *testing.T) {
	redis := &manualRedisSyncStub{outcome: poller.ManualSyncOutcome{Status: "completed"}}
	runner := newManualSyncRunner(redis, &manualMetadataSyncStub{})
	if err := runner.SyncNow(context.Background()); err != nil {
		t.Fatalf("SyncNow returned error: %v", err)
	}

	redis.status = poller.Status{
		Running:     true,
		SyncRunning: true,
		LastStatus:  "failed",
		LastError:   "background failed",
	}
	status := runner.Status()
	if !status.Running || !status.SyncRunning {
		t.Fatalf("expected live background runner state, got %+v", status)
	}
	if status.LastStatus != "completed" || status.LastError != "" {
		t.Fatalf("expected last manual result to remain authoritative, got %+v", status)
	}
}

type blockingManualRedisSyncStub struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
}

func (s *blockingManualRedisSyncStub) Run(context.Context) error { return nil }
func (s *blockingManualRedisSyncStub) Status() poller.Status     { return poller.Status{} }
func (s *blockingManualRedisSyncStub) SyncNow(context.Context) (poller.ManualSyncOutcome, error) {
	s.once.Do(func() { close(s.started) })
	<-s.release
	return poller.ManualSyncOutcome{Status: "completed"}, nil
}

func TestManualSyncRunnerAdmitsOnlyOneConcurrentCommand(t *testing.T) {
	redis := &blockingManualRedisSyncStub{started: make(chan struct{}), release: make(chan struct{})}
	runner := newManualSyncRunner(redis, &manualMetadataSyncStub{})
	firstDone := make(chan error, 1)
	go func() { firstDone <- runner.SyncNow(context.Background()) }()
	<-redis.started

	if err := runner.SyncNow(context.Background()); !errors.Is(err, poller.ErrSyncAlreadyRunning) {
		t.Fatalf("expected concurrent command conflict, got %v", err)
	}
	close(redis.release)
	if err := <-firstDone; err != nil {
		t.Fatalf("first manual command returned error: %v", err)
	}
}
