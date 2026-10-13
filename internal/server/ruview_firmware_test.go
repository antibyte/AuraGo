package server

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/ruview"
)

func TestRuViewFirmwareResponsesPreserveFlashOffsets(t *testing.T) {
	s, _, _ := testDesktopPermissionServer(t)
	s.Cfg.RuView = config.DefaultRuViewConfig()
	s.RuView = ruview.New(s.ConfigSnapshot, nil, slog.New(slog.NewTextHandler(io.Discard, nil)))
	defer s.RuView.Close()
	mux := http.NewServeMux()
	registerRuViewRoutes(mux, s)
	desktopHandler := desktopIntegrationHandler(s, mux)
	admin, _, err := s.TokenManager.Create("firmware catalog fixture", []string{desktopScopeAdmin, "admin"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := ruview.Artifacts()
	if len(want.Firmware) == 0 {
		t.Fatal("firmware catalog is empty")
	}
	for _, prefix := range []string{"/api/ruview/", "/api/desktop/integrations/ruview/"} {
		for _, route := range []string{"firmware", "capabilities"} {
			t.Run(prefix+route, func(t *testing.T) {
				r := httptest.NewRequest(http.MethodGet, "http://aurago.test"+prefix+route, nil)
				r.Header.Set("Authorization", "Bearer "+admin)
				w := httptest.NewRecorder()
				if prefix == "/api/ruview/" {
					mux.ServeHTTP(w, r)
				} else {
					desktopHandler(w, r)
				}
				if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
					t.Fatalf("catalog response: %d %s", w.Code, w.Body.String())
				}
				body := w.Body.Bytes()
				if route == "capabilities" {
					var envelope struct{ Catalog json.RawMessage }
					if err := json.Unmarshal(body, &envelope); err != nil {
						t.Fatal(err)
					}
					body = envelope.Catalog
				}
				var got struct {
					Firmware []struct {
						ID    string
						Parts []struct {
							Name   string
							Offset *int64
						}
					}
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatal(err)
				}
				if len(got.Firmware) != len(want.Firmware) {
					t.Fatalf("got %d profiles, want %d", len(got.Firmware), len(want.Firmware))
				}
				for i, profile := range got.Firmware {
					expected := want.Firmware[i]
					if profile.ID != expected.ID || len(profile.Parts) != len(expected.Parts) {
						t.Fatalf("profile %s differs from catalog %s", profile.ID, expected.ID)
					}
					for j, part := range profile.Parts {
						if part.Name != expected.Parts[j].Name || part.Offset == nil || *part.Offset != expected.Parts[j].Offset {
							t.Errorf("%s/%s: explicit offset %d missing or changed in API response", profile.ID, part.Name, expected.Parts[j].Offset)
						}
					}
				}
			})
		}
	}
}
