package catalog

import "testing"

func TestNormalizeProviderIDAliases(t *testing.T) {
	tests := map[string]string{
		"lm-studio":      "lmstudio",
		"llama.cpp":      "llamacpp",
		"github-copilot": "copilot",
		"openrouter":     "openrouter",
	}
	for input, want := range tests {
		if got := NormalizeProviderID(input); got != want {
			t.Fatalf("NormalizeProviderID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestLoadBundledSnapshotFindsModelsAndProviders(t *testing.T) {
	snapshot, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if snapshot.Metadata.PackageName != "@oh-my-pi/pi-catalog" {
		t.Fatalf("package name = %q", snapshot.Metadata.PackageName)
	}
	if snapshot.Metadata.License != "MIT" {
		t.Fatalf("license = %q, want MIT", snapshot.Metadata.License)
	}
	if len(snapshot.Models) == 0 {
		t.Fatal("expected bundled models")
	}
	if len(snapshot.Providers) == 0 {
		t.Fatal("expected bundled providers")
	}

	provider, ok := snapshot.FindProvider("github-copilot")
	if !ok {
		t.Fatal("expected github-copilot provider")
	}
	if provider.AuraProviderType != "copilot" {
		t.Fatalf("github-copilot alias = %q, want copilot", provider.AuraProviderType)
	}
	if provider.CatalogOnly {
		t.Fatal("github-copilot should map to a runtime provider")
	}

	model, ok := snapshot.FindModel("openai", "gpt-4o")
	if !ok {
		t.Fatal("expected openai/gpt-4o model")
	}
	if model.Source != SourceOhMyPi {
		t.Fatalf("model source = %q, want %q", model.Source, SourceOhMyPi)
	}

	if _, ok := snapshot.FindModel("nonexistent-provider", "gpt-4o"); ok {
		t.Fatal("FindModel should not fall back to another provider")
	}
	if _, ok := snapshot.FindModelByID("gpt-4o"); !ok {
		t.Fatal("FindModelByID should still allow provider-agnostic lookup")
	}
}
