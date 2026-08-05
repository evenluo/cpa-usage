package quota

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"cpa-usage/internal/entities"
)

type refreshHandlerStub struct {
	mu     sync.Mutex
	calls  []string
	block  <-chan struct{}
	output ProviderOutput
	err    error
}

func (s *refreshHandlerStub) Check(ctx context.Context, input ProviderInput) (ProviderOutput, error) {
	if s.block != nil {
		select {
		case <-ctx.Done():
			return ProviderOutput{}, ctx.Err()
		case <-s.block:
		}
	}
	s.mu.Lock()
	s.calls = append(s.calls, input.Identity.Identity)
	s.mu.Unlock()
	if s.err != nil {
		return ProviderOutput{}, s.err
	}
	return s.output, nil
}

func (s *refreshHandlerStub) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.calls)
}

// fakeAuthFileIdentityLookup 以内存身份表实现 AuthFileIdentityLookup，测试不再需要真实数据库。
type fakeAuthFileIdentityLookup struct {
	identities map[string]entities.UsageIdentity
}

func (f fakeAuthFileIdentityLookup) FindActiveAuthFileIdentity(_ context.Context, authIndex string) (entities.UsageIdentity, bool, error) {
	identity, ok := f.identities[strings.TrimSpace(authIndex)]
	if !ok || identity.AuthType != entities.UsageIdentityAuthTypeAuthFile {
		return entities.UsageIdentity{}, false, nil
	}
	return identity, true, nil
}

func (f fakeAuthFileIdentityLookup) HasActiveIdentity(_ context.Context, authIndex string) (bool, error) {
	_, ok := f.identities[authIndex]
	return ok, nil
}

func newRefreshTestService(identities map[string]entities.UsageIdentity, handler ProviderHandler) *Service {
	return NewServiceWithRegistry(fakeAuthFileIdentityLookup{identities: identities}, NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))
}

func claudeAuthFileIdentity(authIndex string) entities.UsageIdentity {
	return entities.UsageIdentity{Identity: authIndex, Provider: "claude", Type: "auth-file", AuthType: entities.UsageIdentityAuthTypeAuthFile}
}

func TestRefreshCreatesTaskPerAuthIndexAndCachesCompletedQuota(t *testing.T) {
	handler := &refreshHandlerStub{output: ProviderOutput{Result: ClaudeResult{Usage: &ClaudeUsagePayload{FiveHour: &ClaudeUsageWindow{Utilization: 25}}}}}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != 1 || response.Skipped != 0 || len(response.Tasks) != 1 {
		t.Fatalf("unexpected refresh response: %+v", response)
	}

	task := waitForRefreshTask(t, service, response.Tasks[0].TaskID, RefreshTaskStatusCompleted)
	if task.AuthIndex != "auth-1" || task.Quota == nil || task.Quota.ID != "auth-1" || len(task.Quota.Quota) != 1 {
		t.Fatalf("expected completed task to expose cached quota, got %+v", task)
	}
	if handler.callCount() != 1 {
		t.Fatalf("expected one provider call, got %d", handler.callCount())
	}
}

func TestRefreshKeepsLatestCompletedCacheWhileNewRefreshRuns(t *testing.T) {
	block := make(chan struct{})
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertCachedUsagePercent(t, service, "auth-1", 25)

	handler.output = claudeUsageOutput(80)
	handler.block = block
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusRunning)

	assertCachedUsagePercent(t, service, "auth-1", 25)
	if handler.callCount() != 1 {
		t.Fatalf("cache lookup should not trigger provider calls while refresh is running, got %d", handler.callCount())
	}
	close(block)
	waitForRefreshTask(t, service, second, RefreshTaskStatusCompleted)
	assertCachedUsagePercent(t, service, "auth-1", 80)
}

func TestRefreshFailureKeepsLatestCompletedCache(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertCachedUsagePercent(t, service, "auth-1", 25)

	handler.err = errors.New("upstream exploded")
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusFailed)

	assertCachedUsagePercent(t, service, "auth-1", 25)
	if handler.callCount() != 2 {
		t.Fatalf("expected one failed refresh provider call and no cache provider calls, got %d", handler.callCount())
	}
}

