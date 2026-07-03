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

func TestRepositoryConfigTemplateWebDAVSecretsStayVaultOnly(t *testing.T) {
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
	webdav, ok := raw["webdav"].(map[string]interface{})
	if !ok {
		t.Fatalf("config_template.yaml missing webdav section")
	}
	if _, ok := webdav["password"]; ok {
		t.Fatalf("webdav template must not include plaintext password")
	}
	if _, ok := webdav["token"]; ok {
		t.Fatalf("webdav template must not include plaintext token")
	}
	readonly, ok := webdav["readonly"].(bool)
	if !ok {
		t.Fatalf("webdav template missing readonly boolean")
	}
	if readonly {
		t.Fatalf("webdav template readonly default = true, want false")
	}
}

func TestRepositoryConfigTemplateJellyfinDefaults(t *testing.T) {
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
	jellyfin, ok := raw["jellyfin"].(map[string]interface{})
	if !ok {
		t.Fatalf("config_template.yaml missing jellyfin section")
	}
	if enabled, ok := jellyfin["enabled"].(bool); !ok || enabled {
		t.Fatalf("jellyfin.enabled = %#v, want false boolean", jellyfin["enabled"])
	}
	if readonly, ok := jellyfin["readonly"].(bool); !ok || readonly {
		t.Fatalf("jellyfin.readonly = %#v, want false boolean", jellyfin["readonly"])
	}
	if destructive, ok := jellyfin["allow_destructive"].(bool); !ok || destructive {
		t.Fatalf("jellyfin.allow_destructive = %#v, want false boolean", jellyfin["allow_destructive"])
	}
	if port, ok := jellyfin["port"].(int); !ok || port != 8096 {
		t.Fatalf("jellyfin.port = %#v, want 8096", jellyfin["port"])
	}
}
