package server

import (
	"strings"
	"testing"
)

func TestBuildDesktopAgentPromptIncludesCodeStudioContext(t *testing.T) {
	ctx := desktopChatContext{
		Source:          "code-studio",
		CurrentFile:     "/workspace/main.go",
		CurrentLanguage: "go",
		CursorLine:      12,
		CursorColumn:    4,
		SelectedText:    "func main() {}",
		OpenFiles:       []string{"/workspace/main.go", "/workspace/app_test.go"},
	}

	prompt := buildDesktopAgentPrompt("Explain this", ctx)

	for _, want := range []string{
		"User request:\n\nExplain this",
		"The user is coding in Code Studio.",
		"Current file: /workspace/main.go",
		"Language: go",
		"Cursor: line 12, column 4",
		"Selected text:\n<external_data>\nfunc main() {}\n</external_data>",
		"Open files: /workspace/main.go, /workspace/app_test.go",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q:\n%s", want, prompt)
		}
	}
}
