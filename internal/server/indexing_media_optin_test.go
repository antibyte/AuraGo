package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/memory"
	"aurago/internal/services"
	"aurago/internal/testutil"

	"gopkg.in/yaml.v3"
)

type mediaOptInFixture struct {
	s          *Server
	cfg        *config.Config
	configPath string
	docs       string
	photos     string
}

// newMediaOptInFixture serves two configured directories from a real config file.
func newMediaOptInFixture(t *testing.T) *mediaOptInFixture {
	t.Helper()
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	initial := "indexing:\n  enabled: true\n  directories:\n    - path: ./docs\n      collection: docs\n    - path: ./photos\n"
	if err := os.WriteFile(configPath, []byte(initial), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	f := &mediaOptInFixture{
		configPath: configPath,
		docs:       filepath.Join(tmp, "docs"),
		photos:     filepath.Join(tmp, "photos"),
	}
	f.cfg = &config.Config{ConfigPath: configPath}
	f.cfg.Indexing.Enabled = true
	f.cfg.Indexing.Directories = []config.IndexingDirectory{
		{Path: f.docs, Collection: "docs"},
		{Path: f.photos},
	}
	f.s = newIndexingHandlerTestServer(f.cfg)
	return f
}

func (f *mediaOptInFixture) serve(t *testing.T, method, rawBody string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/indexing/directories", strings.NewReader(rawBody))
	rec := httptest.NewRecorder()
	handleIndexingDirectories(f.s).ServeHTTP(rec, req)
	return rec
}

func (f *mediaOptInFixture) serveJSON(t *testing.T, method string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	if err != nil {
		t.Fatalf("marshal body: %v", err)
	}
	return f.serve(t, method, string(raw))
}

func (f *mediaOptInFixture) persisted(t *testing.T) []config.IndexingDirectory {
	t.Helper()
	data, err := os.ReadFile(f.configPath)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}
	var doc struct {
		Indexing struct {
			Directories []config.IndexingDirectory `yaml:"directories"`
		} `yaml:"indexing"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("parse persisted config: %v\n%s", err, data)
	}
	return doc.Indexing.Directories
}

func (f *mediaOptInFixture) runtimeFlags() map[string]bool {
	f.s.CfgMu.RLock()
	defer f.s.CfgMu.RUnlock()
	flags := map[string]bool{}
	for _, d := range f.cfg.Indexing.Directories {
		flags[filepath.Base(d.Path)] = d.AllowMediaAnalysis
	}
	return flags
}

func persistedFlags(dirs []config.IndexingDirectory) map[string]bool {
	flags := map[string]bool{}
	for _, d := range dirs {
		flags[filepath.Base(d.Path)] = d.AllowMediaAnalysis
	}
	return flags
}

func TestHandleIndexingDirectoriesGetReportsMediaOptIn(t *testing.T) {
	f := newMediaOptInFixture(t)
	f.cfg.Indexing.Directories[1].AllowMediaAnalysis = true

	decode := func() (dirs []map[string]any, available bool) {
		rec := f.serve(t, http.MethodGet, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("GET status = %d, body=%s", rec.Code, rec.Body.String())
		}
		var body struct {
			Directories    []map[string]any `json:"directories"`
			MediaAvailable *bool            `json:"media_analysis_available"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
			t.Fatalf("decode GET: %v", err)
		}
		if body.MediaAvailable == nil {
			t.Fatalf("GET lacks media_analysis_available: %s", rec.Body.String())
		}
		return body.Directories, *body.MediaAvailable
	}

	dirs, available := decode()
	if available {
		t.Fatal("media_analysis_available = true while index_images and multimodal are off")
	}
	if len(dirs) != 2 {
		t.Fatalf("directories = %v", dirs)
	}
	// The untagged names stay as the UI reads them; the flag is reported for every directory.
	if dirs[0]["Path"] != f.docs || dirs[0]["Collection"] != "docs" || dirs[0]["allow_media_analysis"] != false {
		t.Fatalf("first directory = %v", dirs[0])
	}
	if dirs[1]["Path"] != f.photos || dirs[1]["allow_media_analysis"] != true {
		t.Fatalf("second directory = %v", dirs[1])
	}

	f.cfg.Indexing.IndexImages = true
	if _, available = decode(); !available {
		t.Fatal("media_analysis_available = false although indexing.index_images is on")
	}
	f.cfg.Indexing.IndexImages = false
	f.cfg.Embeddings.Multimodal = true
	if _, available = decode(); !available {
		t.Fatal("media_analysis_available = false although embeddings.multimodal is on")
	}
}

