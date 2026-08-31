package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestChatUsesTypedSystemNotificationsAndIDSpecificRead(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("js", "chat", "chat-history.js"))
	if err != nil {
		t.Fatalf("read chat history source: %v", err)
	}
	source := string(raw)
	for _, marker := range []string{
		"/api/system/notifications",
		"/api/system/notifications/read",
		"note.type === 'morning_briefing'",
		"JSON.stringify({ ids })",
		"chat.system_notification_prefix",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("chat history source missing %q", marker)
		}
	}
	if strings.Contains(source, "fetch('/notifications/read'") {
		t.Fatal("chat UI still acknowledges every legacy notification")
	}
}

func TestSystemNotificationTranslationsExistInEveryChatLocale(t *testing.T) {
	paths, err := filepath.Glob(filepath.Join("lang", "chat", "*.json"))
	if err != nil {
		t.Fatalf("glob chat translations: %v", err)
	}
	if len(paths) != 16 {
		t.Fatalf("chat locales = %d, want 16", len(paths))
	}
	for _, path := range paths {
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		for _, key := range []string{"chat.system_briefing_prefix", "chat.system_notification_prefix"} {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing non-empty %q", path, key)
			}
		}
	}
}
