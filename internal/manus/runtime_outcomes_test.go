package manus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeStoppedLifecycleUsesNonVerboseEvents(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		remoteStatus string
		messages     string
		wantState    string
		wantContent  string
	}{
		{
			name:         "normal Manus completion",
			remoteStatus: "stopped",
			messages:     `[{"id":"event-1","type":"assistant_message","assistant_message":{"content":"finished safely"}},{"id":"event-2","type":"structured_output","structured_output_result":{"success":true,"value":{"answer":42}}}]`,
			wantState:    "completed",
			wantContent:  "finished safely",
		},
		{
			name:         "explicit user stop",
			remoteStatus: "stopped",
			messages:     `[{"id":"event-1","type":"user_stop"}]`,
			wantState:    "stopped",
		},
		{
			name:         "terminal error",
			remoteStatus: "error",
			messages:     `[{"id":"event-1","type":"error_message","error_message":{"message":"remote failure"}}]`,
			wantState:    "error",
			wantContent:  "remote failure",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var messageCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/task.detail":
					_, _ = w.Write([]byte(`{"ok":true,"task":{"id":"task-1","status":"` + tc.remoteStatus + `","task_url":"https://manus.im/app/task-1"}}`))
				case "/v2/task.listMessages":
					messageCalls.Add(1)
					if r.URL.Query().Get("verbose") != "false" {
						t.Fatalf("verbose = %q, want false", r.URL.Query().Get("verbose"))
					}
					_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":` + tc.messages + `}`))
				default:
					t.Fatalf("unexpected path %q", r.URL.Path)
				}
			}))
			defer server.Close()
			client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
			ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
			defer ledger.Close()
			_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
			runtime := NewRuntime(client, ledger, RuntimeConfig{PollInterval: time.Millisecond, MaxWait: time.Second})

			state, err := runtime.WaitForTask(context.Background(), "task-1", time.Second)
			if err != nil {
				t.Fatalf("WaitForTask() error = %v", err)
			}
			if state.State != tc.wantState {
				t.Fatalf("WaitForTask().State = %q, want %q", state.State, tc.wantState)
			}
			if messageCalls.Load() != 1 || len(state.Messages) == 0 {
				t.Fatalf("terminal messages = %#v, calls = %d", state.Messages, messageCalls.Load())
			}
			if tc.wantContent != "" && !strings.Contains(eventText(state.Messages), tc.wantContent) {
				t.Fatalf("terminal messages = %#v, want content %q", state.Messages, tc.wantContent)
			}
		})
	}
}

func eventText(events []TaskEvent) string {
	var parts []string
	for _, event := range events {
		parts = append(parts, event.AssistantMessage.Content)
		for _, value := range event.ErrorMessage {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, " ")
}

func TestRuntimeMutationsPreflightLedgerBeforeRemoteCall(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		run  func(*Runtime) error
	}{
		{name: "create", run: func(runtime *Runtime) error {
			_, err := runtime.CreateTask(context.Background(), CreateTaskRequest{Content: "work", AgentProfile: "manus-1.6"}, nil)
			return err
		}},
		{name: "send", run: func(runtime *Runtime) error {
			_, err := runtime.SendMessage(context.Background(), SendMessageRequest{TaskID: "task-1", Content: "continue"}, nil)
			return err
		}},
		{name: "stop", run: func(runtime *Runtime) error {
			return runtime.StopTask(context.Background(), "task-1")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				calls.Add(1)
				_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","task_url":"https://manus.im/app/task-1"}`))
			}))
			defer server.Close()
			client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
			store := newScriptedTaskStore()
			store.preflightErr = errors.New("ledger is read-only")
			runtime := NewRuntime(client, store, RuntimeConfig{Policy: Policy{
				AllowCreateTasks: true, AllowSendMessages: true, AllowStopTasks: true,
			}})

			err := tc.run(runtime)
			if err == nil || !strings.Contains(err.Error(), "read-only") {
				t.Fatalf("mutation error = %v", err)
			}
			if calls.Load() != 0 {
				t.Fatalf("remote calls = %d, want 0", calls.Load())
			}
		})
	}
}