func TestHandleIndexingDirectoriesPatchTogglesMediaOptIn(t *testing.T) {
	f := newMediaOptInFixture(t)

	// A relative path resolves against the config directory, like for POST and DELETE.
	rec := f.serveJSON(t, http.MethodPatch, map[string]any{"path": "./photos", "allow_media_analysis": true})
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"status":"ok"}` {
		t.Fatalf("PATCH on: status=%d body=%s", rec.Code, rec.Body.String())
	}
	want := map[string]bool{"docs": false, "photos": true}
	if got := f.runtimeFlags(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime flags = %v, want %v", got, want)
	}
	persisted := f.persisted(t)
	if got := persistedFlags(persisted); !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted flags = %v, want %v", got, want)
	}
	if persisted[0].Collection != "docs" {
		t.Fatalf("collection of the untouched directory was lost: %+v", persisted[0])
	}

	// Adding and removing other directories rewrites the list but keeps the opt-in.
	rec = f.serveJSON(t, http.MethodPost, map[string]string{"path": "./extra"})
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got, want := persistedFlags(f.persisted(t)), map[string]bool{"docs": false, "photos": true, "extra": false}; !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted flags after POST = %v, want %v", got, want)
	}
	rec = f.serveJSON(t, http.MethodDelete, map[string]string{"path": "./extra"})
	if rec.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := persistedFlags(f.persisted(t)); !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted flags after DELETE = %v, want %v", got, want)
	}

	// An absolute path matches too, and turning the opt-in off removes the key again.
	rec = f.serveJSON(t, http.MethodPatch, map[string]any{"path": f.photos, "allow_media_analysis": false})
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH off: status=%d body=%s", rec.Code, rec.Body.String())
	}
	off := map[string]bool{"docs": false, "photos": false}
	if got := f.runtimeFlags(); !reflect.DeepEqual(got, off) {
		t.Fatalf("runtime flags after off = %v, want %v", got, off)
	}
	data, err := os.ReadFile(f.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "allow_media_analysis") {
		t.Fatalf("a disabled opt-in must not stay in the config file:\n%s", data)
	}
}

func TestHandleIndexingDirectoriesPatchRejectsBadRequestsAndKeepsState(t *testing.T) {
	f := newMediaOptInFixture(t)
	before, err := os.ReadFile(f.configPath)
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		body string
		want int
	}{
		{"invalid JSON", `{"path": `, http.StatusBadRequest},
		{"empty path", `{"path": "  ", "allow_media_analysis": true}`, http.StatusBadRequest},
		{"missing flag", `{"path": "./photos"}`, http.StatusBadRequest},
		{"unknown directory", `{"path": "./nowhere", "allow_media_analysis": true}`, http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if rec := f.serve(t, http.MethodPatch, tc.body); rec.Code != tc.want {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, tc.want, rec.Body.String())
			}
		})
	}
	after, err := os.ReadFile(f.configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(before, after) {
		t.Fatalf("a rejected request changed the config file:\n%s", after)
	}
	if got := f.runtimeFlags(); !reflect.DeepEqual(got, map[string]bool{"docs": false, "photos": false}) {
		t.Fatalf("a rejected request changed the runtime flags: %v", got)
	}

	// The runtime config follows only a successful save.
	f.cfg.ConfigPath = filepath.Join(filepath.Dir(f.configPath), "missing", "config.yaml")
	rec := f.serveJSON(t, http.MethodPatch, map[string]any{"path": f.photos, "allow_media_analysis": true})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("persist failure status = %d, want 500; body=%s", rec.Code, rec.Body.String())
	}
	if got := f.runtimeFlags(); got["photos"] {
		t.Fatal("runtime opt-in changed although saving the config failed")
	}
}

func TestHandleIndexingDirectoriesPostIgnoresSmuggledMediaOptIn(t *testing.T) {
	f := newMediaOptInFixture(t)

	rec := f.serve(t, http.MethodPost, `{"path": "./inbox", "collection": "inbox", "allow_media_analysis": true, "AllowMediaAnalysis": true}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("POST status = %d body=%s", rec.Code, rec.Body.String())
	}
	if got := f.runtimeFlags()["inbox"]; got {
		t.Fatal("POST enabled media analysis for the new directory at runtime")
	}
	persisted := f.persisted(t)
	if len(persisted) != 3 || persisted[2].Collection != "inbox" || persisted[2].AllowMediaAnalysis {
		t.Fatalf("persisted directories = %+v, want the new one with the opt-in off", persisted)
	}
	data, _ := os.ReadFile(f.configPath)
	if strings.Contains(string(data), "allow_media_analysis") {
		t.Fatalf("POST wrote the opt-in to the config file:\n%s", data)
	}
}

