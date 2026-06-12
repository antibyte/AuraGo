package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"aurago/internal/config"
	"aurago/internal/prompts"
	taskrules "aurago/internal/rules"
	promptsembed "aurago/prompts"
)

type taskRulePromptContext struct {
	TaskRules            string
	HomepageDesignSystem string
	RuleIDs              []string
}

var (
	homepageWorkflowIntentPattern = regexp.MustCompile(`(?i)\b(?:homepage|website|webseite|startseite|landing\s*page|landingpage|web\s*app|netlify|vercel)\b`)
	genericPageIntentPattern      = regexp.MustCompile(`(?i)\b(?:seite|site|page)\b`)
	homepageActionIntentPattern   = regexp.MustCompile(`(?i)\b(?:erstelle|erstellen|baue|bauen|lösche|loesche|neubauen|neu\s+aufsetzen|aufsetzen|redesign|deploy|veröffentliche|veroeffentliche|publish|create|build|rebuild|delete|recreate|redesign|deploy|publish)\b`)
)

func buildTaskRulePromptContext(cfg *config.Config, prompt string, tools, workflows []string, homepageProjectDir string) taskRulePromptContext {
	if cfg == nil || !cfg.Rules.Enabled {
		return taskRulePromptContext{}
	}
	promptsDir := cfg.Directories.PromptsDir
	if strings.TrimSpace(promptsDir) == "" {
		promptsDir = "prompts"
	}
	catalog, err := taskrules.LoadCatalog(taskrules.LoadOptions{
		PromptsDir: promptsDir,
		EmbeddedFS: promptsembed.FS,
	})
	if err != nil {
		return taskRulePromptContext{}
	}
	workflows = append(workflows, inferRuleWorkflows(prompt, tools)...)
	selection := catalog.Match(taskrules.MatchContext{
		Prompt:    prompt,
		Tools:     tools,
		Workflows: workflows,
	})
	if projectDesign := loadHomepageProjectDesign(cfg.Homepage.WorkspacePath, homepageProjectDir); projectDesign != "" {
		if hasRule(selection.Rules, "homepage") {
			selection.Designs = append(selection.Designs, taskrules.Design{
				ID:      "homepage project",
				Content: projectDesign,
				Source:  "project",
			})
		}
	}
	ids := make([]string, 0, len(selection.Rules))
	for _, rule := range selection.Rules {
		ids = append(ids, rule.ID)
	}
	return taskRulePromptContext{
		TaskRules:            taskrules.RenderRules(selection.Rules),
		HomepageDesignSystem: taskrules.RenderDesigns(selection.Designs),
		RuleIDs:              ids,
	}
}

func applyTaskRulePromptContext(flags *prompts.ContextFlags, ctx taskRulePromptContext) {
	if flags == nil {
		return
	}
	flags.TaskRules = ctx.TaskRules
	flags.HomepageDesignSystem = ctx.HomepageDesignSystem
	flags.TaskRuleIDs = append([]string(nil), ctx.RuleIDs...)
}

func ensureTaskRulesBeforeToolExecution(s *agentLoopState, tc ToolCall, lastUserMsg string) (string, bool) {
	if s == nil || s.runCfg.Config == nil || !s.runCfg.Config.Rules.Enabled || tc.Action == "" {
		return "", false
	}
	projectDir := tc.ProjectDir
	if projectDir == "" {
		projectDir = toolArgString(tc.Params, "project_dir")
	}
	ctx := buildTaskRulePromptContext(s.runCfg.Config, lastUserMsg, []string{tc.Action}, nil, projectDir)
	if len(ctx.RuleIDs) == 0 {
		return "", false
	}

	projectDesignChanged := false
	if projectDir != "" && isHomepageRuleTool(tc.Action) {
		trimmedProjectDir := strings.TrimSpace(projectDir)
		if trimmedProjectDir != "" &&
			s.homepageRuleProjectDir != trimmedProjectDir &&
			strings.TrimSpace(ctx.HomepageDesignSystem) != strings.TrimSpace(s.flags.HomepageDesignSystem) {
			projectDesignChanged = true
		}
		s.homepageRuleProjectDir = trimmedProjectDir
	}
	if taskRuleIDsLoaded(s.flags.TaskRuleIDs, ctx.RuleIDs) && !projectDesignChanged {
		return "", false
	}

	applyTaskRulePromptContext(&s.flags, ctx)
	s.cachedSysPromptKey = ""
	s.cachedSysPrompt = ""
	s.cachedSysPromptAt = time.Time{}
	return fmt.Sprintf(`{"status":"blocked","message":"Required task rules have been loaded for %s. The tool was not executed yet. Re-read the TASK RULES section and retry only if the requested action still complies with those rules."}`, tc.Action), true
}

func isHomepageRuleTool(action string) bool {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "homepage", "homepage_tool":
		return true
	default:
		return false
	}
}

func taskRuleIDsLoaded(loaded, required []string) bool {
	if len(required) == 0 {
		return true
	}
	set := map[string]bool{}
	for _, id := range loaded {
		set[id] = true
	}
	for _, id := range required {
		if !set[id] {
			return false
		}
	}
	return true
}

func inferRuleWorkflows(prompt string, tools []string) []string {
	lower := strings.ToLower(prompt)
	workflows := []string{}
	for _, tool := range tools {
		switch strings.ToLower(strings.TrimSpace(tool)) {
		case "homepage", "homepage_tool":
			workflows = append(workflows, "homepage")
		}
	}
	if homepageWorkflowIntentPattern.MatchString(lower) ||
		(genericPageIntentPattern.MatchString(lower) && homepageActionIntentPattern.MatchString(lower)) {
		workflows = append(workflows, "homepage")
	}
	return workflows
}

func loadHomepageProjectDesign(workspacePath, projectDir string) string {
	workspacePath = strings.TrimSpace(workspacePath)
	projectDir = strings.TrimSpace(projectDir)
	if workspacePath == "" || projectDir == "" || filepath.IsAbs(projectDir) {
		return ""
	}
	cleanProject := filepath.Clean(projectDir)
	if cleanProject == "." || strings.HasPrefix(cleanProject, "..") || strings.Contains(cleanProject, string(filepath.Separator)+".."+string(filepath.Separator)) {
		return ""
	}
	workspaceAbs, err := filepath.Abs(workspacePath)
	if err != nil {
		return ""
	}
	designPath := filepath.Join(workspaceAbs, cleanProject, "DESIGN.md")
	designAbs, err := filepath.Abs(designPath)
	if err != nil || !strings.HasPrefix(designAbs, workspaceAbs+string(filepath.Separator)) {
		return ""
	}
	data, err := os.ReadFile(designAbs)
	if err != nil || len(data) > taskrules.MaxDesignBytes {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func hasRule(rules []taskrules.Rule, id string) bool {
	for _, rule := range rules {
		if rule.ID == id {
			return true
		}
	}
	return false
}
