package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestManifestConfigModuleIsRegistered(t *testing.T) {
	mainJS, err := os.ReadFile(filepath.Join("js", "config", "main.js"))
	if err != nil {
		t.Fatalf("read config main.js: %v", err)
	}
	text := string(mainJS)
	for _, marker := range []string{
		"{ key: 'manifest'",
		"manifest: { m: 'manifest', fn: 'renderManifestSection' }",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("config main.js missing Manifest marker %q", marker)
		}
	}
}

func TestManifestConfigModuleControls(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("cfg", "manifest.js"))
	if err != nil {
		t.Fatalf("read cfg/manifest.js: %v", err)
	}
	text := string(raw)
	if strings.Contains(text, "alert(") {
		t.Fatal("cfg/manifest.js must not use alert()")
	}
	for _, marker := range []string{
		"renderManifestSection",
		"manifestTestConnection",
		"manifestStartSidecars",
		"manifestStopSidecars",
		"manifest.api_key",
		"manifest.postgres_password",
		"manifest.better_auth_secret",
		"/api/manifest/test",
		"/api/manifest/start",
		"/api/manifest/stop",
	} {
		if !strings.Contains(text, marker) {
			t.Fatalf("cfg/manifest.js missing marker %q", marker)
		}
	}
}
