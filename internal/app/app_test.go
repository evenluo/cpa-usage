package app

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"cpa-usage/internal/config"
	"cpa-usage/internal/entities"
	"cpa-usage/internal/poller"
	"cpa-usage/internal/quota"
	"github.com/gin-gonic/gin"
)

func TestAppCloseClosesDatabase(t *testing.T) {
	app, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	sqlDB, err := app.DB.DB()
	if err != nil {
		t.Fatalf("load sql db: %v", err)
	}

	if err := app.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected database ping to fail after app close")
	}
}

func TestNewWithConfigBuildsRedisDrainAndRouter(t *testing.T) {
	app, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer app.Close()
	if app.Poller == nil {
		t.Fatal("expected poller to be initialized")
	}
	if app.Router == nil {
		t.Fatal("expected router to be initialized")
	}
	if app.LogCloser == nil {
		t.Fatal("expected log closer to be initialized")
	}
	if app.BackupMaintenance == nil {
		t.Fatal("expected database backup runner to be initialized")
	}
	if app.MetadataSync == nil {
		t.Fatal("expected metadata sync runner to be initialized")
	}
}

func TestNewWithConfigSkipsBackupRunnerWhenDisabled(t *testing.T) {
	cfg := testAppConfig(t)
	cfg.BackupEnabled = false
	app, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer app.Close()
	if app.BackupMaintenance != nil {
		t.Fatal("expected database backup runner to be skipped when backups are disabled")
	}
}

func TestNewWithConfigSelectsRedisDrain(t *testing.T) {
	app, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer app.Close()
	if _, ok := app.Poller.(*poller.RedisDrain); !ok {
		t.Fatalf("expected redis to use redis drain, got %T", app.Poller)
	}
	if app.Maintenance == nil {
		t.Fatal("expected maintenance cleanup runner to be initialized")
	}
}

func TestNewWithConfigCreatesIndependentMaintenanceRunner(t *testing.T) {
	app, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer app.Close()
	if app.Poller == nil {
		t.Fatal("expected sync poller to be initialized")
	}
	if app.Maintenance == nil {
		t.Fatal("expected independent maintenance runner to be initialized")
	}
}

func TestNewWithConfigPassesUsageRollupBackfillConfig(t *testing.T) {
	cfg := testAppConfig(t)
	cfg.UsageRollupBackfillBatchHours = 11
	cfg.UsageRollupBackfillIdleInterval = 4 * time.Second
	cfg.UsageRollupBackfillErrorBackoff = 13 * time.Second

	app, err := NewWithConfig(cfg)
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	defer app.Close()

	if app.RollupBackfill == nil {
		t.Fatal("expected usage rollup backfill runner to be initialized")
	}
	runner := reflect.ValueOf(app.RollupBackfill).Elem()
	if batchHours := int(runner.FieldByName("batchHours").Int()); batchHours != 11 {
		t.Fatalf("expected configured backfill batch hours, got %d", batchHours)
	}
	if idleInterval := time.Duration(runner.FieldByName("idleInterval").Int()); idleInterval != 4*time.Second {
		t.Fatalf("expected configured backfill idle interval, got %s", idleInterval)
	}
	if retryBackoff := time.Duration(runner.FieldByName("retryBackoff").Int()); retryBackoff != 13*time.Second {
		t.Fatalf("expected configured backfill retry backoff, got %s", retryBackoff)
	}
}

