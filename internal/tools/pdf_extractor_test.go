package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- PDF Extractor tests ---

func TestExecutePDFExtract_FileNotFound(t *testing.T) {
	workDir := t.TempDir()
	result := ExecutePDFExtract(workDir, "nonexistent.pdf")

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(result), &resp); err != nil {
		t.Fatalf("Failed to parse result JSON: %v", err)
	}
	if resp["status"] != "error" {
		t.Errorf("expected status=error, got %v", resp["status"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "not found") {
		t.Errorf("expected 'not found' in message, got: %s", msg)
	}
}

func TestExecutePDFExtract_EmptyFilepath(t *testing.T) {
	workDir := t.TempDir()
	result := ExecutePDFExtract(workDir, "")

	var resp map[string]interface{}
	json.Unmarshal([]byte(result), &resp)
	if resp["status"] != "error" {
		t.Errorf("expected status=error, got %v", resp["status"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "filepath is required") {
		t.Errorf("expected 'filepath is required' in message, got: %s", msg)
	}
}

func TestExecutePDFExtract_PathTraversal(t *testing.T) {
	workDir := t.TempDir()

	// Try to escape to root
	result := ExecutePDFExtract(workDir, "../../../etc/passwd")

	var resp map[string]interface{}
	json.Unmarshal([]byte(result), &resp)
	if resp["status"] != "error" {
		t.Errorf("expected status=error for path traversal, got %v", resp["status"])
	}
	msg, _ := resp["message"].(string)
	if !strings.Contains(msg, "traversal") && !strings.Contains(msg, "not found") {
		t.Errorf("expected traversal or not found error, got: %s", msg)
	}
}

func TestExecutePDFExtract_InvalidPDF(t *testing.T) {
	workDir := t.TempDir()

	// Create a fake file that isn't actually a PDF
	fakePDF := filepath.Join(workDir, "fake.pdf")
	os.WriteFile(fakePDF, []byte("this is not a pdf"), 0644)

	result := ExecutePDFExtract(workDir, "fake.pdf")

	var resp map[string]interface{}
	json.Unmarshal([]byte(result), &resp)
	if resp["status"] != "error" {
		t.Errorf("expected status=error for invalid PDF, got %v", resp["status"])
	}
}

func TestSecurePDFPath_WithinWorkspace(t *testing.T) {
	workDir := t.TempDir()

	// Create a file inside the workspace
	testFile := filepath.Join(workDir, "test.pdf")
	os.WriteFile(testFile, []byte("%PDF-"), 0644)

	resolved, err := securePDFPath(workDir, "test.pdf")
	if err != nil {
		t.Fatalf("securePDFPath failed: %v", err)
	}
	if resolved != testFile {
		t.Errorf("expected %s, got %s", testFile, resolved)
	}
}

func TestSecurePDFPath_AbsolutePathOutsideProject(t *testing.T) {
	workDir := t.TempDir()

	_, err := securePDFPath(workDir, "/etc/passwd")
	if err == nil {
		t.Error("expected error for absolute path outside project, got nil")
	}
}

// --- SummaryLLMConfig tests ---

func TestSummaryLLMConfig_Fields(t *testing.T) {
	cfg := SummaryLLMConfig{
		APIKey:  "test-key",
		BaseURL: "https://api.test.com",
		Model:   "test-model",
	}
	if cfg.APIKey != "test-key" {
		t.Errorf("APIKey mismatch")
	}
	if cfg.BaseURL != "https://api.test.com" {
		t.Errorf("BaseURL mismatch")
	}
	if cfg.Model != "test-model" {
		t.Errorf("Model mismatch")
	}
}

// --- Content summary envelope extraction tests ---

func TestSummariseContent_ErrorEnvelope(t *testing.T) {
	// SummariseContent should return an error when the input carries an error envelope
	errJSON := `{"status":"error","message":"upstream failure"}`
	_, err := SummariseContent(nil, SummaryLLMConfig{APIKey: "k", Model: "m"}, nil, errJSON, "q", "test")
	if err == nil {
		t.Fatal("expected error for error envelope, got nil")
	}
	if !strings.Contains(err.Error(), "upstream failure") {
		t.Errorf("expected 'upstream failure' in error, got: %v", err)
	}
}

func TestSummariseContent_NoAPIKey(t *testing.T) {
	_, err := SummariseContent(nil, SummaryLLMConfig{}, nil, "some content", "query", "test")
	if err == nil {
		t.Fatal("expected error for missing API key, got nil")
	}
	if !strings.Contains(err.Error(), "no API key") {
		t.Errorf("expected 'no API key' in error, got: %v", err)
	}
}
