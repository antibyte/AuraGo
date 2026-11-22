package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVideoGenerationConfigUsesAgnesSpecificOptions(t *testing.T) {
	body, err := os.ReadFile("cfg/video_generation.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, expected := range []string{
		"videoGenerationProviderType(curProvider) === 'agnes'",
		"agnesLimited ? ['720P'] : ['720P','1080P','1K','2K']",
		"['16:9','9:16','1:1','4:3','3:4','21:9']",
		"['768P','1080P','720p','4k']",
		"videoGenerationProviderChanged(this)",
		"markDirty()",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("video generation config missing %q", expected)
		}
	}
}

func TestMediaGenerationProviderDescriptionsMentionAgnesInEveryLocale(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	for _, locale := range locales {
		imageValues := readLocaleMap(t, filepath.Join("lang", "config", "image_generation", locale+".json"))
		videoValues := readLocaleMap(t, filepath.Join("lang", "config", "video_generation", locale+".json"))
		for key, value := range map[string]interface{}{
			"config.image_generation.provider_desc": imageValues["config.image_generation.provider_desc"],
			"config.video_gen.provider_desc":        videoValues["config.video_gen.provider_desc"],
		} {
			text, ok := value.(string)
			if !ok || !strings.Contains(text, "Agnes AI") {
				t.Errorf("%s %s = %q, want Agnes AI provider guidance", locale, key, text)
			}
		}
	}
}
