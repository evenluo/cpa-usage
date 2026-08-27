package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"cpa-usage/internal/api"
	"cpa-usage/internal/auth"
	"cpa-usage/internal/config"
	"cpa-usage/internal/cpa"
	"cpa-usage/internal/logging"
	"cpa-usage/internal/poller"
	"cpa-usage/internal/quota"
	"cpa-usage/internal/repository"
	"cpa-usage/internal/service"
	webui "cpa-usage/web"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type Runner interface {
	Run(ctx context.Context) error
	Status() poller.Status
	SyncNow(ctx context.Context) (poller.ManualSyncOutcome, error)
}

// defaultShutdownTimeout bounds accepted HTTP request drain during process
// shutdown. A timeout is explicit and retryable; it never authorizes closing
// resources still owned by a live handler or background runner.
const defaultShutdownTimeout = 10 * time.Second

type Options struct {
	EnvFile string
}

type App struct {
	Config            *config.Config
	DB                *gorm.DB
	Router            *gin.Engine
	Server            *http.Server
	Poller            Runner
	Maintenance       *StorageCleanupRunner
	MetadataSync      *MetadataSyncRunner
	BackupMaintenance *DatabaseBackupRunner
	RollupBackfill    *service.UsageRollupBackfillRunner
	Quota             *quota.Service
	LogCloser         io.Closer

	rollupBackfillReader repository.RollupBackfillReader
	startedAt            time.Time
	manualSync           *manualSyncRunner
	shutdownTimeout      time.Duration
	backgroundCancel     context.CancelFunc
	backgroundWG         sync.WaitGroup

	lifecycleMu    sync.Mutex
	runtimeStarted bool
	closed         bool
	closeErr       error
	closeAttempt   *appCloseAttempt
	httpClosed     atomic.Bool

	metricsMu              sync.Mutex
	lastMetricsSampleAt    time.Time
	lastMetricsEventsTotal int64
}

type appCloseAttempt struct {
	done chan struct{}
	err  error
}

func New() (*App, error) {
	return NewWithOptions(Options{})
}

func NewWithOptions(options Options) (*App, error) {
	cfg, err := config.Load(config.LoadOptions{EnvFile: options.EnvFile})
	if err != nil {
		return nil, err
	}

	return NewWithConfig(*cfg)
}

