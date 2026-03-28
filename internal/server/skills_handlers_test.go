package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/tools"
)

func TestDecodeSkillDraftAcceptsAliasesAndStringLists(t *testing.T) {
	t.Parallel()

	raw := `Here is your draft:
{
  "skill_name": "api_summary",
  "description": "Summarizes API responses",
  "category": "automation",
  "tags": "api, summary, api",
  "dependencies": "requests\npyyaml",
  "python_code": "print('ok')"
}`

	draft, err := decodeSkillDraft(raw)
	if err != nil {
		t.Fatalf("decodeSkillDraft returned error: %v", err)
	}
	if draft.Name != "api_summary" {
		t.Fatalf("expected normalized name, got %q", draft.Name)
	}
	if draft.Code != "print('ok')" {
		t.Fatalf("expected normalized code, got %q", draft.Code)
	}
	if len(draft.Tags) != 2 || draft.Tags[0] != "api" || draft.Tags[1] != "summary" {
		t.Fatalf("unexpected tags: %#v", draft.Tags)
	}
	if len(draft.Dependencies) != 2 || draft.Dependencies[0] != "requests" || draft.Dependencies[1] != "pyyaml" {
		t.Fatalf("unexpected dependencies: %#v", draft.Dependencies)
	}
}

func TestDecodeSkillDraftSkipsEchoedSchemaObject(t *testing.T) {
	t.Parallel()

	raw := `Schema: {"name":"...","description":"...","category":"...","tags":[...],"dependencies":[...],"code":"..."}.

Actual draft:
{
  "name": "mac_lookup",
  "description": "Looks up a MAC vendor",
  "category": "network",
  "tags": ["mac", "network"],
  "dependencies": ["requests"],
  "code": "print('ok')"
}`

	draft, err := decodeSkillDraft(raw)
	if err != nil {
		t.Fatalf("decodeSkillDraft returned error: %v", err)
	}
	if draft.Name != "mac_lookup" {
		t.Fatalf("expected actual draft name, got %q", draft.Name)
	}
	if draft.Code != "print('ok')" {
		t.Fatalf("expected actual draft code, got %q", draft.Code)
	}
}

func TestGeneratedSkillNeedsRepairForSubprocessDraft(t *testing.T) {
	t.Parallel()

	draft := &generatedSkillDraft{
		Name: "dns_test",
		Code: "import sys,json,subprocess\nsubprocess.run(['ping','-c','4','8.8.8.8'])\njson.dump({}, sys.stdout)",
	}

	needsRepair, reason := generatedSkillNeedsRepair(draft)
	if !needsRepair {
		t.Fatal("expected subprocess draft to require repair")
	}
	if reason == "" {
		t.Fatal("expected non-empty repair reason")
	}
}

func TestGeneratedSkillNeedsRepairAllowsSafeDraft(t *testing.T) {
	t.Parallel()

	draft := &generatedSkillDraft{
		Name: "dns_test",
		Code: "import sys, json, socket\nargs = json.load(sys.stdin)\nhost = args.get('host', 'example.com')\njson.dump({'host': host, 'ip': socket.gethostbyname(host)}, sys.stdout)",
	}

	needsRepair, reason := generatedSkillNeedsRepair(draft)
	if needsRepair {
		t.Fatalf("expected safe draft to pass, got reason %q", reason)
	}
}

func TestGeneratedSkillLooksLikePlaceholder(t *testing.T) {
	t.Parallel()

	draft := &generatedSkillDraft{
		Name:         "...",
		Description:  "...",
		Category:     "...",
		Tags:         []string{"..."},
		Dependencies: []string{"..."},
		Code:         "...",
	}

	if !generatedSkillLooksLikePlaceholder(draft) {
		t.Fatal("expected placeholder draft to be rejected")
	}
}

