package ui

import (
	"strings"
	"testing"
)

// Wave 1, batch 5: chromecast, cyd, go2rtc, local_llm, newspaper, ollama, providers,
// realtime_speech, sip, treg and tts use the shared result line / state banner, send
// the draft where a test can run without saving, and keep secrets in the save dock.
func TestConfigWave1Batch5ModuleContracts(t *testing.T) {
	for _, module := range []string{"chromecast", "cyd", "go2rtc", "local_llm", "newspaper", "providers", "realtime_speech", "treg"} {
		if !strings.Contains(readWave0File(t, "cfg/"+module+".js"), "cfgActionResult(") {
			t.Errorf("%s does not use cfgActionResult", module)
		}
	}
	for _, module := range []string{"cyd", "go2rtc", "local_llm", "ollama", "sip", "treg", "tts"} {
		if !strings.Contains(readWave0File(t, "cfg/"+module+".js"), "cfgStatusBanner") {
			t.Errorf("%s does not use cfgStatusBanner", module)
		}
	}

	// Tests that run with the values typed in send the draft (or their own form body).
	drafts := map[string][]string{
		"treg":      {`id="treg-test-btn" data-config-action-draft="true"`, "cfgDraftPatch(['treg'])", "/api/treg/test-connection"},
		"ollama":    {`id="ollama-test-btn" type="button" class="btn btn-secondary btn-sm" data-config-action-draft="true"`, "cfgDraftPatch(['ollama'])", "/api/ollama/models?integration=ollama"},
		"providers": {`id="prov-test-metadata" data-config-action-draft="true"`, `id="prov-test-generation" data-config-action-draft="true"`, `id="meshcentral-test-btn" class="btn-save prov-btn-sm" data-config-action-draft="true"`, "provider: providerTestDraft()"},
	}
	for module, markers := range drafts {
		src := readWave0File(t, "cfg/"+module+".js")
		for _, marker := range markers {
			if !strings.Contains(src, marker) {
				t.Errorf("%s misses %q", module, marker)
			}
		}
	}

	// Tests that need the running service stay locked while something is unsaved.
	runtime := map[string][]string{
		"chromecast": {`id="cfgdev-test-btn-`},
		"cyd":        {`id="cyd-test-btn" data-config-action-draft="false"`},
		"go2rtc":     {`id="go2rtc-test" data-config-action-draft="false"`, `id="go2rtc-hardware-test" data-config-action-draft="false"`, `id="go2rtc-snapshot-' + index + '" data-config-action-draft="false"`},
		"newspaper":  {`id="newspaper-check-btn" class="btn-save dc-test-btn" type="button" data-config-action-draft="false"`},
		"ollama":     {`id="ollama-recreate-btn" type="button" data-config-action-draft="false"`},
		"sip":        {`data-sip-profile="test" data-config-action-draft="false"`},
	}
	for module, markers := range runtime {
		src := readWave0File(t, "cfg/"+module+".js")
		for _, marker := range markers {
			if !strings.Contains(src, marker) {
				t.Errorf("%s misses runtime-only marker %q", module, marker)
			}
		}
	}

	// Secrets live in the save dock: data-path on the input, no own vault button or request.
	secrets := map[string][]string{
		"treg": {"treg.token"},
		"tts":  {"tts.elevenlabs.api_key", "tts.minimax.api_key", "tts.mistral.api_key", "'.api_key"},
	}
	for module, paths := range secrets {
		src := readWave0File(t, "cfg/"+module+".js")
		for _, path := range paths {
			if !strings.Contains(src, path) {
				t.Errorf("%s: secret %s is not in the save dock", module, path)
			}
		}
		for _, banned := range []string{"/api/vault/secrets", "adg-save-btn\" onclick=\"ttsSave", "treg-save-token"} {
			if strings.Contains(src, banned) {
				t.Errorf("%s still has its own vault save (%s)", module, banned)
			}
		}
	}

	// Plain-text save line of the SIP expert form (main.js reads its text).
	if !strings.Contains(readWave0File(t, "cfg/sip.js"), `<span id="sip-action-status" role="status" aria-live="polite"></span>`) {
		t.Error("sip-action-status must stay a plain text line")
	}
	// The static warning of the cloud providers is a note, not a status banner.
	tts := readWave0File(t, "cfg/tts.js")
	if !strings.Contains(tts, "cfg-note-banner cfg-note-banner-warning") || strings.Contains(tts, "adg-status-banner") {
		t.Error("tts notices must use cfg-note-banner-warning")
	}
	// No emoji dots in the banners of this batch.
	for _, module := range []string{"tts", "sip", "cyd", "local_llm", "ollama", "go2rtc", "treg"} {
		src := readWave0File(t, "cfg/"+module+".js")
		for _, dot := range []string{"🟢", "🔴", "⚪", "🟡"} {
			if strings.Contains(src, dot) {
				t.Errorf("%s still draws an emoji dot %s", module, dot)
			}
		}
	}
}