func NewWithConfig(cfg config.Config) (*App, error) {
	logCloser, err := logging.Configure(cfg)
	if err != nil {
		return nil, err
	}

	db, err := repository.OpenDatabase(cfg)
	if err != nil {
		_ = logCloser.Close()
		return nil, err
	}

	syncService := service.NewSyncService(db, cfg)
	backgroundPoller := poller.NewRedisDrain(syncService, poller.RedisDrainConfig{
		IdleInterval: cfg.RedisQueueIdleInterval,
		ErrorBackoff: cfg.RedisQueueErrorBackoff,
	})
	var backupMaintenance *DatabaseBackupRunner
	if cfg.BackupEnabled {
		sqlDB, err := db.DB()
		if err != nil {
			_ = closeGormDB(db)
			_ = logCloser.Close()
			return nil, err
		}
		backupStore := newDatabaseBackupStore(sqlDB, cfg.BackupDir)
		backupMaintenance = NewDatabaseBackupRunner(backupStore, backupStore, cfg.BackupInterval, cfg.BackupRetentionDays)
	}

	usageReader := repository.NewUsageReader(db)
	usageIdentityReader := repository.NewUsageIdentityReader(db)
	analyticsReader := repository.NewAnalyticsReader(db)
	rollupBackfillReader := repository.NewRollupBackfillReader(db)
	rollupBackfillRunner := service.NewUsageRollupBackfillRunner(db, service.UsageRollupBackfillRunnerConfig{
		BatchHours:   cfg.UsageRollupBackfillBatchHours,
		IdleInterval: cfg.UsageRollupBackfillIdleInterval,
		RetryBackoff: cfg.UsageRollupBackfillErrorBackoff,
	})
	keyAliasService := service.NewKeyAliasService(db)
	cpaClient := cpa.NewClient(cfg.CPABaseURL, cfg.CPAManagementKey, cfg.RequestTimeout, cfg.TLSSkipVerify)
	if cfg.TLSSkipVerify {
		slog.Warn("TLS certificate verification is disabled for CPA and Redis queue connections", "cpa_base_url", cfg.CPABaseURL)
	}
	pricingService := service.NewPricingService(db, cpaClient)
	quotaService := quota.NewService(quota.NewRepositoryAuthFileIdentityLookup(db), cpaClient)
	sessionManager := auth.NewSessionManager(cfg.AuthSessionTTL)
	if cfg.AuthSessionSecret != "" {
		sessionManager = auth.NewSignedSessionManager(cfg.AuthSessionTTL, cfg.AuthSessionSecret)
	}
	authConfig := api.AuthConfig{
		Enabled:             cfg.AuthEnabled,
		LoginPassword:       cfg.LoginPassword,
		SharedBearerToken:   cfg.CPAManagementKey,
		SessionTTL:          cfg.AuthSessionTTL,
		BasePath:            cfg.AppBasePath,
		SessionCookieName:   cfg.AuthSessionCookieName,
		SessionCookieDomain: cfg.AuthSessionCookieDomain,
		SessionCookiePath:   cfg.AuthSessionCookiePath,
		TrustedProxies:      cfg.TrustedProxies,
	}
	authHandler := api.NewAuthHandler(authConfig, sessionManager)

	appInstance := &App{
		Config:               &cfg,
		DB:                   db,
		Poller:               backgroundPoller,
		Maintenance:          NewStorageCleanupRunner(syncService),
		MetadataSync:         NewMetadataSyncRunner(syncService, cfg.MetadataSyncInterval),
		BackupMaintenance:    backupMaintenance,
		RollupBackfill:       rollupBackfillRunner,
		Quota:                quotaService,
		LogCloser:            logCloser,
		rollupBackfillReader: rollupBackfillReader,
		startedAt:            time.Now(),
		shutdownTimeout:      defaultShutdownTimeout,
	}
	manualSync := newManualSyncRunner(backgroundPoller, syncService)
	appInstance.manualSync = manualSync
	appInstance.Router = api.NewRouter(
		webui.Static,
		manualSync,
		usageReader,
		pricingService,
		authConfig,
		authHandler,
		cfg.AppBasePath,
		api.OptionalProviders{Analytics: analyticsReader, UsageIdentity: usageIdentityReader, KeyAlias: keyAliasService, Quota: quotaService, RollupBackfill: rollupBackfillReader, Metrics: appInstance},
	)
	appInstance.Server = &http.Server{
		Addr:    ":" + cfg.AppPort,
		Handler: appInstance.httpHandler(),
	}
	return appInstance, nil
}

