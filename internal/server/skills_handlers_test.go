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
