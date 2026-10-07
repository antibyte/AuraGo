package media

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
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

func TestSaveAttachmentContextCancelsAndRemovesPartialFile(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial attachment"))
		w.(http.Flusher).Flush()
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	destDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := SaveAttachmentContext(ctx, server.URL+"/file.bin", "file.bin", destDir)
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("attachment download did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SaveAttachmentContext error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("attachment download did not return after cancellation")
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatalf("read destination directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("canceled attachment left %d partial file(s)", len(entries))
	}
}

func TestSaveURLToDirContextCancelsAndRemovesPartialFile(t *testing.T) {
	requestStarted := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial image"))
		w.(http.Flusher).Flush()
		close(requestStarted)
		<-r.Context().Done()
	}))
	defer server.Close()

	destDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	result := make(chan error, 1)
	go func() {
		_, err := SaveURLToDirContext(ctx, server.URL+"/image.jpg", destDir)
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("image download did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("SaveURLToDirContext error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("image download did not return after cancellation")
	}
	entries, err := os.ReadDir(destDir)
	if err != nil {
		t.Fatalf("read destination directory: %v", err)
	}
	if len(entries) != 0 {
		t.Fatalf("canceled image download left %d partial file(s)", len(entries))
	}
}

func TestDownloadAndSanitizeImageContextCancelsRemoteFetch(t *testing.T) {
	requestStarted := make(chan struct{})
	serverCanceled := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("partial image"))
		w.(http.Flusher).Flush()
		close(requestStarted)
		<-r.Context().Done()
		close(serverCanceled)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	destDir := t.TempDir()
	result := make(chan error, 1)
	go func() {
		_, err := DownloadAndSanitizeImageContext(ctx, server.URL+"/image.jpg", destDir)
		result <- err
	}()

	select {
	case <-requestStarted:
	case <-time.After(2 * time.Second):
		t.Fatal("remote image fetch did not start")
	}
	cancel()
	select {
	case <-serverCanceled:
	case <-time.After(2 * time.Second):
		t.Fatal("remote image request did not observe cancellation")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("DownloadAndSanitizeImageContext error = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("image sanitizer did not return after cancellation")
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
