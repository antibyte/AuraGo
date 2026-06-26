package services

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsBinaryFileAllowsAudioForMultimodalIndexing(t *testing.T) {
	dir := t.TempDir()
	for _, ext := range []string{".mp3", ".wav", ".ogg", ".flac", ".aac", ".m4a", ".wma"} {
		path := filepath.Join(dir, "sample"+ext)
		if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644); err != nil {
			t.Fatalf("write %s: %v", ext, err)
		}
		if IsBinaryFile(path) {
			t.Fatalf("IsBinaryFile(%s) = true, want false for multimodal audio indexing", ext)
		}
	}
}

func TestIsBinaryFileStillBlocksExecutableBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "sample.exe")
	if err := os.WriteFile(path, []byte{0x00, 0x01, 0x02}, 0644); err != nil {
		t.Fatalf("write exe: %v", err)
	}
	if !IsBinaryFile(path) {
		t.Fatal("IsBinaryFile(.exe) = false, want true")
	}
}
