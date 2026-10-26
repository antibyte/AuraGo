package phonon

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("PHONON_TEST_HELPER") == "1" {
		text := os.Args[len(os.Args)-1]
		if text == "wait" {
			time.Sleep(10 * time.Second)
		}
		if os.Getenv("HF_TOKEN") != "" || os.Getenv("AURAGO_MASTER_KEY") != "" || os.Args[len(os.Args)-2] != "--" {
			os.Exit(4)
		}
		for i, arg := range os.Args {
			if arg == "-o" && i+1 < len(os.Args) {
				wav := testWAV()
				if text == "invalid" {
					wav = []byte("invalid audio")
				}
				if text == "truncated" {
					wav = wav[:len(wav)-2]
				}
				if os.WriteFile(os.Args[i+1], wav, 0o600) != nil {
					os.Exit(5)
				}
				os.Exit(0)
			}
		}
		os.Exit(6)
	}
	os.Exit(m.Run())
}

func testWAV() []byte {
	wav := make([]byte, 48)
	copy(wav, "RIFF")
	binary.LittleEndian.PutUint32(wav[4:], 40)
	copy(wav[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(wav[16:], 16)
	binary.LittleEndian.PutUint16(wav[20:], 1)
	binary.LittleEndian.PutUint16(wav[22:], 1)
	binary.LittleEndian.PutUint32(wav[24:], 24000)
	binary.LittleEndian.PutUint32(wav[28:], 48000)
	binary.LittleEndian.PutUint16(wav[32:], 2)
	binary.LittleEndian.PutUint16(wav[34:], 16)
	copy(wav[36:], "data")
	binary.LittleEndian.PutUint32(wav[40:], 4)
	binary.LittleEndian.PutUint16(wav[44:], 1234)
	return wav
}

func TestCatalogAndPersonaDefaults(t *testing.T) {
	if len(manifest.Voices) != 102 || len(manifest.ModelRevision) != 40 || len(manifest.Runtimes) != 5 {
		t.Fatal("incomplete pinned catalog or runtime matrix")
	}
	for _, file := range manifest.ModelFiles {
		if file.Size <= 0 || len(file.SHA256) != 64 {
			t.Fatalf("model resource %s lacks a pinned size or SHA-256", file.Name)
		}
	}
	seen := map[string]bool{}
	bundled := 0
	for _, voice := range manifest.Voices {
		if seen[voice.ID] || len(voice.SHA256) != 64 || voice.Filename != "voices/"+voice.ID+".safetensors" || voice.Size != 256136 {
			t.Fatalf("invalid or duplicate voice %s", voice.ID)
		}
		seen[voice.ID] = true
		if voice.Bundled {
			bundled++
		}
	}
	if bundled != 10 {
		t.Fatalf("bundled %d voices, want ten", bundled)
	}
	for lang, names := range map[string][2]string{"de": {"Nele", "Anton"}, "en": {"Zoey", "Easton"}, "fr": {"Noémie", "Augustin"}, "es": {"Noa", "Marcos"}, "pt": {"Manuela-Lu", "Davi"}} {
		for _, persona := range []string{"neutral", "mistress", "secretary", "punk", "custom", "mcp", "unrecognized", "servant", "professional", "thinker", "evil", "psycho", "friend", "terminator"} {
			voice, ok := Lookup(DefaultVoice(lang+"-XX", persona))
			male := strings.Contains("|servant|professional|thinker|evil|psycho|friend|terminator|", "|"+persona+"|")
			expected := names[0]
			if male {
				expected = names[1]
			}
			if !ok || !voice.Bundled || voice.Name != expected {
				t.Fatalf("%s/%s = %s, want %s", lang, persona, voice.Name, expected)
			}
		}
	}
	for _, lang := range []string{"ru", "it", "xx", "auto", ""} {
		if Language(lang) != "" || DefaultVoice(lang, "neutral") != "" {
			t.Fatalf("unsupported language %q silently became English", lang)
		}
	}
	if Language(" DE_at ") != "de" {
		t.Fatal("regional language normalization failed")
	}
}

func fixture(t *testing.T) (string, string, Voice, []byte) {
	t.Helper()
	before := manifest
	t.Cleanup(func() { manifest = before })
	root, dataDir := t.TempDir(), t.TempDir()
	voiceData := []byte("verified voice embedding")
	hash := sha256.Sum256(voiceData)
	voice := manifest.Voices[0]
	voice.Size, voice.SHA256 = int64(len(voiceData)), hex.EncodeToString(hash[:])
	manifest.Voices = []Voice{voice}
	return root, dataDir, voice, voiceData
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestVoiceDownloadIntegrityAndIdempotency(t *testing.T) {
	root, dataDir, voice, data := fixture(t)
	before := downloadClient
	t.Cleanup(func() { downloadClient = before })
	calls := 0
	downloadClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Scheme != "https" || r.URL.Host != "huggingface.co" || !strings.Contains(r.URL.Path, manifest.VoicesRevision) || r.Header.Get("Authorization") != "" {
			t.Fatal("download escaped its pinned, keyless source")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: http.Header{}}, nil
	})}
	for i := 0; i < 2; i++ {
		if err := DownloadVoice(context.Background(), root, dataDir, voice.ID); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 || !Voices(root, dataDir)[0].Installed {
		t.Fatal("installed voice was not retained/reused")
	}
	for _, id := range []string{"../evil", "https://example.com/file", "missing"} {
		if DownloadVoice(context.Background(), root, dataDir, id) == nil {
			t.Fatal("accepted a caller path or unknown voice")
		}
	}
	for _, bad := range [][]byte{data[:3], append(append([]byte(nil), data...), 'x'), bytes.Repeat([]byte{'x'}, len(data))} {
		path := filepath.Join(dataDir, "phonon", "voices", voice.ID+".safetensors")
		if err := os.WriteFile(path, []byte("previous incomplete file"), 0o600); err != nil {
			t.Fatal(err)
		}
		downloadClient.Transport = roundTripFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(bad)), Header: http.Header{}}, nil
		})
		if DownloadVoice(context.Background(), root, dataDir, voice.ID) == nil {
			t.Fatal("accepted an invalid voice download")
		}
		stored, _ := os.ReadFile(path)
		if string(stored) != "previous incomplete file" {
			t.Fatal("failed download replaced an existing file")
		}
	}
}

