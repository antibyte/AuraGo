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

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "list_tools"},
		&DispatchContext{
			Cfg:      &config.Config{},
			Logger:   logger,
			Manifest: manifest,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle list_tools")
	}

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

func TestDispatchExecSaveToolRejectsBuiltinNameCollision(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("mkdir tools dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.ToolsDir = toolsDir
	manifest := tools.NewManifest(toolsDir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{
			Action:      "save_tool",
			Name:        "virustotal_scan",
			Description: "Collision test",
			Code:        "print('hello')",
		},
		&DispatchContext{
			Cfg:      cfg,
			Logger:   logger,
			Manifest: manifest,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle save_tool")
	}

	if !strings.Contains(out, "collides with built-in tool") {
		t.Fatalf("expected built-in collision error, got:\n%s", out)
	}
}

func TestDispatchExecSaveToolUsesParamsFallback(t *testing.T) {
	tmpDir := t.TempDir()
	toolsDir := filepath.Join(tmpDir, "tools")
	if err := os.MkdirAll(toolsDir, 0o755); err != nil {
		t.Fatalf("mkdir tools dir: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.ToolsDir = toolsDir
	manifest := tools.NewManifest(toolsDir)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{
			Action: "save_tool",
			Params: map[string]interface{}{
				"name":        "demo_tool",
				"description": "Demo via params",
				"code":        "print('hello')",
			},
		},
		&DispatchContext{
			Cfg:      cfg,
			Logger:   logger,
			Manifest: manifest,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle save_tool")
	}
	if !strings.Contains(out, "demo_tool") {
		t.Fatalf("expected save_tool success output, got:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(toolsDir, "demo_tool")); err != nil {
		t.Fatalf("expected saved tool file, got stat error: %v", err)
	}
}
