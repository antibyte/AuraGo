package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatAnnouncesFinalAgentRepliesToPet(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"function normalizeAgentResponseForPetBubble(text)",
		".replace(/```[\\s\\S]*?```/g, ' ')",
		".replace(/\\s+/g, ' ')",
		"function announceAgentResponseToPet(text)",
		"window.PetRuntime && typeof window.PetRuntime.say === 'function'",
		"window.PetRuntime.say(message, 'info')",
		"let petAnnouncementText = '';",
		"announceAgentResponseToPet(petAnnouncementText || streamingContent);",
		"petAnnouncementText = text;",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat pet announcement missing marker %q", marker)
		}
	}

	deltaIdx := strings.Index(source, "if (data.event === 'llm_stream_delta' || data.type === 'llm_stream_delta')")
	thinkingIdx := strings.Index(source, "} else if (event === 'thinking_block')")
	if deltaIdx < 0 || thinkingIdx < deltaIdx {
		t.Fatal("desktop chat stream delta block markers are missing or out of order")
	}
	if strings.Contains(source[deltaIdx:thinkingIdx], "announceAgentResponseToPet") {
		t.Fatal("desktop pet must not announce partial stream deltas")
	}

	petRuntime := readDesktopAssetText(t, "js/desktop/core/pet-runtime.js")
	for _, marker := range []string{
		"const BUBBLE_DURATION_MS = 4000;",
		"const LONG_BUBBLE_DURATION_MS = 8000;",
		"const MAX_BUBBLE_CHARS = 140;",
	} {
		if !strings.Contains(petRuntime, marker) {
			t.Fatalf("desktop pet runtime must keep existing bubble behavior marker %q", marker)
		}
	}
}
