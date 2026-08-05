package quota

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type RefreshTaskStatus string

const (
	RefreshTaskStatusQueued    RefreshTaskStatus = "queued"
	RefreshTaskStatusRunning   RefreshTaskStatus = "running"
	RefreshTaskStatusCompleted RefreshTaskStatus = "completed"
	RefreshTaskStatusFailed    RefreshTaskStatus = "failed"
	refreshUnavailableCode                       = "refresh_unavailable"
)

type CacheRequest struct {
	AuthIndexes []string `json:"auth_indexes"`
	Limit       int      `json:"limit"`
}

type CacheResponse struct {
	Items []CheckResponse `json:"items"`
}

type RefreshRequest struct {
	AuthIndexes []string `json:"auth_indexes"`
	Limit       int      `json:"limit"`
}

type RefreshResponse struct {
	Tasks    []RefreshTaskID            `json:"tasks"`
	Rejected []RefreshRejectedAuthIndex `json:"rejected"`
	Accepted int                        `json:"accepted"`
	Skipped  int                        `json:"skipped"`
	Limit    int                        `json:"limit"`
}

type RefreshTaskID struct {
	AuthIndex string `json:"authIndex"`
	TaskID    string `json:"taskId"`
}

type RefreshRejectedAuthIndex struct {
	AuthIndex string `json:"authIndex"`
	Error     string `json:"error"`
}

type RefreshTaskResponse struct {
	TaskID    string            `json:"taskId"`
	AuthIndex string            `json:"authIndex"`
	Status    RefreshTaskStatus `json:"status"`
	Quota     *CheckResponse    `json:"quota,omitempty"`
	Error     string            `json:"error,omitempty"`
	CachedAt  *time.Time        `json:"cachedAt,omitempty"`
	ExpiresAt *time.Time        `json:"expiresAt,omitempty"`
}

type RefreshTaskRecord struct {
	TaskID     string
	AuthIndex  string
	Status     RefreshTaskStatus
	Quota      *CheckResponse
	Error      string
	CreatedAt  time.Time
	StartedAt  time.Time
	FinishedAt time.Time
	CachedAt   time.Time
	ExpiresAt  time.Time
}

// AttachRefreshWorkerLifecycle 把 refresh worker 的父 context 绑定到应用生命周期：
// 应用关停时由 StopRefreshWorkers 取消内部派生的 worker context，进行中的 provider 调用与排队中的 worker 都会随之退出。
func (s *Service) AttachRefreshWorkerLifecycle(ctx context.Context) {
	s.refreshWorkerMu.Lock()
	defer s.refreshWorkerMu.Unlock()
	s.refreshWorkerCtx, s.refreshWorkerCancel = context.WithCancel(ctx)
}

// StopRefreshWorkers 拒绝新的 refresh worker 并等待进行中的 worker 退出。
// 关闭标志与 worker context 取消在同一把锁下完成，与 startRefreshTask 的接收检查互斥：
// 一旦开始关闭，后续接收必然被拒绝，不会返回 Accepted 但无法执行的任务。
func (s *Service) StopRefreshWorkers() {
	s.refreshWorkerMu.Lock()
	s.refreshWorkersClose = true
	if s.refreshWorkerCancel != nil {
		s.refreshWorkerCancel()
	}
	s.refreshWorkerMu.Unlock()
	s.refreshWorkerWG.Wait()
}

func (s *Service) GetCachedQuota(ctx context.Context, request CacheRequest) (CacheResponse, error) {
	_ = ctx
	// 缓存读取只返回已完成任务的结果，不触发新的 provider 请求。
	limit := request.Limit
	if limit <= 0 {
		return CacheResponse{}, fmt.Errorf("%w: limit is required", ErrValidation)
	}
	response := CacheResponse{Items: make([]CheckResponse, 0, min(limit, len(request.AuthIndexes)))}
	s.refreshTasks.cleanupExpired(time.Now())
	// 按请求顺序去重并读取每个 auth_index 最近一次完成的任务缓存。
	seen := make(map[string]struct{}, len(request.AuthIndexes))
	for _, rawAuthIndex := range request.AuthIndexes {
		if len(response.Items) >= limit {
			break
		}
		authIndex := strings.TrimSpace(rawAuthIndex)
		if authIndex == "" {
			continue
		}
		if _, ok := seen[authIndex]; ok {
			continue
		}
		seen[authIndex] = struct{}{}
		task, ok := s.refreshTasks.latestCompleted(authIndex)
		if !ok {
			continue
		}
		response.Items = append(response.Items, *task.Quota)
	}
	return response, nil
}

