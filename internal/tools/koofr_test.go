package tools

import (
	"aurago/internal/testutil"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
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

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

func TestExecuteKoofrWriteRejectsMissingContent(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var uploadCalled bool
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/content/api/v2/mounts/primary/files/put":
			uploadCalled = true
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "write", "/aurago/pictures", "empty.txt", "", "", t.TempDir())
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "error" {
		t.Fatalf("status = %v, want error; result=%s", parsed["status"], result)
	}
	if !strings.Contains(parsed["message"].(string), "content is required") {
		t.Fatalf("message = %v, want content-required guidance", parsed["message"])
	}
	if uploadCalled {
		t.Fatal("empty write should be rejected before calling Koofr upload endpoint")
	}
}

func TestExecuteKoofrUploadSendsLocalFileBytes(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tmp := t.TempDir()
	workspaceDir := filepath.Join(tmp, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "funny_cat_car.jpeg")
	sourceBytes := []byte{0xff, 0xd8, 0xff, 0xdb, 'j', 'p', 'e', 'g'}
	if err := os.WriteFile(sourcePath, sourceBytes, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var uploadedName string
	var uploadedBytes []byte
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/content/api/v2/mounts/primary/files/put":
			if got := r.URL.Query().Get("path"); got != "/aurago/pictures" {
				t.Errorf("upload path = %q, want /aurago/pictures", got)
			}
			reader, err := r.MultipartReader()
			if err != nil {
				t.Errorf("MultipartReader: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			for {
				part, err := reader.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Errorf("NextPart: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
				if part.FormName() != "content" {
					continue
				}
				uploadedName = part.FileName()
				uploadedBytes, err = io.ReadAll(part)
				if err != nil {
					t.Errorf("ReadAll: %v", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "upload", "/aurago/pictures", "funny_cat_car.jpeg", "", sourcePath, workspaceDir)
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "success" {
		t.Fatalf("status = %v, want success; result=%s", parsed["status"], result)
	}
	if uploadedName != "funny_cat_car.jpeg" {
		t.Fatalf("uploaded filename = %q, want funny_cat_car.jpeg", uploadedName)
	}
	if !bytes.Equal(uploadedBytes, sourceBytes) {
		t.Fatalf("uploaded bytes = %v, want %v", uploadedBytes, sourceBytes)
	}
	if parsed["bytes"].(float64) != float64(len(sourceBytes)) {
		t.Fatalf("reported bytes = %v, want %d", parsed["bytes"], len(sourceBytes))
	}
}

func TestExecuteKoofrListRetriesSlashPathFallback(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var attempted []string
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/api/v2/mounts/primary/files/list":
			attempted = append(attempted, r.URL.Query().Get("path"))
			if r.URL.Query().Get("path") == "/aurgo/" {
				_, _ = w.Write([]byte(`{"files":[]}`))
				return
			}
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "list", "/aurgo", "", "", "", t.TempDir())
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "success" {
		t.Fatalf("status = %v, want success; result=%s", parsed["status"], result)
	}
	if strings.Join(attempted, ",") != "/aurgo,/aurgo/" {
		t.Fatalf("attempted paths = %v, want [/aurgo /aurgo/]", attempted)
	}
}

func TestExecuteKoofrMkdirUsesSlashPathSemantics(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	var parent string
	var folderName string
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/api/v2/mounts/primary/files/folder":
			parent = r.URL.Query().Get("path")
			var payload map[string]string
			_ = json.NewDecoder(r.Body).Decode(&payload)
			folderName = payload["name"]
			w.WriteHeader(http.StatusCreated)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "mkdir", "/aurgo/pictures", "", "", "", t.TempDir())
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "success" {
		t.Fatalf("status = %v, want success; result=%s", parsed["status"], result)
	}
	if parent != "/aurgo" || folderName != "pictures" {
		t.Fatalf("parent/name = %q/%q, want /aurgo/pictures", parent, folderName)
	}
}

func parseKoofrToolJSON(t *testing.T, result string) map[string]interface{} {
	t.Helper()
	const prefix = "Tool Output: "
	if !strings.HasPrefix(result, prefix) {
		t.Fatalf("missing tool prefix: %q", result)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal([]byte(strings.TrimPrefix(result, prefix)), &parsed); err != nil {
		t.Fatalf("json.Unmarshal: %v; result=%s", err, result)
	}
	return parsed
}
