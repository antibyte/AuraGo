package ui

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestHuggingFaceConfigUsesSafeJobAndUploadContracts(t *testing.T) {
	raw, err := os.ReadFile("cfg/huggingface.js")
	if err != nil {
		t.Fatal(err)
	}
	source := string(raw)
	for _, marker := range []string{
		"allow_job_token_injection",
		"job_namespace",
		"huggingFaceNumber('max_upload_mb'",
		", cfg.max_upload_mb, 10)",
	} {
		if !strings.Contains(source, marker) {
			t.Fatalf("Hugging Face config module missing %q", marker)
		}
	}
	if strings.Contains(source, "router_base_url") {
		t.Fatal("dead Hugging Face router_base_url remains in the platform config UI")
	}
}

func TestHuggingFaceConfigTranslationsCoverAllLocales(t *testing.T) {
	locales := []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}
	required := []string{
		"config.huggingface.allow_job_token_injection",
		"config.huggingface.job_namespace",
		"help.huggingface.allow_job_token_injection",
		"help.huggingface.job_namespace",
		"help.huggingface.allowed_namespaces",
		"help.huggingface.max_upload_mb",
	}
	for _, locale := range locales {
		raw, err := os.ReadFile("lang/config/huggingface/" + locale + ".json")
		if err != nil {
			t.Fatal(err)
		}
		var values map[string]string
		if err := json.Unmarshal(raw, &values); err != nil {
			t.Fatalf("%s translation invalid: %v", locale, err)
		}
		for _, key := range required {
			if strings.TrimSpace(values[key]) == "" {
				t.Fatalf("%s translation missing %q", locale, key)
			}
		}
		if _, exists := values["config.huggingface.router_base_url"]; exists {
			t.Fatalf("%s translation retains dead router label", locale)
		}
		if _, exists := values["help.huggingface.router_base_url"]; exists {
			t.Fatalf("%s translation retains dead router help", locale)
		}
	}
}
