package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestWebPerformanceAudit_MissingURL(t *testing.T) {
	result := WebPerformanceAudit(context.Background(), "", "")
	var r webPerfResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
	if !strings.Contains(r.Message, "url is required") {
		t.Errorf("expected url required message, got %s", r.Message)
	}
}

func TestWebPerformanceAudit_InvalidURL(t *testing.T) {
	result := WebPerformanceAudit(context.Background(), "not-a-url", "")
	var r webPerfResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestWebPerformanceAudit_InvalidViewport(t *testing.T) {
	result := WebPerformanceAudit(context.Background(), "https://example.com", "invalid")
	var r webPerfResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
	if !strings.Contains(r.Message, "viewport") {
		t.Errorf("expected viewport error message, got %s", r.Message)
	}
}

func TestWebPerformanceAudit_ViewportOutOfRange(t *testing.T) {
	result := WebPerformanceAudit(context.Background(), "https://example.com", "100x100")
	var r webPerfResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error, got %s", r.Status)
	}
}

func TestWebPerformanceAudit_FTPNotAllowed(t *testing.T) {
	result := WebPerformanceAudit(context.Background(), "ftp://example.com/file", "")
	var r webPerfResult
	if err := json.Unmarshal([]byte(result), &r); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if r.Status != "error" {
		t.Errorf("expected error for ftp scheme, got %s", r.Status)
	}
}
