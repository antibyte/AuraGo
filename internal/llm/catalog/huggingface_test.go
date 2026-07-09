package catalog

import "testing"

func TestHuggingFaceIsRuntimeProviderType(t *testing.T) {
	if !IsRuntimeProviderType("huggingface") {
		t.Fatal("huggingface must be a runtime provider type for router.huggingface.co/v1")
	}
}
