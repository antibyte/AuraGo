package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/webassets"
	"aurago/ui"
)

func init() { uiFiles = ui.Content }

func TestUIBuildVersionUsesAssetIdentity(t *testing.T) {
	if uiBuildVersion != webassets.SetID {
		t.Fatal("UI identity must come from the release asset pin")
	}
}

func TestConfigRouteInjectsBuildVersion(t *testing.T) {
	oldBuildVersion := uiBuildVersion
	uiBuildVersion = "20260605T123456a"
	t.Cleanup(func() { uiBuildVersion = oldBuildVersion })

	cfg := &config.Config{}
	cfg.WebConfig.Enabled = true
	cfg.Server.UILanguage = "en"
	s := &Server{Cfg: cfg, Logger: slog.Default()}
	mux := http.NewServeMux()
	if _, err := s.registerUIRoutes(mux, make(chan struct{})); err != nil {
		t.Fatalf("register UI routes: %v", err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/config", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /config status = %d, body: %s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	for _, want := range []string{
		`/js/shared/shared-core.js?v=20260605T123456a`,
		`"buildVersion":"20260605T123456a"`,
		`/cfg/form-builder.js?v=20260605T123456a`,
		`/js/config/main.js?v=20260605T123456a`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("rendered /config is missing BuildVersion marker %q", want)
		}
	}
	if strings.Contains(body, `?v={{.BuildVersion}}`) || strings.Contains(body, `?v="`) {
		t.Fatalf("rendered /config contains an unexpanded or empty BuildVersion: %s", body)
	}
}
