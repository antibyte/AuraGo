package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigProvidersUsesModelCatalogAPI(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("cfg", "providers.js"))
	if err != nil {
		t.Fatalf("read providers.js: %v", err)
	}
	src := string(data)
	for _, want := range []string{
		"/api/models/catalog",
		"providerCatalogTypes",
		"providerRenderCatalogModelPicker",
	} {
		if !strings.Contains(src, want) {
			t.Fatalf("providers.js missing %q", want)
		}
	}
}
