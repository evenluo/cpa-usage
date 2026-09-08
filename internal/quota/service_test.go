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

// fakeAuthFileIdentityLookup implements Repository with in-memory observations.
type fakeAuthFileIdentityLookup struct {
	mu           sync.Mutex
	identities   map[string]entities.UsageIdentity
	observations map[string]CheckResponse
	saveErr      error
}

func (f *fakeAuthFileIdentityLookup) FindActiveAuthFileIdentity(_ context.Context, authIndex string) (entities.UsageIdentity, bool, error) {
	identity, ok := f.identities[strings.TrimSpace(authIndex)]
	if !ok || identity.AuthType != entities.UsageIdentityAuthTypeAuthFile {
		return entities.UsageIdentity{}, false, nil
	}
	return identity, true, nil
}

func (f *fakeAuthFileIdentityLookup) HasActiveIdentity(_ context.Context, authIndex string) (bool, error) {
	_, ok := f.identities[authIndex]
	return ok, nil
}

func (f *fakeAuthFileIdentityLookup) SaveQuotaObservation(_ context.Context, _ uint, response CheckResponse) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.saveErr != nil {
		return f.saveErr
	}
	if current, ok := f.observations[response.ID]; !ok || response.ObservedAt.After(current.ObservedAt) {
		f.observations[response.ID] = response
	}
	return nil
}

func (f *fakeAuthFileIdentityLookup) ListQuotaObservations(_ context.Context, authIndexes []string, limit int) ([]CheckResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	items := make([]CheckResponse, 0, min(limit, len(authIndexes)))
	seen := map[string]struct{}{}
	for _, authIndex := range authIndexes {
		authIndex = strings.TrimSpace(authIndex)
		if authIndex == "" || len(items) >= limit {
			continue
		}
		if _, ok := seen[authIndex]; ok {
			continue
		}
		seen[authIndex] = struct{}{}
		if observation, ok := f.observations[authIndex]; ok {
			items = append(items, observation)
		}
	}
	return items, nil
}

func newRefreshTestService(identities map[string]entities.UsageIdentity, handler ProviderHandler) *Service {
	for authIndex, identity := range identities {
		if identity.ID == 0 {
			identity.ID = uint(len(authIndex) + 1)
			identities[authIndex] = identity
		}
	}
	repository := &fakeAuthFileIdentityLookup{identities: identities, observations: map[string]CheckResponse{}}
	return NewServiceWithRegistry(repository, NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))
}

func claudeAuthFileIdentity(authIndex string) entities.UsageIdentity {
	return entities.UsageIdentity{Identity: authIndex, Provider: "claude", Type: "auth-file", AuthType: entities.UsageIdentityAuthTypeAuthFile}
}

func TestRefreshCreatesTaskPerAuthIndexAndPersistsCompletedQuota(t *testing.T) {
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
		t.Fatalf("expected completed task to expose observed quota, got %+v", task)
	}
	if handler.callCount() != 1 {
		t.Fatalf("expected one provider call, got %d", handler.callCount())
	}
}

func TestRefreshKeepsLatestObservationWhileNewRefreshRuns(t *testing.T) {
	block := make(chan struct{})
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertObservedUsagePercent(t, service, "auth-1", 25)

	handler.output = claudeUsageOutput(80)
	handler.block = block
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusRunning)

	assertObservedUsagePercent(t, service, "auth-1", 25)
	if handler.callCount() != 1 {
		t.Fatalf("observation lookup should not trigger provider calls while refresh is running, got %d", handler.callCount())
	}
	close(block)
	waitForRefreshTask(t, service, second, RefreshTaskStatusCompleted)
	assertObservedUsagePercent(t, service, "auth-1", 80)
}

func TestRefreshFailureKeepsLatestSuccessfulObservation(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertObservedUsagePercent(t, service, "auth-1", 25)

	handler.err = errors.New("upstream exploded")
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusFailed)

	assertObservedUsagePercent(t, service, "auth-1", 25)
	if handler.callCount() != 2 {
		t.Fatalf("expected one failed refresh provider call and no observation-read provider calls, got %d", handler.callCount())
	}
}

func TestCheckPersistenceFailureIsExplicitAndKeepsPreviousObservation(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(80)}
	identity := claudeAuthFileIdentity("auth-1")
	identity.ID = 1
	observedAt := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	repository := &fakeAuthFileIdentityLookup{
		identities: map[string]entities.UsageIdentity{"auth-1": identity},
		observations: map[string]CheckResponse{"auth-1": {
			ID: "auth-1", ObservedAt: observedAt, Quota: claudeUsageOutput(25).Result.(ClaudeResult).QuotaRows(),
		}},
		saveErr: errors.New("disk unavailable"),
	}
	service := NewServiceWithRegistry(repository, NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))

	if _, err := service.Check(context.Background(), CheckRequest{AuthIndex: "auth-1"}); err == nil || !strings.Contains(err.Error(), "persist quota observation") {
		t.Fatalf("expected explicit persistence failure, got %v", err)
	}
	items, err := service.GetQuotaObservations(context.Background(), ObservationsRequest{AuthIndexes: []string{"auth-1"}, Limit: 1})
	if err != nil || len(items.Items) != 1 || !items.Items[0].ObservedAt.Equal(observedAt) || *items.Items[0].Quota[0].UsedPercent != 25 {
		t.Fatalf("expected previous observation to survive persistence failure, items=%+v err=%v", items, err)
	}
}

