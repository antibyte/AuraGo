package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"aurago/internal/config"
	"aurago/internal/localwiki"
)

func TestDashboardIntegrationFlagsIncludeLocalWikipedia(t *testing.T) {
	cfg := &config.Config{}
	enabled, ok := dashboardIntegrationFlags(cfg)["local_wikipedia"]
	if !ok || enabled {
		t.Fatalf("local_wikipedia flag = %v (present %v), want present and false", enabled, ok)
	}
	cfg.LocalWikipedia.Enabled = true
	if !dashboardIntegrationFlags(cfg)["local_wikipedia"] {
		t.Fatal("local_wikipedia flag must follow local_wikipedia.enabled")
	}
}

func TestDashboardLocalWikipediaSummaryReportsUpdateHint(t *testing.T) {
	cfg := &config.Config{}
	if got := dashboardLocalWikipediaSummary(cfg, nil); got["enabled"] != false || len(got) != 1 {
		t.Fatalf("disabled summary = %#v, want only enabled=false", got)
	}
	cfg.LocalWikipedia.Enabled = true
	if got := dashboardLocalWikipediaSummary(cfg, nil); got["enabled"] != true || len(got) != 1 {
		t.Fatalf("summary without manager = %#v, want only enabled=true", got)
	}
	withUpdate := func() localwiki.Status {
		return localwiki.Status{
			State:           "ready",
			Readable:        true,
			Edition:         &localwiki.Edition{Language: "hi", Variant: localwiki.VariantNoPic, Date: "2026-09"},
			UpdateAvailable: &localwiki.UpdateInfo{Date: "2026-10", Size: 917954595},
		}
	}
	got := dashboardLocalWikipediaSummary(cfg, withUpdate)
	if got["enabled"] != true || got["state"] != "ready" || got["readable"] != true || got["loading"] != false {
		t.Fatalf("summary = %#v", got)
	}
	edition, _ := got["edition"].(map[string]interface{})
	if edition["language"] != "hi" || edition["variant"] != "nopic" || edition["date"] != "2026-09" {
		t.Fatalf("edition = %#v", got["edition"])
	}
	update, _ := got["update_available"].(map[string]interface{})
	if update["date"] != "2026-10" || update["size"] != int64(917954595) {
		t.Fatalf("update_available = %#v", got["update_available"])
	}
	if _, ok := got["error_code"]; ok {
		t.Fatalf("error_code must be absent without an error: %#v", got)
	}
	quiet := func() localwiki.Status { return localwiki.Status{State: "ready", Readable: true} }
	if _, ok := dashboardLocalWikipediaSummary(cfg, quiet)["update_available"]; ok {
		t.Fatal("update_available must be absent when the manager reports no update")
	}
}

// The summary forwards the manager's own readable/loading flags and the stable
// error code; it never re-derives them from the state and never carries the
// English recommendation, which the UI localizes.
func TestDashboardLocalWikipediaSummaryForwardsReadableLoadingAndCode(t *testing.T) {
	cfg := &config.Config{}
	cfg.LocalWikipedia.Enabled = true

	cases := []struct {
		name         string
		status       localwiki.Status
		wantState    string
		wantReadable bool
		wantLoading  bool
		wantCode     string
	}{
		{
			name:      "first load pending",
			status:    localwiki.Status{State: localwiki.StateNotInstalled, Loading: true, ErrorCode: localwiki.CodeBusy, Recommendation: "Wait until the running operation has finished."},
			wantState: localwiki.StateNotInstalled, wantLoading: true, wantCode: localwiki.CodeBusy,
		},
		{
			name: "served edition after a failed update",
			status: localwiki.Status{
				State: localwiki.StateReady, Readable: true,
				Edition:        &localwiki.Edition{Language: "de", Variant: localwiki.VariantNoPic, Date: "2026-10"},
				ErrorCode:      localwiki.CodeDownloadFailed,
				Recommendation: "Check the internet connection and resume the download.",
			},
			wantState: localwiki.StateReady, wantReadable: true, wantCode: localwiki.CodeDownloadFailed,
		},
		{
			name: "unreadable edition",
			status: localwiki.Status{
				State:     localwiki.StateError,
				Edition:   &localwiki.Edition{Language: "de", Variant: localwiki.VariantNoPic, Date: "2026-10"},
				ErrorCode: localwiki.CodeZIMUnreadable, Recommendation: "Delete the edition and install it again.",
			},
			wantState: localwiki.StateError, wantCode: localwiki.CodeZIMUnreadable,
		},
		{
			name:      "unreadable state file",
			status:    localwiki.Status{State: localwiki.StateError, ErrorCode: localwiki.CodeStateUnreadable},
			wantState: localwiki.StateError, wantCode: localwiki.CodeStateUnreadable,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status := tc.status
			got := dashboardLocalWikipediaSummary(cfg, func() localwiki.Status { return status })
			if got["state"] != tc.wantState || got["readable"] != tc.wantReadable || got["loading"] != tc.wantLoading {
				t.Fatalf("summary = %#v, want state %q readable %v loading %v", got, tc.wantState, tc.wantReadable, tc.wantLoading)
			}
			if code, _ := got["error_code"].(string); code != tc.wantCode {
				t.Fatalf("error_code = %#v, want %q", got["error_code"], tc.wantCode)
			}
			if _, ok := got["recommendation"]; ok {
				t.Fatalf("summary must not carry the English recommendation: %#v", got)
			}
		})
	}
}

func TestHandleDashboardOverviewIncludesLocalWikipedia(t *testing.T) {
	cfg := &config.Config{}
	cfg.LocalWikipedia.Enabled = true
	manager := localwiki.NewManager(localwiki.Deps{
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		HTTPClient: &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
			return nil, errors.New("network disabled in test")
		})},
		FreeDiskBytes: func(string) (int64, error) { return 1 << 40, nil },
	})
	manager.Configure(localwiki.Settings{
		Enabled: true, AgentAccess: true, Language: "de", Variant: localwiki.VariantNoPic, DataDir: t.TempDir(),
	})
	t.Cleanup(func() { _ = manager.Shutdown(context.Background()) })
	s := &Server{Cfg: cfg, LocalWiki: manager}

	rec := httptest.NewRecorder()
	handleDashboardOverview(s).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/dashboard/overview", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode overview: %v", err)
	}
	integrations, _ := body["integrations"].(map[string]interface{})
	if integrations["local_wikipedia"] != true {
		t.Fatalf("integrations.local_wikipedia = %#v", integrations["local_wikipedia"])
	}
	summary, ok := body["local_wikipedia"].(map[string]interface{})
	if !ok || summary["enabled"] != true || summary["state"] != "not_installed" {
		t.Fatalf("local_wikipedia summary = %#v", body["local_wikipedia"])
	}
	if summary["readable"] != false || summary["loading"] != false {
		t.Fatalf("a configured, never-started manager is neither readable nor loading: %#v", summary)
	}
	if _, ok := summary["update_available"]; ok {
		t.Fatalf("fresh manager must not report an update: %#v", summary)
	}
}
