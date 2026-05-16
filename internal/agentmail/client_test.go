package agentmail

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestClientListMessagesUsesBearerAuthAndQuery(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/inboxes/inbox-1/messages" {
			t.Fatalf("path = %q, want /v0/inboxes/inbox-1/messages", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("Authorization = %q, want bearer token", got)
		}
		q := r.URL.Query()
		if q.Get("limit") != "2" || q.Get("labels") != "unread" || q.Get("after") != "2026-05-15T00:00:00Z" {
			t.Fatalf("unexpected query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"messages":[{"message_id":"msg-1","subject":"Hello","labels":["unread"]}],"next_cursor":"next"}`))
	}))
	defer srv.Close()

	client, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "test-key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	res, err := client.ListMessages(context.Background(), "inbox-1", ListMessagesOptions{
		Limit:  2,
		Labels: []string{"unread"},
		After:  "2026-05-15T00:00:00Z",
	})
	if err != nil {
		t.Fatalf("ListMessages() error = %v", err)
	}
	if len(res.Messages) != 1 || res.Messages[0].ID != "msg-1" || res.Messages[0].Subject != "Hello" {
		t.Fatalf("unexpected messages: %+v", res.Messages)
	}
	if res.NextCursor != "next" {
		t.Fatalf("NextCursor = %q, want next", res.NextCursor)
	}
}

func TestClientRetriesRateLimitWithRetryAfter(t *testing.T) {
	t.Parallel()

	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			w.Header().Set("Retry-After", "1")
			http.Error(w, `{"error":{"message":"slow down"}}`, http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"inboxes":[{"inbox_id":"inbox-1","email_address":"bot@example.com"}]}`))
	}))
	defer srv.Close()

	var slept time.Duration
	client, err := NewClient(ClientConfig{
		BaseURL:    srv.URL,
		APIKey:     "test-key",
		HTTPClient: srv.Client(),
		MaxRetries: 1,
		RetrySleep: func(d time.Duration) { slept = d },
	})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	res, err := client.ListInboxes(context.Background(), ListInboxesOptions{Limit: 1})
	if err != nil {
		t.Fatalf("ListInboxes() error = %v", err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d, want 2", calls)
	}
	if slept != time.Second {
		t.Fatalf("slept = %v, want 1s from Retry-After", slept)
	}
	if len(res.Inboxes) != 1 || res.Inboxes[0].ID != "inbox-1" {
		t.Fatalf("unexpected inboxes: %+v", res.Inboxes)
	}
}

func TestUpdateMessageLabelsPayload(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPatch {
			t.Fatalf("method = %s, want PATCH", r.Method)
		}
		if r.URL.Path != "/v0/inboxes/inbox-1/messages/msg-1" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var payload UpdateMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !reflect.DeepEqual(payload.AddLabels, []string{"processed", "read"}) {
			t.Fatalf("AddLabels = %#v", payload.AddLabels)
		}
		if !reflect.DeepEqual(payload.RemoveLabels, []string{"unread"}) {
			t.Fatalf("RemoveLabels = %#v", payload.RemoveLabels)
		}
		_, _ = w.Write([]byte(`{"message_id":"msg-1","labels":["processed","read"]}`))
	}))
	defer srv.Close()

	client, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "test-key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	msg, err := client.UpdateMessage(context.Background(), "inbox-1", "msg-1", UpdateMessageRequest{
		AddLabels:    []string{"processed", "read"},
		RemoveLabels: []string{"unread"},
	})
	if err != nil {
		t.Fatalf("UpdateMessage() error = %v", err)
	}
	if msg.ID != "msg-1" || !reflect.DeepEqual(msg.Labels, []string{"processed", "read"}) {
		t.Fatalf("unexpected message: %+v", msg)
	}
}

func TestSendMessagePayloadIncludesAttachments(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/inboxes/inbox-1/messages/send" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		var payload SendMessageRequest
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode payload: %v", err)
		}
		if !reflect.DeepEqual(payload.To, []string{"user@example.com"}) || payload.Subject != "Report" || payload.Text != "See attached" {
			t.Fatalf("unexpected payload: %+v", payload)
		}
		if len(payload.Attachments) != 1 || payload.Attachments[0].Filename != "report.txt" || payload.Attachments[0].ContentBase64 == "" {
			t.Fatalf("unexpected attachments: %+v", payload.Attachments)
		}
		_, _ = w.Write([]byte(`{"message_id":"sent-1","subject":"Report"}`))
	}))
	defer srv.Close()

	client, err := NewClient(ClientConfig{BaseURL: srv.URL, APIKey: "test-key", HTTPClient: srv.Client()})
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	msg, err := client.SendMessage(context.Background(), "inbox-1", SendMessageRequest{
		To:      []string{"user@example.com"},
		Subject: "Report",
		Text:    "See attached",
		Attachments: []OutgoingAttachment{{
			Filename:      "report.txt",
			ContentType:   "text/plain",
			ContentBase64: "cmVwb3J0",
		}},
	})
	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}
	if msg.ID != "sent-1" {
		t.Fatalf("sent ID = %q, want sent-1", msg.ID)
	}
}
