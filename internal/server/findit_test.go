package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/findit"
	"gopkg.in/yaml.v3"
)

func findItHTTPFixture(t *testing.T) (*Server, func(string, string, any) *httptest.ResponseRecorder) {
	t.Helper()
	s := newBackupCreateTestServer(t)
	s.Cfg.VirtualDesktop.Enabled = true
	s.Cfg.FindIt.Enabled = true
	// Keep backup acceptance entirely inside the disposable instance.
	base := filepath.Dir(s.Cfg.ConfigPath)
	s.Cfg.Directories.PromptsDir = filepath.Join(base, "prompts")
	s.Cfg.Directories.SkillsDir = filepath.Join(base, "skills")
	s.Cfg.Directories.ToolsDir = filepath.Join(base, "tools")
	s.Cfg.Directories.WorkspaceDir = filepath.Join(base, "workspace")
	fixtureConfig, err := yaml.Marshal(map[string]any{
		"server": map[string]any{"port": 1234},
		"directories": map[string]any{
			"data_dir":      s.Cfg.Directories.DataDir,
			"prompts_dir":   s.Cfg.Directories.PromptsDir,
			"skills_dir":    s.Cfg.Directories.SkillsDir,
			"tools_dir":     s.Cfg.Directories.ToolsDir,
			"workspace_dir": s.Cfg.Directories.WorkspaceDir,
		},
		"virtual_desktop": map[string]any{"enabled": true},
		"findit":          map[string]any{"enabled": true},
	})
	if err != nil {
		t.Fatalf("marshal FindIt fixture config: %v", err)
	}
	if err := os.WriteFile(s.Cfg.ConfigPath, fixtureConfig, 0o600); err != nil {
		t.Fatalf("write FindIt fixture config: %v", err)
	}
	s.startFindIt()
	if s.findIt == nil || s.findIt.err != nil {
		t.Fatalf("FindIt init: %+v", s.findIt)
	}
	t.Cleanup(s.closeFindIt)
	call := func(method, path string, body any) *httptest.ResponseRecorder {
		var raw []byte
		if body != nil {
			raw, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, "http://localhost"+findItAPI+path, bytes.NewReader(raw))
		r.Header.Set("Content-Type", "application/json")
		r.Header.Set("Origin", "http://localhost")
		w := httptest.NewRecorder()
		handleFindIt(s).ServeHTTP(w, r)
		return w
	}
	return s, call
}

