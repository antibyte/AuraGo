package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatAudioAutoPlaysWithoutInlinePlayer(t *testing.T) {
	t.Parallel()

	renderer := readDesktopAssetText(t, "js/desktop/chat-renderer.js")
	for _, marker := range []string{
		"_audioQueue",
		"_currentAudio",
		"new Audio(src)",
		"enqueueAudioAutoPlay(audioData.path)",
	} {
		if !strings.Contains(renderer, marker) {
			t.Fatalf("desktop chat renderer missing audio autoplay marker %q", marker)
		}
	}
	start := strings.Index(renderer, "appendAudioMessage")
	end := strings.Index(renderer, "appendDocumentMessage")
	if start == -1 || end == -1 || end <= start {
		t.Fatalf("desktop chat renderer audio/document message boundaries changed")
	}
	audioRenderer := renderer[start:end]
	for _, forbidden := range []string{
		"wrapper.className = 'vd-chat-audio-wrapper'",
		"audio.controls = true",
		"chatLog.appendChild(bubble);",
	} {
		if strings.Contains(audioRenderer, forbidden) {
			t.Fatalf("desktop chat audio renderer still contains inline player marker %q", forbidden)
		}
	}
}
