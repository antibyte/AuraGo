package agent

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestDispatchEmailHonorsAgentAccessForReadSendAndListing(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	denied := false
	cfg := &config.Config{}
	cfg.Email.Enabled = true
	cfg.EmailAccounts = []config.EmailAccount{{
		ID: "private", ReceiveProtocol: "imap", IMAPHost: "mail.example.test", IMAPPort: 993,
		SMTPHost: "mail.example.test", SMTPPort: 465, Username: "user", Password: "secret",
		FromAddress: "user@example.test", AgentAccess: &denied,
	}}
	dc := &DispatchContext{Cfg: cfg, Logger: logger}

	for _, action := range []string{"fetch_email", "send_email"} {
		params := map[string]interface{}{"account": "private", "to": "recipient@example.test"}
		result, handled := dispatchEmailCases(context.Background(), ToolCall{Action: action, Params: params}, dc)
		if !handled || !strings.Contains(strings.ToLower(result), "disabled") {
			t.Fatalf("%s was not blocked by agent_access=false: handled=%t result=%q", action, handled, result)
		}
	}
	result, handled := dispatchEmailCases(context.Background(), ToolCall{Action: "list_email_accounts"}, dc)
	if !handled || strings.Contains(result, "private") {
		t.Fatalf("agent email account listing leaked denied account: handled=%t result=%q", handled, result)
	}

	// Legacy IMAP accounts with no explicit grant retain their existing agent access.
	cfg.EmailAccounts[0].AgentAccess = nil
	result, handled = dispatchEmailCases(context.Background(), ToolCall{Action: "list_email_accounts"}, dc)
	if !handled || !strings.Contains(result, `"count":1`) {
		t.Fatalf("legacy email account access changed: handled=%t result=%q", handled, result)
	}
}
