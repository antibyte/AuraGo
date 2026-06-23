package agent

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func setupDispatchAgentSkillManagerWithDB(t *testing.T) (*tools.AgentSkillManager, *sql.DB, string) {
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
	return mgr, db, filepath.Join(tmp, "workspace")
}

func setupDispatchAgentSkillManager(t *testing.T) (*tools.AgentSkillManager, string) {
	mgr, _, workspace := setupDispatchAgentSkillManagerWithDB(t)
	return mgr, workspace
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
	if !strings.Contains(out, "<external_data type=\"agent_skill\"") || !strings.Contains(out, "# Demo") {
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
	if !strings.Contains(out, `\"value\": \"hello\"`) && !strings.Contains(out, `\"value\":\"hello\"`) {
		t.Fatalf("run_agent_skill_script output = %s", out)
	}
	if strings.Count(out, `value`) != 1 {
		t.Fatalf("run_agent_skill_script should include script output once, got %d copies in %s", strings.Count(out, `value`), out)
	}
}

func TestAgentSkillActivationWrapsBodyAsEscapedExternalData(t *testing.T) {
	mgr, workspace := setupDispatchAgentSkillManager(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "wrapper-demo", "Wrapper demo. Use when testing Agent Skill prompt wrapping.", "# Demo\nLiteral </agent_skill> must stay inert.", "test", nil, false)
	if err != nil {
		t.Fatalf("CreateAgentSkill: %v", err)
	}
	if err := mgr.EnableAgentSkill(entry.ID, true, "test"); err != nil {
		t.Fatalf("EnableAgentSkill: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.WorkspaceDir = workspace
	out, ok := dispatchComm(context.Background(), ToolCall{Action: "activate_agent_skill", Skill: "wrapper-demo"}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle activate_agent_skill")
	}
	if strings.Contains(out, "</agent_skill>") {
		t.Fatalf("activation leaked raw wrapper sentinel: %s", out)
	}
	if !strings.Contains(out, "\\u003c/agent_skill\\u003e") {
		t.Fatalf("activation did not JSON-escape wrapper sentinel: %s", out)
	}
}

func TestLoggableToolArgKeysOmitValues(t *testing.T) {
	keys := loggableToolArgKeys(map[string]interface{}{
		"api_token": "secret-value",
		"query":     "visible-value",
	})
	joined := strings.Join(keys, ",")
	if strings.Contains(joined, "secret-value") || strings.Contains(joined, "visible-value") {
		t.Fatalf("loggable keys leaked values: %q", joined)
	}
	if !strings.Contains(joined, "api_token") || !strings.Contains(joined, "query") {
		t.Fatalf("loggable keys missing expected keys: %q", joined)
	}
}

func TestAgentSkillDispatchRejectsStalePackageActivation(t *testing.T) {
	mgr, workspace := setupDispatchAgentSkillManager(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "stale-demo", "Stale Agent Skill. Use when testing stale activation.", "# Demo\nOriginal body.", "test", nil, false)
	if err != nil {
		t.Fatalf("CreateAgentSkill: %v", err)
	}
	if err := mgr.EnableAgentSkill(entry.ID, true, "test"); err != nil {
		t.Fatalf("EnableAgentSkill: %v", err)
	}
	staleBody := `---
name: stale-demo
description: Stale Agent Skill. Use when testing stale activation.
---
# Changed

This changed outside the Agent Skill Manager.
`
	if err := os.WriteFile(filepath.Join(entry.Directory, "SKILL.md"), []byte(staleBody), 0644); err != nil {
		t.Fatalf("mutate SKILL.md: %v", err)
	}

	cfg := &config.Config{}
	cfg.Agent.AllowPython = true
	cfg.Directories.WorkspaceDir = workspace
	dc := &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	out, ok := dispatchComm(context.Background(), ToolCall{Action: "activate_agent_skill", Skill: "stale-demo"}, dc)
	if !ok {
		t.Fatal("expected dispatchComm to handle activate_agent_skill")
	}
	if !strings.Contains(out, "changed since last verification") {
		t.Fatalf("activate_agent_skill output = %s, want stale package error", out)
	}
	if strings.Contains(out, "# Changed") {
		t.Fatalf("activate_agent_skill exposed stale SKILL.md body: %s", out)
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

func TestAgentSkillDispatchListSkipsUnapprovedWarning(t *testing.T) {
	mgr, db, _ := setupDispatchAgentSkillManagerWithDB(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "warn-list", "Warn list test. Use when testing list filter.", "# Warn\nReads /etc/passwd.", "test", nil, false)
	if err != nil {
		t.Fatalf("CreateAgentSkill: %v", err)
	}
	if _, err := db.Exec("UPDATE agent_skills_registry SET enabled = 1 WHERE id = ?", entry.ID); err != nil {
		t.Fatalf("force enabled: %v", err)
	}
	out, ok := dispatchComm(context.Background(), ToolCall{Action: "list_agent_skills"}, &DispatchContext{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected list_agent_skills")
	}
	if strings.Contains(out, "warn-list") {
		t.Fatalf("list should skip unapproved warning: %s", out)
	}
}

func TestAgentSkillPromptCatalogSkipsUnapprovedWarning(t *testing.T) {
	mgr, db, _ := setupDispatchAgentSkillManagerWithDB(t)
	entry, err := mgr.CreateAgentSkill(context.Background(), "warn-cat", "Warn catalog test. Use when testing catalog filter.", "# Warn\nReads /etc/passwd.", "test", nil, false)
	if err != nil {
		t.Fatalf("CreateAgentSkill: %v", err)
	}
	if _, err := db.Exec("UPDATE agent_skills_registry SET enabled = 1 WHERE id = ?", entry.ID); err != nil {
		t.Fatalf("force enabled: %v", err)
	}
	catalog := buildAgentSkillsPromptCatalog()
	if strings.Contains(catalog, "warn-cat") {
		t.Fatalf("catalog should skip unapproved warning: %s", catalog)
	}
}
