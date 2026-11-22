package server

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

// chibiAPIFixture is a server with a desktop workspace, Chibi Sensei switched on and a read and a
// read+write token.
func chibiAPIFixture(t *testing.T) (*Server, string, string) {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	writeToken, _, err := s.TokenManager.Create("chibi rw", []string{desktopScopeRead, desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	configureDesktopAppsHandlerTest(t, s)
	s.Logger = slog.Default()
	s.Cfg.ChibiSensei.Enabled = true
	if err := config.NormalizeChibiSenseiConfig(&s.Cfg.ChibiSensei); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.closeChibiSensei)
	return s, readToken, writeToken
}

func chibiRequest(s *Server, token, method, path string, body io.Reader, headers map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, chibiSenseiAPIBase+path, body)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	handleChibiSensei(s).ServeHTTP(w, r)
	return w
}

func chibiErrorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, w.Body.String())
	}
	return body.Code
}

func TestChibiSenseiCapabilityAndGates(t *testing.T) {
	s, readToken, _ := chibiAPIFixture(t)
	c := desktopCapabilities{s}
	if !c.HasCapability("chibi_sensei") {
		t.Fatal("chibi_sensei capability is off although the feature is on")
	}
	if w := chibiRequest(s, "", http.MethodGet, "/meta", nil, nil); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous /meta = %d, want 401", w.Code)
	}
	if w := chibiRequest(s, readToken, http.MethodGet, "/meta", nil, nil); w.Code != http.StatusOK {
		t.Fatalf("/meta = %d %s", w.Code, w.Body.String())
	}

	cfg := *s.ConfigSnapshot()
	cfg.ChibiSensei.Enabled = false
	s.replaceConfigSnapshot(&cfg)
	if c.HasCapability("chibi_sensei") {
		t.Fatal("capability is on although the feature is off")
	}
	if w := chibiRequest(s, readToken, http.MethodGet, "/meta", nil, nil); w.Code != http.StatusForbidden || chibiErrorCode(t, w) != "chibi_disabled" {
		t.Fatalf("disabled /meta = %d %s", w.Code, w.Body.String())
	}

	cfg.ChibiSensei.Enabled = true
	cfg.VirtualDesktop.Enabled = false
	s.replaceConfigSnapshot(&cfg)
	if c.HasCapability("chibi_sensei") {
		t.Fatal("capability is on without the virtual desktop")
	}
	if w := chibiRequest(s, readToken, http.MethodGet, "/meta", nil, nil); w.Code != http.StatusServiceUnavailable || chibiErrorCode(t, w) != "desktop_disabled" {
		t.Fatalf("no-desktop /meta = %d %s", w.Code, w.Body.String())
	}

	cfg.VirtualDesktop.Enabled = true
	cfg.VirtualDesktop.ReadOnly = true
	s.replaceConfigSnapshot(&cfg)
	w := chibiRequest(s, readToken, http.MethodGet, "/meta", nil, nil)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `"readonly":true`) {
		t.Fatalf("read-only /meta = %d %s", w.Code, w.Body.String())
	}

	cancel := httptest.NewRequest(http.MethodPost, chibiSenseiAPIBase+"/jobs/11111111-1111-4111-8111-111111111111/cancel", nil)
	if desktopRequestOperation(cancel) != desktopStop {
		t.Error("job cancel is not a desktopStop operation")
	}
	if w := chibiRequest(s, readToken, http.MethodGet, "/nope", nil, nil); w.Code != http.StatusNotFound {
		t.Errorf("unknown endpoint = %d, want 404", w.Code)
	}
}

