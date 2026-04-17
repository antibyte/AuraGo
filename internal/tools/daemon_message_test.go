package tools

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// ParseDaemonMessage tests
// ---------------------------------------------------------------------------

func TestParseDaemonMessage_WakeAgent(t *testing.T) {
	line := `{"type":"wake_agent","message":"Disk / at 95%","severity":"warning","data":{"disk":"/","percent":95}}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgWakeAgent {
		t.Errorf("expected type %q, got %q", DaemonMsgWakeAgent, msg.Type)
	}
	if msg.Message != "Disk / at 95%" {
		t.Errorf("unexpected message: %q", msg.Message)
	}
	if msg.Severity != "warning" {
		t.Errorf("expected severity warning, got %q", msg.Severity)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_WakeAgentDefaultSeverity(t *testing.T) {
	line := `{"type":"wake_agent","message":"event happened"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Severity != "info" {
		t.Errorf("expected default severity 'info', got %q", msg.Severity)
	}
}

func TestParseDaemonMessage_Log(t *testing.T) {
	line := `{"type":"log","level":"warn","message":"something happened"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type %q, got %q", DaemonMsgLog, msg.Type)
	}
	if msg.Level != "warn" {
		t.Errorf("expected level warn, got %q", msg.Level)
	}
}

func TestParseDaemonMessage_LogDefaultLevel(t *testing.T) {
	line := `{"type":"log","message":"hello"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Level != "info" {
		t.Errorf("expected default level 'info', got %q", msg.Level)
	}
}

func TestParseDaemonMessage_Heartbeat(t *testing.T) {
	line := `{"type":"heartbeat","timestamp":1712419200}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected type %q, got %q", DaemonMsgHeartbeat, msg.Type)
	}
	if msg.Timestamp != 1712419200 {
		t.Errorf("expected timestamp 1712419200, got %d", msg.Timestamp)
	}
}

func TestParseDaemonMessage_Error(t *testing.T) {
	line := `{"type":"error","message":"disk read failed","fatal":true}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgError {
		t.Errorf("expected type %q, got %q", DaemonMsgError, msg.Type)
	}
	if !msg.Fatal {
		t.Error("expected fatal=true")
	}
}

func TestParseDaemonMessage_Shutdown(t *testing.T) {
	line := `{"type":"shutdown","reason":"monitoring window complete"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgShutdown {
		t.Errorf("expected type %q, got %q", DaemonMsgShutdown, msg.Type)
	}
	if msg.Reason != "monitoring window complete" {
		t.Errorf("unexpected reason: %q", msg.Reason)
	}
}

func TestParseDaemonMessage_EmptyLine(t *testing.T) {
	_, err := ParseDaemonMessage("")
	if err == nil {
		t.Error("expected error for empty line")
	}
}

func TestParseDaemonMessage_InvalidJSON(t *testing.T) {
	_, err := ParseDaemonMessage("this is not json")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseDaemonMessage_MissingType(t *testing.T) {
	_, err := ParseDaemonMessage(`{"message":"no type field"}`)
	if err == nil {
		t.Error("expected error for missing type")
	}
}

func TestParseDaemonMessage_UnknownType(t *testing.T) {
	_, err := ParseDaemonMessage(`{"type":"foobar"}`)
	if err == nil {
		t.Error("expected error for unknown type")
	}
}

func TestParseDaemonMessage_WhitespaceHandling(t *testing.T) {
	line := "  \t" + `{"type":"heartbeat"}` + "  \n"
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected heartbeat, got %q", msg.Type)
	}
}

// ---------------------------------------------------------------------------
// EncodeDaemonCommand tests
// ---------------------------------------------------------------------------

func TestEncodeDaemonCommand_Stop(t *testing.T) {
	cmd := NewStopCommand("user_requested", 30)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode command: %v", err)
	}
	if decoded.Type != DaemonCmdStop {
		t.Errorf("expected type %q, got %q", DaemonCmdStop, decoded.Type)
	}
	if decoded.Reason != "user_requested" {
		t.Errorf("unexpected reason: %q", decoded.Reason)
	}
	if decoded.TimeoutSeconds != 30 {
		t.Errorf("expected timeout 30, got %d", decoded.TimeoutSeconds)
	}
}

func TestEncodeDaemonCommand_ConfigUpdate(t *testing.T) {
	env := map[string]string{"THRESHOLD": "80", "INTERVAL": "60"}
	cmd := NewConfigUpdateCommand(env)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to decode command: %v", err)
	}
	if decoded.Type != DaemonCmdConfigUpdate {
		t.Errorf("expected type %q, got %q", DaemonCmdConfigUpdate, decoded.Type)
	}
	if decoded.Env["THRESHOLD"] != "80" {
		t.Errorf("expected THRESHOLD=80, got %q", decoded.Env["THRESHOLD"])
	}
}

func TestEncodeDaemonCommand_MissingType(t *testing.T) {
	_, err := EncodeDaemonCommand(DaemonCommand{})
	if err == nil {
		t.Error("expected error for missing type")
	}
}

// ---------------------------------------------------------------------------
// ParseDaemonMessage — IPC security / edge-case tests
// ---------------------------------------------------------------------------

func TestParseDaemonMessage_OversizedJSON(t *testing.T) {
	// Build a message with a very large "message" field
	bigMsg := strings.Repeat("A", 1024*1024) // 1 MB
	line := fmt.Sprintf(`{"type":"log","message":"%s"}`, bigMsg)
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		// It's acceptable to reject oversized messages
		return
	}
	// If accepted, type should still be correct
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type log, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_DeeplyNestedJSON(t *testing.T) {
	// Build deeply nested JSON in "data" field
	nested := `{"a":`
	for i := 0; i < 100; i++ {
		nested += `{"b":`
	}
	nested += `"leaf"`
	for i := 0; i < 100; i++ {
		nested += `}`
	}
	nested += `}`
	line := fmt.Sprintf(`{"type":"wake_agent","message":"test","data":%s}`, nested)
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		// It's acceptable to reject deeply nested content
		return
	}
	if msg.Type != DaemonMsgWakeAgent {
		t.Errorf("expected type wake_agent, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_NullBytes(t *testing.T) {
	line := "{\"type\":\"log\",\"message\":\"hello\\u0000world\"}"
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		return // acceptable to reject
	}
	if msg.Type != DaemonMsgLog {
		t.Errorf("expected type log, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_ExtraUnknownFields(t *testing.T) {
	line := `{"type":"heartbeat","unknown_field":"should be ignored","another":42}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("extra fields should be silently ignored, got error: %v", err)
	}
	if msg.Type != DaemonMsgHeartbeat {
		t.Errorf("expected heartbeat, got %q", msg.Type)
	}
}

func TestParseDaemonMessage_TypeAsNumber(t *testing.T) {
	line := `{"type":42,"message":"type confusion"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for numeric type field")
	}
}

func TestParseDaemonMessage_TypeAsNull(t *testing.T) {
	line := `{"type":null,"message":"null type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for null type field")
	}
}

