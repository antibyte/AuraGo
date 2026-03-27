package agent

import (
	"strings"
	"testing"
)

func TestFormatGuardianBlockedMessageAddsClarificationAndNextStep(t *testing.T) {
	msg := formatGuardianBlockedMessage("execute_shell", "remote code execution via curl pipe sh", 0.85, true, false)
	if !strings.Contains(msg, `"_guardian_justification"`) {
		t.Fatalf("expected clarification guidance, got: %s", msg)
	}
	if !strings.Contains(msg, "curl|sh") {
		t.Fatalf("expected remote execution next-step guidance, got: %s", msg)
	}
}

func TestFormatGuardianBlockedMessageForRejectedClarification(t *testing.T) {
	msg := formatGuardianBlockedMessage("execute_shell", "remote code execution via curl pipe sh", 0.85, true, true)
	if !strings.Contains(msg, "Clarification was reviewed but the action remains blocked") {
		t.Fatalf("expected rejected clarification text, got: %s", msg)
	}
	if !strings.Contains(msg, "built-in tool") && !strings.Contains(msg, "built-in") {
		t.Fatalf("expected safer next-step guidance, got: %s", msg)
	}
}
