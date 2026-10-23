package tools

import (
	"context"
	"crypto/md5"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/phonon"
	voicepcm "aurago/internal/voice"
)

func TestPhononFallbackCacheAndCancellation(t *testing.T) {
	cfg := TTSConfig{Provider: "phonon", Language: "ru-RU", DataDir: t.TempDir()}
	cfg.Phonon.Root = t.TempDir()
	resolved := resolvePhononTTSConfig(cfg)
	if resolved.Provider != "sanotts" || resolved.Language != "ru-RU" {
		t.Fatal("unsupported language must use the old local provider unchanged")
	}
	cfg.Language = "de"
	resolved = resolvePhononTTSConfig(cfg)
	if resolved.Provider != "sanotts" || resolved.Phonon.VoiceID != phonon.DefaultVoice("de", "") {
		t.Fatal("missing resources must fall back to local speech")
	}
	cfg.NoProviderFallback = true
	resolved = resolvePhononTTSConfig(cfg)
	if resolved.Provider != "phonon" {
		t.Fatal("glass speech must stay on the selected provider")
	}
	cfg.NoProviderFallback = false
	resolved = resolvePhononTTSConfig(cfg)
	want := fmt.Sprintf("%x.wav", md5.Sum([]byte(ttsCacheKey(resolved, "cached local speech"))))
	if err := os.MkdirAll(filepath.Join(cfg.DataDir, "tts"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfg.DataDir, "tts", want), []byte("existing Sano WAV"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := TTSSynthesize(cfg, "cached local speech")
	if err != nil || got != want {
		t.Fatalf("fallback cache = %s, %v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	cfg.Context = ctx
	if _, err := TTSSynthesize(cfg, "cached local speech"); !errors.Is(err, context.Canceled) {
		t.Fatalf("cached cancellation: %v", err)
	}
	if _, _, err := TTSSynthesizeInMemoryContext(ctx, cfg, "speech"); !errors.Is(err, context.Canceled) {
		t.Fatalf("transient cancellation: %v", err)
	}
	for _, err := range []error{context.Canceled, context.DeadlineExceeded} {
		if phononFallbackAllowed(context.Background(), err) {
			t.Fatal("cancellation started a fallback")
		}
	}
	if !phononFallbackAllowed(context.Background(), errors.New("unsupported runtime")) {
		t.Fatal("runtime failure has no local fallback")
	}
	for _, text := range []string{"   ", strings.Repeat("x", 2001)} {
		if _, _, err := TTSSynthesizeInMemoryContext(context.Background(), cfg, text); err == nil || !strings.Contains(err.Error(), "1–2000") {
			t.Fatal("invalid local speech text started provisioning/synthesis")
		}
	}
}

func TestPhononCPUSmokePaths(t *testing.T) {
	root := os.Getenv("AURAGO_PHONON_SMOKE_DIR")
	if root == "" {
		t.Skip("set AURAGO_PHONON_SMOKE_DIR for actual offline cached/transient synthesis")
	}
	dataDir := t.TempDir()
	for _, voice := range phonon.Voices(root, dataDir) {
		if !voice.Bundled {
			continue
		}
		t.Run(voice.Name, func(t *testing.T) {
			cfg := TTSConfig{Provider: "phonon", Language: voice.Language, DataDir: dataDir}
			cfg.Phonon.Root, cfg.Phonon.VoiceID = root, voice.ID
			text := "Phonon: 123."
			resolved := resolvePhononTTSConfig(cfg)
			if resolved.Provider != "phonon" {
				t.Fatal("actual Phonon resources are not usable")
			}
			want := fmt.Sprintf("%x.wav", md5.Sum([]byte(ttsCacheKey(resolved, text))))
			filename, err := TTSSynthesize(cfg, text)
			if err != nil || filename != want {
				t.Fatalf("Phonon used fallback or failed: %s, %v", filename, err)
			}
			audio, err := os.ReadFile(filepath.Join(dataDir, "tts", filename))
			if err != nil || len(audio) < 1000 {
				t.Fatal("empty cached speech")
			}
			cached, err := TTSSynthesize(cfg, text)
			if err != nil || cached != filename {
				t.Fatal("cache reuse failed")
			}
			data, ext, err := TTSSynthesizeInMemoryContext(context.Background(), cfg, text)
			if err != nil || ext != ".wav" || len(data) < 1000 {
				t.Fatalf("transient speech: %v", err)
			}
			pcm, rate, err := voicepcm.DecodeWAVPCM16Source(data)
			nonzero := false
			for _, sample := range pcm {
				if sample != 0 {
					nonzero = true
					break
				}
			}
			if err != nil || rate != 24000 || !nonzero {
				t.Fatalf("speech is silent or incompatible with telephone PCM decoding: rate=%d, error=%v", rate, err)
			}
		})
	}
}

func TestPhononCacheSeparatesVoicesAndLanguages(t *testing.T) {
	cfg := TTSConfig{Provider: "phonon", Language: "de"}
	cfg.Phonon.VoiceID = phonon.DefaultVoice("de", "neutral")
	female := ttsCacheKey(cfg, "test")
	cfg.Phonon.VoiceID = phonon.DefaultVoice("de", "servant")
	male := ttsCacheKey(cfg, "test")
	cfg.Language = "fr"
	if female == male || male == ttsCacheKey(cfg, "test") || ttsAudioExtension(cfg) != ".wav" {
		t.Fatal("local voice/language cache collision")
	}
	cfg.Language = "de-AT"
	regional := ttsCacheKey(cfg, "test")
	cfg.Language = "de"
	if regional != ttsCacheKey(cfg, "test") {
		t.Fatal("equivalent speech languages use different cache keys")
	}
}
