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

func TestResolveKoofrUploadTargetSplitsFilenameFromPathWhenDestinationMissing(t *testing.T) {
	dir, filename := resolveKoofrUploadTarget("/aurgo/pictures/robot_spaghetti.jpeg", "", "img_20260428_220802_6d68e896a2d2.jpeg")
	if dir != "/aurgo/pictures" {
		t.Fatalf("dir = %q, want /aurgo/pictures", dir)
	}
	if filename != "robot_spaghetti.jpeg" {
		t.Fatalf("filename = %q, want robot_spaghetti.jpeg", filename)
	}
}

func TestKoofrListContainsFilenameDetectsMissingUpload(t *testing.T) {
	listResponse := []byte(`{"files":[{"name":"other.jpeg"}]}`)
	if koofrListContainsFilename(listResponse, "missing_after_upload.jpeg") {
		t.Fatal("missing_after_upload.jpeg should not be considered visible")
	}
}

func TestKoofrListContainsFilenameDetectsVisibleUpload(t *testing.T) {
	listResponse := []byte(`{"files":[{"name":"funny_cat_car.jpeg"}]}`)
	if !koofrListContainsFilename(listResponse, "funny_cat_car.jpeg") {
		t.Fatal("funny_cat_car.jpeg should be considered visible")
	}
}

func TestKoofrUploadURLIncludesFilenameAndInfoQuery(t *testing.T) {
	got := koofrUploadURL("https://app.koofr.net", "primary", "/aurgo/pictures", "cat.jpeg")
	if !strings.Contains(got, "path=%2Faurgo%2Fpictures") {
		t.Fatalf("upload URL missing encoded path: %s", got)
	}
	if !strings.Contains(got, "filename=cat.jpeg") {
		t.Fatalf("upload URL missing filename query: %s", got)
	}
	if !strings.Contains(got, "info=true") {
		t.Fatalf("upload URL missing info query: %s", got)
	}
}

func TestKoofrDirectoryPrefixesBuildsParentsInOrder(t *testing.T) {
	got := koofrDirectoryPrefixes("/aurgo/pictures")
	want := []string{"/aurgo", "/aurgo/pictures"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("prefixes = %v, want %v", got, want)
	}
}

func TestKoofrJoinPathBuildsFileInfoPath(t *testing.T) {
	got := koofrJoinPath("/aurgo/pictures", "cat.jpeg")
	if got != "/aurgo/pictures/cat.jpeg" {
		t.Fatalf("joined = %q, want /aurgo/pictures/cat.jpeg", got)
	}
}

func TestKoofrActionRequiresExplicitResultForUpload(t *testing.T) {
	if !koofrActionRequiresExplicitResult("upload") {
		t.Fatal("upload must never fall through to generic empty success")
	}
}

