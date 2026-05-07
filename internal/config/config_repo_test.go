package config

import (
	"os"
	"path/filepath"
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
}
