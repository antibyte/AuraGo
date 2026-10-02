package ui

import (
	"strings"
	"testing"
)

func TestRealtimeSpeechOutputGoesThroughTheSharedRouter(t *testing.T) {
	t.Parallel()
	common := readDesktopAssetText(t, "js/realtime-speech/provider-common.js")
	for _, marker := range []string{"class AudioOutput", "window.AuraRealtimeAudioOutput", "aurago-realtime-output-tap", "this.outputNode()"} {
		if !strings.Contains(common, marker) {
			t.Fatalf("provider-common.js lacks %q", marker)
		}
	}
	if strings.Contains(common, "connect(this.context.destination)") {
		t.Fatal("PCMPlayer must play through AuraRealtimeAudioOutput, not context.destination")
	}
	openai := readDesktopAssetText(t, "js/realtime-speech/provider-openai.js")
	if !strings.Contains(openai, "localSilent: true") || strings.Contains(openai, "mute.connect(this.outputContext.destination)") {
		t.Fatal("OpenAI output must be tapped through the shared router")
	}
	lab := readDesktopAssetText(t, "js/realtime-speech/provider-speech-lab.js")
	if strings.Contains(lab, "outputAnalyser.connect(this.outputContext.destination)") {
		t.Fatal("Speech Lab output must go through the shared router")
	}
	core := readDesktopAssetText(t, "js/realtime-speech/core.js")
	if !strings.Contains(core, "progressDestination(") || strings.Contains(core, "progressOutputAnalyser.connect(this.progressOutputContext.destination)") {
		t.Fatal("progress narration must go through the shared router")
	}
	tap := readDesktopAssetText(t, "js/realtime-speech/output-tap-worklet.js")
	if !strings.Contains(tap, "registerProcessor('aurago-realtime-output-tap'") {
		t.Fatal("output tap worklet is missing")
	}
}
