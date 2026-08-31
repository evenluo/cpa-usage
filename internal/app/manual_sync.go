package app

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"cpa-usage/internal/poller"
)

type manualSyncRunner struct {
	redis    Runner
	metadata MetadataSyncer
	now      func() time.Time

	mu              sync.Mutex
	admissionClosed bool
	manualRunning   bool
	lastRunAt       time.Time
	lastError       string
	lastWarning     string
	lastStatus      string
}

type manualSyncStageError struct {
	message string
	err     error
}

func (e manualSyncStageError) Error() string {
	return fmt.Sprintf("%s: %v", e.message, e.err)
}

func (e manualSyncStageError) Unwrap() error {
	return e.err
}

func (e manualSyncStageError) UserMessage() string {
	return e.message
}

func newManualSyncRunner(redis Runner, metadata MetadataSyncer) *manualSyncRunner {
	return &manualSyncRunner{redis: redis, metadata: metadata, now: time.Now}
}

func (r *manualSyncRunner) Status() poller.Status {
	if r == nil {
		return poller.Status{}
	}
	status := poller.Status{}
	if r.redis != nil {
		status = r.redis.Status()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	status.SyncRunning = status.SyncRunning || r.manualRunning
	status.LastRunAt = r.lastRunAt
	status.LastError = r.lastError
	status.LastWarning = r.lastWarning
	status.LastStatus = r.lastStatus
	return status
}

func (r *manualSyncRunner) SyncNow(ctx context.Context) error {
	if r == nil {
		return fmt.Errorf("manual sync runner is nil")
	}
	if err := r.begin(); err != nil {
		return err
	}
	defer r.finishRunning()
	if r.redis == nil {
		err := fmt.Errorf("manual redis syncer is nil")
		r.record(poller.ManualSyncOutcome{Status: "failed", Error: err.Error()})
		return err
	}
	outcome, err := r.redis.SyncNow(ctx)
	if err != nil {
		if !errors.Is(err, poller.ErrSyncAlreadyRunning) {
			r.record(outcome)
		}
		return manualSyncStageError{message: "redis sync failed", err: err}
	}
	if r.metadata == nil {
		err := fmt.Errorf("manual metadata syncer is nil")
		r.record(poller.ManualSyncOutcome{Status: "completed_with_warnings", Warning: err.Error()})
		return err
	}
	if err := r.metadata.SyncMetadata(ctx); err != nil {
		r.record(poller.ManualSyncOutcome{Status: "completed_with_warnings", Warning: err.Error()})
		return manualSyncStageError{message: "metadata sync failed", err: err}
	}
	r.record(outcome)
	return nil
}

// CloseAdmission rejects new manual commands while allowing an already
// admitted command to finish under HTTP drain ownership.
func (r *manualSyncRunner) CloseAdmission() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.admissionClosed = true
	r.mu.Unlock()
}

func (r *manualSyncRunner) begin() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.admissionClosed {
		return poller.ErrSyncUnavailable
	}
	if r.manualRunning {
		return poller.ErrSyncAlreadyRunning
	}
	r.manualRunning = true
	return nil
}

func (r *manualSyncRunner) finishRunning() {
	r.mu.Lock()
	r.manualRunning = false
	r.mu.Unlock()
}

func (r *manualSyncRunner) record(outcome poller.ManualSyncOutcome) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.now == nil {
		r.now = time.Now
	}
	r.lastRunAt = r.now().UTC()
	r.lastStatus = outcome.Status
	r.lastError = outcome.Error
	r.lastWarning = outcome.Warning
	if r.lastStatus == "" {
		if outcome.Warning != "" {
			r.lastStatus = "completed_with_warnings"
		} else if outcome.Error != "" {
			r.lastStatus = "failed"
		} else {
			r.lastStatus = "completed"
		}
	}
}
