package ui

import (
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
