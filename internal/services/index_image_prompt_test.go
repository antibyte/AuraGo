package services

import (
	"strings"
	"testing"
)

func TestIndexImagePromptNamesTheFileAndNotAPath(t *testing.T) {
	got := indexImagePrompt("cat.jpg", "")
	for _, want := range []string{"cat.jpg", "Analysiere", "Objekte"} {
		if !strings.Contains(got, want) {
			t.Fatalf("indexImagePrompt() = %q, want it to contain %q", got, want)
		}
	}
	for _, forbidden := range []string{"/", "agent_workspace"} {
		if strings.Contains(got, forbidden) {
			t.Fatalf("indexImagePrompt() = %q, must not contain %q", got, forbidden)
		}
	}
}
