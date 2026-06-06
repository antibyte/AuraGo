package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestRepositoryConfigTemplateYAMLIsParseable(t *testing.T) {
	t.Parallel()

	path := filepath.Join("..", "..", "config_template.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config_template.yaml: %v", err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("parse config_template.yaml: %v", err)
	}
	for _, marker := range []string{
		"additional_prompt: |",
		"## Multilingual natural writing defaults",
		"Do not claim text was written by a human",
	} {
		if !strings.Contains(string(data), marker) {
			t.Fatalf("config_template.yaml missing writer prompt marker %q", marker)
		}
	}
}
