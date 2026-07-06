package server

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/config"
)

func TestHandleTrueNASPoolsHidesClientInitDetails(t *testing.T) {
	t.Parallel()

	cfg := &config.Config{}
	cfg.TrueNAS.Enabled = true

	s := &Server{
		Cfg:    cfg,
		Logger: slog.Default(),
	}

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/pools", nil)
	rec := httptest.NewRecorder()

	handleTrueNASPools(s).ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusInternalServerError, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Failed to initialize TrueNAS client") || strings.Contains(strings.ToLower(body), "host is required") {
		t.Fatalf("expected generic TrueNAS init error, got %q", body)
	}
}

func TestRegisterTrueNASHandlersRoutesDatasetDelete(t *testing.T) {
	t.Parallel()

	upstreamCalled := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		if r.Method != http.MethodDelete {
			t.Fatalf("upstream method = %s, want DELETE", r.Method)
		}
		if r.URL.EscapedPath() != "/api/v2.0/pool/dataset/id/tank%2Fmedia" {
			t.Fatalf("upstream path = %q, want encoded dataset path", r.URL.EscapedPath())
		}
		if r.URL.Query().Get("recursive") != "true" {
			t.Fatalf("recursive query = %q, want true", r.URL.Query().Get("recursive"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	s := newTrueNASTestServer(t, upstream.URL)
	s.Cfg.TrueNAS.AllowDestructive = true

	mux := http.NewServeMux()
	registerTrueNASHandlers(mux, s)

	req := httptest.NewRequest(http.MethodDelete, "/api/truenas/datasets/tank%2Fmedia?recursive=true", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !upstreamCalled {
		t.Fatal("expected upstream TrueNAS delete request")
	}
}

func TestHandleTrueNASPoolScrubRequiresPostAndAllowDestructive(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	t.Run("blocks allow_destructive false", func(t *testing.T) {
		s := newTrueNASTestServer(t, upstream.URL)
		s.Cfg.TrueNAS.AllowDestructive = false

		req := httptest.NewRequest(http.MethodPost, "/api/truenas/pools/1/scrub", nil)
		rec := httptest.NewRecorder()
		handleTrueNASPoolDetail(s).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
		}
	})

	t.Run("rejects non-post methods", func(t *testing.T) {
		s := newTrueNASTestServer(t, upstream.URL)
		s.Cfg.TrueNAS.AllowDestructive = true

		req := httptest.NewRequest(http.MethodGet, "/api/truenas/pools/1/scrub", nil)
		rec := httptest.NewRecorder()
		handleTrueNASPoolDetail(s).ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
		}
	})
}

func TestHandleTrueNASSMBShareDeleteRequiresDeleteAndAllowDestructive(t *testing.T) {
	t.Parallel()

	upstreamCalls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls++
		w.WriteHeader(http.StatusOK)
	}))
	defer upstream.Close()

	t.Run("blocks allow_destructive false", func(t *testing.T) {
		s := newTrueNASTestServer(t, upstream.URL)
		s.Cfg.TrueNAS.AllowDestructive = false

		req := httptest.NewRequest(http.MethodDelete, "/api/truenas/shares/smb/7", nil)
		rec := httptest.NewRecorder()
		handleTrueNASSMBShareDetail(s).ServeHTTP(rec, req)

		if rec.Code != http.StatusForbidden {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusForbidden, rec.Body.String())
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
		}
	})

	t.Run("rejects non-delete methods", func(t *testing.T) {
		s := newTrueNASTestServer(t, upstream.URL)
		s.Cfg.TrueNAS.AllowDestructive = true

		req := httptest.NewRequest(http.MethodGet, "/api/truenas/shares/smb/7", nil)
		rec := httptest.NewRecorder()
		handleTrueNASSMBShareDetail(s).ServeHTTP(rec, req)

		if rec.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
		}
		if upstreamCalls != 0 {
			t.Fatalf("upstream calls = %d, want 0", upstreamCalls)
		}
	})
}

func TestTrueNASCollectionHandlersRejectUnsupportedMethods(t *testing.T) {
	t.Parallel()

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatalf("unsupported method should not reach upstream: %s %s", r.Method, r.URL.Path)
	}))
	defer upstream.Close()

	tests := []struct {
		name    string
		handler http.HandlerFunc
		path    string
	}{
		{"datasets", handleTrueNASDatasets(newTrueNASTestServer(t, upstream.URL)), "/api/truenas/datasets"},
		{"snapshots", handleTrueNASSnapshots(newTrueNASTestServer(t, upstream.URL)), "/api/truenas/snapshots"},
		{"smb", handleTrueNASSMBShares(newTrueNASTestServer(t, upstream.URL)), "/api/truenas/shares/smb"},
		{"nfs", handleTrueNASNFSShares(newTrueNASTestServer(t, upstream.URL)), "/api/truenas/shares/nfs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPatch, tt.path, nil)
			rec := httptest.NewRecorder()
			tt.handler.ServeHTTP(rec, req)
			if rec.Code != http.StatusMethodNotAllowed {
				t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusMethodNotAllowed, rec.Body.String())
			}
		})
	}
}

func TestHandleTrueNASSnapshotsIncludesAgeHours(t *testing.T) {
	t.Parallel()

	created := time.Now().Add(-2 * time.Hour).UnixMilli()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2.0/pool/snapshot" {
			t.Fatalf("upstream path = %q, want snapshot list path", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"name":"tank/media@manual","dataset":"tank/media","snapshot_name":"manual","rawcreation":{"$date":` + strconv.FormatInt(created, 10) + `}}]`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/truenas/snapshots", nil)
	rec := httptest.NewRecorder()
	handleTrueNASSnapshots(newTrueNASTestServer(t, upstream.URL)).ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", rec.Code, http.StatusOK, rec.Body.String())
	}

	var body struct {
		Snapshots []struct {
			Name     string `json:"name"`
			AgeHours int    `json:"age_hours"`
		} `json:"snapshots"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Snapshots) != 1 || body.Snapshots[0].Name != "tank/media@manual" {
		t.Fatalf("unexpected snapshots response: %+v", body.Snapshots)
	}
	if body.Snapshots[0].AgeHours < 1 || body.Snapshots[0].AgeHours > 3 {
		t.Fatalf("age_hours = %d, want about 2", body.Snapshots[0].AgeHours)
	}
}

func newTrueNASTestServer(t *testing.T, upstreamURL string) *Server {
	t.Helper()

	u, err := url.Parse(upstreamURL)
	if err != nil {
		t.Fatalf("parse upstream URL: %v", err)
	}
	host, portString, err := net.SplitHostPort(u.Host)
	if err != nil {
		t.Fatalf("split upstream host: %v", err)
	}
	port, err := strconv.Atoi(portString)
	if err != nil {
		t.Fatalf("parse upstream port: %v", err)
	}

	cfg := &config.Config{}
	cfg.TrueNAS = config.TrueNASConfig{
		Enabled:          true,
		Host:             host,
		Port:             port,
		UseHTTPS:         false,
		APIKey:           "test-key",
		AllowDestructive: true,
	}

	return &Server{
		Cfg:    cfg,
		Logger: slog.Default(),
	}
}