// PATCH is a state-changing method: the auth middleware must demand a session and
// a matching Origin before the handler runs.
func TestIndexingDirectoriesPatchIsBehindAuthAndCSRFChecks(t *testing.T) {
	secret := "0123456789abcdef0123456789abcdef"
	s := &Server{Cfg: &config.Config{}, Logger: slog.Default()}
	s.Cfg.Auth.Enabled = true
	s.Cfg.Auth.SessionSecret = secret
	s.Cfg.Auth.PasswordHash = "configured"
	reached := 0
	handler := authMiddleware(s, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		reached++
		w.WriteHeader(http.StatusNoContent)
	}))

	newRequest := func(origin string, session bool) *http.Request {
		req := httptest.NewRequest(http.MethodPatch, "https://aurago.test/api/indexing/directories", strings.NewReader(`{}`))
		req.Host = "aurago.test"
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		if session {
			req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(secret, time.Now().Add(time.Hour))})
		}
		return req
	}
	for _, tc := range []struct {
		name    string
		req     *http.Request
		want    int
		reaches bool
	}{
		{"anonymous", newRequest("https://aurago.test", false), http.StatusUnauthorized, false},
		{"foreign origin", newRequest("https://foreign.test", true), http.StatusForbidden, false},
		{"same origin session", newRequest("https://aurago.test", true), http.StatusNoContent, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := reached
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, tc.req)
			if rec.Code != tc.want || (reached-before == 1) != tc.reaches {
				t.Fatalf("status=%d reached=%d, want status=%d reaches=%v", rec.Code, reached-before, tc.want, tc.reaches)
			}
		})
	}
}

// Two entries for the same path are one folder for the user: PATCH sets the flag on all of
// them, so a duplicate cannot keep the old permission alive.
func TestHandleIndexingDirectoriesPatchAppliesToEveryEntryOfThePath(t *testing.T) {
	f := newMediaOptInFixture(t)
	f.cfg.Indexing.Directories = append(f.cfg.Indexing.Directories, config.IndexingDirectory{Path: f.photos, Collection: "twin", AllowMediaAnalysis: true})
	initial := "indexing:\n  enabled: true\n  directories:\n    - path: ./docs\n    - path: ./photos\n    - path: ./photos\n      collection: twin\n      allow_media_analysis: true\n"
	if err := os.WriteFile(f.configPath, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}

	for _, allow := range []bool{true, false} {
		rec := f.serveJSON(t, http.MethodPatch, map[string]any{"path": "./photos", "allow_media_analysis": allow})
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH %v: status=%d body=%s", allow, rec.Code, rec.Body.String())
		}
		f.s.CfgMu.RLock()
		runtimeDirs := append([]config.IndexingDirectory(nil), f.cfg.Indexing.Directories...)
		f.s.CfgMu.RUnlock()
		for name, dirs := range map[string][]config.IndexingDirectory{"runtime": runtimeDirs, "persisted": f.persisted(t)} {
			if len(dirs) != 3 {
				t.Fatalf("PATCH %v: %s directories = %+v, want 3 entries", allow, name, dirs)
			}
			if dirs[0].AllowMediaAnalysis || dirs[1].AllowMediaAnalysis != allow || dirs[2].AllowMediaAnalysis != allow {
				t.Fatalf("PATCH %v: %s flags = %v/%v/%v, want false/%v/%v", allow, name,
					dirs[0].AllowMediaAnalysis, dirs[1].AllowMediaAnalysis, dirs[2].AllowMediaAnalysis, allow, allow)
			}
		}
	}
}

