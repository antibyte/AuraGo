package agent

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/sipphone"
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

func TestSIPPhoneToolDailyLimitErrorIsStable(t *testing.T) {
	reset := time.Date(2026, 8, 8, 0, 0, 0, 0, time.Local)
	raw := sipPhoneToolResult(nil, &sipphone.AgentDailyCallLimitError{Used: 10, Limit: 10, RetryAfterSeconds: 120, ResetsAt: reset})
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw[len("Tool Output: "):]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["code"] != "agent_daily_call_limit_reached" || payload["used"] != float64(10) || payload["limit"] != float64(10) {
		t.Fatalf("unexpected tool limit payload: %#v", payload)
	}
}
