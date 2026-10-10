package server

import (
	"aurago/internal/budget"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/tools"
)

func galleryTestServer(t *testing.T) (*Server, string) {
	t.Helper()
	s, dir := testDesktopMediaServer(t)
	var err error
	s.MediaRegistryDB, err = tools.InitMediaRegistryDB(s.Cfg.SQLite.MediaRegistryPath)
	if err != nil {
		t.Fatal(err)
	}
	s.ImageGalleryDB, err = tools.InitImageGalleryDB(s.Cfg.SQLite.ImageGalleryPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.MediaRegistryDB.Close(); s.ImageGalleryDB.Close() })
	if err := os.MkdirAll(filepath.Join(dir, "generated_images"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "generated_images", "cover.png"), append([]byte("\x89PNG\r\n\x1a\n"), make([]byte, 32)...), 0600); err != nil {
		t.Fatal(err)
	}
	return s, dir
}

func galleryRequest(t *testing.T, s *Server, method string, body interface{}) *httptest.ResponseRecorder {
	t.Helper()
	b, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, "/api/desktop/gallery/metadata", bytes.NewReader(b))
	w := httptest.NewRecorder()
	handleDesktopGalleryMetadata(s)(w, r)
	return w
}

func gallerySnapshot(t *testing.T, s *Server) galleryMetadata {
	t.Helper()
	w := galleryRequest(t, s, "GET", nil)
	if w.Code != 200 {
		t.Fatalf("snapshot: %d %s", w.Code, w.Body.String())
	}
	var data struct {
		Items map[string]galleryMetadata `json:"items"`
		Empty galleryMetadata            `json:"empty"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	if m, ok := data.Items["Photos/cover.png"]; ok {
		return m
	}
	return data.Empty
}

func TestDesktopGalleryMetadataLegacyWriteConflictAndAgentRead(t *testing.T) {
	s, _ := galleryTestServer(t)
	_, err := tools.SaveGeneratedImage(s.ImageGalleryDB, &tools.ImageGenResult{Filename: "cover.png", Prompt: "Moon cover", Provider: "fixture", Model: "fixture"})
	if err != nil {
		t.Fatal(err)
	}
	m := gallerySnapshot(t, s)
	if m.Description != "Moon cover" || m.Source != "generate_image" {
		t.Fatalf("legacy metadata: %+v", m)
	}
	body := map[string]interface{}{"path": "Photos/cover.png", "version": m.Version, "description": "Mountain landscape", "tags": []string{"cover", "mountain"}}
	w := galleryRequest(t, s, "PUT", body)
	if w.Code != 200 {
		t.Fatalf("save: %d %s", w.Code, w.Body.String())
	}
	if w = galleryRequest(t, s, "PUT", body); w.Code != 409 {
		t.Fatalf("stale save: %d", w.Code)
	}
	result := tools.DispatchMediaRegistryWithGallery(s.MediaRegistryDB, s.ImageGalleryDB, "", "search", "Mountain landscape", "image", "", nil, "", 0, 20, 0, "", "", "")
	if !strings.Contains(result, "Mountain landscape") || !strings.Contains(result, "mountain") {
		t.Fatal("agent search missing metadata:", result)
	}
	if got := gallerySnapshot(t, s); got.Description != "Mountain landscape" {
		t.Fatalf("saved: %+v", got)
	}
}

func TestDesktopGalleryMetadataPathAndReadonlyBoundaries(t *testing.T) {
	s, _ := galleryTestServer(t)
	m := gallerySnapshot(t, s)
	for _, path := range []string{"Photos/../secret.png", "Photos/sub/../../secret.png", "Photos/C:/secret.png", "Photos/\\secret.png", "Documents/secret.png", "Photos/missing.png"} {
		w := galleryRequest(t, s, "PUT", map[string]interface{}{"path": path, "version": m.Version, "description": "x"})
		if w.Code != 400 && w.Code != 404 {
			t.Fatalf("path %q: %d", path, w.Code)
		}
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	w := galleryRequest(t, s, "PUT", map[string]interface{}{"path": "Photos/cover.png", "version": m.Version})
	if w.Code != 403 {
		t.Fatalf("readonly: %d %s", w.Code, w.Body.String())
	}
	if w := galleryRequest(t, s, "GET", nil); w.Code != 200 {
		t.Fatal("readonly must retain read access")
	}
}

func TestDesktopGalleryVisionUsesProviderAndPreservesTags(t *testing.T) {
	s, _ := galleryTestServer(t)
	_, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "image", Filename: "cover.png", SourceTool: "generate_image", WebPath: "/files/generated_images/cover.png", Prompt: "Cover intent", Tags: []string{"noisemaker", "cover"}})
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		encoded, _ := json.Marshal(body)
		if !strings.Contains(string(encoded), "data:image/png;base64,") || body["model"] != "vision-fixture" {
			t.Error("missing image or configured model")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"choices": []interface{}{map[string]interface{}{"message": map[string]interface{}{"content": `{"description":"A mountain under a blue moon","tags":["mountain","blue","cover"]}`}}}, "usage": map[string]int{"prompt_tokens": 10, "completion_tokens": 20}})
	}))
	defer provider.Close()
	s.Cfg.Vision.BaseURL, s.Cfg.Vision.APIKey, s.Cfg.Vision.Model = provider.URL, "gallery-synthetic-test-key", "vision-fixture"
	m := gallerySnapshot(t, s)
	w := galleryRequest(t, s, "POST", map[string]interface{}{"path": "Photos/cover.png", "version": m.Version})
	if w.Code != 200 {
		t.Fatalf("analyze: %d %s", w.Code, w.Body.String())
	}
	m = gallerySnapshot(t, s)
	if calls != 1 || m.Description != "A mountain under a blue moon" || len(m.Tags) != 4 || m.Tags[0] != "noisemaker" || m.Prompt != "Cover intent" {
		t.Fatalf("analysis metadata: %+v calls=%d", m, calls)
	}
}

func TestDesktopGalleryVisionRejectsInsufficientScope(t *testing.T) {
	s, readToken, _ := testDesktopPermissionServer(t)
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/gallery/metadata", strings.NewReader(`{}`))
	r.Header.Set("Authorization", "Bearer "+readToken)
	w := httptest.NewRecorder()
	handleDesktopGalleryMetadata(s)(w, r)
	if w.Code != 403 {
		t.Fatalf("read token: %d %s", w.Code, w.Body.String())
	}
}

func TestDesktopGalleryVisionDoesNotOverwriteConcurrentEdit(t *testing.T) {
	s, _ := galleryTestServer(t)
	id, _, err := tools.RegisterMedia(s.MediaRegistryDB, tools.MediaItem{MediaType: "image", Filename: "cover.png", SourceTool: "generate_image", WebPath: "/files/generated_images/cover.png", Description: "Original", Tags: []string{"cover"}})
	if err != nil {
		t.Fatal(err)
	}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := tools.UpdateMedia(s.MediaRegistryDB, id, "New user edit", []string{"manual"}); err != nil {
			t.Error(err)
		}
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"description\":\"Stale AI output\",\"tags\":[\"mountain\"]}"}}]}`))
	}))
	defer provider.Close()
	s.Cfg.Vision.BaseURL, s.Cfg.Vision.APIKey = provider.URL, "gallery-conflict-test-key"
	m := gallerySnapshot(t, s)
	w := galleryRequest(t, s, "POST", map[string]interface{}{"path": "Photos/cover.png", "version": m.Version})
	if w.Code != 409 {
		t.Fatalf("concurrent edit: %d %s", w.Code, w.Body.String())
	}
	if m := gallerySnapshot(t, s); m.Description != "New user edit" {
		t.Fatalf("lost user edit: %+v", m)
	}
}

func TestDesktopGalleryVisionBudgetBlocksBeforeProvider(t *testing.T) {
	s, _ := galleryTestServer(t)
	s.Cfg.Budget.Enabled = true
	s.Cfg.Budget.DailyLimitUSD = 0.01
	s.Cfg.Budget.Enforcement = "full"
	s.BudgetTracker = budget.NewTracker(s.Cfg, s.Logger, t.TempDir())
	s.BudgetTracker.RecordCostForCategory("vision", 1)
	m := gallerySnapshot(t, s)
	w := galleryRequest(t, s, "POST", map[string]interface{}{"path": "Photos/cover.png", "version": m.Version})
	if w.Code != 402 {
		t.Fatalf("budget: %d %s", w.Code, w.Body.String())
	}
}