// A directory edit reads the list inside the save lock. Without that, a PATCH that raced a
// DELETE would persist its older copy and write the deleted entry (and its permission) back.
func TestIndexingDirectoryEditsStartFromTheListInsideTheSaveLock(t *testing.T) {
	f := newMediaOptInFixture(t)

	f.s.CfgSaveMu.Lock() // a concurrent writer is in the middle of its read-modify-persist cycle
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		done <- f.serve(t, http.MethodPatch, `{"path": "./photos", "allow_media_analysis": true}`)
	}()
	time.Sleep(100 * time.Millisecond) // the PATCH is now waiting for the lock

	// That writer is a DELETE of ./docs and finishes first.
	if err := os.WriteFile(f.configPath, []byte("indexing:\n  enabled: true\n  directories:\n    - path: ./photos\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	f.s.CfgMu.Lock()
	f.cfg.Indexing.Directories = []config.IndexingDirectory{{Path: f.photos}}
	f.s.CfgMu.Unlock()
	f.s.CfgSaveMu.Unlock()

	select {
	case rec := <-done:
		if rec.Code != http.StatusOK {
			t.Fatalf("PATCH status = %d, body=%s", rec.Code, rec.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("PATCH did not finish after the save lock was released")
	}
	want := map[string]bool{"photos": true}
	if got := persistedFlags(f.persisted(t)); !reflect.DeepEqual(got, want) {
		t.Fatalf("persisted directories = %v, want %v: the PATCH wrote its stale copy back", got, want)
	}
	if got := f.runtimeFlags(); !reflect.DeepEqual(got, want) {
		t.Fatalf("runtime directories = %v, want %v", got, want)
	}
}

func tinyServerTestJPEG(t *testing.T) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 12, G: 34, B: 56, A: 255})
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatalf("encode jpeg: %v", err)
	}
	return buf.Bytes()
}

// Every other config save publishes a new snapshot through replaceConfigSnapshot. The running
// indexer must read that snapshot, or the permission revoked afterwards would keep sending media.
func TestIndexingMediaOptInRevocationReachesTheRunningIndexerAfterASnapshotReplacement(t *testing.T) {
	tmp := t.TempDir()
	configPath := filepath.Join(tmp, "config.yaml")
	if err := os.WriteFile(configPath, []byte("indexing:\n  enabled: true\n  directories:\n    - path: ./photos\n      allow_media_analysis: true\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	photos := filepath.Join(tmp, "photos")
	if err := os.MkdirAll(photos, 0o755); err != nil {
		t.Fatal(err)
	}
	photoPath := filepath.Join(photos, "holiday.jpg")
	if err := os.WriteFile(photoPath, tinyServerTestJPEG(t), 0o644); err != nil {
		t.Fatal(err)
	}

	var visionCalls atomic.Int32
	vision := testutil.NewHTTPServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		visionCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"a described image"}}],"usage":{"prompt_tokens":1,"completion_tokens":2}}`))
	}))
	t.Cleanup(vision.Close)

	cfg := &config.Config{ConfigPath: configPath}
	cfg.Indexing.Enabled = true
	cfg.Indexing.Directories = []config.IndexingDirectory{{Path: photos, AllowMediaAnalysis: true}}
	cfg.Indexing.Extensions = []string{".txt"}
	cfg.Indexing.IndexImages = true
	cfg.Directories.WorkspaceDir = photos // the Vision tool only reads images from the workspace
	cfg.Vision.BaseURL = vision.URL
	cfg.Vision.APIKey = "vision-key"
	cfg.Vision.Model = "vision-model"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	stm, err := memory.NewSQLiteMemory(":memory:", logger)
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	vdb := &knowledgeUploadVectorDB{}
	s := &Server{Cfg: cfg, Logger: logger, ShortTermMem: stm, LongTermMem: vdb}
	s.initConfigSnapshot()
	fi := services.NewFileIndexer(cfg, &s.CfgMu, vdb, stm, logger)
	s.setFileIndexer(fi)

	// An unrelated config save (a provider key, the vault, ...) publishes a new snapshot.
	s.CfgMu.Lock()
	s.replaceConfigSnapshot(cfg.Clone())
	s.CfgMu.Unlock()

	req := httptest.NewRequest(http.MethodPatch, "/api/indexing/directories", strings.NewReader(`{"path": "./photos", "allow_media_analysis": false}`))
	rec := httptest.NewRecorder()
	handleIndexingDirectories(s).ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH status = %d, body=%s", rec.Code, rec.Body.String())
	}

	// The PATCH starts a rescan; wait until it finished.
	deadline := time.Now().Add(10 * time.Second)
	for fi.Status().LastScanAt.IsZero() {
		if time.Now().After(deadline) {
			t.Fatal("the rescan after the PATCH did not finish")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if calls := visionCalls.Load(); calls != 0 {
		t.Fatalf("vision calls after the revocation = %d, want 0: the indexer kept reading the replaced snapshot", calls)
	}
	_, err = fi.IndexFile(context.Background(), config.IndexingDirectory{Path: photos}, photoPath, nil)
	if !errors.Is(err, services.ErrNoIndexableContent) {
		t.Fatalf("IndexFile error = %v, want ErrNoIndexableContent", err)
	}
	if calls := visionCalls.Load(); calls != 0 {
		t.Fatalf("vision calls after IndexFile = %d, want 0", calls)
	}
}
