package ui

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestDesktopChatFallbackI18n(t *testing.T) {
	t.Parallel()

	chat := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	if !strings.Contains(chat, "desktopText('desktop.chat_request_failed')") {
		t.Fatal("agent-chat missing desktop.chat_request_failed fallback")
	}
	if strings.Contains(chat, "|| 'Request failed'") {
		t.Fatal("agent-chat still hardcodes Request failed")
	}

	renderer := readDesktopAssetText(t, "js/desktop/chat-renderer.js")
	for _, want := range []string{
		"this.translate('desktop.chat_live_stream')",
		"this.translate('desktop.chat_document_format_unknown')",
	} {
		if !strings.Contains(renderer, want) {
			t.Fatalf("chat-renderer missing marker %q", want)
		}
	}
	for _, forbidden := range []string{
		"|| 'Live stream'",
		"|| 'FILE'",
	} {
		if strings.Contains(renderer, forbidden) {
			t.Fatalf("chat-renderer still hardcodes %q", forbidden)
		}
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.ToSlash(filepath.Join("lang", "desktop", lang+".json"))
		var values map[string]string
		if err := json.Unmarshal([]byte(rawDesktopAssetText(t, path)), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{
			"desktop.chat_request_failed",
			"desktop.chat_live_stream",
			"desktop.chat_document_format_unknown",
		} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
		if lang == "de" {
			if values["desktop.chat_request_failed"] == "Request failed" {
				t.Fatalf("%s must not copy the English request-failed string", path)
			}
			if values["desktop.chat_live_stream"] == "Live stream" {
				t.Fatalf("%s must not copy the English live-stream string", path)
			}
			if values["desktop.chat_document_format_unknown"] == "FILE" {
				t.Fatalf("%s must not copy the English document-format badge", path)
			}
		}
	}
}
