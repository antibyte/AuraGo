package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTTSConfigOffersMistralProvider(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "tts.js"))
	if err != nil {
		t.Fatalf("read tts.js: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		`<option value="mistral"`,
		`config.tts.provider_mistral`,
		`id="tts-mistral-section"`,
		`tts.mistral.voice_id`,
		`tts.mistral.model_id`,
		`ttsSaveMistralKey`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("tts.js missing Mistral UI contract %q", want)
		}
	}
}

func TestSupertonicStylesRetriesWhileManagedSidecarStarts(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "tts.js"))
	if err != nil {
		t.Fatalf("read tts.js: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		`res.code === 'supertonic_starting'`,
		`response.headers.get('Retry-After')`,
		`supertonicLoadStyles(list, attempt + 1)`,
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("tts.js missing Supertonic startup retry contract %q", want)
		}
	}
}
