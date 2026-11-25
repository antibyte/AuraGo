package server

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/config"
	"aurago/internal/homedash"
	"aurago/internal/homedash/sourcetest"
)

// homeDashTestEnv is a server with auth on, a desktop:admin and a desktop:read token, and a
// HomeDash service on a temporary homedash.db.
type homeDashTestEnv struct {
	s       *Server
	svc     *homeDashService
	admin   string
	adminID string
	read    string
	audits  *homeDashAuditLog
}

type homeDashAuditLog struct {
	mu      sync.Mutex
	entries []string // "entity|command|outcome"
}

func (l *homeDashAuditLog) record(_ context.Context, cmd homedash.Command, outcome string) {
	l.mu.Lock()
	l.entries = append(l.entries, string(cmd.Entity)+"|"+string(cmd.Name)+"|"+outcome)
	l.mu.Unlock()
}

func (l *homeDashAuditLog) has(entry string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, e := range l.entries {
		if e == entry {
			return true
		}
	}
	return false
}

// fakeHomeDashSpec registers a FakeSource that is enabled and controllable.
func fakeHomeDashSpec(f *sourcetest.FakeSource) homeDashSourceSpec {
	return homeDashSourceSpec{ID: f.ID(), ConfigKey: string(f.ID()),
		Settings: func(*config.Config) homedash.SourceSettings {
			return homedash.SourceSettings{Enabled: true, Control: true}
		},
		Factory: func(homedash.RuntimeConfig) (homedash.Source, bool) { return f, true }}
}

func newHomeDashTestEnv(t *testing.T, mutate func(*config.Config), specs ...homeDashSourceSpec) *homeDashTestEnv {
	t.Helper()
	s, readToken, _ := testDesktopPermissionServer(t)
	admin, meta, err := s.TokenManager.Create("homedash admin", []string{desktopScopeAdmin}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := s.Cfg
	cfg.Directories.DataDir = t.TempDir()
	cfg.VirtualDesktop.Enabled = true
	cfg.HomeDash.Enabled = true
	cfg.HomeDash.Sources.System.Enabled = true
	if err := config.NormalizeHomeDashConfig(&cfg.HomeDash); err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(cfg)
	}
	audits := &homeDashAuditLog{}
	svc, err := newHomeDashService(context.Background(), filepath.Join(cfg.Directories.DataDir, config.HomeDashDBFilename), cfg, homeDashDeps{
		Logger: s.Logger, Sources: specs, Audit: audits.record,
		HubOptions: homedash.Options{CoalesceInterval: 10 * time.Millisecond, CatalogTTL: time.Second, IdleStop: 200 * time.Millisecond},
	})
	if err != nil {
		t.Fatal(err)
	}
	s.homeDash.Store(svc)
	t.Cleanup(s.closeHomeDash)
	if cfg.HomeDash.Enabled && len(specs) > 0 {
		sourcetest.WaitFor(t, 2*time.Second, func() bool {
			for _, st := range svc.hub.Sources() {
				if st.State == homedash.StatusDisabled {
					return false
				}
			}
			return true
		}, "HomeDash configuration applied")
	}
	return &homeDashTestEnv{s: s, svc: svc, admin: admin, adminID: meta.ID, read: readToken, audits: audits}
}

// do calls the HomeDash handler directly. body: nil, a raw JSON string, or a value to marshal.
func (e *homeDashTestEnv) do(t *testing.T, token, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var rd io.Reader
	switch b := body.(type) {
	case nil:
	case string:
		rd = strings.NewReader(b)
	default:
		raw, err := json.Marshal(b)
		if err != nil {
			t.Fatal(err)
		}
		rd = bytes.NewReader(raw)
	}
	r := httptest.NewRequest(method, homeDashAPIPrefix+path, rd)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	handleHomeDash(e.s).ServeHTTP(w, r)
	return w
}

func hdDecode(t *testing.T, w *httptest.ResponseRecorder, dst any) {
	t.Helper()
	if err := json.Unmarshal(w.Body.Bytes(), dst); err != nil {
		t.Fatalf("answer is not JSON: %v\n%s", err, w.Body.String())
	}
}

// hdCode returns "code", or "error" for answers of the shared auth layers.
func hdCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct{ Code, Error string }
	hdDecode(t, w, &body)
	if body.Code != "" {
		return body.Code
	}
	return body.Error
}
