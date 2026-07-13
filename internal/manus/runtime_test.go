package manus

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeCreateTaskTracksOnlySuccessfulAuraGoTask(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","task_title":"Research","task_url":"https://manus.im/app/task-1","share_visibility":"private"}`))
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
	ledger, err := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer ledger.Close()
	runtime := NewRuntime(client, ledger, RuntimeConfig{Policy: Policy{AllowCreateTasks: true}})

	result, err := runtime.CreateTask(context.Background(), CreateTaskRequest{Content: "research", AgentProfile: "manus-1.6"}, nil)
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result.TaskID != "task-1" {
		t.Fatalf("CreateTask() = %#v", result)
	}
	tracked, ok, err := ledger.Get(context.Background(), "task-1")
	if err != nil || !ok || tracked.TaskURL == "" || tracked.Status != "running" {
		t.Fatalf("tracked task = %#v, %t, %v", tracked, ok, err)
	}
}

func TestRuntimeSendMessageUploadsApprovedWorkspaceFiles(t *testing.T) {
	t.Parallel()

	workspace := t.TempDir()
	if err := os.WriteFile(filepath.Join(workspace, "result.pdf"), []byte("payload"), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/file.upload":
			_, _ = w.Write([]byte(`{"ok":true,"file":{"id":"file-1","filename":"result.pdf"},"upload_url":"https://203.0.113.10/upload"}`))
		case "/v2/task.sendMessage":
			var body struct {
				Message Message `json:"message"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Fatal(err)
			}
			parts, ok := body.Message.Content.([]any)
			if !ok || len(parts) != 2 {
				t.Fatalf("message content = %#v", body.Message.Content)
			}
			filePart, _ := parts[1].(map[string]any)
			if filePart["type"] != "file" || filePart["file_id"] != "file-1" {
				t.Fatalf("file part = %#v", filePart)
			}
			_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1"}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()
	fileClient := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("")), Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, FileHTTPClient: fileClient})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	defer ledger.Close()
	_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
	runtime := NewRuntime(client, ledger, RuntimeConfig{
		Policy: Policy{AllowSendMessages: true, AllowFileUploads: true}, WorkspaceDir: workspace, MaxFileBytes: 1024,
	})

	result, err := runtime.SendMessage(context.Background(), SendMessageRequest{TaskID: "task-1", Content: "continue"}, []string{"result.pdf"})
	if err != nil || result.TaskID != "task-1" {
		t.Fatalf("SendMessage() = %#v, %v", result, err)
	}
}

func TestRuntimeRejectsUntrackedRemoteTaskBeforeNetworkCall(t *testing.T) {
	t.Parallel()

	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	defer ledger.Close()
	runtime := NewRuntime(client, ledger, RuntimeConfig{})

	if _, err := runtime.GetTask(context.Background(), "foreign-task"); err == nil || !strings.Contains(err.Error(), "not tracked") {
		t.Fatalf("GetTask() error = %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("network calls = %d, want 0", calls.Load())
	}
}

func TestRuntimeNormalizesWaitingStateWithoutExposingConfirmAction(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		eventType string
		wantState string
	}{
		{name: "question", eventType: "messageAskUser", wantState: "needs_user_input"},
		{name: "confirmation", eventType: "email_send", wantState: "needs_human_approval"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/v2/task.detail":
					_, _ = w.Write([]byte(`{"ok":true,"task":{"id":"task-1","status":"waiting","task_url":"https://manus.im/app/task-1"}}`))
				case "/v2/task.listMessages":
					_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":[{"id":"event-1","type":"status_update","status_update":{"agent_status":"waiting","status_detail":{"waiting_for_event_id":"confirm-1","waiting_for_event_type":"` + tc.eventType + `","waiting_description":"Please decide"}}}]}`))
				default:
					t.Fatalf("unexpected path %q", r.URL.Path)
				}
			}))
			defer server.Close()
			client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
			ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
			defer ledger.Close()
			_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1", TaskURL: "https://manus.im/app/task-1"})
			runtime := NewRuntime(client, ledger, RuntimeConfig{PollInterval: time.Millisecond, MaxWait: time.Second})

			state, err := runtime.WaitForTask(context.Background(), "task-1", time.Second)
			if err != nil {
				t.Fatalf("WaitForTask() error = %v", err)
			}
			if state.State != tc.wantState || state.TaskURL == "" || state.WaitingDescription != "Please decide" {
				t.Fatalf("WaitForTask() = %#v", state)
			}
			if state.ConfirmationEventID != "" {
				t.Fatalf("confirmation event ID must not be exposed to the agent: %#v", state)
			}
		})
	}
}

