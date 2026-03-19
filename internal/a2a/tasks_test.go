package a2a

import (
	"context"
	"testing"
	"time"

	"github.com/a2aproject/a2a-go/v2/a2a"
	"github.com/a2aproject/a2a-go/v2/a2asrv/taskstore"
)

func TestTaskStore_Create(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: "ctx-1",
		Status:    a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}

	ver, err := store.Create(ctx, task)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if ver != 1 {
		t.Errorf("expected version 1, got %d", ver)
	}
}

func TestTaskStore_CreateDuplicate(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}

	if _, err := store.Create(ctx, task); err != nil {
		t.Fatalf("first Create failed: %v", err)
	}

	_, err := store.Create(ctx, task)
	if err != taskstore.ErrTaskAlreadyExists {
		t.Errorf("expected ErrTaskAlreadyExists, got %v", err)
	}
}

func TestTaskStore_Get(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: "ctx-1",
		Status:    a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}

	store.Create(ctx, task)

	stored, err := store.Get(ctx, task.ID)
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if stored.Task.ID != task.ID {
		t.Errorf("expected task ID %s, got %s", task.ID, stored.Task.ID)
	}
	if stored.Version != 1 {
		t.Errorf("expected version 1, got %d", stored.Version)
	}
}

func TestTaskStore_GetNotFound(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	_, err := store.Get(ctx, "nonexistent")
	if err != a2a.ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskStore_Update(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}

	ver1, _ := store.Create(ctx, task)

	updated := *task
	updated.Status.State = a2a.TaskStateWorking

	ver2, err := store.Update(ctx, &taskstore.UpdateRequest{
		Task:        &updated,
		PrevVersion: ver1,
	})
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if ver2 != 2 {
		t.Errorf("expected version 2, got %d", ver2)
	}

	stored, _ := store.Get(ctx, task.ID)
	if stored.Task.Status.State != a2a.TaskStateWorking {
		t.Errorf("expected state Working, got %s", stored.Task.Status.State)
	}
}

func TestTaskStore_UpdateConcurrentModification(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}

	store.Create(ctx, task)

	updated := *task
	updated.Status.State = a2a.TaskStateWorking

	_, err := store.Update(ctx, &taskstore.UpdateRequest{
		Task:        &updated,
		PrevVersion: 99, // wrong version
	})
	if err == nil {
		t.Fatal("expected concurrent modification error")
	}
}

func TestTaskStore_UpdateNotFound(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:     "nonexistent",
		Status: a2a.TaskStatus{State: a2a.TaskStateWorking},
	}

	_, err := store.Update(ctx, &taskstore.UpdateRequest{Task: task})
	if err != a2a.ErrTaskNotFound {
		t.Errorf("expected ErrTaskNotFound, got %v", err)
	}
}

func TestTaskStore_List(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		task := &a2a.Task{
			ID:        a2a.NewTaskID(),
			ContextID: "ctx-1",
			Status:    a2a.TaskStatus{State: a2a.TaskStateSubmitted},
		}
		store.Create(ctx, task)
	}

	resp, err := store.List(ctx, &a2a.ListTasksRequest{ContextID: "ctx-1"})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(resp.Tasks) != 5 {
		t.Errorf("expected 5 tasks, got %d", len(resp.Tasks))
	}
}

func TestTaskStore_ListFilterByStatus(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	// Create submitted task
	t1 := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}
	store.Create(ctx, t1)

	// Create completed task
	t2 := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateCompleted},
	}
	store.Create(ctx, t2)

	resp, err := store.List(ctx, &a2a.ListTasksRequest{Status: a2a.TaskStateCompleted})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(resp.Tasks) != 1 {
		t.Errorf("expected 1 completed task, got %d", len(resp.Tasks))
	}
}

func TestTaskStore_ListPagination(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	for i := 0; i < 10; i++ {
		task := &a2a.Task{
			ID:     a2a.NewTaskID(),
			Status: a2a.TaskStatus{State: a2a.TaskStateSubmitted},
		}
		store.Create(ctx, task)
	}

	resp, err := store.List(ctx, &a2a.ListTasksRequest{PageSize: 3})
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(resp.Tasks) != 3 {
		t.Errorf("expected 3 tasks in page, got %d", len(resp.Tasks))
	}
	if resp.TotalSize != 10 {
		t.Errorf("expected total 10, got %d", resp.TotalSize)
	}
	if resp.NextPageToken == "" {
		t.Error("expected non-empty NextPageToken")
	}
}

func TestTaskStore_Cleanup(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	now := time.Now()
	old := now.Add(-2 * time.Hour)

	// Create a completed task with old timestamp
	task := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateCompleted, Timestamp: &old},
	}
	store.Create(ctx, task)

	// Create a working task (non-terminal)
	active := &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateWorking, Timestamp: &old},
	}
	store.Create(ctx, active)

	removed := store.Cleanup(1 * time.Hour)
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if store.Count() != 1 {
		t.Errorf("expected 1 remaining, got %d", store.Count())
	}
}

func TestTaskStore_CountAndActiveCount(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	store.Create(ctx, &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateWorking},
	})
	store.Create(ctx, &a2a.Task{
		ID:     a2a.NewTaskID(),
		Status: a2a.TaskStatus{State: a2a.TaskStateCompleted},
	})

	if store.Count() != 2 {
		t.Errorf("expected count 2, got %d", store.Count())
	}
	if store.ActiveCount() != 1 {
		t.Errorf("expected active count 1, got %d", store.ActiveCount())
	}
}

func TestTaskStore_IsolatesStoredTasks(t *testing.T) {
	store := NewTaskStore()
	ctx := context.Background()

	task := &a2a.Task{
		ID:        a2a.NewTaskID(),
		ContextID: "original",
		Status:    a2a.TaskStatus{State: a2a.TaskStateSubmitted},
	}
	store.Create(ctx, task)

	// Mutate the original — should not affect the stored copy
	task.ContextID = "mutated"

	stored, _ := store.Get(ctx, task.ID)
	if stored.Task.ContextID != "original" {
		t.Errorf("expected stored ContextID 'original', got %q", stored.Task.ContextID)
	}
}
