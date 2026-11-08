package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadAppliesMirageVisionDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	m := cfg.Vision.Mirage
	if m.Enabled {
		t.Fatal("Mirage must be disabled by default")
	}
	if m.Mode != MirageModeFallback || m.ContainerName != MirageDefaultContainerName || m.ContainerPort != 8791 ||
		m.Image != "python:3.12-slim" || m.DataPath != "data/mirage" || m.GPU != "auto" ||
		m.MaxObjects != 20 || m.MaxTextLines != 30 || m.Language != "" {
		t.Fatalf("unexpected defaults: %+v", m)
	}
}

func TestMirageModeNormalisesUnknownValues(t *testing.T) {
	cfg := &Config{}
	for in, want := range map[string]string{"": MirageModeFallback, "ALWAYS": MirageModeAlways, " always ": MirageModeAlways, "weird": MirageModeFallback} {
		cfg.Vision.Mirage.Mode = in
		if got := cfg.MirageMode(); got != want {
			t.Fatalf("MirageMode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMirageBaseURLEndsWithV1(t *testing.T) {
	cfg := &Config{}
	cfg.Vision.Mirage.ContainerName = MirageDefaultContainerName
	cfg.Vision.Mirage.ContainerPort = 8791
	got := cfg.MirageBaseURL()
	if !strings.HasSuffix(got, "/v1") {
		t.Fatalf("MirageBaseURL() = %q, want a /v1 suffix", got)
	}
	if !cfg.Runtime.IsDocker && !probeDockerContainer() && !strings.Contains(got, ":8791") {
		t.Fatalf("MirageBaseURL() = %q, want the published host port", got)
	}
}
