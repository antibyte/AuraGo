package agentmail

import (
	"context"
	"strings"
	"testing"
)

func TestRunWebSocketRefusesNonWSSURL(t *testing.T) {
	svc := NewService(ServiceConfig{Config: Config{APIKey: "k", InboxID: "i", WebSocketURL: "ws://127.0.0.1:1/v0"}})
	err := svc.runWebSocket(context.Background())
	if err == nil || !strings.Contains(err.Error(), "wss://") {
		t.Fatalf("runWebSocket err = %v, want wss:// refusal before dialing", err)
	}
}

func TestPrepareAttachmentsRejectsDescriptorWithoutSource(t *testing.T) {
	_, err := PrepareAttachments(5, []AttachmentInput{{Filename: "report.pdf"}}, nil)
	if err == nil {
		t.Fatal("a descriptor with neither path nor base64 was silently skipped")
	}
}
