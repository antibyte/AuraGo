package agent

import (
	"context"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func setupDispatchAgentSkillManager(t *testing.T) (*tools.AgentSkillManager, string) {
	t.Helper()
	tmp := t.TempDir()
	db, err := tools.InitAgentSkillsDB(filepath.Join(tmp, "agent_skills.db"))
	if err != nil {
		t.Fatalf("InitAgentSkillsDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mgr := tools.NewAgentSkillManager(db, filepath.Join(tmp, "agent_skills"), filepath.Join(tmp, "workspace"), slog.New(slog.NewTextHandler(io.Discard, nil)))
	old := tools.DefaultAgentSkillManager()
	tools.SetDefaultAgentSkillManager(mgr)
	t.Cleanup(func() { tools.SetDefaultAgentSkillManager(old) })
	return mgr, filepath.Join(tmp, "workspace")
}

func TestAgentSkillDispatchListActivateAndRunScript(t *testing.T) {
	mgr, workspace := setupDispatchAgentSkillManager(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "agent-demo", "Demo Agent Skill. Use when testing Agent Skill dispatch.", "# Demo\nUse scripts/echo.py.", "test", nil, false)
	if err != nil {
		t.Fatalf("CreateAgentSkill: %v", err)
	}
	if err := mgr.WriteAgentSkillFile(context.Background(), entry.ID, "scripts/echo.py", `import json
import sys

args = json.loads(sys.stdin.read() or "{}")
print(json.dumps({"value": args.get("value")}))
`, "test", nil, false); err != nil {
		t.Fatalf("WriteAgentSkillFile: %v", err)
	}
	if err := mgr.EnableAgentSkill(entry.ID, true, "test"); err != nil {
		t.Fatalf("EnableAgentSkill: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.WorkspaceDir = workspace
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	dc.Cfg.Directories.WorkspaceDir = workspace

	out, ok := dispatchComm(context.Background(), ToolCall{Action: "list_agent_skills"}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle list_agent_skills")
	}
	if !strings.Contains(out, "agent-demo") || !strings.Contains(out, "activate_agent_skill") {
		t.Fatalf("list_agent_skills output = %s", out)
	}

	out, ok = dispatchComm(context.Background(), ToolCall{Action: "activate_agent_skill", Skill: "agent-demo"}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle activate_agent_skill")
	}
	if !strings.Contains(out, "<agent_skill name=\"agent-demo\"") || !strings.Contains(out, "# Demo") {
		t.Fatalf("activate_agent_skill output = %s", out)
	}

	if exec.Command("python", "--version").Run() != nil && exec.Command("python3", "--version").Run() != nil {
		t.Skip("system Python not available")
	}
	out, ok = dispatchComm(context.Background(), ToolCall{
		Action: "run_agent_skill_script",
		Skill:  "agent-demo",
		Params: map[string]interface{}{
			"script": "scripts/echo.py",
			"args":   map[string]interface{}{"value": "hello"},
		},
	}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle run_agent_skill_script")
	}
	if !strings.Contains(out, `"value": "hello"`) && !strings.Contains(out, `"value":"hello"`) {
		t.Fatalf("run_agent_skill_script output = %s", out)
	}
}

func TestAgentSkillNativeSchemasAreExposed(t *testing.T) {
	names := toolNames(builtinToolSchemas(ToolFeatureFlags{}))
	for _, name := range []string{"list_agent_skills", "activate_agent_skill", "run_agent_skill_script"} {
		if !containsName(names, name) {
			t.Fatalf("builtin schemas missing %s: %v", name, names)
		}
	}
}
