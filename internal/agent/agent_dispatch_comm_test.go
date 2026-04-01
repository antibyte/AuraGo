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

	"aurago/internal/security"
	"aurago/internal/tools"
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

func TestMistakenNativeToolSkillNameDetectsNativeTools(t *testing.T) {
	action, ok := mistakenNativeToolSkillName("upnp_scan")
	if !ok {
		t.Fatal("expected upnp_scan to be recognized as a native tool")
	}
	if action != "upnp_scan" {
		t.Fatalf("action = %q, want upnp_scan", action)
	}
}

func TestMistakenNativeToolSkillNameIgnoresRealSkills(t *testing.T) {
	if _, ok := mistakenNativeToolSkillName("ddg_search"); ok {
		t.Fatal("did not expect builtin skill ddg_search to be treated as native-only tool")
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
