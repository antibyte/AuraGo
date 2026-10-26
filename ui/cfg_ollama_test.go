package ui

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
)

func TestOllamaConfigModelsFieldIsAnArray(t *testing.T) {
	source := string(mustReadUIFile(t, "cfg/ollama.js"))
	const path = `data-path="ollama.managed_instance.default_models"`
	const kind = `data-type="array"`
	for _, line := range strings.Split(source, "\n") {
		if strings.Contains(line, "<input") && strings.Contains(line, path) && strings.Contains(line, kind) {
			return
		}
	}
	t.Fatalf("default_models input is missing %s on the same tag as %s", kind, path)
}

func TestOllamaOperationLocalesAreComplete(t *testing.T) {
	for _, locale := range []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"} {
		var values map[string]string
		if err := json.Unmarshal(mustReadUIFile(t, fmt.Sprintf("lang/config/sections/%s.json", locale)), &values); err != nil {
			t.Fatal(err)
		}
		for _, state := range []string{"queued", "checking", "image_pull", "creating", "starting", "api_ready", "model_pull", "ready", "failed", "cancelled", "unknown", "stopped"} {
			if values["config.ollama.state_"+state] == "" {
				t.Fatal(locale, state)
			}
		}
	}
}
