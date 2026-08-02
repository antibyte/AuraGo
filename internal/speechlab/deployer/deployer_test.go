package deployer

import "testing"

func validManifest() BundleManifest {
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	return BundleManifest{
		SchemaVersion: 1, BundleVersion: "stable", ContractVersion: "speech-lab/v1",
		Publisher: "ghcr.io/antibyte", Network: "aurago-speech-lab",
		Images: ImageSet{
			Gateway:   "ghcr.io/antibyte/s2s-vulkan@sha256:" + digest,
			ASR:       "ghcr.io/antibyte/s2s-whisper-fw@sha256:" + digest,
			TTS:       "ghcr.io/antibyte/s2s-vulkan@sha256:" + digest,
			LLM:       "ghcr.io/antibyte/s2s-llama-granite@sha256:" + digest,
			Web:       "ghcr.io/antibyte/s2s-web@sha256:" + digest,
			ModelInit: "ghcr.io/antibyte/s2s-model-init@sha256:" + digest,
		},
		StartOrder: []string{"model_init", "asr", "llm", "tts", "gateway", "web"},
		Services:   []BundleService{{Role: "gateway", Image: "gateway"}},
	}
}

func TestValidateManifestRequiresAllowlistedFullDigests(t *testing.T) {
	if err := validateManifest(validManifest()); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	tests := []struct {
		name string
		edit func(*BundleManifest)
	}{
		{"wrong publisher", func(m *BundleManifest) { m.Publisher = "ghcr.io/other" }},
		{"mutable image", func(m *BundleManifest) { m.Images.Gateway = "ghcr.io/antibyte/s2s-vulkan:latest" }},
		{"short digest", func(m *BundleManifest) { m.Images.Gateway = "ghcr.io/antibyte/s2s-vulkan@sha256:abcd" }},
		{"wrong contract", func(m *BundleManifest) { m.ContractVersion = "speech-lab/v2" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := validManifest()
			tt.edit(&manifest)
			if err := validateManifest(manifest); err == nil {
				t.Fatal("invalid manifest accepted")
			}
		})
	}
}
