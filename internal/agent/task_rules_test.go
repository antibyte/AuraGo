package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/prompts"
)

func TestBuildTaskRulePromptContextSelectsHomepageRuleAndProjectDesign(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	workspace := filepath.Join(root, "homepage")
	projectDir := filepath.Join(workspace, "my-site")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "DESIGN.md"), []byte("# Project Design\n\n## Colors\n- Primary #FF00AA"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = promptsDir
	cfg.Homepage.WorkspacePath = workspace

	ctx := buildTaskRulePromptContext(cfg, "Build a homepage landing page", []string{"homepage"}, []string{"homepage"}, "my-site")
	if !strings.Contains(ctx.TaskRules, "Homepage Workflow") {
		t.Fatalf("TaskRules missing homepage rule:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.HomepageDesignSystem, "Atmospheric Glass") {
		t.Fatalf("HomepageDesignSystem missing global design:\n%s", ctx.HomepageDesignSystem)
	}
	if !strings.Contains(ctx.HomepageDesignSystem, "Project Design") {
		t.Fatalf("HomepageDesignSystem missing project design:\n%s", ctx.HomepageDesignSystem)
	}
}

func TestBuildTaskRulePromptContextHonorsDisabledConfig(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = false
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "Build a homepage", []string{"homepage"}, []string{"homepage"}, "")
	if ctx.TaskRules != "" || ctx.HomepageDesignSystem != "" {
		t.Fatalf("expected disabled rules to produce empty context, got %+v", ctx)
	}
}

func TestBuildTaskRulePromptContextTreatsGermanPageRebuildAsHomepageWorkflow(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "lösche die ki news seite und erstelle sie komplett neu", nil, nil, "")
	if !strings.Contains(ctx.TaskRules, "Homepage Workflow") {
		t.Fatalf("TaskRules missing homepage rule for German page rebuild request:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.HomepageDesignSystem, "Atmospheric Glass") {
		t.Fatalf("HomepageDesignSystem missing Atmospheric Glass for German page rebuild request:\n%s", ctx.HomepageDesignSystem)
	}
	if !strings.Contains(ctx.TaskRules, "generic dark purple/blue card UI") {
		t.Fatalf("homepage rule should include explicit Atmospheric Glass guardrail:\n%s", ctx.TaskRules)
	}
}

func TestEnsureTaskRulesBeforeToolExecutionLoadsProjectDesignAfterInitialRule(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	promptsDir := filepath.Join(root, "prompts")
	workspace := filepath.Join(root, "homepage")
	projectDir := filepath.Join(workspace, "my-site")
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "DESIGN.md"), []byte("# Project Design\n\n## Colors\n- Accent #00FF99"), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = promptsDir
	cfg.Homepage.WorkspacePath = workspace

	initial := buildTaskRulePromptContext(cfg, "Build a homepage", nil, nil, "")
	if !strings.Contains(initial.TaskRules, "Homepage Workflow") {
		t.Fatalf("initial context missing homepage rule:\n%s", initial.TaskRules)
	}
	if strings.Contains(initial.HomepageDesignSystem, "Project Design") {
		t.Fatalf("project design should not load before project_dir is known:\n%s", initial.HomepageDesignSystem)
	}

	state := &agentLoopState{
		runCfg: RunConfig{Config: cfg},
		flags: prompts.ContextFlags{
			TaskRules:            initial.TaskRules,
			HomepageDesignSystem: initial.HomepageDesignSystem,
			TaskRuleIDs:          initial.RuleIDs,
		},
	}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "homepage", ProjectDir: "my-site"}, "Build a homepage")
	if !blocked {
		t.Fatal("expected homepage tool to be paused so the project DESIGN.md can be injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked tool output, got: %s", result)
	}
	if !strings.Contains(state.flags.HomepageDesignSystem, "Project Design") {
		t.Fatalf("project DESIGN.md was not injected:\n%s", state.flags.HomepageDesignSystem)
	}
	if state.homepageRuleProjectDir != "my-site" {
		t.Fatalf("homepageRuleProjectDir = %q, want my-site", state.homepageRuleProjectDir)
	}
}
