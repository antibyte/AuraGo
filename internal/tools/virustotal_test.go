package tools

import (
	"aurago/internal/testutil"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClassifyVirusTotalResource(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		resource string
		want     string
	}{
		{name: "md5 hash", resource: "44d88612fea8a8f36de82e1278abb02f", want: "file_hash"},
		{name: "sha1 hash", resource: "3395856ce81f2b7382dee72602f798b642f14140", want: "file_hash"},
		{name: "sha256 hash", resource: "275a021bbfb6488d9f5119f5c61d60b4b0b7f38b8a2567b96a8f3a8a2e7f3f29", want: "file_hash"},
		{name: "url", resource: "https://example.com/download.exe", want: "url"},
		{name: "ip", resource: "8.8.8.8", want: "ip_address"},
		{name: "domain", resource: "example.com", want: "domain"},
		{name: "invalid", resource: "not a resource", want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyVirusTotalResource(tt.resource); got != tt.want {
				t.Fatalf("classifyVirusTotalResource(%q) = %q, want %q", tt.resource, got, tt.want)
			}
		})
	}
}

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
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestExecuteVirusTotalScanWithOptionsHashResourceUsesFileEndpoint(t *testing.T) {
	hash := "44d88612fea8a8f36de82e1278abb02f"
	var requestedPath string

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"file","id":"44d88612fea8a8f36de82e1278abb02f"}}`))
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

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{Resource: hash})

	if requestedPath != "/files/"+hash {
		t.Fatalf("requestedPath = %q, want %q", requestedPath, "/files/"+hash)
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["status"] != "success" {
		t.Fatalf("status = %v, want success\noutput=%s", result["status"], out)
	}
	if result["used"] != "file_report" {
		t.Fatalf("used = %v, want file_report\noutput=%s", result["used"], out)
	}
}

func TestExecuteVirusTotalScanWithOptionsDomainResourceUsesDomainEndpoint(t *testing.T) {
	var requestedPath string

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"domain","id":"example.com"}}`))
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

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{Resource: "example.com"})

	if requestedPath != "/domains/example.com" {
		t.Fatalf("requestedPath = %q, want %q", requestedPath, "/domains/example.com")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["used"] != "domain_report" {
		t.Fatalf("used = %v, want domain_report\noutput=%s", result["used"], out)
	}
}

func TestExecuteVirusTotalScanWithOptionsIPResourceUsesIPEndpoint(t *testing.T) {
	var requestedPath string

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		if r.Method != http.MethodGet {
			http.Error(w, "unexpected method", http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"ip_address","id":"8.8.8.8"}}`))
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

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{Resource: "8.8.8.8"})

	if requestedPath != "/ip_addresses/8.8.8.8" {
		t.Fatalf("requestedPath = %q, want %q", requestedPath, "/ip_addresses/8.8.8.8")
	}

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["used"] != "ip_report" {
		t.Fatalf("used = %v, want ip_report\noutput=%s", result["used"], out)
	}
}

func TestExecuteVirusTotalScanWithOptionsURLResourceFallsBackToSubmission(t *testing.T) {
	resource := "https://example.com/download.exe"
	urlID := base64.RawURLEncoding.EncodeToString([]byte(resource))
	var (
		getURLCount      int
		postURLCount     int
		getAnalysisCount int
	)

	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/urls/"+urlID && getURLCount == 0:
			getURLCount++
			http.Error(w, `{"error":{"message":"not found"}}`, http.StatusNotFound)
		case r.Method == http.MethodPost && r.URL.Path == "/urls":
			postURLCount++
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			if got := r.Form.Get("url"); got != resource {
				http.Error(w, "missing url form field", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"analysis","id":"analysis-123"}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/analyses/analysis-123":
			getAnalysisCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"analysis","id":"analysis-123","attributes":{"status":"completed"}}}`))
		case r.Method == http.MethodGet && r.URL.Path == "/urls/"+urlID:
			getURLCount++
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"data":{"type":"url","id":"` + urlID + `"}}`))
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

	out := ExecuteVirusTotalScanWithOptions("vt-key", VirusTotalOptions{Resource: resource})

	var result map[string]interface{}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal output: %v\noutput=%s", err, out)
	}
	if result["status"] != "success" {
		t.Fatalf("status = %v, want success\noutput=%s", result["status"], out)
	}
	if result["used"] != "url_report_after_submission" {
		t.Fatalf("used = %v, want url_report_after_submission\noutput=%s", result["used"], out)
	}
	if _, ok := result["submission"]; !ok {
		t.Fatalf("expected submission payload\noutput=%s", out)
	}
	if _, ok := result["analysis"]; !ok {
		t.Fatalf("expected analysis payload\noutput=%s", out)
	}
	if _, ok := result["result"]; !ok {
		t.Fatalf("expected final URL result payload\noutput=%s", out)
	}
	if postURLCount != 1 {
		t.Fatalf("postURLCount = %d, want 1", postURLCount)
	}
	if getAnalysisCount != 1 {
		t.Fatalf("getAnalysisCount = %d, want 1", getAnalysisCount)
	}
	if getURLCount != 2 {
		t.Fatalf("getURLCount = %d, want 2", getURLCount)
	}
}

func TestExecuteVirusTotalScanWithOptionsHashModeDoesNotUpload(t *testing.T) {
	var (
		lookupCount int
		uploadCount int
	)
	server := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
