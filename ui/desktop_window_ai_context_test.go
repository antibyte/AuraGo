package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopWindowAIContextAssets(t *testing.T) {
	t.Parallel()

	windowRuntime := rawDesktopAssetText(t, "js/desktop/core/window-shell-runtime.js")
	windowInteractions := rawDesktopAssetText(t, "js/desktop/core/window-interactions-runtime.js")
	windowAI := rawDesktopAssetText(t, "js/desktop/core/window-ai-context.js")
	agentChat := rawDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	css := readAllDesktopCSS(t)

	if !strings.Contains(windowRuntime, "aiButtonMarkup(appId)") {
		t.Fatal("desktop window shell runtime missing AI button markup hook")
	}

	for _, marker := range []string{
		`[data-action="ai-context"]`,
		"openAgentChatForWindow(id)",
	} {
		if !strings.Contains(windowInteractions, marker) {
			t.Fatalf("desktop window interaction runtime missing AI context marker %q", marker)
		}
	}

	for _, marker := range []string{
		`data-action="ai-context"`,
		"function aiButtonMarkup(",
		"function buildWindowAIContext(",
		"function oliveTinWindowAIContext(",
		"openApp('agent-chat', { window_context: context })",
		"if (appId !== 'agent-chat')",
		"Shared/OliveTin/config.yaml",
		"/config/config.yaml",
		"Use the virtual_desktop tool to read or edit the OliveTin config.",
	} {
		if !strings.Contains(windowAI, marker) {
			t.Fatalf("desktop window AI context provider missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		"function normalizeChatLaunchWindowContext(",
		"host.dataset.chatWindowContext",
		"desktop.chat_request_context",
		"payload.window_context = windowContext",
		"delete host.dataset.chatWindowContext",
	} {
		if !strings.Contains(agentChat, marker) {
			t.Fatalf("agent chat window context integration missing marker %q", marker)
		}
	}

	for _, marker := range []string{
		`.vd-window-button[data-action="ai-context"]::before`,
		`.vd-window-ai-button-icon`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("desktop window AI context CSS missing marker %q", marker)
		}
	}
}

func TestDesktopWindowAIContextTranslations(t *testing.T) {
	t.Parallel()

	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		path := "lang/desktop/" + lang + ".json"
		raw := rawDesktopAssetText(t, path)
		var values map[string]string
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"desktop.chat_request_context", "desktop.window_ai_context"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
