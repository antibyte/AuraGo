package agent

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/security"
	"aurago/internal/tools"

	openai "github.com/sashabaranov/go-openai"
)

type fakeEmailContentEvaluator struct {
	decision security.Decision
	reason   string
	calls    int
}

func (f *fakeEmailContentEvaluator) EvaluateContent(context.Context, string, string) security.GuardianResult {
	f.calls++
	return security.GuardianResult{Decision: f.decision, Reason: f.reason}
}

func TestSynthesizeExecuteSkillArgsPromotesTopLevelFields(t *testing.T) {
	tc := ToolCall{
		Action:   "execute_skill",
		Skill:    "virustotal_scan",
		Resource: "example.com",
		FilePath: "test_virussignatur.txt",
		Mode:     "auto",
	}

	args := synthesizeExecuteSkillArgs(tc)
	if got, _ := args["resource"].(string); got != "example.com" {
		t.Fatalf("resource = %q, want example.com", got)
	}
	if got, _ := args["file_path"].(string); got != "test_virussignatur.txt" {
		t.Fatalf("file_path = %q, want test_virussignatur.txt", got)
	}
	if got, _ := args["mode"].(string); got != "auto" {
		t.Fatalf("mode = %q, want auto", got)
	}
	if _, ok := args["skill"]; ok {
		t.Fatal("did not expect skill metadata in synthesized args")
	}
	if _, ok := args["action"]; ok {
		t.Fatal("did not expect action metadata in synthesized args")
	}
}

func TestBuiltinArgsFromToolCallMergesRawParams(t *testing.T) {
	tc := ToolCall{
		Action: "web_scraper",
		URL:    "https://example.com",
		Params: map[string]interface{}{
			"search_query": "find pricing",
		},
	}

	args := builtinArgsFromToolCall(tc)
	if got, _ := args["url"].(string); got != "https://example.com" {
		t.Fatalf("url = %q, want https://example.com", got)
	}
	if got, _ := args["search_query"].(string); got != "find pricing" {
		t.Fatalf("search_query = %q, want find pricing", got)
	}
}

func TestExecuteSkillRedirectsNativeToolBeforeFilteringArgs(t *testing.T) {
	resetToolCatalogForTest(t)
	cfg := &config.Config{}
	cfg.Directories.SkillsDir = t.TempDir()
	SetDiscoverToolsState("sess-execute-skill-native", []openai.Tool{
		testToolSchema("yepapi_instagram", "Instagram data via YepAPI"),
	}, nil, "")

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "execute_skill",
		Skill:  "yepapi_instagram",
		SkillArgs: map[string]interface{}{
			"operation": "user",
			"username":  "jopliness",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle execute_skill")
	}
	if !strings.Contains(out, "native AuraGo tool") {
		t.Fatalf("expected native tool redirect, got %s", out)
	}
	if !strings.Contains(out, "yepapi_instagram") {
		t.Fatalf("expected redirect to mention yepapi_instagram, got %s", out)
	}
	if strings.Contains(out, "skill not found") {
		t.Fatalf("expected redirect before skill manager lookup, got %s", out)
	}
}

func TestDispatchCommCallWebhookUsesWebhookNameFromParams(t *testing.T) {
	cfg := &config.Config{}
	cfg.Webhooks.Enabled = true
	cfg.Webhooks.Outgoing = []config.OutgoingWebhook{}

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "call_webhook",
		Params: map[string]interface{}{
			"webhook_name": "Deploy Hook",
			"parameters": map[string]interface{}{
				"branch": "main",
			},
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle call_webhook")
	}
	if !strings.Contains(out, "Webhook 'Deploy Hook' not found") {
		t.Fatalf("expected fallback webhook name in error, got %s", out)
	}
}

func TestDispatchMessagingSendYouTubeVideoHonorsDisabledConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.SendYouTubeVideo.Enabled = false

	out, ok := dispatchMessagingCases(context.Background(), ToolCall{
		Action: "send_youtube_video",
		URL:    "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchMessagingCases to handle send_youtube_video")
	}
	if !strings.Contains(out, "send_youtube_video is disabled") {
		t.Fatalf("expected disabled error, got %s", out)
	}
}

