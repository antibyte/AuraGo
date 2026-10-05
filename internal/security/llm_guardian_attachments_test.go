package security

import (
	"fmt"
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
	if got := guardianPromptParamLimit("subject"); got != 200 {
		t.Fatalf("limit for subject = %d, want the default 200", got)
	}
}

// A recipient in the middle of a long list must reach the prompt: with the default limit of
// 200 bytes only the head and the tail of the list would be shown.
func TestGuardianPromptShowsEveryRecipientOfALongList(t *testing.T) {
	if got := guardianPromptParamLimit("to"); got != 600 {
		t.Fatalf("limit for to = %d, want 600", got)
	}
	recipients := make([]string, 12)
	for i := range recipients {
		recipients[i] = fmt.Sprintf("recipient-%02d@example.org", i+1)
	}
	recipients[6] = "attacker@evil.example" // seventh place
	to := strings.Join(recipients, ", ")
	if len(to) <= 200 {
		t.Fatalf("the list must be longer than the default limit, got %d bytes", len(to))
	}
	prompt := buildGuardianPrompt(GuardianCheck{Operation: "send_email", Parameters: map[string]string{
		"to": to, "recipient_count": "12",
	}})
	if !strings.Contains(prompt, "attacker@evil.example") || !strings.Contains(prompt, to) || strings.Contains(prompt, "omitted") {
		t.Fatalf("a recipient is cut out of the prompt: %s", prompt)
	}
}
