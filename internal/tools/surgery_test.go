package tools

import "testing"

func TestFilterSurgeryEnvKeepsOnlyRequiredKeys(t *testing.T) {
	filtered := filterSurgeryEnv([]string{
		"PATH=/usr/bin",
		"HOME=/tmp/home",
		"GEMINI_API_KEY=secret",
		"AURAGO_MASTER_KEY=must-not-pass",
		"OPENAI_API_KEY=must-not-pass",
		"LC_ALL=en_US.UTF-8",
	})

	joined := make(map[string]bool, len(filtered))
	for _, entry := range filtered {
		joined[entry] = true
	}
	if !joined["PATH=/usr/bin"] || !joined["HOME=/tmp/home"] || !joined["GEMINI_API_KEY=secret"] || !joined["LC_ALL=en_US.UTF-8"] {
		t.Fatalf("expected required env keys to remain, got %v", filtered)
	}
	for _, blocked := range []string{"AURAGO_MASTER_KEY=must-not-pass", "OPENAI_API_KEY=must-not-pass"} {
		if joined[blocked] {
			t.Fatalf("blocked env leaked through filter: %s", blocked)
		}
	}
}
