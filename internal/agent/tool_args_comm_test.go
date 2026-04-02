package agent

import "testing"

func TestDecodeFollowUpArgs(t *testing.T) {
	tc := ToolCall{
		Params: map[string]interface{}{
			"task_prompt":          "re-check the webhook status",
			"delay_seconds":        float64(30),
			"timeout_secs":         float64(120),
			"notify_on_completion": true,
		},
	}

	req := decodeFollowUpArgs(tc)
	if req.TaskPrompt != "re-check the webhook status" {
		t.Fatalf("TaskPrompt = %q", req.TaskPrompt)
	}
	if req.DelaySeconds != 30 || req.TimeoutSecs != 120 {
		t.Fatalf("unexpected timing values: %+v", req)
	}
	if !req.NotifyOnCompletion {
		t.Fatal("NotifyOnCompletion = false, want true")
	}
}

func TestDecodeWaitForEventArgs(t *testing.T) {
	tc := ToolCall{
		Params: map[string]interface{}{
			"event_type":      "http_ready",
			"task_prompt":     "continue deployment",
			"url":             "https://example.com/health",
			"port":            float64(443),
			"timeout_seconds": float64(90),
			"poll_interval":   float64(5),
		},
	}

	req := decodeWaitForEventArgs(tc)
	if req.EventType != "http_ready" || req.TaskPrompt != "continue deployment" {
		t.Fatalf("unexpected event args: %+v", req)
	}
	if req.URL != "https://example.com/health" || req.Port != 443 {
		t.Fatalf("unexpected target args: %+v", req)
	}
	if req.TimeoutSecs != 90 || req.IntervalSecs != 5 {
		t.Fatalf("unexpected timing args: %+v", req)
	}
}

func TestDecodePlanManagementArgs(t *testing.T) {
	tc := ToolCall{
		Params: map[string]interface{}{
			"operation":        "create",
			"title":            "Ship registry fix",
			"description":      "Finish and verify",
			"content":          "Detailed plan body",
			"priority":         float64(3),
			"include_archived": true,
			"items": []interface{}{
				map[string]interface{}{"title": "Write patch", "task_id": "t1"},
				map[string]interface{}{"title": "Run tests", "task_id": "t2"},
			},
		},
	}

	req := decodePlanManagementArgs(tc)
	if req.Operation != "create" || req.Title != "Ship registry fix" {
		t.Fatalf("unexpected plan args: %+v", req)
	}
	if req.Priority != 3 || !req.IncludeArchived {
		t.Fatalf("unexpected plan flags: %+v", req)
	}
	if len(req.Items) != 2 {
		t.Fatalf("Items len = %d, want 2", len(req.Items))
	}
}

func TestDecodeNotesAndJournalArgs(t *testing.T) {
	noteReq := decodeNotesManagementArgs(ToolCall{
		Params: map[string]interface{}{
			"operation": "update",
			"note_id":   float64(42),
			"title":     "Fix docs",
			"category":  "todo",
		},
	})
	if noteReq.NoteID != 42 || noteReq.Title != "Fix docs" || noteReq.Category != "todo" {
		t.Fatalf("unexpected note args: %+v", noteReq)
	}

	journalReq := decodeJournalManagementArgs(ToolCall{
		Params: map[string]interface{}{
			"operation":  "delete",
			"entry_id":   float64(7),
			"entry_type": "reflection",
			"tags":       "alpha, beta",
		},
	})
	if journalReq.EntryID != 7 || journalReq.EntryType != "reflection" {
		t.Fatalf("unexpected journal args: %+v", journalReq)
	}
	tags := journalReq.normalizedTags()
	if len(tags) != 2 || tags[0] != "alpha" || tags[1] != "beta" {
		t.Fatalf("unexpected normalized tags: %#v", tags)
	}
}

func TestDecodeTelnyxAndAddressBookArgs(t *testing.T) {
	smsReq := decodeTelnyxSMSArgs(ToolCall{
		Params: map[string]interface{}{
			"operation":  "send",
			"to":         "+491234",
			"message":    "hello",
			"message_id": "msg-1",
			"media_urls": []interface{}{"https://example.com/a.png"},
		},
	})
	if smsReq.MessageID != "msg-1" || len(smsReq.MediaURLs) != 1 {
		t.Fatalf("unexpected sms args: %+v", smsReq)
	}

	contactReq := decodeAddressBookArgs(ToolCall{
		Params: map[string]interface{}{
			"operation":    "add",
			"name":         "Ada",
			"email":        "ada@example.com",
			"address":      "Main Street 1",
			"relationship": "friend",
			"notes":        "Met at meetup",
		},
	})
	if contactReq.Name != "Ada" || contactReq.ContactAddress != "Main Street 1" || contactReq.Relationship != "friend" {
		t.Fatalf("unexpected contact args: %+v", contactReq)
	}
}