func TestCheckEmptyNormalizedQuotaFailsAndKeepsPreviousObservation(t *testing.T) {
	handler := &refreshHandlerStub{output: ProviderOutput{Result: ClaudeResult{Usage: &ClaudeUsagePayload{}}}}
	identity := claudeAuthFileIdentity("auth-1")
	identity.ID = 1
	observedAt := time.Date(2026, 9, 8, 1, 0, 0, 0, time.UTC)
	repository := &fakeAuthFileIdentityLookup{
		identities: map[string]entities.UsageIdentity{"auth-1": identity},
		observations: map[string]CheckResponse{"auth-1": {
			ID: "auth-1", ObservedAt: observedAt, Quota: claudeUsageOutput(25).Result.(ClaudeResult).QuotaRows(),
		}},
	}
	service := NewServiceWithRegistry(repository, NewProviderRegistry(map[string]ProviderHandler{"claude": handler}))

	if _, err := service.Check(context.Background(), CheckRequest{AuthIndex: "auth-1"}); err == nil || !strings.Contains(err.Error(), "no usable rows") {
		t.Fatalf("expected empty normalized quota failure, got %v", err)
	}
	items, err := service.GetQuotaObservations(context.Background(), ObservationsRequest{AuthIndexes: []string{"auth-1"}, Limit: 1})
	if err != nil || len(items.Items) != 1 || !items.Items[0].ObservedAt.Equal(observedAt) || *items.Items[0].Quota[0].UsedPercent != 25 {
		t.Fatalf("expected previous observation to survive empty response, items=%+v err=%v", items, err)
	}
}

func TestRefreshSuccessReplacesLatestObservation(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	first := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, first, RefreshTaskStatusCompleted)
	assertObservedUsagePercent(t, service, "auth-1", 25)

	handler.output = claudeUsageOutput(80)
	second := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, second, RefreshTaskStatusCompleted)

	assertObservedUsagePercent(t, service, "auth-1", 80)
	if handler.callCount() != 2 {
		t.Fatalf("expected two refresh provider calls and no observation-read provider calls, got %d", handler.callCount())
	}
}

func TestRefreshCleanupRemovesExpiredTaskButKeepsObservation(t *testing.T) {
	handler := &refreshHandlerStub{output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{"auth-1": claudeAuthFileIdentity("auth-1")}, handler)

	taskID := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, taskID, RefreshTaskStatusCompleted)
	assertObservedUsagePercent(t, service, "auth-1", 25)

	service.refreshTasks.mu.Lock()
	service.refreshTasks.tasks[taskID].ExpiresAt = time.Now().Add(-time.Second)
	service.refreshTasks.mu.Unlock()

	if _, err := service.GetRefreshTask(context.Background(), taskID); !errors.Is(err, ErrTaskNotFound) {
		t.Fatalf("expected expired task to be removed, got %v", err)
	}
	service.refreshTasks.mu.Lock()
	_, activeOK := service.refreshTasks.activeTaskIDsByAuth["auth-1"]
	service.refreshTasks.mu.Unlock()
	if activeOK {
		t.Fatalf("expected expired task index to be removed")
	}
	assertObservedUsagePercent(t, service, "auth-1", 25)
	if handler.callCount() != 1 {
		t.Fatalf("task cleanup and observation read should not trigger provider calls, got %d", handler.callCount())
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

func TestCloseRefreshAdmissionRejectsNewTasksWithoutCancelingAcceptedWorker(t *testing.T) {
	block := make(chan struct{})
	handler := &refreshHandlerStub{block: block, output: claudeUsageOutput(25)}
	service := newRefreshTestService(map[string]entities.UsageIdentity{
		"auth-1": claudeAuthFileIdentity("auth-1"),
		"auth-2": claudeAuthFileIdentity("auth-2"),
	}, handler)
	workerCtx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.AttachRefreshWorkerLifecycle(workerCtx)

	taskID := refreshAuthIndex(t, service, "auth-1")
	waitForRefreshTask(t, service, taskID, RefreshTaskStatusRunning)
	service.CloseRefreshAdmission()

	response, err := service.Refresh(context.Background(), RefreshRequest{AuthIndexes: []string{"auth-2"}, Limit: 1})
	if err != nil {
		t.Fatalf("Refresh returned error: %v", err)
	}
	if response.Accepted != 0 || !hasRefreshRejection(response.Rejected, "auth-2", "refresh_unavailable") {
		t.Fatalf("expected closed admission to reject new task, got %+v", response)
	}
	task, err := service.GetRefreshTask(context.Background(), taskID)
	if err != nil {
		t.Fatalf("GetRefreshTask returned error: %v", err)
	}
	if task.Status != RefreshTaskStatusRunning {
		t.Fatalf("expected accepted worker to remain running before stop, got %+v", task)
	}

	close(block)
	service.StopRefreshWorkers()
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
		if task.Status != RefreshTaskStatusFailed {
			t.Fatalf("expected stopped task %s to be failed, got %+v", accepted.TaskID, task)
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

func assertObservedUsagePercent(t *testing.T, service *Service, authIndex string, want float64) {
	t.Helper()
	observations, err := service.GetQuotaObservations(context.Background(), ObservationsRequest{AuthIndexes: []string{authIndex}, Limit: 1})
	if err != nil {
		t.Fatalf("GetQuotaObservations returned error: %v", err)
	}
	if len(observations.Items) != 1 || observations.Items[0].ID != authIndex || len(observations.Items[0].Quota) != 1 || observations.Items[0].Quota[0].UsedPercent == nil {
		t.Fatalf("expected one persisted quota row for %s, got %+v", authIndex, observations.Items)
	}
	if got := *observations.Items[0].Quota[0].UsedPercent; got != want {
		t.Fatalf("expected observed usage percent %.0f, got %.0f", want, got)
	}
	if observations.Items[0].ObservedAt.IsZero() {
		t.Fatalf("expected observation time, got %+v", observations.Items[0])
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
