package poller

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	servicedto "cpa-usage/internal/service/dto"
)

// Redis inbox 处理频率固定为 5 秒：拉取任务只负责把 Redis 原始消息落库，处理任务按这个间隔独立消费本地 inbox。
const redisInboxProcessInterval = 5 * time.Second

type RedisBatchSyncer interface {
	PullRedisUsageInbox(ctx context.Context) (*servicedto.RedisInboxPullResult, error)
	ProcessRedisUsageInbox(ctx context.Context) (*servicedto.RedisBatchSyncResult, error)
}

type RedisDrainConfig struct {
	IdleInterval time.Duration
	ErrorBackoff time.Duration
}

type RedisDrain struct {
	syncer RedisBatchSyncer
	config RedisDrainConfig
	now    func() time.Time
	sleep  func(context.Context, time.Duration) bool

	mu             sync.Mutex
	running        bool
	manualRunning  bool
	pullRunning    bool
	processRunning bool

	processedEventsTotal  atomic.Int64
	processedBatchesTotal atomic.Int64
	lastProcessedAt       atomic.Int64 // unix nano
}

func NewRedisDrain(syncer RedisBatchSyncer, cfg RedisDrainConfig) *RedisDrain {
	return &RedisDrain{
		syncer: syncer,
		config: cfg,
		now:    time.Now,
		sleep:  sleepContext,
	}
}

// Run 启动 Redis 连续同步：一个 goroutine 只执行 Pull，另一个 goroutine 只执行 Process，二者互不串行等待。
func (d *RedisDrain) Run(ctx context.Context) error {
	if err := d.validate(); err != nil {
		return err
	}
	d.setRunning(true)
	defer d.setRunning(false)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		d.runPullLoop(ctx)
	}()
	go func() {
		defer wg.Done()
		d.runProcessLoop(ctx)
	}()
	<-ctx.Done()
	wg.Wait()
	return nil
}

// runPullLoop 只从 CPA Redis 队列 LPOP 数据并写入 redis_usage_inboxes，不解码、不写 usage_events。
func (d *RedisDrain) runPullLoop(ctx context.Context) {
	slog.Info("redis inbox pull task started", "idle_interval", d.config.IdleInterval.String())
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		result, err := d.runRedisPull(ctx)
		if err != nil {
			if shouldLogSyncError(err) {
				slog.Error("redis drain pull failed", "error", err)
			}
			if !d.sleep(ctx, d.config.ErrorBackoff) {
				return
			}
			continue
		}
		if result != nil && result.Empty {
			if !d.sleep(ctx, d.config.IdleInterval) {
				return
			}
		}
	}
}

// runProcessLoop 固定每 5 秒处理已落库的 inbox 行，失败行保留为可重试状态，坏消息单独标记不阻塞后续行。
func (d *RedisDrain) runProcessLoop(ctx context.Context) {
	slog.Info("redis inbox process task started", "interval", redisInboxProcessInterval.String())
	for {
		if !d.sleep(ctx, redisInboxProcessInterval) {
			return
		}
		result, err := d.runRedisProcess(ctx)
		if err != nil && !errors.Is(err, ErrSyncCompletedWithWarnings) {
			if shouldLogSyncError(err) {
				d.logBatchFailure(result, err)
			}
			continue
		}
	}
}

func (d *RedisDrain) logBatchFailure(result *servicedto.RedisBatchSyncResult, err error) {
	status := ""
	empty := false
	insertedEvents := 0
	dedupedEvents := 0
	if result != nil {
		status = result.Status
		empty = result.Empty
		insertedEvents = result.InsertedEvents
		dedupedEvents = result.DedupedEvents
	}
	slog.Error("redis drain batch failed",
		"error", err,
		"status", status,
		"empty", empty,
		"inserted_events", insertedEvents,
		"deduped_events", dedupedEvents,
	)
}

func (d *RedisDrain) Status() Status {
	d.mu.Lock()
	defer d.mu.Unlock()
	return Status{
		Running:     d.running,
		SyncRunning: d.manualRunning || d.pullRunning || d.processRunning,
	}
}

// SyncNow 是手动同步入口：Redis 模式下先 Pull 一次再 Process 一次，保持用户手动触发时立即看到新数据的直觉。
// admission 与后台 Pull/Process 的占用检查在同一把锁内完成；只有同时预留两个
// stage 后才允许第一个远端 Pop 副作用发生。
func (d *RedisDrain) SyncNow(ctx context.Context) (ManualSyncOutcome, error) {
	if err := d.validate(); err != nil {
		return ManualSyncOutcome{Status: "failed", Error: err.Error()}, err
	}
	if err := d.beginManualSync(); err != nil {
		return ManualSyncOutcome{}, err
	}
	defer d.endManualSync()

	pullResult, err := d.syncer.PullRedisUsageInbox(ctx)
	if err != nil {
		outcome := manualPullOutcome(pullResult, err)
		return outcome, err
	}
	processResult, err := d.syncer.ProcessRedisUsageInbox(ctx)
	returnErr := err
	if err != nil && processResult != nil && processResult.Status != "" && processResult.Status != "failed" {
		returnErr = fmt.Errorf("%w: %v", ErrSyncCompletedWithWarnings, err)
	}
	d.recordProcessMetrics(processResult)
	outcome := manualProcessOutcome(processResult, err)
	return outcome, returnErr
}

