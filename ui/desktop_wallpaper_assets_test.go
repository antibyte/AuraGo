package ui

import (
	"bytes"
	"encoding/json"
	"image"
	_ "image/jpeg"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

var desktopWallpaperOptions = []string{
	"groupshoot",
	"alpine_dawn",
	"city_rain",
	"ocean_cliff",
	"aurora_glass",
	"nebula_flow",
	"paper_waves",
}

func TestDesktopWallpaperAssetsAreEmbeddedAndSelectable(t *testing.T) {
	t.Parallel()

	shellText := readDesktopAssetText(t, "js/desktop/main.js")
	foundationText := readDesktopAssetText(t, "js/desktop/core/desktop-foundation.js")
	cssText := readAllDesktopCSS(t)

	defs := desktop.DesktopSettingDefinitions()
	wallpaperValues := map[string]bool{}
	for _, def := range defs {
		if def.Key == "appearance.wallpaper" {
			for _, value := range def.Values {
				wallpaperValues[value] = true
			}
		}
	}

	for _, name := range desktopWallpaperOptions {
		translationKey := "desktop.settings_wallpaper_" + name
		if !strings.Contains(shellText, "'"+name+"'") || !strings.Contains(shellText, translationKey) {
			t.Fatalf("desktop shell is missing wallpaper option %q", name)
		}
		if !strings.Contains(cssText, `data-wallpaper="`+name+`"`) || !strings.Contains(cssText, "wallpapers/"+name+".jpg") {
			t.Fatalf("desktop stylesheet is missing wallpaper background %q", name)
		}
		if !wallpaperValues[name] {
			t.Fatalf("desktop setting definitions missing wallpaper value %q", name)
		}
		data, err := Content.ReadFile("img/wallpapers/" + name + ".jpg")
		if err != nil {
			t.Fatalf("wallpaper asset %q is not embedded: %v", name, err)
		}
		if len(data) < 500000 {
			t.Fatalf("wallpaper asset %q is unexpectedly small: %d bytes", name, len(data))
		}
		cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil {
			t.Fatalf("decode wallpaper %q dimensions: %v", name, err)
		}
		if cfg.Width < 1920 || cfg.Height < 1080 {
			t.Fatalf("wallpaper %q dimensions = %dx%d, want at least 1920x1080", name, cfg.Width, cfg.Height)
		}
		aspect := float64(cfg.Width) / float64(cfg.Height)
		if aspect < 1.73 || aspect > 1.82 {
			t.Fatalf("wallpaper %q aspect ratio = %.3f, want desktop-friendly 16:9-ish", name, aspect)
		}
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
		for _, name := range desktopWallpaperOptions {
			key := "desktop.settings_wallpaper_" + name
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}

	if !strings.Contains(foundationText, "'appearance.wallpaper': 'groupshoot'") {
		t.Fatal("desktop frontend defaults must use groupshoot as the default wallpaper")
	}
}

func TestDesktopChatQuestionPromptAssets(t *testing.T) {
	t.Parallel()

	agentChat := readDesktopAssetText(t, "js/desktop/apps/agent-chat.js")
	cssText := readAllDesktopCSS(t)
	for _, marker := range []string{
		"event === 'question_user'",
		"showDesktopQuestionModal(host, normalizeDesktopQuestionPayload(data))",
		"fetch('/api/agent/question-response'",
		"session_id: 'virtual-desktop'",
	} {
		if !strings.Contains(agentChat, marker) {
			t.Fatalf("desktop chat question UI missing marker %q", marker)
		}
	}
	for _, marker := range []string{
		".vd-chat-question-panel",
		".vd-chat-question-options",
		".vd-chat-question-free-text",
		".vd-chat-question-timer",
	} {
		if !strings.Contains(cssText, marker) {
			t.Fatalf("desktop chat question CSS missing marker %q", marker)
		}
	}

	keys := []string{
		"desktop.chat_question_waiting",
		"desktop.chat_question_select",
		"desktop.chat_question_free_text_placeholder",
		"desktop.chat_question_timeout",
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
		for _, key := range keys {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty translation for %s", path, key)
			}
		}
	}
}
