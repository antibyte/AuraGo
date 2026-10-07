package tools

import (
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
)

func TestManagedSidecarContainerNamesListEveryManagedSidecar(t *testing.T) {
	cfg := &config.Config{}
	got := ManagedSidecarContainerNames(cfg)
	for _, want := range []string{"aurago_gotenberg", "aurago_ollama_managed", "aurago_ollama_embeddings", "aurago-piper-tts",
		"aurago-supertonic-tts", "aurago_ansible", "aurago_browser_automation", "aurago_go2rtc", "aurago_space_agent",
		"aurago_manifest", "aurago_manifest_postgres", "aurago_omniroute", "aurago_dograh_api", "aurago_dograh_ui", "aurago_dograh_ui_proxy",
		"aurago_dograh_postgres", "aurago_dograh_redis", "aurago_dograh_minio", "aurago_dograh_coturn", "aurago-cloudflared"} {
		if !slices.Contains(got, want) {
			t.Fatalf("names = %q, missing %s", got, want)
		}
	}
	// Code Studio and OpenSCAD stay usable (K8), and the names the existing
	// checks already reserve for everyone are not duplicated here.
	for _, allowed := range []string{"aurago-code-studio", "aurago-openscad", "aurago-security-proxy", "aurago-boring-garage", "aurago-homepage"} {
		if slices.Contains(got, allowed) {
			t.Fatalf("names = %q include %s", got, allowed)
		}
	}
	cfg.Ansible.ContainerName = "my-ansible"
	cfg.TTS.Supertonic.ContainerName = "my-tts"
	cfg.Manifest.PostgresContainerName = "manifest-db"
	got = ManagedSidecarContainerNames(cfg)
	for _, want := range []string{"my-ansible", "my-tts", "manifest-db", "aurago_ansible"} {
		if !slices.Contains(got, want) {
			t.Fatalf("names = %q, missing the configured %s", got, want)
		}
	}
	seen := map[string]bool{}
	for _, name := range got {
		if seen[strings.ToLower(name)] {
			t.Fatalf("names = %q list %s twice", got, name)
		}
		seen[strings.ToLower(name)] = true
	}
}
