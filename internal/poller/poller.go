package poller

import (
	"context"
	"errors"
	"time"
)

var ErrSyncAlreadyRunning = errors.New("sync already running")
var ErrSyncCompletedWithWarnings = errors.New("sync completed with warnings")
var ErrSyncUnavailable = errors.New("sync unavailable")

// ManualSyncOutcome carries the Redis stages' manual-command result to the
// application-level manual sync owner. Background loops never write this state.
type ManualSyncOutcome struct {
	Status  string
	Error   string
	Warning string
}

type Status struct {
	Running     bool
	LastRunAt   time.Time
	LastError   string
	LastWarning string
	LastStatus  string
	SyncRunning bool
}

func shouldLogSyncError(err error) bool {
	return err != nil && !errors.Is(err, ErrSyncCompletedWithWarnings) && !errors.Is(err, ErrSyncAlreadyRunning) && !errors.Is(err, context.Canceled)
}
