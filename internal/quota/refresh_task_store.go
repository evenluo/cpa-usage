package quota

import (
	"fmt"
	"sync"
	"time"
)

const defaultRefreshTaskTTL = 20 * time.Minute

// refreshTaskStore owns only transient refresh task state. Its TTL controls
// terminal task cleanup and has no effect on persisted quota observations.
type refreshTaskStore struct {
	mu                  sync.Mutex
	tasks               map[string]*RefreshTaskRecord
	activeTaskIDsByAuth map[string]string
	ttl                 time.Duration
	seq                 uint64
}

func newRefreshTaskStore(ttl time.Duration) *refreshTaskStore {
	return &refreshTaskStore{
		tasks:               make(map[string]*RefreshTaskRecord),
		activeTaskIDsByAuth: make(map[string]string),
		ttl:                 ttl,
	}
}

// enqueue 为 auth_index 创建 queued 任务；已有活跃任务时返回既有任务且 created=false。
func (s *refreshTaskStore) enqueue(authIndex string) (task *RefreshTaskRecord, created bool) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if taskID, ok := s.activeTaskIDsByAuth[authIndex]; ok {
		if existing, ok := s.tasks[taskID]; ok && existing.isActive() {
			return existing, false
		}
	}
	s.seq++
	task = &RefreshTaskRecord{
		TaskID:    fmt.Sprintf("quota-refresh-%d", s.seq),
		AuthIndex: authIndex,
		Status:    RefreshTaskStatusQueued,
		CreatedAt: now,
	}
	s.tasks[task.TaskID] = task
	s.activeTaskIDsByAuth[authIndex] = task.TaskID
	return task, true
}

// snapshot 返回任务的值拷贝，调用方无需持锁即可安全读取。
func (s *refreshTaskStore) snapshot(taskID string) (RefreshTaskRecord, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return RefreshTaskRecord{}, false
	}
	return *task, true
}

// markRunning 把 queued 任务推进到 running 并返回其 auth_index；任务缺失或已被清理时 ok=false。
func (s *refreshTaskStore) markRunning(taskID string) (authIndex string, ok bool) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	task, exists := s.tasks[taskID]
	if !exists || task.Status != RefreshTaskStatusQueued {
		return "", false
	}
	task.Status = RefreshTaskStatusRunning
	task.StartedAt = now
	return task.AuthIndex, true
}

func (s *refreshTaskStore) markCompleted(taskID string, response CheckResponse) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return
	}
	task.Status = RefreshTaskStatusCompleted
	task.FinishedAt = now
	task.ExpiresAt = now.Add(s.ttl)
	task.Quota = &response
}

func (s *refreshTaskStore) markFailed(taskID string, message string) {
	now := time.Now().UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	task, ok := s.tasks[taskID]
	if !ok {
		return
	}
	task.Status = RefreshTaskStatusFailed
	task.FinishedAt = now
	task.ExpiresAt = now.Add(s.ttl)
	task.Error = message
}

// cleanupExpired 删除过期任务并同步清理两个索引，避免映射残留。
func (s *refreshTaskStore) cleanupExpired(now time.Time) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for taskID, task := range s.tasks {
		if task.ExpiresAt.IsZero() || now.Before(task.ExpiresAt) {
			continue
		}
		delete(s.tasks, taskID)
		if s.activeTaskIDsByAuth[task.AuthIndex] == taskID {
			delete(s.activeTaskIDsByAuth, task.AuthIndex)
		}
	}
}