func TestFindItHTTPPolicyAndReadOnlyQueries(t *testing.T) {
	s, call := findItHTTPFixture(t)
	query := map[string]any{"operation": "search", "entity": "items"}
	if got := call("POST", "/query", query); got.Code != 200 {
		t.Fatalf("query: %d %s", got.Code, got.Body)
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if got := call("POST", "/query", query); got.Code != 200 {
		t.Fatalf("readonly query: %d %s", got.Code, got.Body)
	}
	if got := call("POST", "/command", map[string]any{}); got.Code != 403 {
		t.Fatal("readonly write allowed")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.FindIt.ReadOnly = true
	if got := call("POST", "/command", map[string]any{}); got.Code != 403 {
		t.Fatal("FindIt readonly write allowed")
	}
	s.Cfg.FindIt.Enabled = false
	if got := call("POST", "/query", query); got.Code != 403 {
		t.Fatal("disabled inventory exposed")
	}
	if got := call("GET", "/status", nil); got.Code != 200 {
		t.Fatal("disabled setup status unavailable")
	}
	s.Cfg.Auth.Enabled = true
	if got := call("GET", "/status", nil); got.Code != 401 {
		t.Fatal("unauthenticated status exposed")
	}
}

func TestFindItHTTPOriginAndJSONBoundaries(t *testing.T) {
	s, _ := findItHTTPFixture(t)
	for _, tc := range []struct {
		origin, body string
		want         int
	}{
		{"https://attacker.invalid", `{"operation":"search","entity":"items"}`, 403},
		{"", `{"operation":"search","entity":"items"}`, 403},
		{"http://localhost", `{"operation":"search","entity":"items","sql":"DROP TABLE items"}`, 400},
		{"http://localhost", `{} {}`, 400},
	} {
		r := httptest.NewRequest("POST", "http://localhost"+findItAPI+"/query", strings.NewReader(tc.body))
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		handleFindIt(s).ServeHTTP(w, r)
		if w.Code != tc.want {
			t.Fatalf("origin/body boundary: %d %s", w.Code, w.Body)
		}
	}
}

func TestFindItHTTPSettingsRevisionAndPersistence(t *testing.T) {
	s, call := findItHTTPFixture(t)
	revision := findItConfigRevision(s.Cfg)
	if got := call("POST", "/settings", map[string]any{"enabled": true, "agent_access": true, "expected_version": "stale"}); got.Code != 409 {
		t.Fatal("stale settings accepted")
	}
	got := call("POST", "/settings", map[string]any{"enabled": true, "agent_access": true, "readonly": false, "expected_version": revision})
	if got.Code != 200 {
		t.Fatalf("save: %d %s", got.Code, got.Body)
	}
	if !s.ConfigSnapshot().FindIt.AgentAccess {
		t.Fatal("agent grant not published")
	}
	data, err := os.ReadFile(s.Cfg.ConfigPath)
	if err != nil || !bytes.Contains(data, []byte("agent_access: true")) {
		t.Fatal("FindIt setting not persisted")
	}
}

func uploadFindItFixtureImage(t *testing.T, s *Server) string {
	t.Helper()
	im := image.NewNRGBA(image.Rect(0, 0, 320, 160))
	for y := 0; y < 160; y++ {
		for x := 0; x < 320; x++ {
			im.Set(x, y, color.NRGBA{R: 20, G: 100, B: 120, A: 255})
		}
	}
	var picture bytes.Buffer
	if err := png.Encode(&picture, im); err != nil {
		t.Fatal(err)
	}
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "../private-name.png")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(picture.Bytes())
	_ = writer.Close()
	r := httptest.NewRequest("POST", "http://localhost"+findItAPI+"/images", &body)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	r.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	handleFindIt(s).ServeHTTP(w, r)
	if w.Code != 201 {
		t.Fatalf("upload %d %s", w.Code, w.Body)
	}
	var result map[string]string
	if json.Unmarshal(w.Body.Bytes(), &result) != nil || !findItImageID.MatchString(result["id"]) {
		t.Fatal("invalid image reference")
	}
	return result["id"]
}

func TestFindItPhotoAndBackupRestoreRoundTrip(t *testing.T) {
	s, call := findItHTTPFixture(t)
	imageID := uploadFindItFixtureImage(t, s)
	photo := call("GET", "/images/"+imageID+"/thumbnail", nil)
	if photo.Code != 200 {
		t.Fatalf("thumbnail %d", photo.Code)
	}
	im, err := png.Decode(bytes.NewReader(photo.Body.Bytes()))
	if err != nil || im.Bounds().Dx() > 256 {
		t.Fatal("thumbnail not normalized")
	}
	created := call("POST", "/command", map[string]any{"operation": "create", "entity": "items", "request_id": "fixture-create-item", "data": map[string]any{"name": "Fixture opamp", "unit": "piece", "image_id": imageID}})
	if created.Code != 200 {
		t.Fatalf("create %d %s", created.Code, created.Body)
	}
	var item findit.Item
	if json.Unmarshal(created.Body.Bytes(), &item) != nil || item.ID == "" {
		t.Fatal("missing item ID")
	}
	backup := httptest.NewRecorder()
	handleBackupCreate(s).ServeHTTP(backup, httptest.NewRequest("POST", "/api/backup/create", strings.NewReader(`{}`)))
	if backup.Code != 200 {
		t.Fatalf("backup %d %s", backup.Code, backup.Body)
	}
	destination := newBackupCreateTestServer(t)
	var upload bytes.Buffer
	writer := multipart.NewWriter(&upload)
	part, err := writer.CreateFormFile("file", "findit.ago")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = part.Write(backup.Body.Bytes())
	_ = writer.Close()
	r := httptest.NewRequest("POST", "/api/backup/import", &upload)
	r.Header.Set("Content-Type", writer.FormDataContentType())
	restored := httptest.NewRecorder()
	handleBackupImport(destination).ServeHTTP(restored, r)
	if restored.Code != 200 {
		t.Fatalf("restore %d %s", restored.Code, restored.Body)
	}
	var restoreStatus struct {
		Pending              bool   `json:"pending_restore"`
		AwaitingConfirmation bool   `json:"awaiting_confirmation"`
		RestoreID            string `json:"restore_id"`
	}
	if err := json.Unmarshal(restored.Body.Bytes(), &restoreStatus); err != nil || !restoreStatus.Pending {
		t.Fatalf("restore was not staged: %s", restored.Body)
	}
	if restoreStatus.AwaitingConfirmation {
		confirmRestoreForTest(t, destination, restoreStatus.RestoreID)
	}
	applyPendingRestoreForTest(t, destination.Cfg.ConfigPath)
	store, err := findit.Open(filepath.Join(destination.Cfg.Directories.DataDir, "findit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	result, err := store.Query(context.Background(), findit.QueryRequest{Operation: "get", Entity: "items", ID: item.ID})
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(result)
	if !bytes.Contains(data, []byte(imageID)) {
		t.Fatal("photo reference lost on restore")
	}
	file, err := os.Open(filepath.Join(destination.Cfg.Directories.DataDir, "findit-images", imageID+"_thumb.png"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	imageBytes, _ := io.ReadAll(file)
	if !bytes.Equal(imageBytes, photo.Body.Bytes()) {
		t.Fatal("restored photo differs")
	}
}
