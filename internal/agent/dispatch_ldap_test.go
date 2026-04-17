package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
)

func decodeLDAPDispatchOutput(t *testing.T, out string) map[string]interface{} {
	t.Helper()
	if !strings.HasPrefix(out, "Tool Output: ") {
		t.Fatalf("expected Tool Output prefix, got %q", out)
	}
	raw := strings.TrimPrefix(out, "Tool Output: ")
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", raw, err)
	}
	return payload
}

func TestDispatchServicesLDAPDisabled(t *testing.T) {
	out, ok := dispatchServices(context.Background(), ToolCall{
		Action: "ldap",
		Params: map[string]interface{}{"operation": "search"},
	}, &DispatchContext{
		Cfg:    &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle ldap")
	}
	payload := decodeLDAPDispatchOutput(t, out)
	if payload["status"] != "error" {
		t.Fatalf("payload = %v", payload)
	}
}

func TestDispatchServicesLDAPReadOnlyBlocksMutations(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.ReadOnly = true

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action: "ldap",
		Params: map[string]interface{}{
			"operation": "add_user",
			"dn":        "cn=jane,dc=example,dc=com",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle ldap")
	}
	payload := decodeLDAPDispatchOutput(t, out)
	if payload["message"] != "LDAP is in read-only mode." {
		t.Fatalf("payload = %v", payload)
	}
}

func TestDispatchServicesLDAPPrefixesOutputOnlyOnce(t *testing.T) {
	cfg := &config.Config{}
	cfg.LDAP.Enabled = true
	cfg.LDAP.Host = "ldap.example.com"
	cfg.LDAP.BaseDN = "dc=example,dc=com"
	cfg.LDAP.BindDN = "cn=svc,dc=example,dc=com"

	out, ok := dispatchServices(context.Background(), ToolCall{
		Action: "ldap",
		Params: map[string]interface{}{"operation": "unknown_operation"},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchServices to handle ldap")
	}
	if strings.Count(out, "Tool Output: ") != 1 {
		t.Fatalf("output = %q, want exactly one Tool Output prefix", out)
	}
	payload := decodeLDAPDispatchOutput(t, out)
	if payload["status"] != "error" {
		t.Fatalf("payload = %v", payload)
	}
}
