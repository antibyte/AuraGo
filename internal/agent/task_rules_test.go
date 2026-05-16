package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
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
