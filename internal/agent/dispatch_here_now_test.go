package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func dispatchHereNowForTest(t *testing.T, cfg *config.Config, action, operation string, params map[string]interface{}) string {
	t.Helper()
	if params == nil {
		params = map[string]interface{}{}
	}
	params["operation"] = operation
	out, handled := dispatchCloud(context.Background(), ToolCall{Action: action, Params: params}, &DispatchContext{
		Cfg: cfg, Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !handled {
		t.Fatalf("%s was not handled", action)
	}
	return out
}

func TestDispatchHereNowMutationPermissionGatesRunBeforeCredentials(t *testing.T) {
	tests := []struct {
		name      string
		configure func(*config.Config)
		op        string
		params    map[string]interface{}
		want      string
	}{
		{
			name: "read_only", op: "publish", params: map[string]interface{}{"project_dir": "site"},
			configure: func(cfg *config.Config) { cfg.HereNow.ReadOnly = true; cfg.HereNow.AllowPublish = true },
			want:      "read-only mode",
		},
		{
			name: "publish", op: "publish", params: map[string]interface{}{"project_dir": "site"},
			configure: func(cfg *config.Config) {}, want: "publishing is disabled",
		},
		{
			name: "site_management", op: "update_metadata", params: map[string]interface{}{"slug": "site"},
			configure: func(cfg *config.Config) {}, want: "Site management is disabled",
		},
		{
			name: "access_management", op: "update_access", params: map[string]interface{}{"slug": "site", "mode": "anyone_with_link"},
			configure: func(cfg *config.Config) {}, want: "access management is disabled",
		},
		{
			name: "delete", op: "delete_site", params: map[string]interface{}{"slug": "site", "confirm": true},
			configure: func(cfg *config.Config) {}, want: "deletion is disabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.HereNow.Enabled = true
			tt.configure(cfg)
			out := dispatchHereNowForTest(t, cfg, "here_now_site", tt.op, tt.params)
			if !strings.Contains(out, tt.want) {
				t.Fatalf("output = %s, want %q", out, tt.want)
			}
			if strings.Contains(out, "API key") || strings.Contains(out, "Vault is unavailable") {
				t.Fatalf("permission gate ran after credential lookup: %s", out)
			}
		})
	}
}

func TestDispatchHereNowDeleteRequiresExactIdentifiersAndConfirmation(t *testing.T) {
	cfg := &config.Config{}
	cfg.HereNow.Enabled = true
	cfg.HereNow.AllowDelete = true

	out := dispatchHereNowForTest(t, cfg, "here_now_site", "delete_site", map[string]interface{}{"slug": "site"})
	if !strings.Contains(out, "confirm=true") {
		t.Fatalf("delete without confirmation = %s", out)
	}
	out = dispatchHereNowForTest(t, cfg, "here_now_site", "delete_site", map[string]interface{}{"confirm": true})
	if !strings.Contains(out, "slug is required") {
		t.Fatalf("delete without exact slug = %s", out)
	}
	out = dispatchHereNowForTest(t, cfg, "here_now_site", "delete_version", map[string]interface{}{"slug": "site", "confirm": true})
	if !strings.Contains(out, "version_id is required") {
		t.Fatalf("delete without exact version = %s", out)
	}
	if strings.Contains(out, "Vault is unavailable") {
		t.Fatalf("identifier validation ran after client setup: %s", out)
	}
}

func TestDispatchHereNowReadAndPublishInputValidation(t *testing.T) {
	cfg := &config.Config{}
	cfg.HereNow.Enabled = true
	cfg.HereNow.AllowPublish = true

	out := dispatchHereNowForTest(t, cfg, "here_now_site", "publish", nil)
	if !strings.Contains(out, "project_dir is required") {
		t.Fatalf("publish validation = %s", out)
	}
	out = dispatchHereNowForTest(t, cfg, "here_now_site", "update", map[string]interface{}{"project_dir": "site"})
	if !strings.Contains(out, "slug is required") {
		t.Fatalf("update validation = %s", out)
	}
	out = dispatchHereNowForTest(t, cfg, "here_now_sites", "list_accounts", nil)
	if !strings.Contains(out, "Vault is unavailable") {
		t.Fatalf("read operation must require configured credentials: %s", out)
	}
}

func TestParseHereNowAccessReadsCompleteProviderEnvelope(t *testing.T) {
	raw := json.RawMessage(`{"access":{"mode":"restricted","accessPolicyVersion":1,"allowedEmails":["viewer@example.com"],"allowedDomains":["example.org"]}}`)
	got, err := tools.ParseHereNowAccessPolicy(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.Mode != "restricted" || len(got.AllowedEmails) != 1 || got.AllowedEmails[0] != "viewer@example.com" || len(got.AllowedDomains) != 1 || got.AllowedDomains[0] != "example.org" {
		t.Fatalf("parseHereNowAccess = %+v", got)
	}
}

func TestDecodeHereNowArgsTracksExplicitAllowlistReplacement(t *testing.T) {
	omitted := decodeHereNowArgs(ToolCall{Params: map[string]interface{}{}})
	if omitted.AllowedEmailsSet || omitted.AllowedDomainsSet {
		t.Fatalf("omitted allowlists marked explicit: %+v", omitted)
	}
	explicit := decodeHereNowArgs(ToolCall{Params: map[string]interface{}{"allowed_emails": []interface{}{}, "allowed_domains": []interface{}{"example.org"}}})
	if !explicit.AllowedEmailsSet || !explicit.AllowedDomainsSet || len(explicit.AllowedEmails) != 0 || len(explicit.AllowedDomains) != 1 {
		t.Fatalf("explicit allowlists not preserved: %+v", explicit)
	}
}

func TestHereNowSiteSchemaNeverAcceptsPasswordValues(t *testing.T) {
	schemas := builtinToolSchemas(ToolFeatureFlags{HereNowEnabled: true})
	for _, schema := range schemas {
		if schema.Function == nil || schema.Function.Name != "here_now_site" {
			continue
		}
		encoded, err := json.Marshal(schema.Function.Parameters)
		if err != nil {
			t.Fatal(err)
		}
		text := string(encoded)
		if strings.Contains(text, `"password"`) || strings.Contains(text, `"password_value"`) {
			t.Fatalf("password value leaked into here_now_site schema: %s", text)
		}
		if !strings.Contains(text, "set_password") || !strings.Contains(schema.Function.Description, "request_vault_secret") {
			t.Fatalf("safe password workflow missing: %s", text)
		}
		return
	}
	t.Fatal("here_now_site schema missing")
}