func TestGeneratedSkillPlaceholderIssuesDetectsSchemaEchoCode(t *testing.T) {
	t.Parallel()

	draft := &generatedSkillDraft{
		Name:        "dns_test",
		Description: "Checks DNS resolution",
		Category:    "category",
		Code:        "python code as a single JSON string",
	}

	issues := generatedSkillPlaceholderIssues(draft)
	if len(issues) == 0 {
		t.Fatal("expected placeholder issues for schema echo draft")
	}
}

func TestDecodeSkillDraftAcceptsNestedDraftObject(t *testing.T) {
	t.Parallel()

	raw := `{"status":"ok","draft":{"name":"api_summary","description":"Summarizes APIs","category":"automation","tags":["api"],"dependencies":["requests"],"code":"print('ok')"}}`

	draft, err := decodeSkillDraft(raw)
	if err != nil {
		t.Fatalf("decodeSkillDraft returned error: %v", err)
	}
	if draft.Name != "api_summary" {
		t.Fatalf("expected nested draft name, got %q", draft.Name)
	}
}

func TestDecodeSkillDraftAcceptsSingleQuotedObject(t *testing.T) {
	t.Parallel()

	raw := `{
  'name': 'dns_test',
  'description': 'Checks DNS resolution',
  'category': 'network',
  'tags': ['dns', 'network',],
  'dependencies': ['dnspython'],
  'code': "print('ok')",
}`

	draft, err := decodeSkillDraft(raw)
	if err != nil {
		t.Fatalf("decodeSkillDraft returned error: %v", err)
	}
	if draft.Name != "dns_test" {
		t.Fatalf("expected single-quoted name, got %q", draft.Name)
	}
	if draft.Code != "print('ok')" {
		t.Fatalf("expected single-quoted draft code, got %q", draft.Code)
	}
	if len(draft.Tags) != 2 {
		t.Fatalf("expected normalized tags, got %#v", draft.Tags)
	}
}

func TestDecodeSkillDraftAcceptsPythonStyleLiterals(t *testing.T) {
	t.Parallel()

	raw := `{
  'draft': {
    'name': 'dns_test',
    'description': 'Checks DNS resolution',
    'category': 'Network',
    'tags': ['dns', 'network'],
    'dependencies': None,
    'enabled': False,
    'code': 'print("ok")'
  }
}`

	draft, err := decodeSkillDraft(raw)
	if err != nil {
		t.Fatalf("decodeSkillDraft returned error: %v", err)
	}
	if draft.Name != "dns_test" {
		t.Fatalf("expected python-style draft name, got %q", draft.Name)
	}
	if draft.Code != `print("ok")` {
		t.Fatalf("expected python-style draft code, got %q", draft.Code)
	}
}

func TestDecodeSkillDraftRequiresCode(t *testing.T) {
	t.Parallel()

	_, err := decodeSkillDraft(`{"name":"empty","description":"no code"}`)
	if err == nil {
		t.Fatal("expected error for missing code")
	}
}

func TestHandleGenerateSkillDraftRequiresSkillManager(t *testing.T) {
	t.Parallel()

	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}

	req := httptest.NewRequest(http.MethodPost, "/api/skills/generate", bytes.NewBufferString(`{"prompt":"make a skill"}`))
	rec := httptest.NewRecorder()

	handleGenerateSkillDraft(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("expected error message in response")
	}
}

func TestHandleGenerateSkillDraftRequiresLLMClient(t *testing.T) {
	t.Parallel()

	s := &Server{
		Cfg:          &config.Config{},
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		SkillManager: &tools.SkillManager{},
	}

	req := httptest.NewRequest(http.MethodPost, "/api/skills/generate", bytes.NewBufferString(`{"prompt":"make a skill"}`))
	rec := httptest.NewRecorder()

	handleGenerateSkillDraft(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", rec.Code)
	}

	var payload map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("failed to decode error response: %v", err)
	}
	if payload["error"] == "" {
		t.Fatal("expected error message in response")
	}
}
