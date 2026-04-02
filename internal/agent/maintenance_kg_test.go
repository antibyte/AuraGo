package agent

import (
	"strings"
	"testing"

	"aurago/internal/memory"

	"github.com/sashabaranov/go-openai"
)

func TestBuildKGExtractionInputCombinesSources(t *testing.T) {
	input := buildKGExtractionInput(
		[]openai.ChatCompletionMessage{
			{Role: openai.ChatMessageRoleUser, Content: "Please update the proxmox backup job."},
		},
		[]memory.JournalEntry{
			{EntryType: "decision", Title: "Backup target", Content: "Use the NAS as replication target."},
		},
		[]memory.ActivityTurn{
			{
				Intent:          "configure backup",
				UserRequest:     "set up nightly replication",
				UserGoal:        "reliable restore path",
				ImportantPoints: []string{"proxmox host", "NAS destination"},
			},
		},
	)

	for _, needle := range []string{"Conversation:", "Activity turns:", "Journal entries:", "proxmox", "NAS"} {
		if !strings.Contains(input, needle) {
			t.Fatalf("expected combined KG input to contain %q, got: %s", needle, input)
		}
	}
}
