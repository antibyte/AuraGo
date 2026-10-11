package webhooks

import (
	"strings"
	"testing"
)

func TestPresetsIncludeDograhCallback(t *testing.T) {
	var found *Preset
	for _, preset := range Presets() {
		if preset.Key == "dograh" {
			p := preset
			found = &p
			break
		}
	}
	if found == nil {
		t.Fatal("Dograh webhook preset not found")
	}
	if found.Label != "Dograh" {
		t.Fatalf("label = %q, want Dograh", found.Label)
	}
	if len(found.Format.AcceptedContentTypes) == 0 {
		t.Fatal("Dograh preset must declare accepted content types")
	}
	if found.PromptHint == "" {
		t.Fatal("Dograh preset must include a prompt hint")
	}
	for _, preset := range Presets() {
		if preset.Key != "gitlab" {
			continue
		}
		if !strings.Contains(preset.Format.Description, "X-Gitlab-Token is checked in addition to the bearer token and does not replace it.") {
			t.Fatalf("GitLab description = %q", preset.Format.Description)
		}
	}
}