func TestParseDaemonMessage_TypeAsArray(t *testing.T) {
	line := `{"type":["wake_agent"],"message":"array type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for array type field")
	}
}

func TestParseDaemonMessage_EmptyType(t *testing.T) {
	line := `{"type":"","message":"empty type"}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for empty type string")
	}
}

func TestParseDaemonMessage_BinaryGarbage(t *testing.T) {
	line := "\x80\x81\x82\x83\xff\xfe"
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for binary garbage")
	}
}

func TestParseDaemonMessage_UnicodeMessage(t *testing.T) {
	line := `{"type":"log","message":"日本語テスト 🚀 Ümlauts"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error for unicode message: %v", err)
	}
	if !strings.Contains(msg.Message, "日本語") {
		t.Error("expected unicode content to be preserved")
	}
}

func TestParseDaemonMessage_DataAsString(t *testing.T) {
	// data should be json.RawMessage — a string is valid JSON
	line := `{"type":"wake_agent","message":"test","data":"just a string"}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_DataAsArray(t *testing.T) {
	line := `{"type":"wake_agent","message":"test","data":[1,2,3]}`
	msg, err := ParseDaemonMessage(line)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg.Data == nil {
		t.Error("expected data to be non-nil")
	}
}

func TestParseDaemonMessage_TrailingComma(t *testing.T) {
	line := `{"type":"heartbeat","message":"test",}`
	_, err := ParseDaemonMessage(line)
	if err == nil {
		t.Error("expected error for trailing comma (invalid JSON)")
	}
}

// ---------------------------------------------------------------------------
// EncodeDaemonCommand — additional tests
// ---------------------------------------------------------------------------

func TestEncodeDaemonCommand_StopRoundTrip(t *testing.T) {
	original := NewStopCommand("test_reason", 45)
	data, err := EncodeDaemonCommand(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if decoded.Type != original.Type {
		t.Errorf("type mismatch: %q vs %q", decoded.Type, original.Type)
	}
	if decoded.Reason != original.Reason {
		t.Errorf("reason mismatch: %q vs %q", decoded.Reason, original.Reason)
	}
	if decoded.TimeoutSeconds != original.TimeoutSeconds {
		t.Errorf("timeout mismatch: %d vs %d", decoded.TimeoutSeconds, original.TimeoutSeconds)
	}
}

func TestEncodeDaemonCommand_ConfigUpdateRoundTrip(t *testing.T) {
	env := map[string]string{"A": "B", "KEY": "VALUE"}
	original := NewConfigUpdateCommand(env)
	data, err := EncodeDaemonCommand(original)
	if err != nil {
		t.Fatalf("encode error: %v", err)
	}

	var decoded DaemonCommand
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if len(decoded.Env) != 2 {
		t.Errorf("expected 2 env vars, got %d", len(decoded.Env))
	}
	if decoded.Env["A"] != "B" {
		t.Errorf("expected A=B, got %q", decoded.Env["A"])
	}
}

func TestEncodeDaemonCommand_ConfigUpdateNilEnv(t *testing.T) {
	cmd := NewConfigUpdateCommand(nil)
	data, err := EncodeDaemonCommand(cmd)
	if err != nil {
		t.Fatalf("should not fail for nil env: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty encoded output")
	}
}
