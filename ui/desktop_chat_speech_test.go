package ui

import (
	"strings"
	"testing"
)

func TestDesktopChatProvidesSpeechToTextInput(t *testing.T) {
	t.Parallel()

	html := readDesktopAssetText(t, "desktop.html")
	for _, marker := range []string{
		`/css/stt-overlay.css`,
		`/js/chat/modules/speech-to-text.js`,
		`/js/chat/modules/voice-recorder.js`,
		`/js/chat/ui-icons.js`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("desktop.html missing speech-to-text asset %q", marker)
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

	css := readDesktopAssetText(t, "css/desktop-apps.css")
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
