package catalogsync

import "testing"

func TestBuildSnapshotNormalizesModelsProvidersAndMetadata(t *testing.T) {
	modelsJSON := []byte(`{
		"openai": {
			"gpt-4o": {
				"id": "gpt-4o",
				"name": "GPT-4o",
				"api": "openai-completions",
				"provider": "openai",
				"baseUrl": "https://api.openai.com/v1",
				"reasoning": true,
				"input": ["text", "image"],
				"cost": {
					"input": 2.5,
					"output": 10,
					"cacheRead": 1.25,
					"cacheWrite": 0
				},
				"contextWindow": 128000,
				"maxTokens": 16384
			}
		}
	}`)
	descriptorsTS := []byte(`export const CATALOG_PROVIDERS = [
		{
			id: "lm-studio",
			defaultModel: "qwen3",
			envVars: ["LM_STUDIO_API_KEY"],
			allowUnauthenticated: true,
			catalogDiscovery: { label: "LM Studio" },
		},
		{
			id: "example-provider",
			defaultModel: "example-model",
			envVars: ["EXAMPLE_API_KEY"],
			catalogDiscovery: { label: "Example Provider", oauthProvider: "example" },
		},
	] satisfies ProviderCatalogEntry[];`)

	snapshot, err := BuildSnapshot(modelsJSON, descriptorsTS, PackageMetadata{
		Name:          "@oh-my-pi/pi-catalog",
		Version:       "16.1.10",
		TarballURL:    "https://registry.npmjs.org/@oh-my-pi/pi-catalog/-/pi-catalog-16.1.10.tgz",
		License:       "MIT",
		RepositoryURL: "https://github.com/can1357/oh-my-pi",
	})
	if err != nil {
		t.Fatalf("BuildSnapshot() error = %v", err)
	}
	if snapshot.Metadata.PackageName != "@oh-my-pi/pi-catalog" {
		t.Fatalf("package name = %q", snapshot.Metadata.PackageName)
	}
	if snapshot.Metadata.License != "MIT" {
		t.Fatalf("license = %q, want MIT", snapshot.Metadata.License)
	}
	if len(snapshot.Metadata.SourceFiles) == 0 {
		t.Fatal("expected source file metadata")
	}

	model, ok := snapshot.FindModel("openai", "gpt-4o")
	if !ok {
		t.Fatal("expected openai/gpt-4o model in snapshot")
	}
	if model.Provider != "openai" || model.ID != "gpt-4o" {
		t.Fatalf("unexpected model identity: %+v", model)
	}
	if !model.Multimodal || !model.Reasoning {
		t.Fatalf("model capabilities not normalized: %+v", model)
	}
	if model.ContextWindow != 128000 || model.MaxTokens != 16384 {
		t.Fatalf("model token limits not normalized: %+v", model)
	}
	if model.Cost.Input != 2.5 || model.Cost.Output != 10 || model.Cost.CacheRead != 1.25 {
		t.Fatalf("model cost not normalized: %+v", model.Cost)
	}

	lmStudio, ok := snapshot.FindProvider("lm-studio")
	if !ok {
		t.Fatal("expected lm-studio provider")
	}
	if lmStudio.AuraProviderType != "lmstudio" {
		t.Fatalf("lm-studio alias = %q, want lmstudio", lmStudio.AuraProviderType)
	}
	if lmStudio.CatalogOnly {
		t.Fatal("lm-studio should be runtime selectable")
	}
	if !lmStudio.AllowUnauthenticated {
		t.Fatal("lm-studio should be marked unauthenticated")
	}

	example, ok := snapshot.FindProvider("example-provider")
	if !ok {
		t.Fatal("expected example-provider")
	}
	if !example.CatalogOnly {
		t.Fatal("unknown provider should be catalog_only")
	}
	if example.OAuthProvider != "example" {
		t.Fatalf("oauth provider = %q", example.OAuthProvider)
	}
}

func TestBuildSnapshotRejectsMissingLicenseMetadata(t *testing.T) {
	_, err := BuildSnapshot([]byte(`{}`), []byte(`export const CATALOG_PROVIDERS = [];`), PackageMetadata{
		Name:       "@oh-my-pi/pi-catalog",
		Version:    "16.1.10",
		TarballURL: "https://example.invalid/catalog.tgz",
	})
	if err == nil {
		t.Fatal("expected missing license metadata to fail")
	}
}
