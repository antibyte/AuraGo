package decisions

import (
	"strings"
	"testing"
)

func TestDefaultManifestPinsBothArtifactsAtOneRevision(t *testing.T) {
	manifest := DefaultManifest()
	if err := manifest.Validate(); err != nil {
		t.Fatal(err)
	}
	if manifest.Model.Filename != "jeff-2B.Q4_K_M.gguf" || manifest.Model.Size != 1274396448 {
		t.Fatalf("unexpected model pin: %#v", manifest.Model)
	}
	if manifest.Projector.Filename != "jeff-2B.mmproj.gguf" || manifest.Projector.Size != 671372608 {
		t.Fatalf("unexpected projector pin: %#v", manifest.Projector)
	}
	if manifest.Model.Revision != manifest.Projector.Revision || len(manifest.Model.SHA256) != 64 || len(manifest.Projector.SHA256) != 64 {
		t.Fatalf("model and projector pins are incomplete: %#v", manifest)
	}
	if strings.Contains(artifactURL(manifest, manifest.Model), "guuf") {
		t.Fatal("artifact URL contains the known typo extension")
	}
}

func TestManifestRejectsMutableOrInvalidPins(t *testing.T) {
	manifest := DefaultManifest()
	manifest.Projector.Revision = "main"
	if err := manifest.Validate(); err == nil {
		t.Fatal("mutable projector revision accepted")
	}
	manifest = DefaultManifest()
	manifest.Model.SHA256 = "not-a-hash"
	if err := manifest.Validate(); err == nil {
		t.Fatal("invalid model hash accepted")
	}
}
