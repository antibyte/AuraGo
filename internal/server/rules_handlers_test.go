package server

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
)

type rulesCandidateTest struct {
	ID string `json:"id"`
}

func TestRulesHandlersListGetUpdateAndRestore(t *testing.T) {
	t.Parallel()

	promptsDir := filepath.Join(t.TempDir(), "prompts")
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.Rules.Enabled = true
	s.Cfg.Directories.PromptsDir = promptsDir

	listRec := httptest.NewRecorder()
	handleConfigRules(s).ServeHTTP(listRec, httptest.NewRequest(http.MethodGet, "/api/config/rules", nil))
	if listRec.Code != http.StatusOK {
		t.Fatalf("list status = %d body=%s", listRec.Code, listRec.Body.String())
	}
	var list struct {
		Rules []struct {
			ID      string `json:"id"`
			BuiltIn bool   `json:"built_in"`
		} `json:"rules"`
		Candidates struct {
			Tools     []rulesCandidateTest `json:"tools"`
			Workflows []rulesCandidateTest `json:"workflows"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list.Rules) == 0 || list.Rules[0].ID != "homepage" || !list.Rules[0].BuiltIn {
		t.Fatalf("unexpected list response: %+v", list)
	}
	if !candidateIDsContain(list.Candidates.Tools, "filesystem") {
		t.Fatalf("rules list must include tool candidates, got %+v", list.Candidates.Tools)
	}
	if !candidateIDsContain(list.Candidates.Workflows, "homepage") || !candidateIDsContain(list.Candidates.Workflows, "cronjobs") {
		t.Fatalf("rules list must include workflow candidates, got %+v", list.Candidates.Workflows)
	}

	updateBody := `{"title":"Custom Homepage","enabled":true,"priority":77,"tools":["homepage"],"workflows":["homepage"],"keywords":["custom-homepage"],"body":"Use the custom design rule.","design":"# Custom DESIGN.md\n\n## Colors"}`
	updateRec := httptest.NewRecorder()
	handleConfigRuleByID(s).ServeHTTP(updateRec, httptest.NewRequest(http.MethodPut, "/api/config/rules/homepage", bytes.NewBufferString(updateBody)))
	if updateRec.Code != http.StatusOK {
		t.Fatalf("update status = %d body=%s", updateRec.Code, updateRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	handleConfigRuleByID(s).ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/config/rules/homepage", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), "Custom Homepage") || !strings.Contains(getRec.Body.String(), "Custom DESIGN.md") {
		t.Fatalf("get did not return disk override: %s", getRec.Body.String())
	}

	restoreRec := httptest.NewRecorder()
	handleConfigRuleRestore(s).ServeHTTP(restoreRec, httptest.NewRequest(http.MethodPost, "/api/config/rules/homepage/restore", nil))
	if restoreRec.Code != http.StatusOK {
		t.Fatalf("restore status = %d body=%s", restoreRec.Code, restoreRec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(promptsDir, "rules", "homepage", "rule.md")); !os.IsNotExist(err) {
		t.Fatalf("restore should remove disk override, stat err=%v", err)
	}
}

func candidateIDsContain(items []rulesCandidateTest, id string) bool {
	for _, item := range items {
		if item.ID == id {
			return true
		}
	}
	return false
}

func TestRulesHandlerRejectsTraversalID(t *testing.T) {
	t.Parallel()

	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Rules.Enabled = true
	s.Cfg.Directories.PromptsDir = t.TempDir()

	rec := httptest.NewRecorder()
	handleConfigRuleByID(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config/rules/../homepage", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestSaveRuleOverrideDefaultsEnabledForNewRules(t *testing.T) {
	t.Parallel()

	promptsDir := filepath.Join(t.TempDir(), "prompts")
	s := &Server{
		Cfg:    &config.Config{},
		Logger: slog.Default(),
	}
	s.Cfg.Rules.Enabled = true
	s.Cfg.Directories.PromptsDir = promptsDir

	// POST without explicit enabled field should default to true.
	createBody := `{"id":"my-rule","title":"My Rule","priority":50,"tools":["filesystem"],"workflows":[],"keywords":[],"body":"body"}`
	createRec := httptest.NewRecorder()
	handleConfigRules(s).ServeHTTP(createRec, httptest.NewRequest(http.MethodPost, "/api/config/rules", bytes.NewBufferString(createBody)))
	if createRec.Code != http.StatusOK {
		t.Fatalf("create status = %d body=%s", createRec.Code, createRec.Body.String())
	}

	getRec := httptest.NewRecorder()
	handleConfigRuleByID(s).ServeHTTP(getRec, httptest.NewRequest(http.MethodGet, "/api/config/rules/my-rule", nil))
	if getRec.Code != http.StatusOK {
		t.Fatalf("get status = %d body=%s", getRec.Code, getRec.Body.String())
	}
	if !strings.Contains(getRec.Body.String(), `"enabled":true`) {
		t.Fatalf("expected new rule to default to enabled=true, got: %s", getRec.Body.String())
	}
}
