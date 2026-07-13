package manus

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientCreateTaskForcesPrivateVisibility(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/task.create" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if body["share_visibility"] != "private" || body["hide_in_task_list"] != false {
			t.Fatalf("unsafe task visibility payload: %#v", body)
		}
		message := body["message"].(map[string]any)
		if connectors, ok := message["connectors"].([]any); !ok || len(connectors) != 0 {
			t.Fatalf("connectors must be sent as an explicit empty array: %#v", message)
		}
		_, _ = w.Write([]byte(`{"ok":true,"request_id":"req-create","task_id":"task-1","task_title":"Research","task_url":"https://manus.im/app/task-1","share_visibility":"private"}`))
	}))
	defer server.Close()

	client, err := NewClient("secret", ClientConfig{BaseURL: server.URL})
	if err != nil {
		t.Fatal(err)
	}
	result, err := client.CreateTask(context.Background(), CreateTaskRequest{
		Content:      "Research this",
		Title:        "Research",
		AgentProfile: "manus-1.6",
		Connectors:   []string{},
	})
	if err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if result.TaskID != "task-1" || result.Visibility != "private" {
		t.Fatalf("CreateTask() = %#v", result)
	}
}

func TestClientListMessagesNeverRequestsVerboseReasoning(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/task.listMessages" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("task_id") != "task-1" || query.Get("cursor") != "cursor-1" || query.Get("limit") != "25" {
			t.Fatalf("query = %v", query)
		}
		if query.Get("verbose") != "false" {
			t.Fatalf("verbose = %q, want false", query.Get("verbose"))
		}
		_, _ = w.Write([]byte(`{"ok":true,"request_id":"req-list","task_id":"task-1","messages":[{"id":"event-1","type":"assistant_message","assistant_message":{"content":"done"}}],"has_more":true,"next_cursor":"cursor-2"}`))
	}))
	defer server.Close()

	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
	page, err := client.ListMessages(context.Background(), ListMessagesOptions{TaskID: "task-1", Cursor: "cursor-1", Limit: 25})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Messages) != 1 || page.Messages[0].AssistantMessage.Content != "done" || page.NextCursor != "cursor-2" {
		t.Fatalf("ListMessages() = %#v", page)
	}
}

func TestClientRetriesRateLimitedReadsOnly(t *testing.T) {
	t.Parallel()

	getAttempts := 0
	postAttempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v2/task.detail":
			getAttempts++
			if getAttempts == 1 {
				w.Header().Set("Retry-After", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"rate_limited","message":"slow down"}}`))
				return
			}
			_, _ = w.Write([]byte(`{"ok":true,"request_id":"req-detail","task":{"id":"task-1","status":"running"}}`))
		case "/v2/task.stop":
			postAttempts++
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"ok":false,"error":{"code":"rate_limited","message":"slow down"}}`))
		default:
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
	}))
	defer server.Close()

	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, RetryBaseDelay: time.Millisecond})
	if _, err := client.GetTask(context.Background(), "task-1"); err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if getAttempts != 2 {
		t.Fatalf("GET attempts = %d, want 2", getAttempts)
	}
	if err := client.StopTask(context.Background(), "task-1"); err == nil {
		t.Fatal("StopTask() error = nil, want rate limit error")
	}
	if postAttempts != 1 {
		t.Fatalf("POST attempts = %d, want 1", postAttempts)
	}
}

func TestClientRejectsOversizedResponse(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":true,"data":{"total_credits":12345}}`))
	}))
	defer server.Close()

	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL, MaxResultBytes: 16})
	if _, err := client.AvailableCredits(context.Background()); err == nil {
		t.Fatal("AvailableCredits() error = nil, want response size error")
	}
}

func TestClientSendMessageUsesFollowUpEndpointWithoutMutationRetry(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v2/task.sendMessage" {
			t.Fatalf("request = %s %s", r.Method, r.URL.Path)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		if body["task_id"] != "task-1" || body["clear_connectors"] != false {
			t.Fatalf("send payload = %#v", body)
		}
		_, _ = w.Write([]byte(`{"ok":true,"request_id":"req-send","task_id":"task-1"}`))
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})
	result, err := client.SendMessage(context.Background(), SendMessageRequest{
		TaskID: "task-1", Content: "continue", Connectors: []string{},
	})
	if err != nil || result.TaskID != "task-1" {
		t.Fatalf("SendMessage() = %#v, %v", result, err)
	}
}

func TestClientRejectsOKFalseEnvelopeOnSuccessfulHTTPStatus(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"ok":false,"request_id":"req-fail","error":{"code":"invalid_request","message":"public failure"}}`))
	}))
	defer server.Close()
	client, _ := NewClient("secret", ClientConfig{BaseURL: server.URL})

	_, err := client.AvailableCredits(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid_request") || !strings.Contains(err.Error(), "public failure") {
		t.Fatalf("AvailableCredits() error = %v", err)
	}
}
