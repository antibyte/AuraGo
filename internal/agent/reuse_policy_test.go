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

func TestBuildReuseLookupSkipsErrorRecoveryCheatsheetsForNormalRequests(t *testing.T) {
	logger := newReuseTestLogger()
	csDB := setupReuseCheatsheetDB(t).DB
	if _, err := tools.CheatsheetCreate(csDB, "Error Your Last Workflow", "ERROR: Your last response was text-only. Emit the tool call again using the native function-calling API.", "agent"); err != nil {
		t.Fatalf("CheatsheetCreate error sheet: %v", err)
	}
	if _, err := tools.CheatsheetCreate(csDB, "Image Handling Workflow", "Use the configured image analysis path for uploaded screenshots and photos.", "agent"); err != nil {
		t.Fatalf("CheatsheetCreate image sheet: %v", err)
	}

	lookup := buildReuseLookup("analyze the uploaded image at agent_workspace/workdir/attachments/example.png", nil, csDB, logger)
	for _, hit := range lookup.CheatsheetHits {
		if hit.Name == "Error Your Last Workflow" {
			t.Fatalf("unexpected recovery cheatsheet hit: %+v", lookup.CheatsheetHits)
		}
	}
	if len(lookup.CheatsheetHits) == 0 || lookup.CheatsheetHits[0].Name != "Image Handling Workflow" {
		t.Fatalf("expected image workflow hit, got %+v", lookup.CheatsheetHits)
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

// TestEvaluateReusabilityGatesOnToolErrors verifies that auto-materialisation
// is suppressed when the run had any tool error, regardless of how
// confidently the final answer claims success.
func TestEvaluateReusabilityGatesOnToolErrors(t *testing.T) {
	query := "automate recurring database backup and restore validation"
	answer := "Resolved by exporting snapshots and restoring them into a validation database with verified row counts."
	tools := []string{"sql_query", "archive", "execute_shell"}
	summaries := []string{
		"export the production snapshot with the configured backup command",
		"restore the snapshot into the validation database",
		"verify schema and row counts to confirm integrity",
	}
	lookup := ReuseLookupResult{Complexity: TaskComplexityNonTrivial}

	// Sanity: without the gate, the run is materialisable.
	if got := evaluateReusabilityWithOutcome(query, answer, tools, summaries, lookup, RunOutcome{}, true); got.Decision == ReusableArtifactNone {
		t.Fatalf("baseline decision must not be none, got reason=%q", got.Reason)
	}

	t.Run("any_tool_error_blocks", func(t *testing.T) {
		got := evaluateReusabilityWithOutcome(query, answer, tools, summaries, lookup, RunOutcome{AnyToolError: true}, true)
		if got.Decision != ReusableArtifactNone {
			t.Fatalf("Decision=%q, want none", got.Decision)
		}
		if got.Reason != "gate_blocked_any_tool_error" {
			t.Fatalf("Reason=%q, want gate_blocked_any_tool_error", got.Reason)
		}
	})

	t.Run("last_tool_error_blocks", func(t *testing.T) {
		got := evaluateReusabilityWithOutcome(query, answer, tools, summaries, lookup, RunOutcome{LastToolError: true}, true)
		if got.Decision != ReusableArtifactNone {
			t.Fatalf("Decision=%q, want none", got.Decision)
		}
	})

	t.Run("gate_disabled_passes_through", func(t *testing.T) {
		got := evaluateReusabilityWithOutcome(query, answer, tools, summaries, lookup, RunOutcome{AnyToolError: true}, false)
		if got.Decision == ReusableArtifactNone && strings.HasPrefix(got.Reason, "gate_blocked_") {
			t.Fatalf("expected gate to be bypassed when requireSuccessSignal=false, got reason=%q", got.Reason)
		}
	})
}

// TestDeriveRunOutcomeFromSummaries verifies error detection from compact
// activity summaries produced by compactActivityToolResult.
func TestDeriveRunOutcomeFromSummaries(t *testing.T) {
	cases := []struct {
		name             string
		summaries        []string
		wantAnyError     bool
		wantLastError    bool
		wantRecoveryHits int
	}{
		{
			name:      "all_completed",
			summaries: []string{"docker: completed - 3 containers running", "execute_shell: completed - exit 0"},
		},
		{
			name:             "trailing_error",
			summaries:        []string{"docker: completed - 3 containers running", "execute_shell: error - exit 1: command not found"},
			wantAnyError:     true,
			wantLastError:    true,
			wantRecoveryHits: 1,
		},
		{
			name:             "recovered_run_still_blocked",
			summaries:        []string{"execute_shell: error - file not found", "execute_shell: completed - retry succeeded"},
			wantAnyError:     true,
			wantRecoveryHits: 1,
		},
		{
			name:             "permission_denied_treated_as_error",
			summaries:        []string{"manage_files: permission denied while writing /etc/hosts"},
			wantAnyError:     true,
			wantLastError:    true,
			wantRecoveryHits: 1,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DeriveRunOutcomeFromSummaries(tc.summaries)
			if got.AnyToolError != tc.wantAnyError {
				t.Errorf("AnyToolError=%v, want %v", got.AnyToolError, tc.wantAnyError)
			}
			if got.LastToolError != tc.wantLastError {
				t.Errorf("LastToolError=%v, want %v", got.LastToolError, tc.wantLastError)
			}
			if got.RecoveryLoopHits != tc.wantRecoveryHits {
				t.Errorf("RecoveryLoopHits=%d, want %d", got.RecoveryLoopHits, tc.wantRecoveryHits)
			}
		})
	}
}

// TestIsAcceptableArtifactName ensures fragmentary names like
// "Gro Bugfix Teste Workflow" are rejected while substantive prose-style
// and snake_case skill names are accepted.
func TestIsAcceptableArtifactName(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"empty_rejected", "", false},
		{"single_token_rejected", "Workflow", false},
		{"too_short_rejected", "X Y", false},
		{"prose_name_accepted", "Docker Deployment Recovery Workflow", true},
		{"snake_skill_accepted", "automate_database_backup_validation_skill", true},
		{"only_generic_tokens_rejected", "Workflow Skill Tool", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isAcceptableArtifactName(tc.in); got != tc.want {
				t.Fatalf("isAcceptableArtifactName(%q)=%v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

// TestReuseFirstSessionCoolDown verifies that a session that already created
// one cheatsheet and one skill is gated from creating more in the same run.
func TestReuseFirstSessionCoolDown(t *testing.T) {
	const sid = "test-cooldown-session"
	if reuseFirstSessionAtCap(sid, 1) {
		t.Fatal("fresh session must not be at cap")
	}
	reuseFirstSessionRecord(sid, ReusableArtifactCreateCheatsheet)
	if reuseFirstSessionAtCap(sid, 1) {
		t.Fatal("after one cheatsheet only, skill slot still free, must not be at cap")
	}
	reuseFirstSessionRecord(sid, ReusableArtifactCreateSkill)
	if !reuseFirstSessionAtCap(sid, 1) {
		t.Fatal("after one cheatsheet and one skill, must be at cap")
	}
	// Cap of 0 disables the limiter.
	if reuseFirstSessionAtCap(sid, 0) {
		t.Fatal("cap<=0 must disable the limiter")
	}
}
