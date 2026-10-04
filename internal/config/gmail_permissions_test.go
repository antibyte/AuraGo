package config

import (
	"gopkg.in/yaml.v3"
	"strings"
	"testing"
)

func TestGmailOAuthGrantsAreIndependent(t *testing.T) {
	for _, tc := range []struct {
		name                         string
		read, send, modify, readonly bool
		wantModify, wantSend         bool
	}{
		{name: "read", read: true}, {name: "send", send: true, wantSend: true}, {name: "labels", modify: true, wantModify: true}, {name: "readonly", read: true, send: true, modify: true, readonly: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{}
			cfg.GoogleWorkspace.Gmail = tc.read
			cfg.GoogleWorkspace.GmailSend = tc.send
			cfg.GoogleWorkspace.GmailModifyLabels = tc.modify
			cfg.GoogleWorkspace.ReadOnly = tc.readonly
			scopes := cfg.googleWorkspaceOAuthScopes()
			if strings.Contains(scopes, "gmail.modify") != tc.wantModify || strings.Contains(scopes, "gmail.send") != tc.wantSend {
				t.Fatal(scopes)
			}
		})
	}
	var cfg Config
	if err := yaml.Unmarshal([]byte("google_workspace:\n  enabled: true\n  gmail: true\n  gmail_send: true\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.GoogleWorkspace.GmailModifyLabels {
		t.Fatal("legacy config gained label permission")
	}
}