func TestRuntimeRetriesOnlyBusyPostMutationPersistence(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","task_title":"Done","task_url":"https://manus.im/app/task-1"}`))
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
	store := newScriptedTaskStore()
	store.upsertErrors = []error{errors.New("SQLITE_BUSY: database is locked"), errors.New("SQLITE_LOCKED"), nil}
	runtime := NewRuntime(client, store, RuntimeConfig{Policy: Policy{AllowCreateTasks: true}})

	result, err := runtime.CreateTask(context.Background(), CreateTaskRequest{Content: "work", AgentProfile: "manus-1.6"}, nil)
	if err != nil || result.TaskID != "task-1" {
		t.Fatalf("CreateTask() = %#v, %v", result, err)
	}
	if store.upsertCalls != 3 {
		t.Fatalf("Upsert calls = %d, want 3", store.upsertCalls)
	}
}

func TestRuntimeReportsRemoteAppliedWhenPostMutationPersistenceFails(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		path string
		run  func(*Runtime) (string, error)
	}{
		{name: "create", path: "/v2/task.create", run: func(runtime *Runtime) (string, error) {
			result, err := runtime.CreateTask(context.Background(), CreateTaskRequest{Content: "work", AgentProfile: "manus-1.6"}, nil)
			return result.TaskID, err
		}},
		{name: "send", path: "/v2/task.sendMessage", run: func(runtime *Runtime) (string, error) {
			result, err := runtime.SendMessage(context.Background(), SendMessageRequest{TaskID: "task-1", Content: "continue"}, nil)
			return result.TaskID, err
		}},
		{name: "stop", path: "/v2/task.stop", run: func(runtime *Runtime) (string, error) {
			return "task-1", runtime.StopTask(context.Background(), "task-1")
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tc.path {
					t.Fatalf("path = %q, want %q", r.URL.Path, tc.path)
				}
				_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","task_title":"Done","task_url":"https://manus.im/app/task-1"}`))
			}))
			defer server.Close()
			client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
			store := newScriptedTaskStore()
			store.upsertErrors = []error{errors.New("disk failure")}
			runtime := NewRuntime(client, store, RuntimeConfig{Policy: Policy{
				AllowCreateTasks: true, AllowSendMessages: true, AllowStopTasks: true,
			}})

			taskID, err := tc.run(runtime)
			if taskID != "task-1" {
				t.Fatalf("task ID = %q, want task-1", taskID)
			}
			var applied *RemoteAppliedError
			if !errors.As(err, &applied) || applied.TaskID != "task-1" || applied.TaskURL == "" {
				t.Fatalf("mutation error = %#v", err)
			}
			if store.upsertCalls != 1 {
				t.Fatalf("Upsert calls = %d, want 1 for non-busy error", store.upsertCalls)
			}
		})
	}
}

type scriptedTaskStore struct {
	mu           sync.Mutex
	preflightErr error
	upsertErrors []error
	upsertCalls  int
	records      map[string]TaskRecord
}

func newScriptedTaskStore() *scriptedTaskStore {
	return &scriptedTaskStore{records: map[string]TaskRecord{
		"task-1": {TaskID: "task-1", TaskURL: "https://manus.im/app/task-1"},
	}}
}

func (s *scriptedTaskStore) PreflightWrite(context.Context) error {
	return s.preflightErr
}

func (s *scriptedTaskStore) Upsert(_ context.Context, record TaskRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.upsertCalls++
	if len(s.upsertErrors) > 0 {
		err := s.upsertErrors[0]
		s.upsertErrors = s.upsertErrors[1:]
		if err != nil {
			return err
		}
	}
	s.records[record.TaskID] = record
	return nil
}

func (s *scriptedTaskStore) Get(_ context.Context, taskID string) (TaskRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	record, ok := s.records[taskID]
	return record, ok, nil
}

func (s *scriptedTaskStore) List(context.Context, int) ([]TaskRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := make([]TaskRecord, 0, len(s.records))
	for _, record := range s.records {
		result = append(result, record)
	}
	return result, nil
}
