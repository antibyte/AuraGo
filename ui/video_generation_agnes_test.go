package ui

import (
	"os"
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
		"['480p','720p','768P','1080p']",
		"['16:9','9:16','1:1','4:3','3:4']",
		"['768P','1080P','720p','4k']",
		"videoGenerationProviderChanged(this)",
		"markDirty()",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("video generation config missing %q", expected)
		}
	}
}
