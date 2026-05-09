package agent

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestToolPermissionErrorIsMachineReadable(t *testing.T) {
	got := formatToolPermissionDenied("file_editor", "runtime_permissions", "agent.allow_filesystem_write", "")
	raw := strings.TrimPrefix(got, "Tool Output: ")
	var payload struct {
		Status  string                 `json:"status"`
		Code    string                 `json:"code"`
		Details map[string]interface{} `json:"details"`
	}
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("permission error is not valid JSON: %v\n%s", err, got)
	}
	if payload.Status != "error" || payload.Code != "permission_denied" {
		t.Fatalf("unexpected payload: %+v", payload)
	}
	if payload.Details["required"] != "agent.allow_filesystem_write" {
		t.Fatalf("required = %#v, want agent.allow_filesystem_write", payload.Details["required"])
	}
}