func TestFilterExecuteSkillArgsUsesManifestParameters(t *testing.T) {
	skillsDir := t.TempDir()
	manifest := `{
  "name": "virustotal_scan",
  "description": "Scan with VirusTotal",
  "executable": "__builtin__",
  "parameters": {
    "resource": "Resource to scan",
    "file_path": "Local file path",
    "mode": "Scan mode"
  }
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "virustotal_scan.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	filtered := filterExecuteSkillArgs(skillsDir, "virustotal_scan", map[string]interface{}{
		"resource":  "example.com",
		"file_path": "sample.txt",
		"mode":      "auto",
		"title":     "should be removed",
	})

	if len(filtered) != 3 {
		t.Fatalf("filtered arg count = %d, want 3", len(filtered))
	}
	if _, ok := filtered["title"]; ok {
		t.Fatal("did not expect unrelated field 'title' to survive filtering")
	}
}

func TestFilterExecuteSkillArgsAliasesFilePathVariants(t *testing.T) {
	skillsDir := t.TempDir()
	manifest := `{
  "name": "pdf_extractor",
  "description": "Extract text from PDF",
  "executable": "__builtin__",
  "parameters": {
    "filepath": "Path to the PDF file"
  }
}`
	if err := os.WriteFile(filepath.Join(skillsDir, "pdf_extractor.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	filtered := filterExecuteSkillArgs(skillsDir, "pdf_extractor", map[string]interface{}{
		"file_path": "docs/report.pdf",
	})

	if got, _ := filtered["filepath"].(string); got != "docs/report.pdf" {
		t.Fatalf("filepath = %q, want docs/report.pdf", got)
	}
}

func TestBuiltinSkillManifestParametersStayInSync(t *testing.T) {
	skillsDir := filepath.Join("..", "..", "agent_workspace", "skills")
	skills, err := tools.ListSkills(skillsDir)
	if err != nil {
		t.Fatalf("ListSkills: %v", err)
	}

	expected := map[string][]string{
		"brave_search":       {"count", "country", "lang", "query"},
		"ddg_search":         {"max_results", "query", "search_query"},
		"git_backup_restore": {"action", "commit_hash", "commit_message", "limit", "mode"},
		"paperless":          {"category", "content", "document_id", "limit", "name", "operation", "query", "tags", "title"},
		"pdf_extractor":      {"filepath", "search_query"},
		"virustotal_scan":    {"file_path", "mode", "path", "resource"},
		"web_scraper":        {"search_query", "url"},
		"wikipedia_search":   {"language", "query", "search_query"},
	}

	found := map[string]bool{}
	for _, skill := range skills {
		if skill.Executable != "__builtin__" {
			continue
		}
		want, ok := expected[skill.Name]
		if !ok {
			continue
		}
		found[skill.Name] = true

		var got []string
		for key := range skill.Parameters {
			got = append(got, key)
		}
		sort.Strings(got)
		sort.Strings(want)

		if len(got) != len(want) {
			t.Fatalf("skill %q parameters = %v, want %v", skill.Name, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("skill %q parameters = %v, want %v", skill.Name, got, want)
			}
		}
	}

	for name := range expected {
		if !found[name] {
			t.Fatalf("expected builtin skill manifest %q in %s", name, skillsDir)
		}
	}
}

func TestDispatchCommManageDaemonDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = false

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "list",
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, "disabled") {
		t.Fatalf("expected disabled message, got %s", out)
	}
}

func TestDispatchCommManageDaemonNilSupervisor(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = true

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "list",
	}, &DispatchContext{
		Cfg:              cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		DaemonSupervisor: nil,
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, "not initialized") {
		t.Fatalf("expected not initialized message, got %s", out)
	}
}

func TestDispatchCommManageDaemonList(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = true

	sup := tools.NewDaemonSupervisor(
		tools.DaemonSupervisorConfig{Enabled: false},
		nil, nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "list",
	}, &DispatchContext{
		Cfg:              cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		DaemonSupervisor: sup,
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, `"status":"success"`) {
		t.Fatalf("expected success response, got %s", out)
	}
	if !strings.Contains(out, `"count":0`) {
		t.Fatalf("expected count 0, got %s", out)
	}
}

func TestDispatchCommWaitForEventAllowsEmptyTaskPrompt(t *testing.T) {
	bgMgr := tools.NewBackgroundTaskManager(t.TempDir(), slog.New(slog.NewTextHandler(io.Discard, nil)))
	tools.SetDefaultBackgroundTaskManager(bgMgr)
	t.Cleanup(func() {
		tools.SetDefaultBackgroundTaskManager(nil)
		_ = bgMgr.Close()
	})

	cfg := &config.Config{}
	cfg.Agent.BackgroundTasks.Enabled = true
	cfg.Agent.BackgroundTasks.WaitDefaultTimeoutSecs = 30
	cfg.Agent.BackgroundTasks.WaitPollIntervalSecs = 5

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action: "wait_for_event",
		Params: map[string]interface{}{
			"event_type": "http_available",
			"url":        "http://127.0.0.1:65535",
		},
	}, &DispatchContext{
		Cfg:    cfg,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle wait_for_event")
	}
	if !strings.Contains(out, "scheduled as background task") {
		t.Fatalf("expected wait_for_event to be scheduled, got %s", out)
	}
}

func TestDispatchCommManageDaemonStatusMissingID(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = true

	sup := tools.NewDaemonSupervisor(
		tools.DaemonSupervisorConfig{Enabled: false},
		nil, nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "status",
	}, &DispatchContext{
		Cfg:              cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		DaemonSupervisor: sup,
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, "skill_id") {
		t.Fatalf("expected skill_id required message, got %s", out)
	}
}

func TestDispatchCommManageDaemonStatusNotFound(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = true

	sup := tools.NewDaemonSupervisor(
		tools.DaemonSupervisorConfig{Enabled: false},
		nil, nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "status",
		SkillID:   "nonexistent",
	}, &DispatchContext{
		Cfg:              cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		DaemonSupervisor: sup,
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, "not found") {
		t.Fatalf("expected not found message, got %s", out)
	}
}

func TestDispatchCommManageDaemonUnknownOp(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.DaemonSkills.Enabled = true

	sup := tools.NewDaemonSupervisor(
		tools.DaemonSupervisorConfig{Enabled: false},
		nil, nil, nil, nil,
		slog.New(slog.NewTextHandler(io.Discard, nil)),
	)

	out, ok := dispatchComm(context.Background(), ToolCall{
		Action:    "manage_daemon",
		Operation: "destroy",
	}, &DispatchContext{
		Cfg:              cfg,
		Logger:           slog.New(slog.NewTextHandler(io.Discard, nil)),
		DaemonSupervisor: sup,
	})
	if !ok {
		t.Fatal("expected dispatchComm to handle manage_daemon")
	}
	if !strings.Contains(out, "Unknown daemon operation") {
		t.Fatalf("expected unknown operation message, got %s", out)
	}
}

func TestSanitizeFetchedEmailsBlocksWithLLMGuardian(t *testing.T) {
	guardian := security.NewGuardian(slog.New(slog.NewTextHandler(io.Discard, nil)))
	llmGuardian := &fakeEmailContentEvaluator{decision: security.DecisionBlock, reason: "prompt injection"}
	messages := []tools.EmailMessage{{UID: 7, From: "evil@example.com", Subject: "invoice", Body: "Please review the attached project summary."}}

	sanitized := sanitizeFetchedEmails(context.Background(), nil, guardian, llmGuardian, true, messages)

	if llmGuardian.calls != 1 {
		t.Fatalf("llm guardian calls = %d, want 1", llmGuardian.calls)
	}
	if got := sanitized[0].Subject; !strings.Contains(got, "llm guardian blocked") {
		t.Fatalf("subject = %q, want blocked marker", got)
	}
	if got := sanitized[0].Body; !strings.Contains(got, "prompt injection") {
		t.Fatalf("body = %q, want reason included", got)
	}
}
