package tools

import (
	"aurago/internal/config"
	"strings"
	"testing"
)

func TestGmailLabelChangesRequireOwnGrant(t *testing.T) {
	for _, tc := range []struct {
		modify, readonly bool
		want             string
	}{{false, false, "disabled"}, {true, true, "read-only"}, {true, false, "initialize"}} {
		cfg := config.Config{}
		cfg.GoogleWorkspace.Enabled = true
		cfg.GoogleWorkspace.Gmail = true
		cfg.GoogleWorkspace.GmailSend = true
		cfg.GoogleWorkspace.GmailModifyLabels = tc.modify
		cfg.GoogleWorkspace.ReadOnly = tc.readonly
		result := ExecuteGoogleWorkspace(cfg, nil, "gmail_modify_labels", map[string]interface{}{"message_id": "fixture"})
		if !strings.Contains(result, tc.want) {
			t.Fatal(result)
		}
	}
}
