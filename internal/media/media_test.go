package media

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveAttachmentRejectsOversizedContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "104857601")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	_, err := SaveAttachment(srv.URL+"/file.bin", "file.bin", t.TempDir())
	if err == nil {
		t.Fatal("expected oversized attachment error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestDownloadFileRejectsOversizedContentLength(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "52428801")
		_, _ = w.Write([]byte("x"))
	}))
	defer srv.Close()

	_, err := DownloadFile(srv.URL+"/voice.ogg", "voice")
	if err == nil {
		t.Fatal("expected oversized download error")
	}
	if !strings.Contains(err.Error(), "exceeds maximum size") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestDownloadFileUsesURLPathExtensionForSignedURLs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer srv.Close()

	path, err := DownloadFile(srv.URL+"/voice.ogg?token=abc#fragment", "voice")
	if err != nil {
		t.Fatalf("DownloadFile: %v", err)
	}
	defer os.Remove(path)

	if filepath.Ext(path) != ".ogg" {
		t.Fatalf("temp path = %q, want .ogg extension", path)
	}
}

func TestSaveURLToDirStripsFragmentBeforeExtension(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("RIFFxxxxWEBP"))
	}))
	defer srv.Close()

	path, err := SaveURLToDir(srv.URL+"/image.webp#signed-fragment", t.TempDir())
	if err != nil {
		t.Fatalf("SaveURLToDir: %v", err)
	}
	if filepath.Ext(path) != ".webp" {
		t.Fatalf("saved path = %q, want .webp extension", path)
	}
}
