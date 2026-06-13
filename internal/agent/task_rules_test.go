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

func TestBuildTaskRulePromptContextIncludesHomepageLocalAssetGuidance(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "Build a homepage with local images", []string{"homepage"}, []string{"homepage"}, "")
	for _, marker := range []string{"Local Asset References", "project-relative web URLs", "public/assets/hero.jpg", "/assets/hero.jpg", "file://"} {
		if !strings.Contains(ctx.TaskRules, marker) {
			t.Fatalf("homepage rule missing local asset guidance marker %q:\n%s", marker, ctx.TaskRules)
		}
	}
}

func TestBuildTaskRulePromptContextSelectsPDFRuleByKeyword(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "erstelle ein PDF aus diesem Bericht", nil, nil, "")
	if !strings.Contains(ctx.TaskRules, "PDF Creation Workflow") {
		t.Fatalf("TaskRules missing PDF rule for PDF creation request:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "Always inspect the rendered PDF visually") {
		t.Fatalf("PDF rule should require visual verification:\n%s", ctx.TaskRules)
	}
}

func TestBuildTaskRulePromptContextSelectsVirtualDesktopRuleByKeyword(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "erstelle im virtuellen Desktop eine App mit Widget", nil, nil, "")
	if !strings.Contains(ctx.TaskRules, "Generated App And Widget Creation Workflow") {
		t.Fatalf("TaskRules missing virtual desktop rule for app/widget request:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "Use `install_app` for generated apps") {
		t.Fatalf("virtual desktop rule should include app creation guidance:\n%s", ctx.TaskRules)
	}
}

func TestBuildTaskRulePromptContextSelectsSkillCreationRuleByKeyword(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "erstelle einen skill der docker als internes tool nutzt", nil, nil, "")
	if !strings.Contains(ctx.TaskRules, "Skill Creation Workflow") {
		t.Fatalf("TaskRules missing skill creation rule for skill request:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "tools.python_tool_bridge.allowed_tools") {
		t.Fatalf("skill creation rule should include tool bridge allowlist guidance:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "Choose The Skill Type First") || !strings.Contains(ctx.TaskRules, "Agent Skill Package Shape") {
		t.Fatalf("skill creation rule should include Python vs Agent Skill guidance:\n%s", ctx.TaskRules)
	}
}

func TestBuildTaskRulePromptContextSelectsSkillCreationRuleForAgentSkillKeyword(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	ctx := buildTaskRulePromptContext(cfg, "erstelle einen Agent Skill nach agentskills.io mit SKILL.md", nil, nil, "")
	if !strings.Contains(ctx.TaskRules, "Skill Creation Workflow") {
		t.Fatalf("TaskRules missing skill creation rule for Agent Skill request:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "Agent Skill Package Shape") {
		t.Fatalf("skill creation rule should include Agent Skill package guidance:\n%s", ctx.TaskRules)
	}
	if !strings.Contains(ctx.TaskRules, "Agent Skill Manager Workflow") || !strings.Contains(ctx.TaskRules, "POST /api/agent-skills/import") {
		t.Fatalf("skill creation rule should include concrete Agent Skill Manager workflow guidance:\n%s", ctx.TaskRules)
	}
}

func TestEnsureTaskRulesBeforeHomepageToolDoesNotDependOnIntentLanguage(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{Config: cfg}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "homepage"}, "xyzzy")
	if !blocked {
		t.Fatal("expected homepage tool call to be blocked until required rules are injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "Homepage Workflow") {
		t.Fatalf("homepage tool did not inject required rule:\n%s", state.flags.TaskRules)
	}
	if !strings.Contains(state.flags.HomepageDesignSystem, "Atmospheric Glass") {
		t.Fatalf("homepage tool did not inject design system:\n%s", state.flags.HomepageDesignSystem)
	}
}

func TestEnsureTaskRulesBeforeHomepageToolAppliesToMissionRuns(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{
		Config:        cfg,
		IsMission:     true,
		MessageSource: "mission",
	}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "homepage"}, "mission: build a landing page")
	if !blocked {
		t.Fatal("expected mission homepage tool call to be blocked until required rules are injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "Homepage Workflow") {
		t.Fatalf("mission homepage tool did not inject required rule:\n%s", state.flags.TaskRules)
	}
	if !strings.Contains(state.flags.HomepageDesignSystem, "Atmospheric Glass") {
		t.Fatalf("mission homepage tool did not inject design system:\n%s", state.flags.HomepageDesignSystem)
	}
}

func TestEnsureTaskRulesBeforeVirtualDesktopToolLoadsRule(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{Config: cfg}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "virtual_desktop"}, "erstelle ein desktop widget")
	if !blocked {
		t.Fatal("expected virtual_desktop tool call to be blocked until required rules are injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "Generated App And Widget Creation Workflow") {
		t.Fatalf("virtual_desktop tool did not inject required rule:\n%s", state.flags.TaskRules)
	}
}

func TestEnsureTaskRulesBeforeCreateSkillToolLoadsRule(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{Config: cfg}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "create_skill_from_template"}, "erstelle einen skill")
	if !blocked {
		t.Fatal("expected create_skill_from_template call to be blocked until required rules are injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "Skill Creation Workflow") {
		t.Fatalf("create_skill_from_template did not inject required rule:\n%s", state.flags.TaskRules)
	}
	if !strings.Contains(state.flags.TaskRules, "Assign Internal Tools") {
		t.Fatalf("skill creation rule should tell the agent to mention user tool approval:\n%s", state.flags.TaskRules)
	}
}

func TestEnsureTaskRulesBeforeDocumentCreatorToolLoadsPDFRule(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{Config: cfg}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "document_creator"}, "erstelle das dokument")
	if !blocked {
		t.Fatal("expected document_creator call to be blocked until PDF rules are injected")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "PDF Creation Workflow") {
		t.Fatalf("document_creator tool did not inject required PDF rule:\n%s", state.flags.TaskRules)
	}
}

func TestEnsureTaskRulesBeforeGenericFilesystemUsesHomepageIntentFallback(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.Rules.Enabled = true
	cfg.Directories.PromptsDir = t.TempDir()

	state := &agentLoopState{runCfg: RunConfig{Config: cfg}}
	result, blocked := ensureTaskRulesBeforeToolExecution(state, ToolCall{Action: "filesystem"}, "lösche die ki news seite und erstelle sie komplett neu")
	if !blocked {
		t.Fatal("expected generic filesystem call to be blocked by homepage intent fallback")
	}
	if !strings.Contains(result, `"status":"blocked"`) {
		t.Fatalf("expected blocked result, got: %s", result)
	}
	if !strings.Contains(state.flags.TaskRules, "Homepage Workflow") {
		t.Fatalf("homepage fallback did not inject required rule:\n%s", state.flags.TaskRules)
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
