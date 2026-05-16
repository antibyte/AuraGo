package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestAgentMailToolSchemaOnlyAppearsWhenEnabled(t *testing.T) {
	if containsName(toolNames(builtinToolSchemas(ToolFeatureFlags{})), "agentmail") {
		t.Fatal("agentmail tool should not be exposed when AgentMailEnabled is false")
	}

	schemas := builtinToolSchemas(ToolFeatureFlags{AgentMailEnabled: true})
	if !containsName(toolNames(schemas), "agentmail") {
		t.Fatal("agentmail tool should be exposed when AgentMailEnabled is true")
	}
}

func TestBuildToolFlagsFromConfigEnablesAgentMail(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentMail.Enabled = true

	flags := buildToolFlagsFromConfig(cfg)
	if !flags.AgentMailEnabled {
		t.Fatal("AgentMailEnabled = false, want true")
	}
}

func TestDispatchAgentMailReadOnlyBlocksMutatingOperation(t *testing.T) {
	cfg := &config.Config{}
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.ReadOnly = true
	cfg.AgentMail.APIKey = "test-key"
	cfg.AgentMail.InboxID = "inbox-1"
	cfg.AgentMail.BaseURL = "http://example.invalid"

	out, handled := dispatchAgentMailCases(context.Background(), ToolCall{
		Action:    "agentmail",
		Operation: "send_message",
		Params:    map[string]interface{}{"operation": "send_message", "to": []interface{}{"user@example.com"}},
	}, &DispatchContext{Cfg: cfg, Logger: slog.Default()})

	if !handled {
		t.Fatal("dispatchAgentMailCases handled = false")
	}
	if !strings.Contains(out, "read-only") {
		t.Fatalf("expected read-only error, got %s", out)
	}
}

func TestDecodeAgentMailArgsAcceptsNativeArrayRecipients(t *testing.T) {
	var tc ToolCall
	raw := []byte(`{
		"action":"agentmail",
		"operation":"send_message",
		"to":["user@example.com"],
		"cc":["copy@example.com"],
		"bcc":["hidden@example.com"],
		"subject":"Test",
		"text":"Hello"
	}`)
	if err := json.Unmarshal(raw, &tc); err != nil {
		t.Fatalf("ToolCall should accept AgentMail recipient arrays: %v", err)
	}

	req := decodeAgentMailArgs(tc, "inbox-1")
	if got := strings.Join(req.To, ","); got != "user@example.com" {
		t.Fatalf("To = %q", got)
	}
	if got := strings.Join(req.CC, ","); got != "copy@example.com" {
		t.Fatalf("CC = %q", got)
	}
	if got := strings.Join(req.BCC, ","); got != "hidden@example.com" {
		t.Fatalf("BCC = %q", got)
	}
}

func TestDispatchAgentMailListMessages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v0/inboxes/inbox-1/messages" {
			t.Fatalf("path = %q", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("missing bearer auth")
		}
		_, _ = w.Write([]byte(`{"messages":[{"message_id":"msg-1","subject":"Hello"}]}`))
	}))
	defer srv.Close()

	cfg := &config.Config{}
	cfg.AgentMail.Enabled = true
	cfg.AgentMail.APIKey = "test-key"
	cfg.AgentMail.InboxID = "inbox-1"
	cfg.AgentMail.BaseURL = srv.URL

	out, handled := dispatchAgentMailCases(context.Background(), ToolCall{
		Action:    "agentmail",
		Operation: "list_messages",
		Params:    map[string]interface{}{"operation": "list_messages", "limit": float64(1)},
	}, &DispatchContext{Cfg: cfg, Logger: slog.Default()})

	if !handled {
		t.Fatal("dispatchAgentMailCases handled = false")
	}
	var payload struct {
		Status   string `json:"status"`
		Messages []struct {
			ID      string `json:"message_id"`
			Subject string `json:"subject"`
		} `json:"messages"`
	}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &payload); err != nil {
		t.Fatalf("decode output %q: %v", out, err)
	}
	if payload.Status != "success" || len(payload.Messages) != 1 || payload.Messages[0].ID != "msg-1" {
		t.Fatalf("unexpected output: %+v", payload)
	}
}