func (d *RedisDrain) beginManualSync() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.manualRunning || d.pullRunning || d.processRunning {
		return ErrSyncAlreadyRunning
	}
	d.manualRunning = true
	// Reserve both stages for the whole command so background work cannot enter
	// between the manual Pull and Process operations.
	d.pullRunning = true
	d.processRunning = true
	return nil
}

func (d *RedisDrain) endManualSync() {
	d.mu.Lock()
	d.manualRunning = false
	d.pullRunning = false
	d.processRunning = false
	d.mu.Unlock()
}

func manualPullOutcome(result *servicedto.RedisInboxPullResult, err error) ManualSyncOutcome {
	status := "failed"
	if result != nil && result.Status != "" {
		status = result.Status
	}
	outcome := ManualSyncOutcome{Status: status}
	if err != nil {
		outcome.Error = err.Error()
	}
	return outcome
}

func manualProcessOutcome(result *servicedto.RedisBatchSyncResult, err error) ManualSyncOutcome {
	status := ""
	if result != nil && result.Status != "" {
		status = result.Status
	}
	if status == "" {
		if err != nil {
			status = "failed"
		} else {
			status = "completed"
		}
	}
	outcome := ManualSyncOutcome{Status: status}
	if err == nil {
		return outcome
	}
	if status != "failed" {
		outcome.Warning = err.Error()
		return outcome
	}
	outcome.Error = err.Error()
	return outcome
}

// runRedisPull 只防止 Pull 自身重入，不阻塞 Process；这样 Redis 长轮询或退避不会跳过本地 inbox 处理周期。
func (d *RedisDrain) runRedisPull(ctx context.Context) (*servicedto.RedisInboxPullResult, error) {
	d.mu.Lock()
	if d.pullRunning {
		d.mu.Unlock()
		return nil, ErrSyncAlreadyRunning
	}
	d.pullRunning = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.pullRunning = false
		d.mu.Unlock()
	}()

	result, err := d.syncer.PullRedisUsageInbox(ctx)
	return result, err
}

// runRedisProcess 只防止 Process 自身重入，不阻塞 Pull；Process 的输入必须来自已持久化的 redis_usage_inboxes。
func (d *RedisDrain) runRedisProcess(ctx context.Context) (*servicedto.RedisBatchSyncResult, error) {
	d.mu.Lock()
	if d.processRunning {
		d.mu.Unlock()
		return nil, ErrSyncAlreadyRunning
	}
	d.processRunning = true
	d.mu.Unlock()

	defer func() {
		d.mu.Lock()
		d.processRunning = false
		d.mu.Unlock()
	}()

	result, err := d.syncer.ProcessRedisUsageInbox(ctx)
	returnErr := err
	if err != nil && result != nil && result.Status != "" && result.Status != "failed" {
		returnErr = fmt.Errorf("%w: %v", ErrSyncCompletedWithWarnings, err)
	}
	d.recordProcessMetrics(result)
	return result, returnErr
}

// ProcessMetrics 是 Redis inbox 处理的累计吞吐快照，供运行时指标推导处理速率。
type ProcessMetrics struct {
	EventsTotal     int64
	BatchesTotal    int64
	LastProcessedAt time.Time
}

// ProcessMetricsProvider 由 RedisDrain 实现；App 通过类型断言读取，不扩展现有 Runner 接口。
type ProcessMetricsProvider interface {
	ProcessMetrics() ProcessMetrics
}

func (d *RedisDrain) ProcessMetrics() ProcessMetrics {
	lastProcessedAt := time.Time{}
	if unixNano := d.lastProcessedAt.Load(); unixNano != 0 {
		lastProcessedAt = time.Unix(0, unixNano).UTC()
	}
	return ProcessMetrics{
		EventsTotal:     d.processedEventsTotal.Load(),
		BatchesTotal:    d.processedBatchesTotal.Load(),
		LastProcessedAt: lastProcessedAt,
	}
}

// recordProcessMetrics 累计已落库事件的批次数与事件数；
// 失败批次与空批次（无待处理行）不计入，避免把重试或空闲轮询误算为吞吐。
func (d *RedisDrain) recordProcessMetrics(result *servicedto.RedisBatchSyncResult) {
	if result == nil || result.Status == "failed" {
		return
	}
	events := int64(result.InsertedEvents + result.DedupedEvents)
	if events == 0 {
		return
	}
	d.processedBatchesTotal.Add(1)
	d.processedEventsTotal.Add(events)
	d.lastProcessedAt.Store(d.now().UnixNano())
}

func (d *RedisDrain) setRunning(running bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.running = running
}

func (d *RedisDrain) validate() error {
	if d == nil {
		return fmt.Errorf("redis drain is nil")
	}
	if d.syncer == nil {
		return fmt.Errorf("redis drain syncer is nil")
	}
	if d.config.IdleInterval <= 0 {
		return fmt.Errorf("redis drain idle interval must be greater than zero")
	}
	if d.config.ErrorBackoff <= 0 {
		return fmt.Errorf("redis drain error backoff must be greater than zero")
	}
	if d.now == nil {
		d.now = time.Now
	}
	if d.sleep == nil {
		d.sleep = sleepContext
	}
	return nil
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
