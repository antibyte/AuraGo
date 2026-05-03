package server

import (
	"strings"
	"testing"
)

func TestBuildDesktopAgentPromptKeepsCodeStudioOutOfHomepageWorkspace(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Explain the current file.", desktopChatContext{
		Source:          "code-studio",
		CurrentFile:     "/workspace/hello.go",
		CurrentLanguage: "go",
		CurrentContent:  "package main\n\nfunc main() {}\n",
		OpenFiles:       []string{"/workspace/hello.go"},
	})

	for _, want := range []string{
		"Code Studio files live inside the dedicated Code Studio container workspace",
		"not the homepage workspace",
		"Do not use the homepage tool for Code Studio file questions",
		"Current file: /workspace/hello.go",
		"Current file content:\n<external_data>\npackage main",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("Code Studio prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptPrefersSelectedCodeOverWholeFile(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Explain selected code.", desktopChatContext{
		Source:         "code-studio",
		CurrentFile:    "/workspace/hello.go",
		CurrentContent: "package main\n\nfunc main() {}\n",
		SelectedText:   "func main() {}",
	})

	if !strings.Contains(prompt, "Selected text:\n<external_data>\nfunc main() {}") {
		t.Fatalf("Code Studio prompt should include selected text, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Current file content:") {
		t.Fatalf("Code Studio prompt should not include whole file when selected text is available:\n%s", prompt)
	}
}
