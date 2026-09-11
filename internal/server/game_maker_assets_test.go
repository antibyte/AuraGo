package server

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/gamemaker"
)

func TestGameMakerAssetPackRoutes(t *testing.T) {
	root := t.TempDir()
	svc, err := gamemaker.NewService(gamemaker.Options{DBPath: filepath.Join(root, "gm.db"), WorkspacePath: filepath.Join(root, "workspace"), Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()
	s := &Server{Cfg: &config.Config{}, GameMaker: svc}
	for _, tc := range []struct {
		path   string
		status int
		mime   string
	}{
		{"", 200, "application/json"}, {"/space-shooter/sheet.json", 200, "application/json"}, {"/space-shooter/sheet.png", 200, "image/png"},
		{"/aurago-effects/manifest.json", 200, "application/json"}, {"/aurago-sounds/sounds/rifle.wav", 200, "audio/wav"},
		{"/aurago-sounds/sources.json", 404, ""}, {"/aurago-sounds/sounds/unknown.wav", 404, ""}, {"/runtime/aurago-effects-3d-1.js", 200, "text/javascript"},
		{"/unknown/sheet.png", 404, ""}, {"/space-shooter/../sheet.png", 404, ""}, {"/production/manifest.json", 404, ""}, {"/space-shooter/source.png", 404, ""},
	} {
		path := "/api/game-maker/asset-packs" + tc.path
		if isAuthBypassed(path) {
			t.Fatalf("asset route bypasses authentication: %s", path)
		}
		w := httptest.NewRecorder()
		handleGameMakerAssetPacks(s)(w, httptest.NewRequest(http.MethodGet, path, nil))
		if w.Code != tc.status {
			t.Fatalf("%s: status %d body %s", path, w.Code, w.Body.String())
		}
		if tc.mime != "" && !strings.HasPrefix(w.Header().Get("Content-Type"), tc.mime) {
			t.Fatalf("%s: MIME %s", path, w.Header().Get("Content-Type"))
		}
	}
	s.Cfg.Auth.Enabled = true
	w := httptest.NewRecorder()
	handleGameMakerAssetPacks(s)(w, httptest.NewRequest(http.MethodGet, "/api/game-maker/asset-packs", nil))
	if w.Code != 401 && w.Code != 403 {
		t.Fatalf("unauthenticated catalog: %d", w.Code)
	}
	s.Cfg.Auth.Enabled = false
	svc.UpdatePolicy(gamemaker.Policy{})
	w = httptest.NewRecorder()
	handleGameMakerAssetPacks(s)(w, httptest.NewRequest(http.MethodGet, "/api/game-maker/asset-packs", nil))
	if w.Code != 403 {
		t.Fatalf("disabled catalog: %d", w.Code)
	}
}
