package tools

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAnalyzeProcesses_InvalidOperation(t *testing.T) {
	result := AnalyzeProcesses("invalid_op", "", 0, 10)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error status, got %s", r.Status)
	}
	if !strings.Contains(r.Message, "unknown operation") {
		t.Errorf("expected unknown operation message, got %s", r.Message)
	}
}

func TestAnalyzeProcesses_TopCPU(t *testing.T) {
	result := AnalyzeProcesses("top_cpu", "", 0, 5)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "success" {
		t.Errorf("expected success, got %s: %s", r.Status, r.Message)
	}
	if r.Operation != "top_cpu" {
		t.Errorf("expected operation top_cpu, got %s", r.Operation)
	}
}

func TestAnalyzeProcesses_TopMemory(t *testing.T) {
	result := AnalyzeProcesses("top_memory", "", 0, 3)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "success" {
		t.Errorf("expected success, got %s: %s", r.Status, r.Message)
	}
}

func TestAnalyzeProcesses_FindRequiresName(t *testing.T) {
	result := AnalyzeProcesses("find", "", 0, 10)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error for find without name, got %s", r.Status)
	}
}

func TestAnalyzeProcesses_InfoRequiresPID(t *testing.T) {
	result := AnalyzeProcesses("info", "", 0, 10)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error for info without pid, got %s", r.Status)
	}
}

func TestAnalyzeProcesses_TreeRequiresPID(t *testing.T) {
	result := AnalyzeProcesses("tree", "", 0, 10)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error for tree without pid, got %s", r.Status)
	}
}

func TestAnalyzeProcesses_InfoNonExistentPID(t *testing.T) {
	result := AnalyzeProcesses("info", "", 99999999, 10)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error for non-existent PID, got %s", r.Status)
	}
}

func TestAnalyzeProcesses_DefaultLimit(t *testing.T) {
	// limit=0 should default to 10
	result := AnalyzeProcesses("top_cpu", "", 0, 0)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "success" {
		t.Errorf("expected success, got %s: %s", r.Status, r.Message)
	}
}

func TestAnalyzeProcesses_LimitCap(t *testing.T) {
	// limit>100 should cap to 10
	result := AnalyzeProcesses("top_cpu", "", 0, 200)
	var r processAnalyzerResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "success" {
		t.Errorf("expected success, got %s: %s", r.Status, r.Message)
	}
}

func TestContainsIgnoreCase(t *testing.T) {
	tests := []struct {
		s, sub string
		want   bool
	}{
		{"Hello World", "hello", true},
		{"Hello World", "WORLD", true},
		{"foo", "bar", false},
		{"", "test", false},
		{"test", "", false},
		{"GoProcess", "process", true},
	}
	for _, tc := range tests {
		got := containsIgnoreCase(tc.s, tc.sub)
		if got != tc.want {
			t.Errorf("containsIgnoreCase(%q, %q) = %v, want %v", tc.s, tc.sub, got, tc.want)
		}
	}
}
