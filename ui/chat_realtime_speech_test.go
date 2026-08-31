package ui

import (
	"os"
	"strings"
	"testing"
)

// TestChatIndexLoadsAllRealtimeSpeechProviders guards against regressions where
// the webchat Live Speech panel offers a provider (e.g. Speech Lab) that has no
// matching adapter script loaded, causing "Unsupported realtime speech
// provider" failures at session start.
func TestChatIndexLoadsAllRealtimeSpeechProviders(t *testing.T) {
	t.Parallel()

	indexContent, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	indexHTML := string(indexContent)

	coreIndex := strings.Index(indexHTML, "/js/realtime-speech/core.js")
	if coreIndex < 0 {
		t.Fatal("index.html must load /js/realtime-speech/core.js")
	}
	for _, marker := range []string{
		"/js/realtime-speech/provider-common.js",
		"/js/realtime-speech/provider-openai.js",
		"/js/realtime-speech/provider-xai.js",
		"/js/realtime-speech/provider-gemini.js",
		"/js/realtime-speech/provider-speech-lab.js",
	} {
		providerIndex := strings.Index(indexHTML, marker)
		if providerIndex < 0 {
			t.Fatalf("index.html missing realtime speech provider script %q", marker)
		}
		if providerIndex > coreIndex {
			t.Fatalf("realtime speech provider script %q must load before core.js", marker)
		}
	}
}
