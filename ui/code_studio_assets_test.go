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
