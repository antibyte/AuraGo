package agent

import (
	"slices"
	"testing"

	"aurago/internal/config"
)

func TestSIPPhoneSchemaFollowsRuntimePermissions(t *testing.T) {
	var sipCfg config.SIPConfig
	config.ApplySIPDefaults(&sipCfg)
	cfg := &config.Config{SIP: sipCfg}
	if names := builtinToolNames(buildToolFlagsFromConfig(cfg)); slices.Contains(names, "sip_phone") {
		t.Fatalf("disabled SIP phone is visible: %v", names)
	}
	cfg.SIP.Enabled = true
	readOnly := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "sip_phone")
	if !slices.Contains(readOnly, "status") || !slices.Contains(readOnly, "list_calls") || slices.Contains(readOnly, "dial") {
		t.Fatalf("unexpected read-only SIP operations: %v", readOnly)
	}
	cfg.SIP.ReadOnly = false
	cfg.SIP.Permissions.OriginateOutbound = true
	cfg.SIP.Permissions.AnswerInbound = true
	cfg.SIP.Permissions.SendDTMF = true
	operations := toolOperationNames(t, builtinToolSchemas(buildToolFlagsFromConfig(cfg)), "sip_phone")
	for _, operation := range []string{"dial", "answer", "reject", "hangup", "send_dtmf"} {
		if !slices.Contains(operations, operation) {
			t.Fatalf("%s missing from %v", operation, operations)
		}
	}
}
