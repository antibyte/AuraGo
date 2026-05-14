package server

import (
	"os"
	"path/filepath"
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

func TestBuildDesktopAgentPromptIncludesDesktopFileContext(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("What should I know about this file?", desktopChatContext{
		Source:      "desktop-file",
		CurrentFile: "Documents/report.md",
		OpenFiles:   []string{"Documents/report.md", "Desktop/notes.txt"},
	})

	for _, want := range []string{
		"The user has attached desktop workspace file context.",
		"Use the virtual_desktop tool",
		"Current desktop file:\n<external_data type=\"desktop_current_file\">\nDocuments/report.md",
		"Attached desktop files:\n<external_data type=\"desktop_open_files\">\nDocuments/report.md\nDesktop/notes.txt",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("desktop file prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptKeepsEditorTasksInEditor(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Please improve this text.", desktopChatContext{
		Source:      "desktop-file",
		OriginApp:   "editor",
		CurrentFile: "Documents/note.txt",
		OpenFiles:   []string{"Documents/note.txt"},
	})

	for _, want := range []string{
		"This task was launched from the Editor app",
		"write the result back to the same desktop file",
		"open_in_app with app_id \"editor\"",
		"Do not open Writer",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("editor-origin prompt missing marker %q in:\n%s", want, prompt)
		}
	}
}

func TestBuildDesktopAgentPromptForbidsGenericFileToolsForDesktopPaths(t *testing.T) {
	t.Parallel()

	prompt := buildDesktopAgentPrompt("Edit Apps/space-invaders/game.js", desktopChatContext{})

	for _, want := range []string{
		"Never use file_editor",
		"Apps/",
		"Widgets/",
		"virtual_desktop",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("desktop prompt missing routing guard %q in:\n%s", want, prompt)
		}
	}
}

func TestDesktopChatHandlersUseRequestContextForLoopback(t *testing.T) {
	t.Parallel()

	files := []string{"desktop_handlers.go", "desktop_handlers_chat.go"}
	var source string
	for _, name := range files {
		sourceBytes, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("ReadFile %s: %v", name, err)
		}
		source += string(sourceBytes)
	}
	for _, marker := range []string{
		"runDesktopAgentChat(r.Context(), s, body.Message, body.Context)",
		"agent.LoopbackContext(llmCtx, runCfg, prompt, combinedBroker)",
		"agent.LoopbackContext(ctx, runCfg, prompt, broker)",
		"context.WithTimeout(ctx, 10*time.Minute)",
		"lockSessionRequest(\"virtual-desktop\")",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat cancellation missing marker %q", marker)
		}
	}
	if strings.Contains(source, "context.WithTimeout(context.Background(), 10*time.Minute)") {
		t.Fatal("desktop chat must not use context.Background for agent loopback timeout")
	}
}

func TestDesktopChatUIRestoresAndClearsVirtualDesktopHistory(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "quickconnect-launchpad-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"data-chat-clear-history",
		"loadDesktopChatHistory(host)",
		"api('/history?session_id=virtual-desktop')",
		"api('/clear?session_id=virtual-desktop', { method: 'DELETE' })",
		"type=[\"']desktop_user_request",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat history UI missing marker %q", marker)
		}
	}
}

func TestDesktopChatUILaunchContextSupportsFileAutosend(t *testing.T) {
	t.Parallel()

	sourceBytes, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "apps", "quickconnect-launchpad-chat.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop chat UI: %v", err)
	}
	source := string(sourceBytes)
	for _, marker := range []string{
		"chat_autosend",
		"submitDesktopChatMessage(host, input.value.trim())",
		"if (context.chat_autosend && input.value.trim() && !state.chatBusy)",
		"if (context.chat_autosend && state.chatBusy)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat autosend UI missing marker %q", marker)
		}
	}

	windowRuntime, err := os.ReadFile(filepath.Join("..", "..", "ui", "js", "desktop", "core", "window-shell-runtime.js"))
	if err != nil {
		t.Fatalf("ReadFile desktop window runtime: %v", err)
	}
	if !strings.Contains(string(windowRuntime), "applyChatLaunchContext(existing.id, context)") {
		t.Fatal("agent-chat launches should reuse the existing chat window and merge launch context")
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
