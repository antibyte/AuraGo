package server

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestFormatUIBuildVersionIncludesStartupTime(t *testing.T) {
	t.Parallel()

	first := formatUIBuildVersion(time.Date(2026, 5, 31, 10, 0, 1, 0, time.UTC))
	second := formatUIBuildVersion(time.Date(2026, 5, 31, 10, 0, 2, 0, time.UTC))

	if first == "20260531a" {
		t.Fatalf("BuildVersion must include startup time, got legacy date-only value %q", first)
	}
	if first == second {
		t.Fatalf("same-day restarts must produce different cache-busting versions, got %q", first)
	}
	if !strings.HasPrefix(first, "20260531T100001") || !strings.HasSuffix(first, "a") {
		t.Fatalf("BuildVersion = %q, want compact date/time with suffix", first)
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
		`window.AURAGO_BUILD_VERSION = "20260605T123456a"`,
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
