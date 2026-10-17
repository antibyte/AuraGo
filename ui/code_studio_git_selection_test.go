package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodeStudioGitSelectionSendsCheckedPaths(t *testing.T) {
	gitJS, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", "git.js"))
	if err != nil {
		t.Fatal(err)
	}
	coreJS, err := os.ReadFile(filepath.Join("js", "desktop", "apps", "code-studio", "core.js"))
	if err != nil {
		t.Fatal(err)
	}
	git := string(gitJS)
	core := string(coreJS)
	for _, marker := range []string{
		"data-git-select",
		"codeStudio.commitNeedsSelection",
		"codeStudio.gitIdentityHint",
		"git_identity_required",
		"add_all: false",
		"action === 'create'",
	} {
		if !strings.Contains(git, marker) && !strings.Contains(core, marker) {
			t.Errorf("code studio source lacks %q", marker)
		}
	}
	if strings.Contains(git, "gitCommit(message, true)") || strings.Contains(git, "add_all: true") || strings.Contains(core, "add_all: true") {
		t.Fatal("commit still stages every change")
	}
	if !strings.Contains(core, "method: 'POST'") || !strings.Contains(core, "/api/code-studio/git/branch") {
		t.Fatal("branch changes do not use POST")
	}

	langs := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	keys := []string{
		"codeStudio.commitNeedsSelection",
		"codeStudio.gitIdentityHint",
		"codeStudio.identityName",
		"codeStudio.identityEmail",
		"codeStudio.saveIdentity",
		"codeStudio.selectChange",
	}
	var english map[string]string
	for _, lang := range langs {
		raw, err := os.ReadFile(filepath.Join("lang", "desktop", lang+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if lang == "en" {
			english = values
		}
		for _, key := range keys {
			value := strings.TrimSpace(values[key])
			if value == "" || strings.ContainsRune(value, '�') {
				t.Errorf("%s: %s missing or broken", lang, key)
			}
		}
		if lang == "de" {
			for _, key := range keys {
				for _, formal := range []string{" Sie ", " Ihre ", " Ihnen "} {
					if strings.Contains(" "+values[key]+" ", formal) {
						t.Errorf("de: %s uses formal address", key)
					}
				}
			}
		}
		if lang != "en" {
			if values["codeStudio.commitNeedsSelection"] == english["codeStudio.commitNeedsSelection"] ||
				values["codeStudio.gitIdentityHint"] == english["codeStudio.gitIdentityHint"] {
				t.Errorf("%s copies an English git string", lang)
			}
		}
	}
}
