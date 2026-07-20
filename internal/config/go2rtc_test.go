package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGo2RTCDefaultsAreDisabledAndPinned(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Go2RTC.Enabled {
		t.Fatal("go2rtc must be disabled by default")
	}
	if !cfg.Go2RTC.AutoStart || !cfg.Go2RTC.AgentAccess || !cfg.Go2RTC.StoreMedia {
		t.Fatalf("unexpected go2rtc opt-in defaults: %+v", cfg.Go2RTC)
	}
	if cfg.Go2RTC.WebUIEnabled || cfg.Go2RTC.WebRTC.Enabled {
		t.Fatalf("go2rtc LAN-facing features must be disabled by default: %+v", cfg.Go2RTC)
	}
	if cfg.Go2RTC.Image != Go2RTCDefaultImage {
		t.Fatalf("go2rtc image = %q, want pinned image %q", cfg.Go2RTC.Image, Go2RTCDefaultImage)
	}
	if cfg.Go2RTC.APIHostPort != 1984 || cfg.Go2RTC.WebRTC.Port != 8555 {
		t.Fatalf("unexpected go2rtc ports: API=%d WebRTC=%d", cfg.Go2RTC.APIHostPort, cfg.Go2RTC.WebRTC.Port)
	}
}

func TestGo2RTCVaultHydrationAndSaveNeverSerializeSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte("go2rtc: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cfg.Go2RTC.Streams = []Go2RTCStreamConfig{{
		ID:      "front-door",
		Name:    "Front door",
		Enabled: true,
		Source:  "rtsp://camera-user:camera-password@camera.local/live",
	}}
	cfg.Go2RTC.APIPassword = "internal-api-password"

	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read saved config: %v", err)
	}
	for _, forbidden := range []string{"camera-password", "camera-user", "internal-api-password", "rtsp://"} {
		if strings.Contains(string(saved), forbidden) {
			t.Fatalf("saved config leaked %q:\n%s", forbidden, saved)
		}
	}

	reloaded, err := Load(path)
	if err != nil {
		t.Fatalf("reload config: %v", err)
	}
	if got := reloaded.Go2RTC.Streams[0].Source; got != "" {
		t.Fatalf("reloaded source = %q, want vault-only empty value", got)
	}
	vault := &testSecretVault{data: map[string]string{
		Go2RTCAPIPasswordVaultKey:                "vault-api-password",
		Go2RTCStreamSourceVaultKey("front-door"): "rtsps://camera.local/secure",
	}}
	reloaded.ApplyVaultSecrets(vault)
	if reloaded.Go2RTC.APIPassword != "vault-api-password" {
		t.Fatal("go2rtc API password was not hydrated from vault")
	}
	if reloaded.Go2RTC.Streams[0].Source != "rtsps://camera.local/secure" || !reloaded.Go2RTC.Streams[0].SourceConfigured {
		t.Fatalf("go2rtc stream source was not hydrated: %+v", reloaded.Go2RTC.Streams[0])
	}
}

func TestGo2RTCStreamSourceVaultKeyRejectsUnstableIDs(t *testing.T) {
	if got := Go2RTCStreamSourceVaultKey(" Front_Door-2 "); got != "go2rtc_stream_front_door-2_source" {
		t.Fatalf("vault key = %q", got)
	}
	for _, invalid := range []string{"", "../camera", "front door", "front/door", "ümlaut"} {
		if got := Go2RTCStreamSourceVaultKey(invalid); got != "" {
			t.Fatalf("Go2RTCStreamSourceVaultKey(%q) = %q, want empty", invalid, got)
		}
	}
}

func TestMigratePlaintextSecretsToVaultMovesGo2RTCSources(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	legacy := `
go2rtc:
  enabled: true
  api_password: legacy-internal-password
  streams:
    - id: front-door
      name: Front door
      enabled: true
      source: rtsp://camera-user:camera-password@camera.local/live
`
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatalf("write legacy config: %v", err)
	}
	vault := &testSecretVault{data: map[string]string{}}
	MigratePlaintextSecretsToVault(path, vault, slog.Default())

	if got := vault.data[Go2RTCAPIPasswordVaultKey]; got != "legacy-internal-password" {
		t.Fatalf("migrated API password = %q", got)
	}
	if got := vault.data[Go2RTCStreamSourceVaultKey("front-door")]; got != "rtsp://camera-user:camera-password@camera.local/live" {
		t.Fatalf("migrated stream source = %q", got)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read migrated config: %v", err)
	}
	for _, forbidden := range []string{"api_password", "camera-password", "camera-user", "rtsp://"} {
		if strings.Contains(string(saved), forbidden) {
			t.Fatalf("migrated config leaked %q:\n%s", forbidden, saved)
		}
	}
}
