package a2a

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
)

type storedEntry struct {
	task    *a2a.Task
	version taskstore.TaskVersion
}

// TaskStore is an in-memory task store implementing taskstore.Store.
type TaskStore struct {
	mu      sync.RWMutex
	entries map[a2a.TaskID]*storedEntry
}

// NewTaskStore returns a new in-memory TaskStore.
func NewTaskStore() *TaskStore {
	return &TaskStore{entries: make(map[a2a.TaskID]*storedEntry)}
}

// Ensure TaskStore implements taskstore.Store.
var _ taskstore.Store = (*TaskStore)(nil)

func (s *TaskStore) Create(_ context.Context, task *a2a.Task) (taskstore.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.entries[task.ID]; exists {
		return taskstore.TaskVersionMissing, taskstore.ErrTaskAlreadyExists
	}
	cp := *task
	ver := taskstore.TaskVersion(1)
	s.entries[task.ID] = &storedEntry{task: &cp, version: ver}
	return ver, nil
}

func (s *TaskStore) Update(_ context.Context, req *taskstore.UpdateRequest) (taskstore.TaskVersion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[req.Task.ID]
	if !ok {
		return taskstore.TaskVersionMissing, a2a.ErrTaskNotFound
	}
	if req.PrevVersion != taskstore.TaskVersionMissing && req.PrevVersion != entry.version {
		return taskstore.TaskVersionMissing, fmt.Errorf("%w: expected %d, got %d",
			taskstore.ErrConcurrentModification, entry.version, req.PrevVersion)
	}
	cp := *req.Task
	newVer := entry.version + 1
	s.entries[req.Task.ID] = &storedEntry{task: &cp, version: newVer}
	return newVer, nil
}

func (s *TaskStore) Get(_ context.Context, taskID a2a.TaskID) (*taskstore.StoredTask, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	entry, ok := s.entries[taskID]
	if !ok {
		return nil, a2a.ErrTaskNotFound
	}
	cp := *entry.task
	return &taskstore.StoredTask{Task: &cp, Version: entry.version}, nil
}

func (s *TaskStore) List(_ context.Context, req *a2a.ListTasksRequest) (*a2a.ListTasksResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var filtered []*a2a.Task
	for _, e := range s.entries {
		t := e.task
		if req.ContextID != "" && t.ContextID != req.ContextID {
			continue
		}
		if req.Status != "" && t.Status.State != req.Status {
			continue
		}
		if req.StatusTimestampAfter != nil && t.Status.Timestamp != nil {
			if t.Status.Timestamp.Before(*req.StatusTimestampAfter) {
				continue
			}
		}
		cp := *t
		filtered = append(filtered, &cp)
	}

	total := len(filtered)

	// Pagination via numeric offset token
	offset := 0
	if req.PageToken != "" {
		if n, err := strconv.Atoi(req.PageToken); err == nil && n >= 0 {
			offset = n
		}
	}
	if offset > len(filtered) {
		offset = len(filtered)
	}
	filtered = filtered[offset:]

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = len(filtered)
	}

	nextToken := ""
	if len(filtered) > pageSize {
		filtered = filtered[:pageSize]
		nextToken = strconv.Itoa(offset + pageSize)
	}

	return &a2a.ListTasksResponse{
		Tasks:         filtered,
		TotalSize:     total,
		PageSize:      pageSize,
		NextPageToken: nextToken,
	}, nil
}

// Cleanup removes tasks in terminal states older than maxAge.
func (s *TaskStore) Cleanup(maxAge time.Duration) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	cutoff := time.Now().Add(-maxAge)
	removed := 0
	for id, e := range s.entries {
		if e.task.Status.State.Terminal() && e.task.Status.Timestamp != nil && e.task.Status.Timestamp.Before(cutoff) {
			delete(s.entries, id)
			removed++
		}
	}
	return removed
}

// StartCleanupLoop starts a background goroutine that periodically cleans up old tasks.
func (s *TaskStore) StartCleanupLoop(ctx context.Context, interval, maxAge time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.Cleanup(maxAge)
			}
		}
	}()
}

// Count returns the total number of tasks in the store.
func (s *TaskStore) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// ActiveCount returns the number of non-terminal tasks.
func (s *TaskStore) ActiveCount() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	count := 0
	for _, e := range s.entries {
		if !e.task.Status.State.Terminal() {
			count++
		}
	}
	return count
}
