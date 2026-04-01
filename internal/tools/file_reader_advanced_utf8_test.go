package tools

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestClampFileReaderContentPreservesUTF8(t *testing.T) {
	content := strings.Repeat("界", fileReaderAdvancedMaxChars)
	content += strings.Repeat("界", 200)

	clamped, truncated := clampFileReaderContent(content)

	if !truncated {
		t.Fatal("expected content to be truncated")
	}
	if !utf8.ValidString(clamped) {
		t.Fatalf("expected valid UTF-8 content, got %q", clamped)
	}
	if !strings.Contains(clamped, "[...truncated for prompt safety") {
		t.Fatalf("expected truncation notice, got %q", clamped)
	}
	if len(clamped) > fileReaderAdvancedMaxChars+200 {
		t.Fatalf("unexpectedly large clamped result: %d bytes", len(clamped))
	}
}
