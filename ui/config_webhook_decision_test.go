package ui

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestWebhookEditorDecisionBlockHasAllLocales(t *testing.T) {
	source := normalizeAssetText(mustReadUIFile(t, "cfg/webhooks.js"))
	keys := []string{"title", "intro", "enable", "question_label", "question_placeholder", "mode_label", "mode_hint", "mode_gate", "gate_warning"}
	for _, key := range keys {
		if !strings.Contains(source, "config.webhooks.decision_"+key) {
			t.Errorf("webhooks.js does not use decision_%s", key)
		}
	}
	for _, want := range []string{"wh-f-decision-enabled", "wh-f-decision-question", "wh-f-decision-mode", "decision: whCollectDecision()"} {
		if !strings.Contains(source, want) {
			t.Errorf("webhooks.js missing %q", want)
		}
	}
	for _, lang := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var table map[string]string
		if err := json.Unmarshal([]byte(mustReadUIFile(t, "lang/config/webhooks/"+lang+".json")), &table); err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		for _, key := range keys {
			if strings.TrimSpace(table["config.webhooks.decision_"+key]) == "" {
				t.Errorf("%s missing decision_%s", lang, key)
			}
		}
	}
}