func TestRefreshSuccessReplacesLatestCompletedCache(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertCachedUsagePercent(t, service, "auth-1", 25)

	handler.output = claudeUsageOutput(80)
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusCompleted)

	assertCachedUsagePercent(t, service, "auth-1", 80)
	if handler.callCount() != 2 {
		t.Fatalf("expected two refresh provider calls and no cache provider calls, got %d", handler.callCount())
	}
}

func TestRefreshCleanupRemovesExpiredLatestCompletedCache(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	taskID := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, taskID, RefreshTaskStatusCompleted)
	assertCachedUsagePercent(t, service, "auth-1", 25)

	service.refreshTasks.mu.Lock()
	service.refreshTasks.tasks[taskID].ExpiresAt = time.Now().Add(-time.Second)
	service.refreshTasks.mu.Unlock()

	cache, err := service.GetCachedQuota(context.Background(), CacheRequest{AuthIndexes: []string{"auth-1"}, Limit: 1})
	if err != nil {
		t.Fatalf("GetCachedQuota returned error: %v", err)
	}
	if len(cache.Items) != 0 {
		t.Fatalf("expected expired cache to be removed, got %+v", cache.Items)
	}
	service.refreshTasks.mu.Lock()
	_, latestOK := service.refreshTasks.latestCompletedTaskIDsByAuth["auth-1"]
	_, activeOK := service.refreshTasks.activeTaskIDsByAuth["auth-1"]
	service.refreshTasks.mu.Unlock()
	if latestOK || activeOK {
		t.Fatalf("expected expired task indexes to be removed, latest=%v active=%v", latestOK, activeOK)
	}
	if handler.callCount() != 1 {
		t.Fatalf("cache cleanup should not trigger provider calls, got %d", handler.callCount())
	}
}

func TestRefreshRejectsInvalidEntriesAndIgnoresRunningTask(t *testing.T) {
	block := make(chan struct{})
	handler := &refreshHandlerStub{block: block, output: ProviderOutput{Result: ClaudeResult{Usage: &ClaudeUsagePayload{FiveHour: &ClaudeUsageWindow{Utilization: 25}}}}}
	service := newRefreshTestService(map[string]entities.UsageIdentity{
		"auth-1":     claudeAuthFileIdentity("auth-1"),
		"provider-1": {Identity: "provider-1", Provider: "openai", Type: "openai", AuthType: entities.UsageIdentityAuthTypeAIProvider},
	}, handler)

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1", "auth-1", "provider-1", "deleted-1", "missing"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != 1 || response.Skipped != 4 || len(response.Tasks) != 1 || len(response.Rejected) != 4 {
		t.Fatalf("unexpected refresh response: %+v", response)
	}
	if !hasRefreshRejection(response.Rejected, "auth-1", "duplicate") || !hasRefreshRejection(response.Rejected, "provider-1", "not_auth_file") || !hasRefreshRejection(response.Rejected, "deleted-1", "not_found") || !hasRefreshRejection(response.Rejected, "missing", "not_found") {
		t.Fatalf("unexpected rejected entries: %+v", response.Rejected)
	}

	firstTaskID := response.Tasks[0].TaskID
	waitForRefreshTask(t, service, firstTaskID, RefreshTaskStatusRunning)
	second, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 20})
	if err != nil {
		t.Fatalf("second Refresh returned error: %v", err)
	}
	if second.Accepted != 0 || second.Skipped != 1 || len(second.Tasks) != 0 || !hasRefreshRejection(second.Rejected, "auth-1", "duplicate") {
		t.Fatalf("expected running task to be ignored as duplicate, got %+v", second)
	}
	close(block)
	waitForRefreshTask(t, service, firstTaskID, RefreshTaskStatusCompleted)
	if handler.callCount() != 1 {
		t.Fatalf("expected duplicate refresh to reuse provider call, got %d", handler.callCount())
	}
}

