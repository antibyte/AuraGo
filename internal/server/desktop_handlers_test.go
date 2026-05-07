package server

import (
	"os"
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
		"Current file:\n<external_data type=\"desktop_current_file\">\n/workspace/hello.go",
		"Current file content:\n<external_data type=\"desktop_current_content\">\npackage main",
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

	if !strings.Contains(prompt, "Selected text:\n<external_data type=\"desktop_selected_text\">\nfunc main() {}") {
		t.Fatalf("Code Studio prompt should include selected text, got:\n%s", prompt)
	}
	if strings.Contains(prompt, "Current file content:") {
		t.Fatalf("Code Studio prompt should not include whole file when selected text is available:\n%s", prompt)
	}
}

func TestDesktopChatHandlersUseRequestContextForLoopback(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile("desktop_handlers.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_handlers.go: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"runDesktopAgentChat(r.Context(), s, body.Message, body.Context)",
		"agent.LoopbackContext(ctx, runCfg, prompt, combinedBroker)",
		"agent.LoopbackContext(ctx, runCfg, prompt, broker)",
		"context.WithTimeout(ctx, 10*time.Minute)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat cancellation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "context.WithTimeout(context.Background(), 10*time.Minute)") {
		t.Fatal("desktop chat must not use context.Background for agent loopback timeout")
	}
}

func TestDesktopJSONHandlersUseBodyLimits(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"desktop_handlers.go", "desktop_office_handlers.go", "desktop_looper_handlers.go"} {
		sourceBytes, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		source := string(sourceBytes)
		if name != "desktop_handlers.go" && strings.Contains(source, "json.NewDecoder(r.Body).Decode") {
			t.Fatalf("%s decodes request JSON without desktop body limit helper", name)
		}
	}
	sourceBytes, err := os.ReadFile("desktop_handlers.go")
	if err != nil {
		t.Fatalf("ReadFile desktop_handlers.go: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"const desktopSmallJSONBodyLimit",
		"func decodeDesktopJSON",
		"http.MaxBytesReader(w, r.Body, maxBytes)",
		"decodeDesktopJSON(w, r,",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop body limit helper missing marker %q", marker)
		}
	}
	if strings.Count(source, "json.NewDecoder(r.Body).Decode") != 1 {
		t.Fatal("desktop_handlers.go should use json.NewDecoder(r.Body) only inside decodeDesktopJSON")
	}
}
