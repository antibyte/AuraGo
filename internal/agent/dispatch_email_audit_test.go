package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/security"
	"aurago/internal/tools"
)

func TestFetchedEmailHeadersAreIsolatedAsExternalData(t *testing.T) {
	guardian := security.NewGuardian(slog.New(slog.NewTextHandler(io.Discard, nil)))
	for name, g := range map[string]*security.Guardian{"guardian": guardian, "nil guardian": nil} {
		t.Run(name, func(t *testing.T) {
			msgs := []tools.EmailMessage{{
				UID: 1, From: "Boss <boss@example.com>", To: "me@example.com",
				Subject: "Quarterly numbers", Date: "Mon, 2 Jan 2026", Body: "see you", Snippet: "see",
			}}
			got := sanitizeFetchedEmails(context.Background(), nil, g, nil, false, msgs)[0]
			for field, v := range map[string]string{"from": got.From, "to": got.To, "subject": got.Subject, "date": got.Date, "body": got.Body} {
				if !strings.Contains(v, "<external_data>") {
					t.Errorf("%s = %q, want it inside <external_data>", field, v)
				}
			}
		})
	}
}

func TestAgentMailMessageIDIsNotSatisfiedByGenericID(t *testing.T) {
	got := decodeAgentMailArgs(ToolCall{
		ID:     "call-123",
		Params: map[string]interface{}{"operation": "delete_message", "id": "msg-9"},
	}, "inbox-1")
	if got.MessageID != "" {
		t.Fatalf("MessageID = %q, want empty: only message_id may address a message", got.MessageID)
	}
	got = decodeAgentMailArgs(ToolCall{Params: map[string]interface{}{"operation": "delete_message", "message_id": "msg-9"}}, "inbox-1")
	if got.MessageID != "msg-9" {
		t.Fatalf("MessageID = %q, want msg-9", got.MessageID)
	}
}