func TestChibiSenseiMetaLimits(t *testing.T) {
	s, readToken, _ := chibiAPIFixture(t)
	cfg := *s.ConfigSnapshot()
	cfg.ChibiSensei.Dataset.MaxRows = 20000
	s.replaceConfigSnapshot(&cfg)
	s.LLMClient = nil
	w := chibiRequest(s, readToken, http.MethodGet, "/meta", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("/meta = %d %s", w.Code, w.Body.String())
	}
	var meta struct {
		Readonly bool `json:"readonly"`
		Limits   struct {
			MaxRows     int `json:"max_rows"`
			MaxMessages int `json:"max_messages"`
			PageMax     int `json:"page_max"`
		} `json:"limits"`
		Recipes []struct {
			ID        string   `json:"id"`
			TaskKinds []string `json:"task_kinds"`
		} `json:"recipes"`
		Profiles  []any `json:"profiles"`
		Generator struct {
			Available bool `json:"available"`
		} `json:"generator"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
		t.Fatal(err)
	}
	if meta.Limits.MaxRows != 20000 || meta.Limits.MaxMessages != 64 || meta.Limits.PageMax != 200 {
		t.Fatalf("limits = %+v", meta.Limits)
	}
	if len(meta.Recipes) != 3 || strings.Join(meta.Recipes[1].TaskKinds, " ") != "classify extract rewrite to_json" {
		t.Fatalf("recipes = %+v", meta.Recipes)
	}
	if len(meta.Profiles) != 3 {
		t.Fatalf("profiles = %v, want the three catalog profiles", meta.Profiles)
	}
	if meta.Generator.Available {
		t.Fatal("generator available without an LLM client")
	}
}

// R1: /meta reports whether the caller is an administrator.
func TestChibiSenseiMetaAdminField(t *testing.T) {
	s, readToken, _ := chibiAPIFixture(t)
	admin := func(token string) bool {
		t.Helper()
		w := chibiRequest(s, token, http.MethodGet, "/meta", nil, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("/meta = %d %s", w.Code, w.Body.String())
		}
		var meta map[string]any
		if err := json.Unmarshal(w.Body.Bytes(), &meta); err != nil {
			t.Fatal(err)
		}
		v, present := meta["admin"].(bool)
		if !present {
			t.Fatalf("/meta has no admin field: %s", w.Body.String())
		}
		return v
	}
	if admin(readToken) {
		t.Error("a desktop-read token is reported as admin")
	}
	cfg := *s.ConfigSnapshot()
	cfg.Auth.Enabled = false
	s.replaceConfigSnapshot(&cfg)
	if !admin("") {
		t.Error("the built-in admin (auth disabled) is not reported as admin")
	}
}

// R2: a runtime that was replaced or shut down closes its store only after its last holder let go,
// and its job runner stops before that.
func TestChibiSenseiRuntimeOutlivesDataDirChange(t *testing.T) {
	s, _, _ := chibiAPIFixture(t)
	ctx := context.Background()
	old, releaseOld, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	cfg := *s.ConfigSnapshot()
	cfg.Directories.DataDir = filepath.Join(t.TempDir(), "data2")
	s.replaceConfigSnapshot(&cfg)
	fresh, releaseFresh, err := s.acquireChibiSensei()
	if err != nil {
		t.Fatal(err)
	}
	if fresh == old {
		t.Fatal("a changed data directory kept the old runtime")
	}
	if _, err := old.store.ListProjects(ctx, false); err != nil {
		t.Fatalf("the old store was closed while a handler held it: %v", err)
	}
	select {
	case <-old.retiring:
	default:
		t.Fatal("the old runtime was not retired")
	}
	if _, err := old.jobs.Start(ctx, nil, "p", "export", nil, 0, nil); err == nil {
		t.Fatal("the job runner of a retired runtime still accepts jobs")
	}
	releaseOld()
	select {
	case <-old.drained:
	case <-time.After(2 * time.Second):
		t.Fatal("the old store stayed open after its last holder let go")
	}
	if _, err := old.store.ListProjects(ctx, false); err == nil {
		t.Fatal("the old store is still open")
	}

	done := make(chan struct{})
	go func() { s.closeChibiSensei(); close(done) }()
	select {
	case <-done:
		t.Fatal("shutdown closed the store while a handler held it")
	case <-time.After(100 * time.Millisecond):
	}
	if _, err := fresh.store.ListProjects(ctx, false); err != nil {
		t.Fatalf("the store was closed while a handler held it: %v", err)
	}
	releaseFresh()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("shutdown did not finish after the last holder let go")
	}
	if _, err := fresh.store.ListProjects(ctx, false); err == nil {
		t.Fatal("the store is still open after shutdown")
	}
	if _, _, err := s.acquireChibiSensei(); err == nil {
		t.Fatal("a runtime was handed out after shutdown")
	}
}

func TestChibiSenseiConfigValidationRejectsBadValuesBeforeWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.yaml")
	original := []byte("server:\n  host: 127.0.0.1\nchibi_sensei:\n  enabled: true\n")
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = path
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	patch := `{"chibi_sensei":{"dataset":{"max_rows":5}}}`
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(patch), &m); err != nil {
		t.Fatal(err)
	}
	if _, err := applyConfigPatch(s, m); err == nil || !strings.Contains(err.Error(), "chibi_sensei:") {
		t.Fatalf("setup patch: %v", err)
	}
	rec := httptest.NewRecorder()
	handleUpdateConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodPut, "/api/config", strings.NewReader(patch)))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "chibi_sensei:") {
		t.Fatalf("config save: %d %s", rec.Code, rec.Body.String())
	}
	stored, err := os.ReadFile(path)
	if err != nil || string(stored) != string(original) {
		t.Fatalf("a rejected patch changed the config file: %v", err)
	}
}
