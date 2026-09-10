package tools

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSanoTTSCacheAndCancellation(t *testing.T) {
	cfg := TTSConfig{Provider: "sanotts", Language: "de-DE", DataDir: t.TempDir()}
	other := cfg
	other.Language = "de_AT"
	if ttsCacheKey(cfg, "Hallo") != ttsCacheKey(other, "Hallo") {
		t.Fatal("same effective voice should share cache")
	}
	other.Language = "en"
	if ttsCacheKey(cfg, "Hallo") == ttsCacheKey(other, "Hallo") || ttsAudioExtension(cfg) != ".wav" {
		t.Fatal("language/format cache contract broken")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := TTSSynthesizeInMemoryContext(ctx, cfg, "Hallo"); err == nil {
		t.Fatal("cancelled synthesis succeeded")
	}
}

// Opt-in real CPU/first-use test; artifacts stay under the supplied disposable directory.
func TestSanoTTSCPUSmoke(t *testing.T) {
	dir := os.Getenv("AURAGO_SANOTTS_SMOKE_DIR")
	if dir == "" {
		t.Skip("set AURAGO_SANOTTS_SMOKE_DIR for real CPU synthesis and dependency installation")
	}
	cfg := TTSConfig{Provider: "sanotts", Language: "de-DE", DataDir: dir}
	file, err := TTSSynthesize(cfg, "Hallo! Ich bin AuraGo. Diese Stimme läuft lokal auf deiner CPU.")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, "tts", file))
	if err != nil || !bytes.HasPrefix(data, []byte("RIFF")) || len(data) < 1000 {
		t.Fatalf("invalid German audio: %d bytes, %v", len(data), err)
	}
	cfg.Language = "ja"
	data, ext, err := TTSSynthesizeInMemory(cfg, "Hello! English is the fallback voice.")
	if err != nil || ext != ".wav" || len(data) < 1000 {
		t.Fatalf("English fallback failed: %s, %d bytes, %v", ext, len(data), err)
	}
	if err := os.WriteFile(filepath.Join(dir, "fallback.wav"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}