func TestRunStartsPollerAndMaintenanceIndependently(t *testing.T) {
	cfg := testAppConfig(t)
	cfg.AppPort = "0"
	pollerStarted := make(chan struct{})
	maintenanceStarted := make(chan struct{})
	metadataStarted := make(chan struct{})
	backupStarted := make(chan struct{})
	maintenance := NewStorageCleanupRunner(&maintenanceSyncStub{})
	maintenance.sleep = func(context.Context, time.Duration) bool {
		close(maintenanceStarted)
		return false
	}
	metadataRunner := NewMetadataSyncRunner(&metadataSyncStub{}, time.Second)
	metadataRunner.sleep = func(context.Context, time.Duration) bool {
		close(metadataStarted)
		return false
	}
	backupRunner := NewDatabaseBackupRunner(&databaseBackupWriterStub{}, nil, time.Second, 0)
	backupRunner.sleep = func(context.Context, time.Duration) bool {
		close(backupStarted)
		return false
	}
	app := &App{
		Config:            &cfg,
		Router:            gin.New(),
		shutdownTimeout:   time.Second,
		Poller:            &appRunStub{started: pollerStarted},
		Maintenance:       maintenance,
		MetadataSync:      metadataRunner,
		BackupMaintenance: backupRunner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- app.Run(ctx) }()
	select {
	case <-pollerStarted:
	case <-time.After(time.Second):
		t.Fatal("expected poller runner to start")
	}
	select {
	case <-maintenanceStarted:
	case <-time.After(time.Second):
		t.Fatal("expected maintenance runner to start")
	}
	select {
	case <-metadataStarted:
	case <-time.After(time.Second):
		t.Fatal("expected metadata sync runner to start")
	}
	select {
	case <-backupStarted:
	case <-time.After(time.Second):
		t.Fatal("expected database backup runner to start")
	}
	cancel()
	if err := <-runDone; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestRunCancelsBackgroundTasksWhenContextStops(t *testing.T) {
	cfg := testAppConfig(t)
	cfg.AppPort = "0"
	backupStarted := make(chan struct{})
	backupCanceled := make(chan struct{})
	backupRunner := NewDatabaseBackupRunner(&databaseBackupWriterStub{}, nil, time.Second, 0)
	backupRunner.sleep = func(ctx context.Context, _ time.Duration) bool {
		close(backupStarted)
		<-ctx.Done()
		close(backupCanceled)
		return false
	}
	app := &App{
		Config:            &cfg,
		Router:            gin.New(),
		shutdownTimeout:   time.Second,
		BackupMaintenance: backupRunner,
	}

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- app.Run(ctx) }()
	select {
	case <-backupStarted:
	case <-time.After(time.Second):
		t.Fatal("expected database backup runner to start")
	}
	cancel()
	select {
	case <-backupCanceled:
	case <-time.After(time.Second):
		t.Fatal("expected database backup runner context to be canceled")
	}
	if err := <-runDone; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
}

func TestCloseDrainsBlockingHTTPHandlerBeforeClosingResources(t *testing.T) {
	application, sqlDB, logCloser := newLifecycleTestApp(t)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	application.Server, _ = startBlockingHTTPServer(t, handlerStarted, releaseHandler)
	application.shutdownTimeout = time.Second

	closeDone := make(chan error, 1)
	go func() { closeDone <- application.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before the accepted handler drained: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("database closed while HTTP handler was active: %v", err)
	}
	if logCloser.isClosed() {
		t.Fatal("log resource closed while HTTP handler was active")
	}

	close(releaseHandler)
	if err := <-closeDone; err != nil {
		t.Fatalf("Close returned error after handler drain: %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected database to close after handler drain")
	}
	if !logCloser.isClosed() {
		t.Fatal("expected log resource to close after handler drain")
	}
}

func TestCloseDeadlineKeepsResourcesLiveAndAllowsRetry(t *testing.T) {
	application, sqlDB, logCloser := newLifecycleTestApp(t)
	handlerStarted := make(chan struct{})
	releaseHandler := make(chan struct{})
	application.Server, _ = startBlockingHTTPServer(t, handlerStarted, releaseHandler)
	application.shutdownTimeout = 25 * time.Millisecond

	err := application.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected explicit HTTP drain deadline, got %v", err)
	}
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("database closed after incomplete drain: %v", err)
	}
	if logCloser.isClosed() {
		t.Fatal("log resource closed after incomplete drain")
	}
	if err := application.manualSync.SyncNow(context.Background()); !errors.Is(err, poller.ErrSyncUnavailable) {
		t.Fatalf("expected manual admission to stay closed after timeout, got %v", err)
	}

	close(releaseHandler)
	application.shutdownTimeout = time.Second
	if err := application.Close(); err != nil {
		t.Fatalf("retry Close returned error: %v", err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected retry to close database after handler exit")
	}
	if !logCloser.isClosed() {
		t.Fatal("expected retry to close log resource")
	}
}