func TestDownloadCancellationAndRedirects(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := DownloadVoice(ctx, t.TempDir(), t.TempDir(), manifest.Voices[0].ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled download: %v", err)
	}
	for _, target := range []string{"http://huggingface.co/a", "https://localhost/a", "https://huggingface.co.evil.test/a", "https://huggingface.co:444/a"} {
		u, _ := url.Parse(target)
		if downloadClient.CheckRedirect(&http.Request{URL: u}, nil) == nil {
			t.Fatalf("trusted redirect %s", target)
		}
	}
	u, _ := url.Parse("https://cdn-lfs.hf.co/a")
	if err := downloadClient.CheckRedirect(&http.Request{URL: u}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestNativeRunnerCancellationAndWAV(t *testing.T) {
	root, dataDir, voice, data := fixture(t)
	if runtime.GOOS == "windows" && runtime.GOARCH == "arm64" {
		t.Skip("no native upstream runtime")
	}
	t.Setenv("PHONON_TEST_HELPER", "1")
	t.Setenv("HF_TOKEN", "must-not-reach-child")
	t.Setenv("AURAGO_MASTER_KEY", "must-not-reach-child")
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	bin, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(Binary(root)), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Binary(root), bin, 0o700); err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(bin)
	manifest.Runtimes = slices.Clone(manifest.Runtimes)
	for i := range manifest.Runtimes {
		if manifest.Runtimes[i].Target == runtime.GOOS+"_"+runtime.GOARCH {
			manifest.Runtimes[i].BinarySHA256, manifest.Runtimes[i].BinarySize = hex.EncodeToString(hash[:]), int64(len(bin))
			break
		}
	}
	manifest.ModelFiles = nil
	if err := os.MkdirAll(filepath.Join(root, "voices"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, voice.Filename), data, 0o600); err != nil {
		t.Fatal(err)
	}
	wav, err := Synthesize(context.Background(), root, dataDir, "en-US", voice.ID, "--option-like speech")
	if err != nil || !bytes.Equal(wav, testWAV()) {
		t.Fatalf("runner: %v", err)
	}
	for _, text := range []string{"invalid", "truncated"} {
		if _, err := Synthesize(context.Background(), root, dataDir, "en", voice.ID, text); err == nil {
			t.Fatalf("accepted %s WAV", text)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	if _, err := Synthesize(ctx, root, dataDir, "en", voice.ID, "wait"); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("cancellation: %v", err)
	}
}

func TestCPUSmoke(t *testing.T) {
	root := os.Getenv("AURAGO_PHONON_SMOKE_DIR")
	if root == "" {
		t.Skip("set AURAGO_PHONON_SMOKE_DIR to the prepared model/runtime directory")
	}
	texts := map[string]string{"de": "Hallo, das ist ein lokaler Sprachtest.", "en": "Hello, this is a local speech test.", "fr": "Bonjour, ceci est un test de voix locale.", "es": "Hola, esta es una prueba de voz local.", "pt": "Olá, este é um teste de voz local."}
	for _, voice := range manifest.Voices {
		if !voice.Bundled {
			continue
		}
		t.Run(voice.Name, func(t *testing.T) {
			wav, err := Synthesize(context.Background(), root, t.TempDir(), voice.Language, voice.ID, texts[voice.Language])
			if err != nil {
				t.Fatal(err)
			}
			if len(wav) < 1000 {
				t.Fatal("speech WAV is empty")
			}
		})
	}
}