func TestRuntimeNormalizesRunningAndTerminalTaskStatuses(t *testing.T) {
	t.Parallel()

	tests := []struct {
		remoteStatus string
		wantState    string
	}{
		{remoteStatus: "running", wantState: "running"},
		{remoteStatus: "completed", wantState: "completed"},
		{remoteStatus: "success", wantState: "completed"},
		{remoteStatus: "stopped", wantState: "completed"},
		{remoteStatus: "error", wantState: "error"},
		{remoteStatus: "failed", wantState: "error"},
	}
	for _, tc := range tests {
		t.Run(tc.remoteStatus, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"ok":true,"task":{"id":"task-1","status":"` + tc.remoteStatus + `","task_url":"https://manus.im/app/task-1"}}`))
			}))
			defer server.Close()
			client, _ := NewClient("api-token", ClientConfig{BaseURL: server.URL})
			ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
			defer ledger.Close()
			_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
			runtime := NewRuntime(client, ledger, RuntimeConfig{PollInterval: 5 * time.Millisecond, MaxWait: time.Millisecond})

			state, err := runtime.WaitForTask(context.Background(), "task-1", time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			if state.State != tc.wantState {
				t.Fatalf("status %q normalized to %q, want %q", tc.remoteStatus, state.State, tc.wantState)
			}
		})
	}
}

func TestRuntimeSanitizesInternalEventsAndFailedStructuredOutput(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":[{"id":"event-1","type":"status_update","status_update":{"agent_status":"waiting","brief":"internal plan","description":"private reasoning","status_detail":{"waiting_for_event_id":"confirm-secret","waiting_for_event_type":"email_send","waiting_description":"Approve in Manus","confirm_input_schema":{"type":"object"}}}},{"id":"event-2","type":"structured_output","structured_output_result":{"success":false,"value":{"must_not":"escape"},"error":"extraction failed"}}]}`))
	}))
	defer server.Close()
	client, _ := NewClient("api-token", ClientConfig{BaseURL: server.URL})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	defer ledger.Close()
	_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
	runtime := NewRuntime(client, ledger, RuntimeConfig{})

	page, err := runtime.ListMessages(context.Background(), ListMessagesOptions{TaskID: "task-1"})
	if err != nil {
		t.Fatal(err)
	}
	status := page.Messages[0].StatusUpdate
	if status.Brief != "" || status.Description != "" || status.StatusDetail.WaitingForEventID != "" || status.StatusDetail.ConfirmInputSchema != nil {
		t.Fatalf("internal event details were exposed: %#v", status)
	}
	result := page.Messages[1].StructuredOutputResult
	if result == nil || result.Success || len(result.Value) != 0 {
		t.Fatalf("failed structured value was exposed: %#v", result)
	}
}

func TestRuntimeDownloadsOnlyAttachmentsFromTrackedTaskEvent(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/task.listMessages" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true,"task_id":"task-1","messages":[{"id":"event-1","type":"assistant_message","assistant_message":{"content":"done","attachments":[{"type":"file","filename":"result.txt","url":"https://203.0.113.12/result"}]}}]}`))
	}))
	defer server.Close()
	fileClient := &http.Client{Transport: roundTripFunc(func(_ *http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("result")), ContentLength: 6, Header: make(http.Header)}, nil
	})}
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, FileHTTPClient: fileClient})
	ledger, _ := OpenLedger(filepath.Join(t.TempDir(), "manus.db"))
	defer ledger.Close()
	_ = ledger.Upsert(context.Background(), TaskRecord{TaskID: "task-1"})
	workspace := t.TempDir()
	downloadRoot := filepath.Join(workspace, "workdir", "manus")
	runtime := NewRuntime(client, ledger, RuntimeConfig{
		Policy: Policy{AllowFileDownloads: true}, WorkspaceDir: workspace, DownloadRoot: downloadRoot, MaxFileBytes: 1024,
	})
	paths, err := runtime.DownloadAttachments(context.Background(), "task-1", "")
	if err != nil {
		t.Fatalf("DownloadAttachments() error = %v", err)
	}
	if len(paths) != 1 || !strings.HasSuffix(paths[0], "result.txt") {
		t.Fatalf("DownloadAttachments() = %#v", paths)
	}
}
