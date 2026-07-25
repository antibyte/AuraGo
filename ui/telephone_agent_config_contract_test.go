package ui

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTelephoneAgentConfigSectionContract(t *testing.T) {
	t.Parallel()

	mainJS := normalizeAssetText(mustReadUIFile(t, "js/config/main.js"))
	for _, marker := range []string{
		`{ key: 'telephone_agent'`,
		`telephone_agent: { m: 'telephone_agent', fn: 'renderTelephoneAgentSection' }`,
		"window.telephoneAgentHasUnsavedChanges",
		"window.telephoneAgentSaveUnsaved",
		"window.telephoneAgentDiscardUnsaved",
		"const specialDirty = sipDirty || telephoneAgentDirty",
	} {
		if !strings.Contains(mainJS, marker) {
			t.Fatalf("config main.js missing telephone agent marker %q", marker)
		}
	}

	module := normalizeAssetText(mustReadUIFile(t, "cfg/telephone_agent.js"))
	for _, marker := range []string{
		"/api/sip/agent",
		"/api/sip/agent/catalog",
		"/api/sip/agent/test",
		`data-ta="inbound_route"`,
		`data-ta="auto_answer_delay_ms"`,
		`<select class="field-select" data-ta="voice.backend">`,
		`<select class="field-select" data-ta="voice.agent_provider_id">`,
		`<select class="field-select" data-ta="voice.classic.asr_provider_id">`,
		`<select class="field-select" data-ta="voice.classic.tts_provider">`,
		`<select class="field-select" data-ta="voice.realtime_profile_id">`,
		`data-ta="voice.behavior.additional_prohibitions"`,
		`data-ta="voice.behavior.unavailable_request_behavior"`,
		`data-ta-search`,
		`data-ta-tool`,
		`data-ta="voice.persist_transcripts"`,
		`data-ta="voice.idle_timeout_seconds"`,
		`data-ta-live-confirm`,
		"taComparable(taRead()) !== telephoneAgentSaved",
		"navigateToConfigSection('sip')",
	} {
		if !strings.Contains(module, marker) {
			t.Fatalf("telephone agent module missing contract marker %q", marker)
		}
	}
	if strings.Contains(module, "alert(") || strings.Contains(module, "confirm(") || strings.Contains(module, "prompt(") {
		t.Fatal("telephone agent config must not use native browser dialogs")
	}
	if strings.Contains(module, `type="text" data-ta="voice.agent_provider_id"`) ||
		strings.Contains(module, `type="text" data-ta="voice.classic.asr_provider_id"`) {
		t.Fatal("provider references must use catalog dropdowns")
	}
}

func TestSIPExpertFormDelegatesTelephoneAgentFields(t *testing.T) {
	t.Parallel()

	source := normalizeAssetText(mustReadUIFile(t, "cfg/sip.js"))
	for _, marker := range []string{
		"config.telephone_agent.sip_summary",
		`data-sip-action="telephone-agent"`,
		"navigateToConfigSection('telephone_agent')",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("SIP expert form missing telephone agent delegation marker %q", marker)
		}
	}
	for _, duplicate := range []string{
		`data-sip="inbound.route"`,
		`data-sip="inbound.auto_answer_delay_ms"`,
		`data-sip="voice.backend"`,
		`data-sip="voice.realtime_profile_id"`,
		`data-sip="voice.language"`,
		`data-sip="voice.allowed_tools"`,
		`data-sip="voice.persist_transcripts"`,
		`data-sip="voice.max_call_duration_seconds"`,
	} {
		if strings.Contains(source, duplicate) {
			t.Fatalf("SIP expert form still duplicates telephone field %q", duplicate)
		}
	}
}

func TestTelephoneAgentConfigStylesAreScopedAndResponsive(t *testing.T) {
	t.Parallel()

	page := normalizeAssetText(mustReadUIFile(t, "config.html"))
	if !strings.Contains(page, `/css/config-telephone-agent.css?v={{.BuildVersion}}`) {
		t.Fatal("config page does not load telephone agent stylesheet")
	}
	css := normalizeAssetText(mustReadUIFile(t, "css/config-telephone-agent.css"))
	for _, marker := range []string{
		`[data-workspace-page="config"] .telephone-agent-section`,
		`[data-workspace-page="config"] .ta-status-card`,
		`[data-workspace-page="config"] .ta-grid`,
		`grid-template-columns: repeat(2, minmax(0, 1fr))`,
		`[data-workspace-page="config"] .ta-tools`,
		`@media (max-width: 820px)`,
	} {
		if !strings.Contains(css, marker) {
			t.Fatalf("telephone agent stylesheet missing %q", marker)
		}
	}
	for _, line := range strings.Split(css, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, ".ta-") || strings.HasPrefix(line, "#telephone-agent") {
			t.Fatalf("unscoped telephone agent selector %q", line)
		}
	}
}

func TestTelephoneAgentTranslationsComplete(t *testing.T) {
	t.Parallel()

	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	read := func(locale string) map[string]string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("lang", "config", locale+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(data, &values); err != nil {
			t.Fatalf("%s: %v", locale, err)
		}
		return values
	}
	english := read("en")
	required := make([]string, 0)
	for key := range english {
		if strings.HasPrefix(key, "config.telephone_agent.") || strings.HasPrefix(key, "config.section.telephone_agent.") {
			required = append(required, key)
		}
	}
	if len(required) < 50 {
		t.Fatalf("telephone agent translation contract is unexpectedly small: %d", len(required))
	}
	for _, locale := range locales {
		values := read(locale)
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s missing %s", locale, key)
			}
		}
	}
	if read("de")["config.telephone_agent.title"] == english["config.telephone_agent.title"] {
		t.Fatal("German telephone agent title was left untranslated")
	}
}
