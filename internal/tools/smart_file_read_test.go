package tools

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSmartFileReadAnalyzeLargeTextFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "server.log")
	content := strings.Repeat("line with details\n", 5000)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	raw := ExecuteSmartFileRead(context.Background(), SummaryLLMConfig{}, nil, "analyze", "server.log", "", "", 0, 0, dir)
	var result SmartFileReadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	data := result.Data.(map[string]interface{})
	if data["is_large"] != true {
		t.Fatalf("expected is_large=true, got %v", data["is_large"])
	}
	if data["recommended_tool"].(string) == "" {
		t.Fatal("expected recommended_tool to be set")
	}
}

func TestSmartFileReadSampleDistributed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "app.log")
	var b strings.Builder
	for i := 1; i <= 120; i++ {
		b.WriteString("line ")
		b.WriteString(strconv.Itoa(i))
		b.WriteString("\n")
	}
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	raw := ExecuteSmartFileRead(context.Background(), SummaryLLMConfig{}, nil, "sample", "app.log", "", "distributed", 0, 5, dir)
	var result SmartFileReadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	data := result.Data.(map[string]interface{})
	content := data["content"].(string)
	if !strings.Contains(content, "[HEAD SAMPLE]") || !strings.Contains(content, "[MIDDLE SAMPLE") || !strings.Contains(content, "[TAIL SAMPLE]") {
		t.Fatalf("expected distributed sample sections, got: %s", content)
	}
}

func TestSmartFileReadStructureDetectsCSV(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.csv")
	if err := os.WriteFile(path, []byte("name,value\ncpu,80\nram,65\n"), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	raw := ExecuteSmartFileRead(context.Background(), SummaryLLMConfig{}, nil, "structure", "report.csv", "", "", 0, 0, dir)
	var result SmartFileReadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	data := result.Data.(map[string]interface{})
	if got := data["format"].(string); got != "csv" {
		t.Fatalf("format = %q, want csv", got)
	}
}

func TestSmartFileReadSummarizeUsesSummaryLLM(t *testing.T) {
	old := smartFileReadSummariseFunc
	defer func() { smartFileReadSummariseFunc = old }()

	smartFileReadSummariseFunc = func(ctx context.Context, llmCfg SummaryLLMConfig, logger *slog.Logger, rawContent string, searchQuery string, sourceName string) (string, error) {
		if !strings.Contains(rawContent, "error spike") {
			t.Fatalf("expected sampled content in summary input, got: %s", rawContent)
		}
		if searchQuery != "Find the root cause" {
			t.Fatalf("searchQuery = %q, want Find the root cause", searchQuery)
		}
		return `{"status":"success","content":"<external_data source=\"summary\">Root cause summary</external_data>"}`, nil
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "errors.log")
	if err := os.WriteFile(path, []byte(strings.Repeat("error spike on node A\n", 200)), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	raw := ExecuteSmartFileRead(context.Background(), SummaryLLMConfig{APIKey: "x", BaseURL: "https://example.com", Model: "test"}, nil, "summarize", "errors.log", "Find the root cause", "distributed", 1500, 10, dir)
	var result SmartFileReadResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("unmarshal result: %v", err)
	}
	if result.Status != "success" {
		t.Fatalf("status = %q, want success (%s)", result.Status, result.Message)
	}
	data := result.Data.(map[string]interface{})
	if !strings.Contains(data["content"].(string), "Root cause summary") {
		t.Fatalf("unexpected summary content: %v", data["content"])
	}
}