func TestRefreshQueueUsesFiveWorkersAndTwentySecondTimeout(t *testing.T) {
	if defaultRefreshWorkerLimit != 5 {
		t.Fatalf("expected refresh worker limit 5, got %d", defaultRefreshWorkerLimit)
	}
	if defaultRefreshTaskTimeout != 20*time.Second {
		t.Fatalf("expected refresh task timeout 20s, got %s", defaultRefreshTaskTimeout)
	}
}

func TestStopRefreshWorkersCancelsQueuedAndRunningWorkers(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	handler := &refreshHandlerStub{block: block, output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{
		"auth-1": claudeAuthFileIdentity("auth-1"),
		"auth-2": claudeAuthFileIdentity("auth-2"),
	}, handler)

	workerCtx, cancel := context.WithCancel(context.Background())
	service.AttachRefreshWorkerLifecycle(workerCtx)

	taskID := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, taskID, RefreshTaskStatusRunning)

	cancel()
	duringShutdown, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-2"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh after lifecycle cancellation returned error: %v", err)
	}
	if duringShutdown.Accepted != 0 || duringShutdown.Skipped != 1 || len(duringShutdown.Tasks) != 0 || !hasRefreshRejection(duringShutdown.Rejected, "auth-2", "refresh_unavailable") {
		t.Fatalf("expected cancelled refresh lifecycle to reject new tasks, got %+v", duringShutdown)
	}

	stopped := make(chan struct{})
	go func() {
		service.StopRefreshWorkers()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("StopRefreshWorkers did not return after worker context cancellation")
	}

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh after StopRefreshWorkers returned error: %v", err)
	}
	if response.Accepted != 0 || response.Skipped != 1 || len(response.Tasks) != 0 || !hasRefreshRejection(response.Rejected, "auth-1", "refresh_unavailable") {
		t.Fatalf("expected stopped refresh workers to reject new tasks, got %+v", response)
	}
}

func TestStopRefreshWorkersCancelsWorkersWithoutExternalCancel(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	handler := &refreshHandlerStub{block: block, output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.AttachRefreshWorkerLifecycle(workerCtx)

	taskID := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, taskID, RefreshTaskStatusRunning)

	// 关闭标志与 worker context 取消都在 StopRefreshWorkers 内部完成，调用方无需先取消 ctx。
	stopped := make(chan struct{})
	go func() {
		service.StopRefreshWorkers()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(2 * time.Second):
		t.Fatal("StopRefreshWorkers did not return after internal worker context cancellation")
	}

	task, err := service.GetRefreshTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetRefreshTask returned error: %v", err)
	}
	if task.Status != RefreshTaskStatusFailed {
		t.Fatalf("expected running task to be failed after stop, got %+v", task)
	}

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh after StopRefreshWorkers returned error: %v", err)
	}
	if response.Accepted != 0 || response.Skipped != 1 || len(response.Tasks) != 0 || !hasRefreshRejection(response.Rejected, "auth-1", "refresh_unavailable") {
		t.Fatalf("expected stopped refresh workers to reject new tasks, got %+v", response)
	}
}

