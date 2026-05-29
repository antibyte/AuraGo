package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatProvidesSpeechToTextInput(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	for _, forbidden := range []string{
		`href="/css/stt-overlay.css`,
		`src="/js/chat/modules/speech-to-text.js`,
		`src="/js/chat/modules/voice-recorder.js`,
		`src="/js/chat/ui-icons.js`,
	} {
		if strings.Contains(html, forbidden) {
			t.Fatalf("desktop.html should lazy-load speech-to-text asset %q", forbidden)
		}
	}

	moduleLoader := readDesktopAssetText(t, "js/desktop/core/module-loader.js")
	for _, marker := range []string{
		`/css/stt-overlay.css`,
		`/js/chat/modules/speech-to-text.js`,
		`/js/chat/modules/voice-recorder.js`,
		`/js/chat/ui-icons.js`,
	} {
		if !strings.Contains(moduleLoader, marker) {
			t.Fatalf("desktop lazy asset registry missing speech-to-text asset %q", marker)
		}
	}

	chat := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		`class="vd-chat-voice"`,
		`data-i18n-title="desktop.chat_voice_input"`,
		`initDesktopChatVoice(host, input, voiceBtn)`,
		`window.SpeechToText.init(sttOptions)`,
		`window.VoiceRecorder.init(recorderOptions)`,
		`window.SpeechToText.start()`,
		`window.VoiceRecorder.start()`,
	} {
		if !strings.Contains(chat, marker) {
			t.Fatalf("desktop chat missing speech-to-text marker %q", marker)
		}
	}

	css := readAllDesktopAppCSS(t)
	for _, marker := range []string{
		".vd-chat-voice",
		".vd-chat-voice.is-active",
		"grid-template-columns: 1fr auto auto;",
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop chat CSS missing voice marker %q", marker)
		}
	}

	iconCSS := readDesktopAssetText(t, "css/desktop-icons.css")
	if !strings.Contains(iconCSS, ".vd-chat-voice-icon") ||
		!strings.Contains(iconCSS, "background-image: url('/img/desktop-icons-sprite.png')") {
		t.Fatal("desktop chat voice icon must be rendered through the desktop sprite CSS")
	}
}
