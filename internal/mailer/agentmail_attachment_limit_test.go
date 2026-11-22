package mailer

import (
	"context"
	"strings"
	"testing"

	"aurago/internal/agentmail"
)

func TestAgentMailAttachmentLimitBytes(t *testing.T) {
	tests := []struct {
		name string
		mb   int
		want int64
	}{
		{name: "configured 1 MB", mb: 1, want: 1 << 20},
		{name: "unset falls back to default", mb: 0, want: int64(agentmail.DefaultMaxAttachMB) << 20},
		{name: "negative falls back to default", mb: -5, want: int64(agentmail.DefaultMaxAttachMB) << 20},
		{name: "provider ceiling itself", mb: 25, want: maxAgentMailAttachmentBytes},
		{name: "above ceiling is clamped", mb: 100, want: maxAgentMailAttachmentBytes},
		{name: "huge value does not overflow", mb: int(^uint(0) >> 1), want: maxAgentMailAttachmentBytes},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := agentMailAttachmentLimitBytes(TransportConfig{AgentMailMaxAttachmentMB: tc.mb}); got != tc.want {
				t.Fatalf("limit for %d MB = %d, want %d", tc.mb, got, tc.want)
			}
		})
	}
}

func TestAgentMailSendRejectsAttachmentsAboveConfiguredLimit(t *testing.T) {
	adapter, err := newAgentMailAdapter(TransportConfig{ID: "inbox", Source: "agentmail", AgentMailInboxID: "inbox", AgentMailAPIKey: "fixture",
		AgentMailBaseURL: "https://api.agentmail.test", AgentMailMaxAttachmentMB: 1})
	if err != nil {
		t.Fatal(err)
	}
	out := Outgoing{Draft: Draft{To: []string{"to@example.test"}, Subject: "s", Text: "t",
		Attachments: []Attachment{{Filename: "big.bin", ContentType: "application/octet-stream", Data: make([]byte, 2<<20)}}}}
	res, err := adapter.Send(context.Background(), out)
	if err == nil || !strings.Contains(err.Error(), "attachments exceed size limit") {
		t.Fatalf("Send error = %v, want attachments exceed size limit", err)
	}
	if !res.Rejected {
		t.Fatalf("oversized attachment must be a definitive rejection: %#v", res)
	}
}
