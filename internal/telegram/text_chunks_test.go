package telegram

import (
	"strings"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

func TestTelegramLongUnicodeText(t *testing.T) {
	text := strings.Repeat("Grüße 🌍\n", 1700)
	parts := telegramTextChunks(text)
	if len(parts) < 2 || strings.Join(parts, "") != text {
		t.Fatal("chunking lost content")
	}
	for _, p := range parts {
		if !utf8.ValidString(p) || len(utf16.Encode([]rune(p))) > 4096 {
			t.Fatal("invalid message chunk")
		}
	}
}
