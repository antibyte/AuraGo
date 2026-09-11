package sanotts

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIUsesManagedVenv(t *testing.T) {
	got := CLI("/data")
	if runtime.GOOS == "windows" {
		if !strings.Contains(got, filepath.Join("sanotts", "venv", "Scripts", "sanotts.exe")) {
			t.Fatalf("cli=%q", got)
		}
		return
	}
	if !strings.Contains(filepath.ToSlash(got), "sanotts/venv/bin/sanotts") {
		t.Fatalf("cli=%q", got)
	}
}

func TestVoicesMatchBundledRuntime(t *testing.T) {
	if got := fmt.Sprintf("%x", sha256.Sum256(Wheel)); got != "0582dabc20d1b6376bd1e266b889bbca6d839733d3d4bcf2be493748cf139a23" {
		t.Fatalf("unexpected runtime wheel digest %s", got)
	}
	z, err := zip.NewReader(bytes.NewReader(Wheel), int64(len(Wheel)))
	if err != nil {
		t.Fatal(err)
	}
	f, err := z.Open("sanotts/tables/voices.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	var catalog struct {
		Voices map[string]json.RawMessage `json:"voices"`
	}
	if err := json.NewDecoder(f).Decode(&catalog); err != nil {
		t.Fatal(err)
	}
	for language, want := range map[string]string{
		"Deutsch": "de-tiny", "de-DE": "de-tiny", "DE_at": "de-tiny",
		"en": "heart-nano", "English": "heart-nano", "fr-CA": "fr",
		"it": "it-tiny", "es": "es-tiny", "pt-BR": "pt-tiny",
		"cs": "cs-tiny", "ru": "ru-tiny", "ro": "ro-tiny", "tr": "tr-tiny",
		"id": "id", "vi": "vi", "ar": "ar",
		"ja": "heart-nano", "hi": "heart-nano", "ne": "heart-nano", "zh": "heart-nano",
		"nl": "heart-nano", "unknown": "heart-nano", "": "heart-nano",
	} {
		if got := Voice(language); got != want || catalog.Voices[got] == nil {
			t.Errorf("Voice(%q) = %q, want packaged %q", language, got, want)
		}
	}
	// Reject bad input before touching a process or the filesystem.
	if _, err := Synthesize(context.Background(), "missing", "de-tiny", "", ""); err == nil {
		t.Fatal("empty text accepted")
	}
}