func (s *Service) Refresh(ctx context.Context, request RefreshRequest) (RefreshResponse, error) {
	// 刷新入口只负责校验、去重、建任务；实际 provider 调用交给后台 worker。
	limit := request.Limit
	if limit <= 0 {
		return RefreshResponse{}, fmt.Errorf("%w: limit is required", ErrValidation)
	}
	response := RefreshResponse{Limit: limit}
	seen := make(map[string]struct{}, len(request.AuthIndexes))
	s.refreshTasks.cleanupExpired(time.Now())

	for _, rawAuthIndex := range request.AuthIndexes {
		// 每个 auth_index 独立生成任务，便于前端逐行轮询和展示错误。
		authIndex := strings.TrimSpace(rawAuthIndex)
		if authIndex == "" {
			response.Rejected = append(response.Rejected, RefreshRejectedAuthIndex{AuthIndex: authIndex, Error: "invalid"})
			continue
		}
		if _, ok := seen[authIndex]; ok {
			response.Rejected = append(response.Rejected, RefreshRejectedAuthIndex{AuthIndex: authIndex, Error: "duplicate"})
			continue
		}
		seen[authIndex] = struct{}{}
		if response.Accepted >= limit {
			response.Rejected = append(response.Rejected, RefreshRejectedAuthIndex{AuthIndex: authIndex, Error: "invalid"})
			continue
		}
		if rejection, err := s.validateRefreshAuthIndex(ctx, authIndex); err != nil {
			return RefreshResponse{}, err
		} else if rejection != "" {
			response.Rejected = append(response.Rejected, RefreshRejectedAuthIndex{AuthIndex: authIndex, Error: rejection})
			continue
		}

		task, rejection := s.startRefreshTask(authIndex)
		if rejection != "" {
			response.Rejected = append(response.Rejected, RefreshRejectedAuthIndex{AuthIndex: authIndex, Error: rejection})
			continue
		}
		response.Tasks = append(response.Tasks, RefreshTaskID{AuthIndex: authIndex, TaskID: task.TaskID})
		response.Accepted++
	}
	response.Skipped = len(response.Rejected)
	return response, nil
}

func (s *Service) GetRefreshTask(ctx context.Context, taskID string) (RefreshTaskResponse, error) {
	_ = ctx
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return RefreshTaskResponse{}, fmt.Errorf("%w: task_id is required", ErrValidation)
	}
	s.refreshTasks.cleanupExpired(time.Now())
	task, ok := s.refreshTasks.snapshot(taskID)
	if !ok {
		return RefreshTaskResponse{}, ErrTaskNotFound
	}
	return task.response(), nil
}

func (s *Service) validateRefreshAuthIndex(ctx context.Context, authIndex string) (string, error) {
	// 先按 auth-file 身份查找；查不到时再区分“非 auth file”和“不存在”。
	identity, found, err := s.identityLookup.FindActiveAuthFileIdentity(ctx, authIndex)
	if err != nil {
		return "", err
	}
	if found {
		if _, _, ok := s.resolveQuotaHandler(identity.Provider, identity.Type); !ok {
			return "unsupported", nil
		}
		return "", nil
	}

	active, err := s.identityLookup.HasActiveIdentity(ctx, authIndex)
	if err != nil {
		return "", err
	}
	if active {
		return "not_auth_file", nil
	}
	return "not_found", nil
}

// startRefreshTask 原子地接收任务并启动后台 worker。
// 任务创建、worker 计数与关闭标志由同一把生命周期锁协调，保证返回 Accepted 的任务一定有 worker 接管。
func (s *Service) startRefreshTask(authIndex string) (*RefreshTaskRecord, string) {
	s.refreshWorkerMu.Lock()
	if s.refreshWorkersClose || s.refreshWorkerCtx.Err() != nil {
		s.refreshWorkerMu.Unlock()
		return nil, refreshUnavailableCode
	}
	task, created := s.refreshTasks.enqueue(authIndex)
	if !created {
		s.refreshWorkerMu.Unlock()
		return task, "duplicate"
	}
	s.refreshWorkerWG.Add(1)
	s.refreshWorkerMu.Unlock()
	go func() {
		defer s.refreshWorkerWG.Done()
		s.runRefreshTask(task.TaskID)
	}()
	return task, ""
}

func (s *Service) runRefreshTask(taskID string) {
	s.refreshWorkerMu.Lock()
	workerCtx := s.refreshWorkerCtx
	s.refreshWorkerMu.Unlock()

	// worker token 控制全局并发，防止一次批量刷新同时压垮 CPA/上游接口。
	select {
	case s.refreshWorkerTokens <- struct{}{}:
		defer func() { <-s.refreshWorkerTokens }()
	case <-workerCtx.Done():
		s.refreshTasks.markFailed(taskID, refreshTaskErrorMessage(workerCtx.Err()))
		return
	}

	authIndex, ok := s.refreshTasks.markRunning(taskID)
	if !ok {
		return
	}
	// 每个任务独立设置超时；超时或 provider 错误都会沉淀到任务状态里给前端展示。
	ctx, cancel := context.WithTimeout(workerCtx, defaultRefreshTaskTimeout)
	defer cancel()
	response, err := s.Check(ctx, CheckRequest{AuthIndex: authIndex})
	if err != nil {
		s.refreshTasks.markFailed(taskID, refreshTaskErrorMessage(err))
		return
	}
	s.refreshTasks.markCompleted(taskID, response)
}

func refreshTaskErrorMessage(err error) string {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "Quota refresh timed out. Please try again later."
	}
	if errors.Is(err, ErrProviderInput) {
		return ProviderInputErrorMessage(err, "Quota request is missing required parameters.")
	}
	if strings.HasPrefix(err.Error(), "HTTP ") {
		return err.Error()
	}
	return "Quota refresh failed. Please try again later."
}

func (t *RefreshTaskRecord) isActive() bool {
	return t.Status == RefreshTaskStatusQueued || t.Status == RefreshTaskStatusRunning
}

func (t *RefreshTaskRecord) response() RefreshTaskResponse {
	response := RefreshTaskResponse{
		TaskID:    t.TaskID,
		AuthIndex: t.AuthIndex,
		Status:    t.Status,
		Error:     t.Error,
	}
	if t.Quota != nil {
		quota := *t.Quota
		response.Quota = &quota
	}
	if !t.CachedAt.IsZero() {
		cachedAt := t.CachedAt
		response.CachedAt = &cachedAt
	}
	if !t.ExpiresAt.IsZero() {
		expiresAt := t.ExpiresAt
		response.ExpiresAt = &expiresAt
	}
	return response
}