func TestConcurrentCloseUsesOneOwnerAndSharesResult(t *testing.T) {
	closer := &blockingCloser{entered: make(chan struct{}), release: make(chan struct{})}
	application := &App{LogCloser: closer}
	const callers = 8
	results := make(chan error, callers)
	go func() { results <- application.Close() }()
	<-closer.entered
	for range callers - 1 {
		go func() { results <- application.Close() }()
	}
	select {
	case err := <-results:
		t.Fatalf("concurrent Close waiter returned before owner: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	close(closer.release)
	for range callers {
		if err := <-results; err != nil {
			t.Fatalf("Close returned error: %v", err)
		}
	}
	if calls := closer.calls.Load(); calls != 1 {
		t.Fatalf("expected one resource close, got %d", calls)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("idempotent Close returned error: %v", err)
	}
	if calls := closer.calls.Load(); calls != 1 {
		t.Fatalf("expected idempotent Close not to repeat resource close, got %d", calls)
	}
}

func TestCloseRejectsNewHTTPAdmission(t *testing.T) {
	router := gin.New()
	router.GET("/accepted", func(c *gin.Context) { c.Status(http.StatusNoContent) })
	application := &App{Router: router}
	handler := application.httpHandler()

	before := httptest.NewRecorder()
	handler.ServeHTTP(before, httptest.NewRequest(http.MethodGet, "/accepted", nil))
	if before.Code != http.StatusNoContent {
		t.Fatalf("expected request admission before close, got %d", before.Code)
	}
	if err := application.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}
	after := httptest.NewRecorder()
	handler.ServeHTTP(after, httptest.NewRequest(http.MethodGet, "/accepted", nil))
	if after.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected new HTTP admission rejection after close, got %d", after.Code)
	}
}

func TestRunWaitsForBackgroundOwnerBeforeDatabaseAndLogClose(t *testing.T) {
	application, sqlDB, logCloser := newLifecycleTestApp(t)
	application.Config.AppPort = "0"
	application.Server = &http.Server{Addr: "127.0.0.1:0", Handler: application.Router}
	runner := &resourceOrderRunner{
		started:   make(chan struct{}),
		finished:  make(chan error, 1),
		database:  sqlDB,
		logCloser: logCloser,
	}
	application.Poller = runner
	application.Maintenance = nil
	application.MetadataSync = nil
	application.BackupMaintenance = nil
	application.RollupBackfill = nil

	ctx, cancel := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() { runDone <- application.Run(ctx) }()
	<-runner.started
	cancel()
	if err := <-runDone; err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if err := <-runner.finished; err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Ping(); err == nil {
		t.Fatal("expected database close after runner exit")
	}
	if !logCloser.isClosed() {
		t.Fatal("expected log close after runner exit")
	}
}

func TestAppCloseRejectsQuotaRefreshAdmission(t *testing.T) {
	service := quota.NewServiceWithRegistry(
		lifecycleIdentityLookup{},
		quota.NewProviderRegistry(map[string]quota.ProviderHandler{"claude": lifecycleQuotaHandler{}}),
	)
	application := &App{Quota: service}
	if err := application.Close(); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	response, err := service.Refresh(context.Background(), quota.RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 1})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != 0 || len(response.Rejected) != 1 || response.Rejected[0].Error != "refresh_unavailable" {
		t.Fatalf("expected shutdown quota admission rejection, got %+v", response)
	}
}

