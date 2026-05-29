package ui

import (
	"strings"
	"testing"
)

func TestCodeStudioEditorBundleIsEmbedded(t *testing.T) {
	t.Parallel()

	bundle, err := Content.ReadFile("js/vendor/codemirror-bundle.esm.js")
	if err != nil {
		t.Fatalf("Code Studio CodeMirror bundle missing from embedded UI: %v", err)
	}
	text := string(bundle)
	for _, want := range []string{"EditorView", "EditorState", "javascript", "python", "go", "rust"} {
		if !strings.Contains(text, want) {
			t.Fatalf("Code Studio CodeMirror bundle does not contain %q", want)
		}
	}
}

func TestJavaScriptLibraryAuditWiring(t *testing.T) {
	t.Parallel()

	skillsHTML := readEmbeddedText(t, "skills.html")
	if strings.Contains(skillsHTML, "codemirror6.min.js") {
		t.Fatal("skills page still loads legacy CodeMirror bundle")
	}

	skillsJS := readEmbeddedText(t, "js/skills/main.js")
	if !strings.Contains(skillsJS, "import('/js/vendor/codemirror-bundle.esm.js')") {
		t.Fatal("skills editor does not lazy-load the shared CodeMirror ESM bundle")
	}

	if _, err := Content.ReadFile("js/vendor/codemirror6.min.js"); err == nil {
		t.Fatal("legacy CodeMirror bundle is still embedded")
	}

	indexHTML := readEmbeddedText(t, "index.html")
	if strings.Contains(indexHTML, "mermaid-renderer.js") {
		t.Fatal("chat page still loads the duplicate Mermaid renderer")
	}
	if strings.Contains(indexHTML, `src="/chart.min.js"`) {
		t.Fatal("chat page should lazy-load Chart.js instead of loading it up front")
	}
	for _, want := range []string{
		"/js/chat/bundles/chat-vendor.bundle.js",
		"/js/chat/bundles/chat-runtime.bundle.js",
	} {
		if !strings.Contains(indexHTML, want) {
			t.Fatalf("chat page missing %q", want)
		}
	}

	desktopHTML := readEmbeddedText(t, "desktop.html")
	if !strings.Contains(desktopHTML, "/js/shared/render-markdown.js") {
		desktopLoader := readEmbeddedText(t, "js/desktop/core/module-loader.js")
		if !strings.Contains(desktopLoader, "/js/shared/render-markdown.js") {
			t.Fatal("desktop lazy asset registry does not load the shared Markdown renderer")
		}
	}

	chatJS := readEmbeddedText(t, "js/chat/chat-messages.js")
	for _, want := range []string{"window.AuraMarkdown", "window.ChatChartRenderer.processBlocks"} {
		if !strings.Contains(chatJS, want) {
			t.Fatalf("chat renderer missing %q", want)
		}
	}

	desktopChatJS := readEmbeddedText(t, "js/desktop/chat-renderer.js")
	if !strings.Contains(desktopChatJS, "window.AuraMarkdown") {
		t.Fatal("desktop chat renderer does not use the shared Markdown renderer")
	}
}

func readEmbeddedText(t *testing.T, path string) string {
	t.Helper()
	raw, err := Content.ReadFile(path)
	if err != nil {
		t.Fatalf("read embedded %s: %v", path, err)
	}
	return string(raw)
}
