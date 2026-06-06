package ui

import (
	"strings"
	"testing"
)

func TestConfigCoAgentsWriterPromptTextareaIsRoomier(t *testing.T) {
	t.Parallel()

	coAgentsJS := readDesktopAssetText(t, "cfg/co_agents.js")
	for _, marker := range []string{
		"var promptRows = role.key === 'writer' ? 10 : 3;",
		`rows="' + promptRows + '" data-path="' + basePath + '.additional_prompt"`,
	} {
		if !strings.Contains(coAgentsJS, marker) {
			t.Fatalf("co-agents config module missing marker %q", marker)
		}
	}
}