func TestHTTPDrainDeadlineStartsBeforeAcceptedQuotaWorkersStop(t *testing.T) {
	quotaStarted := make(chan struct{})
	releaseQuota := make(chan struct{})
	quotaHandler := &blockingLifecycleQuotaHandler{started: quotaStarted, release: releaseQuota}
	service := quota.NewServiceWithRegistry(
		lifecycleIdentityLookup{},
		quota.NewProviderRegistry(map[string]quota.ProviderHandler{"claude": quotaHandler}),
	)
	refresh, err := service.Refresh(context.Background(), quota.RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 1})
	if err != nil || refresh.Accepted != 1 {
		t.Fatalf("expected initial quota refresh admission, response=%+v err=%v", refresh, err)
	}
	<-quotaStarted

	httpStarted := make(chan struct{})
	releaseHTTP := make(chan struct{})
	server, _ := startBlockingHTTPServer(t, httpStarted, releaseHTTP)
	application := &App{Quota: service, Server: server, shutdownTimeout: 25 * time.Millisecond}
	startedAt := time.Now()
	err = application.Close()
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected HTTP drain deadline while quota worker remains accepted, got %v", err)
	}
	if elapsed := time.Since(startedAt); elapsed > 500*time.Millisecond {
		t.Fatalf("HTTP deadline started too late behind quota worker wait: %s", elapsed)
	}
	quotaTask, err := service.GetRefreshTask(context.Background(), refresh.Tasks[0].TaskID)
	if err != nil || quotaTask.Status != quota.RefreshTaskStatusRunning {
		t.Fatalf("expected quota worker to remain running after incomplete HTTP drain, task=%+v err=%v", quotaTask, err)
	}

	close(releaseHTTP)
	retryDone := make(chan error, 1)
	go func() { retryDone <- application.Close() }()
	select {
	case err := <-retryDone:
		t.Fatalf("retry returned before accepted quota worker stopped: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseQuota)
	if err := <-retryDone; err != nil {
		t.Fatalf("retry Close returned error: %v", err)
	}
}

func TestRunAndCloseSerializeBackgroundWaitGroupAdmission(t *testing.T) {
	for index := range 20 {
		cfg := testAppConfig(t)
		cfg.AppPort = "0"
		application := &App{
			Config:          &cfg,
			Router:          gin.New(),
			Server:          &http.Server{Addr: "127.0.0.1:0", Handler: gin.New()},
			Poller:          &cancelOnlyRunner{},
			shutdownTimeout: time.Second,
		}
		ctx, cancel := context.WithCancel(context.Background())
		runDone := make(chan error, 1)
		closeDone := make(chan error, 1)
		go func() { runDone <- application.Run(ctx) }()
		go func() { closeDone <- application.Close() }()
		cancel()
		select {
		case <-runDone:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: Run did not finish", index)
		}
		select {
		case <-closeDone:
		case <-time.After(time.Second):
			t.Fatalf("iteration %d: Close did not finish", index)
		}
	}
}

type recordingCloser struct {
	mu     sync.Mutex
	closed bool
}

func (c *recordingCloser) Close() error {
	c.mu.Lock()
	c.closed = true
	c.mu.Unlock()
	return nil
}

func (c *recordingCloser) isClosed() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closed
}

type blockingCloser struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	calls   atomic.Int32
}

func (c *blockingCloser) Close() error {
	c.calls.Add(1)
	c.once.Do(func() { close(c.entered) })
	<-c.release
	return nil
}

func newLifecycleTestApp(t *testing.T) (*App, *sql.DB, *recordingCloser) {
	t.Helper()
	application, err := NewWithConfig(testAppConfig(t))
	if err != nil {
		t.Fatalf("NewWithConfig returned error: %v", err)
	}
	sqlDB, err := application.DB.DB()
	if err != nil {
		t.Fatalf("load sql db: %v", err)
	}
	originalLogCloser := application.LogCloser
	t.Cleanup(func() { _ = originalLogCloser.Close() })
	logCloser := &recordingCloser{}
	application.LogCloser = logCloser
	return application, sqlDB, logCloser
}

