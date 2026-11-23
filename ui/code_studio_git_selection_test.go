package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// The Git workbench stages explicitly: every staging action is a POST of explicit paths, commit
// sends staged_only, and add_all is only sent after the "stage all and commit" confirmation.
func TestCodeStudioGitStagingSourceAndLocales(t *testing.T) {
	read := func(name string) string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	markers := map[string][]string{
		"git.js":         {"data-git-stage", "data-git-unstage", "data-git-discard", "data-git-init", "codeStudio.gitStageAllAndCommit", "codeStudio.gitIdentityHint", "git_identity_required", "reloadCleanTabFromDisk"},
		"git-history.js": {"cs-blame-gutter", "openGitCommit", "codeStudio.gitNotCommitted", "csBlame"},
		"core.js":        {"staged_only: !(options && options.addAll)", "action === 'create'", "method: 'POST'", "/api/code-studio/git/branch", "/api/code-studio/git/stage", "/api/code-studio/git/discard", "function isVirtualTab(tab)"},
		"editor.js":      {"view.csBlame = blame;", "blame.of(gitBlameExtensions(cm, tab))"},
	}
	for name, wanted := range markers {
		source := read(name)
		for _, marker := range wanted {
			if !strings.Contains(source, marker) {
				t.Errorf("%s lacks %q", name, marker)
			}
		}
	}
	if strings.Contains(read("git.js"), "data-git-select") {
		t.Error("git.js still renders commit checkboxes")
	}

	load := func(lang string) map[string]string {
		t.Helper()
		raw, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		return values
	}
	english := load("en")
	keys := []string{"codeStudio.gitStagedChanges", "codeStudio.gitStageAllAndCommit", "codeStudio.gitInitRepo", "codeStudio.gitDiscardUntrackedConfirm", "codeStudio.gitNotCommitted", "codeStudio.gitIdentityHint"}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := load(lang)
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, utf8.RuneError) {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
			if lang != "en" && value == english[key] {
				t.Errorf("%s copies the English %s", lang, key)
			}
			if lang == "de" {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+value+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
		if !strings.Contains(values["codeStudio.gitDiscardUntrackedConfirm"], "{{name}}") {
			t.Errorf("%s: gitDiscardUntrackedConfirm must name the file", lang)
		}
	}
}
