package catalog

import "testing"

func TestAgnesIsRuntimeProviderType(t *testing.T) {
	if !IsRuntimeProviderType("agnes") {
		t.Fatal("agnes should be a runtime provider type")
	}
}
