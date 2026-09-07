package ui

import (
	"strings"
	"testing"
)

func TestChatSanitizerKeepsIframesOpaque(t *testing.T) {
	source := readEmbeddedText(t, "js/shared/chat-core.js")
	if strings.Contains(source, "allow-same-origin") {
		t.Fatal("chat sanitizer must not grant iframes a first-party origin")
	}
	if !strings.Contains(source, "node.setAttribute('sandbox', 'allow-scripts')") {
		t.Fatal("chat sanitizer must force iframe sandbox=allow-scripts")
	}
	if !strings.Contains(source, "trimmed.startsWith('//')") {
		t.Fatal("chat sanitizer must reject protocol-relative URLs")
	}
}
