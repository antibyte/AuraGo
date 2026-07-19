package ui

import (
	"strings"
	"testing"
)

func TestSetupIncludesAgnesProviderAndAutomaticSubsystemModels(t *testing.T) {
	t.Parallel()

	html := normalizeAssetText(mustReadUIFile(t, "setup.html"))
	for _, marker := range []string{
		`value="agnes"`,
		`data-i18n="setup.step0_provider_agnes"`,
	} {
		if !strings.Contains(html, marker) {
			t.Fatalf("setup.html missing Agnes AI marker %q", marker)
		}
	}

	script := normalizeAssetText(mustReadUIFile(t, "js/setup/main.js"))
	for _, marker := range []string{
		`baseUrl: 'https://apihub.agnes-ai.com/v1'`,
		`defaultModel: 'agnes-2.0-flash'`,
		`helper_model: (p.features && p.features.helper && m.helper) ? getQuickProfileSubsystemRuntime(p, m.helper).model : ''`,
		`provider === 'agnes'`,
		`model: 'agnes-image-2.1-flash'`,
		`model: 'agnes-video-v2.0'`,
		`patch.image_generation = {`,
		`patch.video_generation = {`,
	} {
		if !strings.Contains(script, marker) {
			t.Fatalf("setup JS missing Agnes AI marker %q", marker)
		}
	}
}
