package agent

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManusManualUsesNativeArgumentsAndNormalizedTerminalStates(t *testing.T) {
	t.Parallel()

	manual, err := os.ReadFile(filepath.Join("..", "..", "prompts", "tools_manuals", "manus.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(manual)
	for _, marker := range []string{
		`"message":`, `"enable_skill_ids":`, `"force_skill_ids":`,
		"`completed`", "`stopped`", "`error`", "`needs_user_input`", "`needs_human_approval`",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("Manus manual missing %q", marker)
		}
	}
	for _, obsolete := range []string{`"prompt":`, `"skill_ids":`} {
		if strings.Contains(text, obsolete) {
			t.Fatalf("Manus manual contains obsolete argument %q", obsolete)
		}
	}
}

func TestManusManualDocumentsStructuredOutputObjectAndWaitCap(t *testing.T) {
	t.Parallel()

	manual, err := os.ReadFile(filepath.Join("..", "..", "prompts", "tools_manuals", "manus.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(manual)
	for _, marker := range []string{
		`"structured_output_schema":{"type":"object"`,
		"lower configured maximum",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("Manus manual missing %q", marker)
		}
	}
}