func TestKoofrMovePayloadUsesKoofrClientShape(t *testing.T) {
	payloadBytes := koofrMovePayload("primary", "/aurgo/archive/cat.jpeg")
	var payload map[string]interface{}
	if err := json.Unmarshal(payloadBytes, &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v", err)
	}
	if payload["toMountId"] != "primary" {
		t.Fatalf("toMountId = %v, want primary", payload["toMountId"])
	}
	if payload["toPath"] != "/aurgo/archive/cat.jpeg" {
		t.Fatalf("toPath = %v, want /aurgo/archive/cat.jpeg", payload["toPath"])
	}
	if _, ok := payload["to"]; ok {
		t.Fatalf("payload must not use legacy 'to' field: %v", payload)
	}
	if _, ok := payload["modified"]; ok {
		t.Fatalf("move payload must not include copy-only modified field: %v", payload)
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
			if got := r.URL.Query().Get("filename"); got != "funny_cat_car.jpeg" {
				t.Errorf("upload filename query = %q, want funny_cat_car.jpeg", got)
			}
			if got := r.URL.Query().Get("info"); got != "true" {
				t.Errorf("upload info query = %q, want true", got)
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
				if part.FormName() != "file" {
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
			_, _ = w.Write([]byte(`{"name":"funny_cat_car.jpeg"}`))
		case "/api/v2/mounts/primary/files/list":
			switch got := r.URL.Query().Get("path"); got {
			case "/aurago", "/aurago/pictures":
				_, _ = w.Write([]byte(`{"files":[{"name":"funny_cat_car.jpeg"}]}`))
			default:
				t.Errorf("list path = %q, want /aurago or /aurago/pictures", got)
				w.WriteHeader(http.StatusNotFound)
			}
		case "/api/v2/mounts/primary/files/info":
			if got := r.URL.Query().Get("path"); got != "/aurago/pictures/funny_cat_car.jpeg" {
				t.Errorf("info path = %q, want /aurago/pictures/funny_cat_car.jpeg", got)
			}
			_, _ = w.Write([]byte(`{"name":"funny_cat_car.jpeg"}`))
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
	if uploadedName != "dummy" {
		t.Fatalf("uploaded multipart filename = %q, want dummy", uploadedName)
	}
	if !bytes.Equal(uploadedBytes, sourceBytes) {
		t.Fatalf("uploaded bytes = %v, want %v", uploadedBytes, sourceBytes)
	}
	if parsed["bytes"].(float64) != float64(len(sourceBytes)) {
		t.Fatalf("reported bytes = %v, want %d", parsed["bytes"], len(sourceBytes))
	}
}

func TestExecuteKoofrUploadSplitsFilenameFromPathWhenDestinationMissing(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tmp := t.TempDir()
	workspaceDir := filepath.Join(tmp, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "img_20260428_220802_6d68e896a2d2.jpeg")
	if err := os.WriteFile(sourcePath, []byte{0xff, 0xd8, 0xff, 0xdb}, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	var uploadDir string
	var uploadedName string
	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/content/api/v2/mounts/primary/files/put":
			uploadDir = r.URL.Query().Get("path")
			if got := r.URL.Query().Get("filename"); got != "robot_spaghetti.jpeg" {
				t.Errorf("upload filename query = %q, want robot_spaghetti.jpeg", got)
			}
			reader, err := r.MultipartReader()
			if err != nil {
				t.Errorf("MultipartReader: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			part, err := reader.NextPart()
			if err != nil {
				t.Errorf("NextPart: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			uploadedName = part.FileName()
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case "/api/v2/mounts/primary/files/list":
			_, _ = w.Write([]byte(`{"files":[{"name":"robot_spaghetti.jpeg"}]}`))
		case "/api/v2/mounts/primary/files/info":
			if got := r.URL.Query().Get("path"); got != "/aurgo/pictures/robot_spaghetti.jpeg" {
				t.Errorf("info path = %q, want /aurgo/pictures/robot_spaghetti.jpeg", got)
			}
			_, _ = w.Write([]byte(`{"name":"robot_spaghetti.jpeg"}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "upload", "/aurgo/pictures/robot_spaghetti.jpeg", "", "", sourcePath, workspaceDir)
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "success" {
		t.Fatalf("status = %v, want success; result=%s", parsed["status"], result)
	}
	if uploadDir != "/aurgo/pictures" {
		t.Fatalf("upload path = %q, want /aurgo/pictures", uploadDir)
	}
	if uploadedName != "dummy" {
		t.Fatalf("uploaded multipart filename = %q, want dummy", uploadedName)
	}
}

func TestExecuteKoofrUploadErrorsWhenUploadedFileIsNotListed(t *testing.T) {
	t.Setenv("AURAGO_SSRF_ALLOW_LOOPBACK", "1")

	tmp := t.TempDir()
	workspaceDir := filepath.Join(tmp, "agent_workspace", "workdir")
	if err := os.MkdirAll(workspaceDir, 0o755); err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	sourcePath := filepath.Join(workspaceDir, "missing_after_upload.jpeg")
	if err := os.WriteFile(sourcePath, []byte{0xff, 0xd8, 0xff, 0xdb}, 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	srv := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/mounts":
			_, _ = w.Write([]byte(`{"mounts":[{"id":"primary","isPrimary":true}]}`))
		case "/content/api/v2/mounts/primary/files/put":
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{}`))
		case "/api/v2/mounts/primary/files/list":
			switch got := r.URL.Query().Get("path"); got {
			case "/aurgo":
				_, _ = w.Write([]byte(`{"files":[{"name":"pictures"}]}`))
			case "/aurgo/pictures":
				_, _ = w.Write([]byte(`{"files":[]}`))
			default:
				t.Errorf("list path = %q, want /aurgo or /aurgo/pictures", got)
				w.WriteHeader(http.StatusNotFound)
			}
		case "/api/v2/mounts/primary/files/info":
			w.WriteHeader(http.StatusNotFound)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	result := ExecuteKoofr(KoofrConfig{BaseURL: srv.URL, Username: "user", AppPassword: "pass"}, "upload", "/aurgo/pictures", "missing_after_upload.jpeg", "", sourcePath, workspaceDir)
	parsed := parseKoofrToolJSON(t, result)
	if parsed["status"] != "error" {
		t.Fatalf("status = %v, want error; result=%s", parsed["status"], result)
	}
	if !strings.Contains(parsed["message"].(string), "not visible") {
		t.Fatalf("message = %v, want visibility verification error", parsed["message"])
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
