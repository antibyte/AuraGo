package agent

import (
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func newReuseTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelError}))
}

func setupReuseCheatsheetDB(t *testing.T) *sqlDBWrapper {
	t.Helper()
	db, err := tools.InitCheatsheetDB(filepath.Join(t.TempDir(), "cheatsheets.db"))
	if err != nil {
		t.Fatalf("InitCheatsheetDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return &sqlDBWrapper{DB: db}
}

type sqlDBWrapper struct {
	DB *sql.DB
}

func setupReuseSkillManager(t *testing.T) (*tools.SkillManager, string) {
	t.Helper()
	logger := newReuseTestLogger()
	root := t.TempDir()
	skillsDir := filepath.Join(root, "skills")
	db, err := tools.InitSkillsDB(filepath.Join(root, "skills.db"))
	if err != nil {
		t.Fatalf("InitSkillsDB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	mgr := tools.NewSkillManager(db, skillsDir, logger)
	tools.SetDefaultSkillManager(mgr)
	t.Cleanup(func() { tools.SetDefaultSkillManager(nil) })
	return mgr, skillsDir
}

func TestClassifyTaskComplexity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
		want  TaskComplexity
	}{
		{name: "simple factual ask stays trivial", query: "what time is it", want: TaskComplexityTrivial},
		{name: "debugging becomes non trivial", query: "debug the failing docker deployment and capture the recurring fix", want: TaskComplexityNonTrivial},
		{name: "multi step request becomes non trivial", query: "analyze the logs, fix the issue, then verify the service", want: TaskComplexityNonTrivial},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := classifyTaskComplexity(tt.query); got != tt.want {
				t.Fatalf("classifyTaskComplexity(%q) = %q, want %q", tt.query, got, tt.want)
			}
		})
	}
}

func TestBuildReuseLookupFindsCheatsheetAndSkill(t *testing.T) {
	logger := newReuseTestLogger()
	csDB := setupReuseCheatsheetDB(t).DB
	if _, err := tools.CheatsheetCreate(csDB, "Docker Recovery Workflow", "Check the container logs and validate the restarted service.", "agent"); err != nil {
		t.Fatalf("CheatsheetCreate: %v", err)
	}

	mgr, _ := setupReuseSkillManager(t)
	skill, err := mgr.CreateSkillEntry("log_triage", "Analyze recurring deployment logs", "print('ok')\n", tools.SkillTypeAgent, "agent", "ops", []string{"logs"})
	if err != nil {
		t.Fatalf("CreateSkillEntry: %v", err)
	}
	if err := mgr.EnableSkill(skill.ID, true, "test"); err != nil {
		t.Fatalf("EnableSkill: %v", err)
	}

	lookup := buildReuseLookup("debug the docker deployment logs after the restart error", nil, csDB, logger)
	if lookup.Complexity != TaskComplexityNonTrivial {
		t.Fatalf("Complexity = %q, want %q", lookup.Complexity, TaskComplexityNonTrivial)
	}
	if !lookup.Performed {
		t.Fatal("expected lookup to be performed")
	}
	if len(lookup.CheatsheetHits) == 0 || lookup.CheatsheetHits[0].Name != "Docker Recovery Workflow" {
		t.Fatalf("expected cheatsheet hit, got %+v", lookup.CheatsheetHits)
	}
	if len(lookup.SkillHits) == 0 || lookup.SkillHits[0].Name != "log_triage" {
		t.Fatalf("expected skill hit, got %+v", lookup.SkillHits)
	}
	if strings.TrimSpace(lookup.Prompt) == "" {
		t.Fatal("expected reuse prompt context to be populated")
	}
}

func TestEvaluateReusabilityUsesAgentOwnershipForUpdates(t *testing.T) {
	agentCheatsheet := &tools.CheatSheet{ID: "cs-agent", Name: "Nginx Recovery", CreatedBy: "agent", Content: "old"}
	agentSkill := &tools.SkillRegistryEntry{ID: "sk-agent", Name: "log_analyzer_helper", CreatedBy: "agent", Category: "ops"}

	lookup := ReuseLookupResult{
		Query:      "automate docker deployment recovery workflow after restart failures",
		Complexity: TaskComplexityNonTrivial,
		Performed:  true,
		CheatsheetHits: []reuseArtifactHit{{
			Name:       agentCheatsheet.Name,
			Ownership:  "agent",
			Score:      0.9,
			Cheatsheet: agentCheatsheet,
		}},
		SkillHits: []reuseArtifactHit{{
			Name:      agentSkill.Name,
			Ownership: "agent",
			Score:     0.88,
			Skill:     agentSkill,
		}},
	}

	eval := evaluateReusability(
		"automate docker deployment recovery workflow after restart failures",
		"Resolved the restart failure by fixing environment variables, rebuilding the container, restarting the stack, and validating health checks. Captured a repeatable recovery workflow.",
		[]string{"docker", "execute_shell", "manage_files"},
		[]string{
			"inspect compose configuration and identify missing environment variables",
			"rebuild and restart the docker stack",
			"validate container health checks and service responses",
		},
		lookup,
	)
	if eval.Decision != ReusableArtifactUpdateBoth {
		t.Fatalf("Decision = %q, want %q", eval.Decision, ReusableArtifactUpdateBoth)
	}
	if eval.ExistingAgentCheatsheet == nil || eval.ExistingAgentSkill == nil {
		t.Fatal("expected existing agent-owned artifacts to be selected for update")
	}
}

