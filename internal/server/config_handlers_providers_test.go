package server

import (
	"reflect"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestBuildSchemaIncludesRocketChatAuthTokenAsSensitive(t *testing.T) {
	schema := buildSchema(reflect.TypeOf(config.Config{}), "")

	var rocketchat *SchemaField
	for i := range schema {
		if schema[i].YAMLKey == "rocketchat" {
			rocketchat = &schema[i]
			break
		}
	}
	if rocketchat == nil {
		t.Fatal("rocketchat section not found in schema")
	}

	for _, field := range rocketchat.Children {
		if field.YAMLKey != "auth_token" {
			continue
		}
		if field.Key != "rocketchat.auth_token" {
			t.Fatalf("unexpected field key: %s", field.Key)
		}
		if !field.Sensitive {
			t.Fatal("expected rocketchat.auth_token to be marked sensitive")
		}
		return
	}

	t.Fatal("rocketchat.auth_token field not found in schema")
}

func TestNormalizeOllamaModelsBaseURLRejectsUnexpectedPrivateHost(t *testing.T) {
	t.Parallel()

	if _, err := normalizeOllamaModelsBaseURL("http://169.254.169.254:11434/v1"); err == nil {
		t.Fatal("expected metadata-service URL to be rejected")
	}
}

func TestNormalizeOllamaModelsBaseURLAllowsLocalOllamaPort(t *testing.T) {
	t.Parallel()

	got, err := normalizeOllamaModelsBaseURL("http://localhost:11434/v1")
	if err != nil {
		t.Fatalf("expected localhost Ollama URL to be allowed: %v", err)
	}
	if got != "http://localhost:11434" {
		t.Fatalf("normalized URL = %q, want http://localhost:11434", got)
	}
}

func TestResolveMeshCentralTestURLRejectsDifferentOverrideHost(t *testing.T) {
	t.Parallel()

	_, err := resolveMeshCentralTestURL("https://169.254.169.254", "https://mesh.example.com")
	if err == nil || !strings.Contains(err.Error(), "configured MeshCentral host") {
		t.Fatalf("expected configured-host override rejection, got %v", err)
	}
}

func TestResolveMeshCentralTestURLAllowsConfiguredPrivateURL(t *testing.T) {
	t.Parallel()

	got, err := resolveMeshCentralTestURL("", "https://192.168.1.20")
	if err != nil {
		t.Fatalf("expected configured private MeshCentral URL to be allowed: %v", err)
	}
	if got != "https://192.168.1.20" {
		t.Fatalf("resolved URL = %q", got)
	}
}

func TestBuildSchemaHidesHelperOwnedLegacyLLMSelectionFields(t *testing.T) {
	schema := buildSchema(reflect.TypeOf(config.Config{}), "")

	findSection := func(name string) *SchemaField {
		for i := range schema {
			if schema[i].YAMLKey == name {
				return &schema[i]
			}
		}
		return nil
	}
	hasChild := func(section *SchemaField, key string) bool {
		if section == nil {
			return false
		}
		for _, child := range section.Children {
			if child.YAMLKey == key {
				return true
			}
		}
		return false
	}

	personality := findSection("personality")
	if hasChild(personality, "v2_provider") {
		t.Fatal("expected personality.v2_provider to be hidden from config schema")
	}

	memoryAnalysis := findSection("memory_analysis")
	if hasChild(memoryAnalysis, "provider") || hasChild(memoryAnalysis, "model") {
		t.Fatal("expected memory_analysis.provider/model to be hidden from config schema")
	}

	tools := findSection("tools")
	if tools == nil {
		t.Fatal("tools section not found in schema")
	}
	for _, toolKey := range []string{"web_scraper", "wikipedia", "ddg_search", "pdf_extractor"} {
		var toolSection *SchemaField
		for i := range tools.Children {
			if tools.Children[i].YAMLKey == toolKey {
				toolSection = &tools.Children[i]
				break
			}
		}
		if hasChild(toolSection, "summary_provider") {
			t.Fatalf("expected %s.summary_provider to be hidden from config schema", toolKey)
		}
	}
}
