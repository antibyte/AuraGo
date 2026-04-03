package telnyx

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestTruncateSMSMessage_PreservesUTF8(t *testing.T) {
	input := strings.Repeat("ä", 1502)
	got := truncateSMSMessage(input, 1500)

	if !strings.HasSuffix(got, "...") {
		t.Fatalf("expected ellipsis suffix")
	}
	if len([]rune(got)) != 1500 {
		t.Fatalf("expected 1500 runes, got %d", len([]rune(got)))
	}
	if !utf8.ValidString(got) {
		t.Fatal("expected valid UTF-8 output")
	}
}

func TestTruncateSMSMessage_LeavesShortMessagesUntouched(t *testing.T) {
	input := "hello äöü"
	if got := truncateSMSMessage(input, 1500); got != input {
		t.Fatalf("expected unchanged message, got %q", got)
	}
}
