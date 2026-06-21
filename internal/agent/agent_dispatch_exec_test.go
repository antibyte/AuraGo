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
	"aurago/internal/memory"
	"aurago/internal/tools"
)

func TestDispatchExecManageScheduleBlocksEnableInReadOnlyMode(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Scheduler.Enabled = true
	cfg.Tools.Scheduler.ReadOnly = true
	cronManager := tools.NewCronManager(t.TempDir())
	t.Cleanup(func() { _ = cronManager.Close() })

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "manage_schedule", Operation: "enable", Params: map[string]interface{}{"id": "job-1"}},
		&DispatchContext{
			Cfg:         cfg,
			Logger:      slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
			CronManager: cronManager,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle manage_schedule")
	}
	if !strings.Contains(out, "read-only mode") {
		t.Fatalf("expected read-only error, got %s", out)
	}
}

func TestDispatchExecOptimizeMemorySkipsKnowledgeGraphWhenReadOnly(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.MemoryMaintenance.Enabled = true
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.KnowledgeGraph.ReadOnly = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	if err := kg.AddNode("temporary", "Temporary", nil); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "optimize_memory", ThresholdLow: 1},
		&DispatchContext{
			Cfg:          cfg,
			Logger:       logger,
			ShortTermMem: stm,
			LongTermMem:  &fakeVectorDB{},
			KG:           kg,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle optimize_memory")
	}
	if !strings.Contains(out, `"graph_nodes_archived": 0`) {
		t.Fatalf("expected KG optimization to be skipped in read-only mode, got %s", out)
	}
	if node, err := kg.GetNode("temporary"); err != nil || node == nil {
		t.Fatalf("read-only optimize_memory removed KG node, node=%v err=%v", node, err)
	}
}

func TestDispatchExecKnowledgeGraphSupportsDocumentedOptimizeGraphAlias(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.MemoryMaintenance.Enabled = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	stm, err := memory.NewSQLiteMemory(filepath.Join(t.TempDir(), "memory.db"), logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "knowledge_graph", Operation: "optimize_graph", Preview: true},
		&DispatchContext{
			Cfg:          cfg,
			Logger:       logger,
			ShortTermMem: stm,
			LongTermMem:  &fakeVectorDB{},
			KG:           kg,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle knowledge_graph")
	}
	if strings.Contains(out, "Unknown graph operation") {
		t.Fatalf("expected optimize_graph alias to be accepted, got %s", out)
	}
	if !strings.Contains(out, `"preview": true`) {
		t.Fatalf("expected optimize_graph alias to run memory orchestrator preview, got %s", out)
	}
}

func TestDispatchExecKnowledgeGraphHealth(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.KnowledgeGraph.ReadOnly = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	if err := kg.AddNode("alpha", "Alpha", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode alpha: %v", err)
	}
	if err := kg.AddNode("beta", "Beta", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode beta: %v", err)
	}
	if err := kg.AddEdge("alpha", "beta", "co_mentioned_with", map[string]string{"source": "pending", "weight": "1"}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "knowledge_graph", Operation: "graph_health"},
		&DispatchContext{
			Cfg:    cfg,
			Logger: logger,
			KG:     kg,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle knowledge_graph")
	}
	payloadJSON := strings.TrimPrefix(out, "Tool Output: ")
	var payload struct {
		Status  string                             `json:"status"`
		Stats   memory.KnowledgeGraphStats         `json:"stats"`
		Quality memory.KnowledgeGraphQualityReport `json:"quality"`
	}
	if err := json.Unmarshal([]byte(payloadJSON), &payload); err != nil {
		t.Fatalf("unmarshal graph_health payload: %v\n%s", err, out)
	}
	if payload.Status != "success" {
		t.Fatalf("Status = %q, want success", payload.Status)
	}
	if payload.Stats.PendingCoMentionEdges != 1 || payload.Quality.LowConfidenceEdges != 1 {
		t.Fatalf("unexpected graph_health payload: %+v", payload)
	}
}

func TestDispatchExecRecallMemoryReadsByID(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Memory.Enabled = true
	longTerm := &fakeVectorDB{documents: map[string]string{
		"mem-1": "Retrieved deployment memory",
	}}
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "recall_memory", IDs: []string{"mem-1", "missing"}},
		&DispatchContext{
			Cfg:         cfg,
			Logger:      logger,
			LongTermMem: longTerm,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle recall_memory")
	}
	if !strings.Contains(out, `"id":"mem-1"`) || !strings.Contains(out, "Retrieved deployment memory") {
		t.Fatalf("recall_memory output missing retrieved memory: %s", out)
	}
	if !strings.Contains(out, `"missing":["missing"]`) {
		t.Fatalf("recall_memory output should report missing ids: %s", out)
	}
}

func TestDispatchExecExploreKGReadOnlyWrapper(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.KnowledgeGraph.Enabled = true
	cfg.Tools.KnowledgeGraph.ReadOnly = true

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	kg, err := memory.NewKnowledgeGraph(":memory:", "", logger)
	if err != nil {
		t.Fatalf("NewKnowledgeGraph: %v", err)
	}
	t.Cleanup(func() { _ = kg.Close() })
	nodeSensitiveValue := "node-sensitive-fixture-123456789"
	edgeSensitiveValue := "edge-sensitive-fixture-123456789"
	if err := kg.AddNode("alpha", "Alpha", map[string]string{"type": "service", "api_key": nodeSensitiveValue}); err != nil {
		t.Fatalf("AddNode alpha: %v", err)
	}
	if err := kg.AddNode("beta", "Beta", map[string]string{"type": "service"}); err != nil {
		t.Fatalf("AddNode beta: %v", err)
	}
	if err := kg.AddEdge("alpha", "beta", "depends_on", map[string]string{"token": edgeSensitiveValue}); err != nil {
		t.Fatalf("AddEdge: %v", err)
	}

	out, ok := dispatchExec(
		context.Background(),
		ToolCall{Action: "explore_kg", IDs: []string{"alpha"}, Depth: 1, Limit: 5},
		&DispatchContext{
			Cfg:    cfg,
			Logger: logger,
			KG:     kg,
		},
	)
	if !ok {
		t.Fatal("expected dispatchExec to handle explore_kg")
	}
	if !strings.Contains(out, `"center_id":"alpha"`) || !strings.Contains(out, `"relation":"depends_on"`) {
		t.Fatalf("explore_kg output missing subgraph data: %s", out)
	}
	if strings.Contains(out, nodeSensitiveValue) || strings.Contains(out, edgeSensitiveValue) {
		t.Fatalf("explore_kg output leaked sensitive KG properties: %s", out)
	}
	if !strings.Contains(out, "[redacted]") {
		t.Fatalf("explore_kg output should mark sensitive KG properties as redacted: %s", out)
	}
}

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
