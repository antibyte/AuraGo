package server

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"aurago/internal/config"
)

func TestHandleGetConfigShowsEffectivePersonaBoosterValues(t *testing.T) {
	configPath := filepath.Join(t.TempDir(), "config.yaml")
	raw := "personality:\n  core_personality: punk\n  persona_booster:\n    intensity: wild\n"
	if err := os.WriteFile(configPath, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConfigPath = configPath
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	rec := httptest.NewRecorder()
	handleGetConfig(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d; body=%s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	personality, _ := body["personality"].(map[string]interface{})
	booster, _ := personality["persona_booster"].(map[string]interface{})
	if booster["enabled"] != true || booster["intensity"] != "wild" || booster["spontaneous"] != true || booster["cooldown_minutes"] != float64(20) {
		t.Fatalf("persona_booster view = %#v", booster)
	}
}
