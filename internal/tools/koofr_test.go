package tools

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestKoofrSuccessContentResponseEscapesStructuredContent(t *testing.T) {
	raw := []byte("line1\n\"quoted\"\tend")
	result := koofrSuccessContentResponse(raw)
	if !strings.HasPrefix(result, "Tool Output: ") {
		t.Fatalf("unexpected prefix: %q", result)
	}
	var parsed map[string]string
	if err := json.Unmarshal([]byte(strings.TrimPrefix(result, "Tool Output: ")), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if parsed["status"] != "success" {
		t.Fatalf("status = %q, want success", parsed["status"])
	}
	if parsed["content"] != string(raw) {
		t.Fatalf("content = %q, want %q", parsed["content"], string(raw))
	}
}

func TestDoKoofrRequestRejectsOversizeResponse(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(bytes.Repeat([]byte("x"), int(maxHTTPResponseSize+1)))
	}))
	defer srv.Close()

	_, err := doKoofrRequest(http.MethodGet, srv.URL, "user", "pass", "", nil)
	if err == nil {
		t.Fatal("expected oversize response error")
	}
	if !strings.Contains(err.Error(), "exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClassifyKoofrContentRejectsBinaryAudio(t *testing.T) {
	contentType, isText := classifyKoofrContent([]byte("ID3\x03\x00\x00binary"))
	if isText {
		t.Fatal("expected binary audio sample to be rejected as text")
	}
	if contentType != "audio/mpeg" {
		t.Fatalf("contentType = %q, want audio/mpeg", contentType)
	}
}

func TestResolveKoofrDownloadDestinationSupportsWorkdirAlias(t *testing.T) {
	workspaceDir := filepath.Join(t.TempDir(), "agent_workspace", "workdir")
	got, err := resolveKoofrDownloadDestination(workspaceDir, "/workdir/audio/song.mp3")
	if err != nil {
		t.Fatalf("resolveKoofrDownloadDestination: %v", err)
	}
	want := filepath.Join(workspaceDir, "audio", "song.mp3")
	if got != want {
		t.Fatalf("resolved = %q, want %q", got, want)
	}
}