func TestEvaluateReusabilityDoesNotUpdateUserOwnedArtifacts(t *testing.T) {
	userCheatsheet := &tools.CheatSheet{ID: "cs-user", Name: "Nginx Recovery", CreatedBy: "user", Content: "user content"}
	userSkill := &tools.SkillRegistryEntry{ID: "sk-user", Name: "log_analyzer_helper", CreatedBy: "user", Category: "ops"}

	lookup := ReuseLookupResult{
		Query:      "automate docker deployment recovery workflow after restart failures",
		Complexity: TaskComplexityNonTrivial,
		Performed:  true,
		CheatsheetHits: []reuseArtifactHit{{
			Name:       userCheatsheet.Name,
			Ownership:  "user",
			Score:      0.9,
			Cheatsheet: userCheatsheet,
		}},
		SkillHits: []reuseArtifactHit{{
			Name:      userSkill.Name,
			Ownership: "user",
			Score:     0.88,
			Skill:     userSkill,
		}},
	}

	eval := evaluateReusability(
		"automate docker deployment recovery workflow after restart failures",
		"Resolved the restart failure by fixing environment variables, rebuilding the container, restarting the stack, and validating health checks. Captured a repeatable recovery workflow.",
		[]string{"docker", "execute_shell", "manage_files"},
		[]string{
			"inspect compose configuration and identify missing environment variables",
			"rebuild and restart the docker stack",
			"validate container health checks and service responses",
		},
		lookup,
	)
	if eval.Decision != ReusableArtifactCreateBoth {
		t.Fatalf("Decision = %q, want %q", eval.Decision, ReusableArtifactCreateBoth)
	}
	if eval.ExistingAgentCheatsheet != nil || eval.ExistingAgentSkill != nil {
		t.Fatal("did not expect user-owned artifacts to be selected for automatic updates")
	}
}

func TestEvaluateReusabilitySkipsArtifactCreationWithoutExecutedTools(t *testing.T) {
	evaluation := evaluateReusability(
		"du sollst obsidian erneut testen schreiben",
		"Ich wuerde jetzt Obsidian testen.",
		nil,
		nil,
		ReuseLookupResult{Complexity: TaskComplexityNonTrivial},
	)

	if evaluation.Decision != ReusableArtifactNone {
		t.Fatalf("Decision=%q, want none", evaluation.Decision)
	}
	if evaluation.Reason != "task_not_substantial_enough" {
		t.Fatalf("Reason=%q, want task_not_substantial_enough", evaluation.Reason)
	}
}

func TestEvaluateReusabilitySkipsSimpleVerificationTasks(t *testing.T) {
	evaluation := evaluateReusability(
		"teste nochmal ob obsidian schreiben geht",
		"Ich habe den Schreibzugriff noch einmal geprueft und den Inhalt wieder eingelesen.",
		[]string{"obsidian"},
		[]string{
			"create a temporary note in obsidian",
			"read the note back to compare the content",
		},
		ReuseLookupResult{Complexity: TaskComplexityNonTrivial},
	)

	if evaluation.Decision != ReusableArtifactNone {
		t.Fatalf("Decision=%q, want none", evaluation.Decision)
	}
	if evaluation.Reason != "task_not_substantial_enough" {
		t.Fatalf("Reason=%q, want task_not_substantial_enough", evaluation.Reason)
	}
}

func TestEvaluateReusabilityCreatesOnlyCheatsheetForResolvedFailure(t *testing.T) {
	evaluation := evaluateReusability(
		"debug the failing nginx deployment and document the fix",
		"Root cause resolved after checking logs, fixing the missing upstream config, restarting nginx, and validating the endpoint response.",
		[]string{"execute_shell", "manage_files", "http_request"},
		[]string{
			"inspect nginx error logs to isolate the upstream misconfiguration",
			"update the nginx configuration and restart the service",
			"verify the endpoint returns the expected response",
		},
		ReuseLookupResult{Complexity: TaskComplexityNonTrivial},
	)

	if evaluation.Decision != ReusableArtifactCreateCheatsheet {
		t.Fatalf("Decision=%q, want %q", evaluation.Decision, ReusableArtifactCreateCheatsheet)
	}
}

