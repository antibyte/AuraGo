package agent

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDispatchExecListToolsClarifiesBuiltinSkills(t *testing.T) {
	tmpDir := t.TempDir()
	manifest := tools.NewManifest(filepath.Join(tmpDir, "tools"))
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	out := dispatchExec(
		context.Background(),
		ToolCall{Action: "list_tools"},
		&DispatchContext{
			Cfg:      &config.Config{},
			Logger:   logger,
			Manifest: manifest,
		},
	)

	for _, snippet := range []string{
		"list_tools' ONLY lists custom reusable Python tools",
		"virustotal_scan",
		"list_skills",
		"Do NOT assume an integration is unavailable",
	} {
		if !strings.Contains(out, snippet) {
			t.Fatalf("expected list_tools output to contain %q, got:\n%s", snippet, out)
		}
	}
}

func TestBuildMemoryReflectionOutputSerializesResult(t *testing.T) {
	out, err := buildMemoryReflectionOutput(map[string]interface{}{"summary": "ok"})
	if err != nil {
		t.Fatalf("buildMemoryReflectionOutput returned error: %v", err)
	}
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("expected success envelope, got %s", out)
	}
	if !strings.Contains(out, `"summary":"ok"`) {
		t.Fatalf("expected marshaled reflection payload, got %s", out)
	}

	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(out, "Tool Output: ")), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v", err)
	}
}

func TestBuildMemoryReflectionOutputReturnsMarshalError(t *testing.T) {
	if _, err := buildMemoryReflectionOutput(map[string]interface{}{"bad": make(chan int)}); err == nil {
		t.Fatal("expected marshal error for unsupported reflection payload")
	}
}
