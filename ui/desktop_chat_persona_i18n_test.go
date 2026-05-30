package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestDesktopChatWelcomeTranslationsExistInAllLanguages(t *testing.T) {
	t.Parallel()

	required := []string{
		"desktop.chat_sessions",
		"desktop.chat_new",
		"desktop.chat_search",
		"desktop.chat_toggle_sidebar",
		"desktop.chat_scroll_bottom",
		"desktop.chat_drop_files",
		"desktop.chat_current_session",
		"desktop.chat_welcome_heading",
		"desktop.chat_welcome_sub",
		"desktop.chat_prompt_what_can_you_do",
		"desktop.chat_prompt_help_code",
		"desktop.chat_prompt_analyze_files",
		"desktop.chat_prompt_explain",
		"desktop.copy",
		"desktop.copied",
		"desktop.edit",
		"desktop.retry",
		"desktop.clear",
		"desktop.remove",
	}

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := filepath.Join("lang", "desktop", lang+".json")
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty desktop chat translation for %s", path, key)
			}
		}
	}
}

func TestDesktopChatDesktopTextKeysAreAudited(t *testing.T) {
	t.Parallel()

	sources := []string{
		readDesktopAssetText(t, "js/desktop/apps/agent-chat.js"),
		readDesktopAssetText(t, "js/desktop/chat-renderer.js"),
	}
	desktopTextKey := regexp.MustCompile(`(?:desktopText|translate)\(\s*['"]((?:desktop|common|chat)\.[^'"]+)['"]`)
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		values := readMergedLangMap(t, lang)
		for _, source := range sources {
			for _, match := range desktopTextKey.FindAllStringSubmatch(source, -1) {
				key := match[1]
				if strings.TrimSpace(values[key]) == "" {
					t.Fatalf("%s missing desktop chat translation key %s", lang, key)
				}
			}
		}
	}
}

func TestDesktopChatInitializesActivePersonaForAvatars(t *testing.T) {
	t.Parallel()

	source := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	for _, marker := range []string{
		"function desktopPersonaPreviewKey(name, isCore)",
		"async function ensureDesktopChatPersona()",
		"api('/api/personalities')",
		"window._activePersonaIconKey = key;",
		"new CustomEvent('aurago:persona-icon-change'",
		".vd-chat-avatar .persona-avatar-img",
		"ensureDesktopChatPersona().finally(() => loadDesktopChatHistory(host).finally(",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("desktop chat active-persona initialization missing marker %q", marker)
		}
	}
}

func TestDesktopChatSpriteIconClassesAreRenderable(t *testing.T) {
	t.Parallel()

	css := readDesktopAssetText(t, "css/desktop-icons.css")
	for _, className := range []string{
		".vd-chat-sidebar-icon",
		".vd-chat-toolbar-icon",
		".vd-chat-scroll-icon",
		".vd-chat-voice-icon",
		".vd-chat-send-icon",
		".vd-chat-context-icon",
		".vd-qc-btn-icon",
	} {
		if !strings.Contains(css, className) {
			t.Fatalf("desktop chat sprite CSS missing renderable icon class %s", className)
		}
	}
}