func closeGormDB(db *gorm.DB) error {
	if db == nil {
		return nil
	}
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func (a *App) Close() error {
	if a == nil {
		return nil
	}

	attempt, owner, closedErr := a.beginCloseAttempt()
	if !owner {
		if attempt == nil {
			return closedErr
		}
		<-attempt.done
		return attempt.err
	}

	closeErr, complete := a.closeRuntime()
	a.finishCloseAttempt(attempt, closeErr, complete)
	return closeErr
}

func (a *App) Run(ctx context.Context) error {
	if a == nil || a.Router == nil || a.Config == nil {
		return fmt.Errorf("application is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	serverDone, err := a.startRuntime()
	if err != nil {
		return errors.Join(err, a.Close())
	}
	select {
	case serveErr := <-serverDone:
		if errors.Is(serveErr, http.ErrServerClosed) {
			return nil
		}
		return errors.Join(fmt.Errorf("serve HTTP: %w", serveErr), a.Close())
	case <-ctx.Done():
		if err := a.Close(); err != nil {
			return err
		}
		serveErr := <-serverDone
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", serveErr)
		}
		return nil
	}
}

func (a *App) startRuntime() (<-chan error, error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.closed || a.closeAttempt != nil {
		return nil, fmt.Errorf("application is closing")
	}
	if a.runtimeStarted {
		return nil, fmt.Errorf("application is already running")
	}
	if a.Server == nil {
		a.Server = &http.Server{Addr: ":" + a.Config.AppPort, Handler: a.httpHandler()}
	}
	listener, err := net.Listen("tcp", a.Server.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen HTTP: %w", err)
	}

	backgroundCtx, cancel := context.WithCancel(context.Background())
	a.backgroundCancel = cancel
	if a.Quota != nil {
		a.Quota.AttachRefreshWorkerLifecycle(backgroundCtx)
	}
	tasks := a.backgroundTasks(backgroundCtx)
	a.backgroundWG.Add(len(tasks))
	for _, task := range tasks {
		task := task
		go func() {
			defer a.backgroundWG.Done()
			task()
		}()
	}
	a.runtimeStarted = true
	serverDone := make(chan error, 1)
	go func() {
		serverDone <- a.Server.Serve(listener)
	}()
	return serverDone, nil
}

func (a *App) backgroundTasks(ctx context.Context) []func() {
	tasks := make([]func(), 0, 5)
	if a.Poller != nil {
		tasks = append(tasks, func() {
			if err := a.Poller.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("poller stopped", "error", err)
			}
		})
	}
	if a.Maintenance != nil {
		tasks = append(tasks, func() {
			if err := a.Maintenance.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("maintenance cleanup stopped", "error", err)
			}
		})
	}
	if a.MetadataSync != nil {
		tasks = append(tasks, func() {
			if err := a.MetadataSync.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("metadata sync stopped", "error", err)
			}
		})
	}
	if a.BackupMaintenance != nil {
		tasks = append(tasks, func() {
			if err := a.BackupMaintenance.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("database backup stopped", "error", err)
			}
		})
	}
	if a.RollupBackfill != nil {
		tasks = append(tasks, func() {
			if err := a.RollupBackfill.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
				slog.Error("usage rollup backfill stopped", "error", err)
			}
		})
	}
	return tasks
}

func (a *App) beginCloseAttempt() (*appCloseAttempt, bool, error) {
	a.lifecycleMu.Lock()
	defer a.lifecycleMu.Unlock()
	if a.closed {
		return nil, false, a.closeErr
	}
	if a.closeAttempt != nil {
		return a.closeAttempt, false, nil
	}
	attempt := &appCloseAttempt{done: make(chan struct{})}
	a.closeAttempt = attempt
	return attempt, true, nil
}

func (a *App) finishCloseAttempt(attempt *appCloseAttempt, err error, complete bool) {
	a.lifecycleMu.Lock()
	attempt.err = err
	if complete {
		a.closed = true
		a.closeErr = err
	}
	a.closeAttempt = nil
	close(attempt.done)
	a.lifecycleMu.Unlock()
}

func (a *App) closeRuntime() (error, bool) {
	// Admission closes before HTTP drain. Accepted handlers retain their owners
	// until Shutdown confirms that every active handler has returned.
	a.httpClosed.Store(true)
	if a.manualSync != nil {
		a.manualSync.CloseAdmission()
	}
	if a.Quota != nil {
		a.Quota.CloseRefreshAdmission()
	}
	if a.Server != nil {
		timeout := a.shutdownTimeout
		if timeout <= 0 {
			timeout = defaultShutdownTimeout
		}
		shutdownCtx, cancel := context.WithTimeout(context.Background(), timeout)
		err := a.Server.Shutdown(shutdownCtx)
		cancel()
		if err != nil {
			// A drain timeout is retryable. Background owners and their DB/log
			// resources remain live until a later Close proves the drain complete.
			return fmt.Errorf("drain HTTP server: %w", err), false
		}
	}
	if a.Quota != nil {
		a.Quota.StopRefreshWorkers()
	}

	if a.backgroundCancel != nil {
		a.backgroundCancel()
		a.backgroundCancel = nil
	}
	a.backgroundWG.Wait()

	var closeErr error
	if a.DB != nil {
		closeErr = errors.Join(closeErr, closeGormDB(a.DB))
		a.DB = nil
	}
	if a.LogCloser != nil {
		closeErr = errors.Join(closeErr, a.LogCloser.Close())
		a.LogCloser = nil
	}
	return closeErr, true
}

func (a *App) httpHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if a.httpClosed.Load() {
			http.Error(w, "service unavailable", http.StatusServiceUnavailable)
			return
		}
		a.Router.ServeHTTP(w, request)
	})
}
