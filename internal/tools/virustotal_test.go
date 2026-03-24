package tools

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestComputeVirusTotalFileHashes(t *testing.T) {
	filePath := filepath.Join(t.TempDir(), "sample.txt")
	content := []byte("EICAR test payload")
	if err := os.WriteFile(filePath, content, 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	hashes, size, err := computeVirusTotalFileHashes(filePath)
	if err != nil {
		t.Fatalf("computeVirusTotalFileHashes returned error: %v", err)
	}
	if size != int64(len(content)) {
		t.Fatalf("size = %d, want %d", size, len(content))
	}

	md5Sum := md5.Sum(content)
	sha1Sum := sha1.Sum(content)
	sha256Sum := sha256.Sum256(content)

	if hashes.MD5 != hex.EncodeToString(md5Sum[:]) {
		t.Fatalf("MD5 = %s, want %s", hashes.MD5, hex.EncodeToString(md5Sum[:]))
	}
	if hashes.SHA1 != hex.EncodeToString(sha1Sum[:]) {
		t.Fatalf("SHA1 = %s, want %s", hashes.SHA1, hex.EncodeToString(sha1Sum[:]))
	}
	if hashes.SHA256 != hex.EncodeToString(sha256Sum[:]) {
		t.Fatalf("SHA256 = %s, want %s", hashes.SHA256, hex.EncodeToString(sha256Sum[:]))
	}
}

func TestExecuteVirusTotalScanWithOptionsAutoUploadsWhenHashUnknown(t *testing.T) {
	var (
		lookupCount int
		uploadCount int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/files/"):
			lookupCount++
			http.Error(w, `{"error":{"message":"not found"}}`, http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/files":
			uploadCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"analysis","id":"analysis-123"}}`))
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	oldBaseURL := virustotalBaseURL
	oldClient := virustotalHTTPClient
	virustotalBaseURL = server.URL
	virustotalHTTPClient = server.Client()
	defer func() {
		virustotalBaseURL = oldBaseURL
		virustotalHTTPClient = oldClient
	}()

	filePath := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(filePath, []byte("EICAR test payload"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{
		FilePath: filePath,
		Mode:     "auto",
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["status"] != "success" {
		t.Fatalf("status = %v, want success\noutput=%s", result["status"], out)
	}
	if result["used"] != "file_upload" {
		t.Fatalf("used = %v, want file_upload\noutput=%s", result["used"], out)
	}
	if _, ok := result["hashes"]; !ok {
		t.Fatalf("expected hashes in output\noutput=%s", out)
	}
	if _, ok := result["upload"]; !ok {
		t.Fatalf("expected upload result in output\noutput=%s", out)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want 1", lookupCount)
	}
	if uploadCount != 1 {
		t.Fatalf("uploadCount = %d, want 1", uploadCount)
	}
}

func TestExecuteVirusTotalScanWithOptionsHashModeDoesNotUpload(t *testing.T) {
	var (
		lookupCount int
		uploadCount int
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/files/"):
			lookupCount++
			http.Error(w, `{"error":{"message":"not found"}}`, http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/files":
			uploadCount++
			http.Error(w, "unexpected upload", http.StatusBadRequest)
		default:
			http.Error(w, "unexpected request", http.StatusBadRequest)
		}
	}))
	defer server.Close()

	oldBaseURL := virustotalBaseURL
	oldClient := virustotalHTTPClient
	virustotalBaseURL = server.URL
	virustotalHTTPClient = server.Client()
	defer func() {
		virustotalBaseURL = oldBaseURL
		virustotalHTTPClient = oldClient
	}()

	filePath := filepath.Join(t.TempDir(), "sample.txt")
	if err := os.WriteFile(filePath, []byte("EICAR test payload"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{
		FilePath: filePath,
		Mode:     "hash",
	})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["status"] != "success" {
		t.Fatalf("status = %v, want success\noutput=%s", result["status"], out)
	}
	if result["used"] != "hash_lookup" {
		t.Fatalf("used = %v, want hash_lookup\noutput=%s", result["used"], out)
	}
	if uploadCount != 0 {
		t.Fatalf("uploadCount = %d, want 0", uploadCount)
	}
	if lookupCount != 1 {
		t.Fatalf("lookupCount = %d, want 1", lookupCount)
	}
}