func TestEvaluateReusabilityCreatesOnlySkillForAutomatedWorkflow(t *testing.T) {
	evaluation := evaluateReusability(
		"automate recurring database backup and restore validation",
		"Built a repeatable backup workflow that exports snapshots, restores them into a validation database, and verifies the schema and row counts automatically.",
		[]string{"sql_query", "archive", "execute_shell"},
		[]string{
			"export the production snapshot with the configured backup command",
			"restore the snapshot into the validation database",
			"compare schema and row counts to confirm integrity",
		},
		ReuseLookupResult{Complexity: TaskComplexityNonTrivial},
	)

	if evaluation.Decision != ReusableArtifactCreateSkill {
		t.Fatalf("Decision=%q, want %q", evaluation.Decision, ReusableArtifactCreateSkill)
	}
}

func TestApplyReusableCheatsheetKeepsUserOwnedEntryUntouched(t *testing.T) {
	logger := newReuseTestLogger()
	csDB := setupReuseCheatsheetDB(t).DB
	if _, err := tools.CheatsheetCreate(csDB, "Recurring Docker Workflow", "user instructions", "user"); err != nil {
		t.Fatalf("CheatsheetCreate user: %v", err)
	}

	runCfg := RunConfig{
		Config:       &config.Config{},
		CheatsheetDB: csDB,
	}
	eval := ReusabilityEvaluation{
		Decision:          ReusableArtifactCreateCheatsheet,
		CheatsheetName:    "Recurring Docker Workflow",
		CheatsheetContent: "# Trigger\n- docker restart loop",
	}
	if err := applyReusableCheatsheet(runCfg, logger, eval); err != nil {
		t.Fatalf("applyReusableCheatsheet: %v", err)
	}

	sheets, err := tools.CheatsheetList(csDB, false)
	if err != nil {
		t.Fatalf("CheatsheetList: %v", err)
	}
	if len(sheets) != 2 {
		t.Fatalf("expected 2 cheatsheets after agent supplement, got %d", len(sheets))
	}
	var userSheet, agentSheet *tools.CheatSheet
	for i := range sheets {
		switch {
		case sheets[i].CreatedBy == "user":
			userSheet = &sheets[i]
		case sheets[i].CreatedBy == "agent":
			agentSheet = &sheets[i]
		}
	}
	if userSheet == nil || userSheet.Content != "user instructions" {
		t.Fatalf("user cheatsheet was modified unexpectedly: %+v", userSheet)
	}
	if agentSheet == nil || agentSheet.Name != "Recurring Docker Workflow (Agent)" {
		t.Fatalf("expected agent supplement cheatsheet, got %+v", agentSheet)
	}
}

func TestApplyReusableSkillCreatesAgentVariantWhenUserSkillExists(t *testing.T) {
	logger := newReuseTestLogger()
	mgr, skillsDir := setupReuseSkillManager(t)
	userSkill, err := mgr.CreateSkillEntry("nginx_logs_log_analyzer", "User maintained log skill", "print('user')\n", tools.SkillTypeUser, "user", "ops", []string{"logs"})
	if err != nil {
		t.Fatalf("CreateSkillEntry user: %v", err)
	}
	if err := mgr.EnableSkill(userSkill.ID, true, "test"); err != nil {
		t.Fatalf("EnableSkill user: %v", err)
	}

	cfg := &config.Config{}
	cfg.Directories.SkillsDir = skillsDir
	cfg.Directories.WorkspaceDir = filepath.Join(t.TempDir(), "workspace")

	runCfg := RunConfig{Config: cfg}
	eval := ReusabilityEvaluation{
		Decision:         ReusableArtifactCreateSkill,
		TemplateName:     "log_analyzer",
		SkillName:        "nginx_logs_log_analyzer",
		SkillDescription: "Agent maintained nginx log analyzer",
		SkillCategory:    "ops",
		SkillTags:        []string{"logs", "reuse-first"},
	}
	if err := applyReusableSkill(runCfg, logger, eval); err != nil {
		t.Fatalf("applyReusableSkill: %v", err)
	}

	created := findSkillByName(mgr, "nginx_logs_log_analyzer_agent")
	if created == nil {
		t.Fatal("expected agent variant skill to be created")
	}
	if created.CreatedBy != "agent" {
		t.Fatalf("CreatedBy = %q, want %q", created.CreatedBy, "agent")
	}
}
