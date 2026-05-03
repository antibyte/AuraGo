package ui

import (
	"strings"
	"testing"
)

func TestCodeStudioAgentContextIncludesCurrentContent(t *testing.T) {
	t.Parallel()

	jsBytes, err := Content.ReadFile("js/desktop/apps/code-studio.js")
	if err != nil {
		t.Fatalf("Code Studio app missing from embedded UI: %v", err)
	}
	js := string(jsBytes)
	for _, want := range []string{
		"const content = tab ? editorValue(tab) : '';",
		"current_content: selection.text ? '' : content,",
		"selected_text: selection.text",
	} {
		if !strings.Contains(js, want) {
			t.Fatalf("Code Studio agent context missing marker %q", want)
		}
	}
}