func TestStopRefreshWorkersMarksQueuedTasksFailed(t *testing.T) {
	block := make(chan struct{})
	defer close(block)
	handler := &refreshHandlerStub{block: block, output: claudeUsageOutput(25)}
	identities := make(map[string]entities.UsageIdentity, defaultRefreshWorkerLimit+1)
	authIndexes := make([]string, 0, defaultRefreshWorkerLimit+1)
	for index := 0; index <= defaultRefreshWorkerLimit; index++ {
		authIndex := fmt.Sprintf("auth-%d", index)
		identities[authIndex] = claudeAuthFileIdentity(authIndex)
		authIndexes = append(authIndexes, authIndex)
	}
	service := newRefreshTestService(identities, handler)

	workerCtx, cancel := context.WithCancel(context.Background())
	service.AttachRefreshWorkerLifecycle(workerCtx)
	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: authIndexes, Limit: len(authIndexes)})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != len(authIndexes) {
		t.Fatalf("expected all refresh tasks to be accepted, got %+v", response)
	}

	waitForRefreshTaskCounts(t, service, response.Tasks, defaultRefreshWorkerLimit, 1)
	cancel()
	service.StopRefreshWorkers()

	for _, accepted := range response.Tasks {
		task, err := service.GetRefreshTask(context.Background(), accepted.TaskID)
		if err != nil {
			t.Fatalf("GetRefreshTask(%s) returned error: %v", accepted.TaskID, err)
		}
		if task.Status != RefreshTaskStatusFailed || task.ExpiresAt == nil {
			t.Fatalf("expected stopped task %s to be failed with an expiry, got %+v", accepted.TaskID, task)
		}
	}
}

func TestRefreshTaskFailureReturnsFriendlyMessage(t *testing.T) {
	handler := &refreshHandlerStub{err: errors.New("upstream exploded")}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-1"}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	task := waitForRefreshTask(t, service, response.Tasks[0].TaskID, RefreshTaskStatusFailed)
	if task.Error != "Quota refresh failed. Please try again later." {
		t.Fatalf("expected friendly error message, got %q", task.Error)
	}
}

func claudeUsageOutput(usedPercent float64) ProviderOutput {
	return ProviderOutput{Result: ClaudeResult{Usage: &ClaudeUsagePayload{FiveHour: &ClaudeUsageWindow{Utilization: usedPercent}}}}
}

func refreshAuthIndex(t *testing.T, service *Service, authIndex string) string {
	t.Helper()
	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{authIndex}, Limit: 20})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != 1 || len(response.Tasks) != 1 {
		t.Fatalf("expected one accepted refresh task, got %+v", response)
	}
	return response.Tasks[0].TaskID
}

func assertCachedUsagePercent(t *testing.T, service *Service, authIndex string, want float64) {
	t.Helper()
	cache, err := service.GetCachedQuota(context.Background(), CacheRequest{AuthIndexes: []string{authIndex}, Limit: 1})
	if err != nil {
		t.Fatalf("GetCachedQuota returned error: %v", err)
	}
	if len(cache.Items) != 1 || cache.Items[0].ID != authIndex || len(cache.Items[0].Quota) != 1 || cache.Items[0].Quota[0].UsedPercent == nil {
		t.Fatalf("expected one cached quota row for %s, got %+v", authIndex, cache.Items)
	}
	if got := *cache.Items[0].Quota[0].UsedPercent; got != want {
		t.Fatalf("expected cached usage percent %.0f, got %.0f", want, got)
	}
}

func waitForRefreshTask(t *testing.T, service *Service, taskID string, status RefreshTaskStatus) RefreshTaskResponse {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	var task RefreshTaskResponse
	var err error
	for time.Now().Before(deadline) {
		task, err = service.GetRefreshTask(context.Background(), taskID)
		if err == nil && task.Status == status {
			return task
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("task %s did not reach status %s, last task=%+v err=%v", taskID, status, task, err)
	return RefreshTaskResponse{}
}

func waitForRefreshTaskCounts(t *testing.T, service *Service, tasks []RefreshTaskID, running int, queued int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		runningCount := 0
		queuedCount := 0
		for _, accepted := range tasks {
			task, err := service.GetRefreshTask(context.Background(), accepted.TaskID)
			if err != nil {
				continue
			}
			switch task.Status {
			case RefreshTaskStatusRunning:
				runningCount++
			case RefreshTaskStatusQueued:
				queuedCount++
			}
		}
		if runningCount == running && queuedCount == queued {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("refresh tasks did not reach running=%d queued=%d", running, queued)
}

func hasRefreshRejection(rejections []RefreshRejectedAuthIndex, authIndex string, code string) bool {
	for _, rejection := range rejections {
		if rejection.AuthIndex == authIndex && rejection.Error == code {
			return true
		}
	}
	return false
}