func startBlockingHTTPServer(t *testing.T, started chan struct{}, release <-chan struct{}) (*http.Server, <-chan error) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		close(started)
		<-release
		_, _ = io.WriteString(w, "ok")
	})}
	serveDone := make(chan error, 1)
	go func() { serveDone <- server.Serve(listener) }()
	requestDone := make(chan error, 1)
	go func() {
		response, err := http.Get("http://" + listener.Addr().String())
		if err == nil {
			_ = response.Body.Close()
		}
		requestDone <- err
	}()
	select {
	case <-started:
	case err := <-requestDone:
		t.Fatalf("request ended before handler started: %v", err)
	case <-time.After(time.Second):
		t.Fatal("blocking handler did not start")
	}
	t.Cleanup(func() {
		_ = server.Close()
		select {
		case <-serveDone:
		default:
		}
	})
	return server, serveDone
}

type resourceOrderRunner struct {
	started   chan struct{}
	finished  chan error
	database  *sql.DB
	logCloser *recordingCloser
}

func (r *resourceOrderRunner) Run(ctx context.Context) error {
	close(r.started)
	<-ctx.Done()
	if err := r.database.Ping(); err != nil {
		r.finished <- fmt.Errorf("database closed before runner exit: %w", err)
		return nil
	}
	if r.logCloser.isClosed() {
		r.finished <- errors.New("log resource closed before runner exit")
		return nil
	}
	r.finished <- nil
	return nil
}

func (r *resourceOrderRunner) Status() poller.Status { return poller.Status{} }
func (r *resourceOrderRunner) SyncNow(context.Context) (poller.ManualSyncOutcome, error) {
	return poller.ManualSyncOutcome{}, nil
}

type cancelOnlyRunner struct{}

func (r *cancelOnlyRunner) Run(ctx context.Context) error {
	<-ctx.Done()
	return nil
}
func (r *cancelOnlyRunner) Status() poller.Status { return poller.Status{} }
func (r *cancelOnlyRunner) SyncNow(context.Context) (poller.ManualSyncOutcome, error) {
	return poller.ManualSyncOutcome{}, nil
}

type lifecycleIdentityLookup struct{}

func (lifecycleIdentityLookup) FindActiveAuthFileIdentity(context.Context, string) (entities.UsageIdentity, bool, error) {
	return entities.UsageIdentity{Identity: "auth-1", Provider: "claude", Type: "auth-file", AuthType: entities.UsageIdentityAuthTypeAuthFile}, true, nil
}

func (lifecycleIdentityLookup) HasActiveIdentity(context.Context, string) (bool, error) {
	return true, nil
}

type lifecycleQuotaHandler struct{}

func (lifecycleQuotaHandler) Check(context.Context, quota.ProviderInput) (quota.ProviderOutput, error) {
	return quota.ProviderOutput{}, nil
}

type blockingLifecycleQuotaHandler struct {
	started chan struct{}
	release <-chan struct{}
	once    sync.Once
}

func (h *blockingLifecycleQuotaHandler) Check(context.Context, quota.ProviderInput) (quota.ProviderOutput, error) {
	h.once.Do(func() { close(h.started) })
	<-h.release
	return quota.ProviderOutput{}, nil
}

type appRunStub struct {
	started chan struct{}
}

func (s *appRunStub) Run(context.Context) error {
	close(s.started)
	return nil
}

func (s *appRunStub) Status() poller.Status {
	return poller.Status{}
}

func (s *appRunStub) SyncNow(context.Context) (poller.ManualSyncOutcome, error) {
	return poller.ManualSyncOutcome{}, nil
}

func captureAppInfoLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var logs bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
	t.Cleanup(func() {
		slog.SetDefault(previous)
	})
	return &logs
}

func testAppConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		AppPort:                "8080",
		CPABaseURL:             "https://cpa.example.com",
		CPAManagementKey:       "secret",
		RedisQueueIdleInterval: time.Second,
		RedisQueueErrorBackoff: 10 * time.Second,
		MetadataSyncInterval:   30 * time.Second,
		SQLitePath:             t.TempDir() + "/app.db",
		BackupEnabled:          true,
		BackupDir:              t.TempDir() + "/backups",
		BackupRetentionDays:    7,
		RequestTimeout:         5 * time.Second,
		LogLevel:               "info",
		LogFileEnabled:         false,
		LogRetentionDays:       7,
	}
}
