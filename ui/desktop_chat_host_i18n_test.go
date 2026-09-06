package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatHostI18n(t *testing.T) {
	t.Parallel()

	chat := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	if !strings.Contains(chat, "desktopText('desktop.load_failed')") {
		t.Fatal("agent chat missing-host throw must localize desktop.load_failed")
	}
	if strings.Contains(chat, "Desktop chat window content is not available") {
		t.Fatal("agent chat still hardcodes Desktop chat window content is not available")
	}

	speech := readDesktopAssetText(t, "js/desktop/apps/live-speech.js")
	if !strings.Contains(speech, "text('desktop.load_failed')") {
		t.Fatal("live speech missing-host throw must localize desktop.load_failed")
	}
	if strings.Contains(speech, "Live Speech window content is not available") {
		t.Fatal("live speech still hardcodes Live Speech window content is not available")
	}
}
