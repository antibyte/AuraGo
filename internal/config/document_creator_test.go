package config

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestDocumentCreatorPolicyAndURLDefaults(t *testing.T) {
	for _, tc := range []struct{ url, want string }{
		{"", defaultSidecarURL(probeDockerContainer(), "gotenberg", 3000)},
		{"http://gotenberg:3000", defaultSidecarURL(probeDockerContainer(), "gotenberg", 3000)},
		{"http://127.0.0.1:3100", "http://127.0.0.1:3100"},
		{"http://gotenberg:3100", "http://gotenberg:3100"},
		{"https://renderer.example:9443/prefix", "https://renderer.example:9443/prefix"},
	} {
		t.Run(tc.url, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(path, []byte("tools:\n  document_creator:\n    gotenberg:\n      url: "+strconv.Quote(tc.url)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			cfg, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Tools.DocumentCreator.AllowRemoteResources || cfg.Tools.DocumentCreator.Gotenberg.URL != tc.want {
				t.Fatalf("policy=%v url=%q want=%q", cfg.Tools.DocumentCreator.AllowRemoteResources, cfg.Tools.DocumentCreator.Gotenberg.URL, tc.want)
			}
		})
	}
}

func TestDocumentCreatorRemotePolicyRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("tools:\n  document_creator:\n    allow_remote_resources: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if !cfg.Tools.DocumentCreator.AllowRemoteResources {
		t.Fatal("explicit resource grant was lost")
	}
	cfg.Tools.DocumentCreator.AllowRemoteResources = false
	if err := cfg.Save(path); err != nil {
		t.Fatal(err)
	}
	reloaded, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Tools.DocumentCreator.AllowRemoteResources {
		t.Fatal("false policy did not survive save/load")
	}
}
