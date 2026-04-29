package tools

import (
	"strings"
	"testing"
)

func TestOllamaReadOnlyBlocksDirectMutations(t *testing.T) {
	cfg := OllamaConfig{ReadOnly: true}

	for name, got := range map[string]string{
		"pull":   OllamaPullModel(cfg, "llama3:latest"),
		"delete": OllamaDeleteModel(cfg, "llama3:latest"),
		"copy":   OllamaCopyModel(cfg, "llama3:latest", "llama3-copy:latest"),
		"load":   OllamaLoadModel(cfg, "llama3:latest"),
		"unload": OllamaUnloadModel(cfg, "llama3:latest"),
	} {
		t.Run(name, func(t *testing.T) {
			if !strings.Contains(got, "read-only mode") {
				t.Fatalf("response = %s, want read-only denial", got)
			}
		})
	}
}
