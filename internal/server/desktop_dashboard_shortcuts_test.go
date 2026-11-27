package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"aurago/internal/desktop"
)

type dashShortcutEnv struct {
	*homeDashTestEnv
	desk  *desktop.Service
	hub   *desktop.Hub
	write string
}

// newDashShortcutEnv puts a real desktop service on a HomeDash test server.
// Dashboards named in seed are created before a read-only desktop locks the HomeDash API.
func newDashShortcutEnv(t *testing.T, readOnly bool, seed ...string) (*dashShortcutEnv, []string) {
	t.Helper()
	env := newHomeDashTestEnv(t, nil)
	s := env.s
	var ids []string
	for _, name := range seed {
		w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc(name)})
		if w.Code != http.StatusCreated {
			t.Fatalf("create = %d %s", w.Code, w.Body.String())
		}
		var c hdSaved
		hdDecode(t, w, &c)
		ids = append(ids, c.Dashboard.ID)
	}
	s.Cfg.VirtualDesktop.ReadOnly = readOnly
	s.Cfg.VirtualDesktop.WorkspaceDir = filepath.Join(t.TempDir(), "desktop")
	s.Cfg.SQLite.VirtualDesktopPath = filepath.Join(t.TempDir(), "desktop.db")
	svc, err := desktop.NewService(desktop.ConfigFromAuraConfig(s.Cfg))
	if err != nil {
		t.Fatalf("NewService: %v", err)
	}
	svc.SetIntegritySecretStore(s.Vault)
	if err := svc.Init(context.Background()); err != nil {
		t.Fatalf("Init: %v", err)
	}
	t.Cleanup(func() { _ = svc.Close() })
	norm := svc.Config()
	s.Cfg.VirtualDesktop.WorkspaceDir = norm.WorkspaceDir
	s.Cfg.SQLite.VirtualDesktopPath = norm.DBPath
	s.Cfg.Tools.DocumentCreator.OutputDir = norm.DocumentDir
	s.Cfg.SQLite.MediaRegistryPath = norm.MediaRegistryPath
	s.Cfg.SQLite.ImageGalleryPath = norm.ImageGalleryPath
	s.DesktopService = svc
	s.DesktopHub = desktop.NewHub(8)
	t.Cleanup(s.DesktopHub.Close)
	write, _, err := s.TokenManager.Create("dash write", []string{desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	return &dashShortcutEnv{homeDashTestEnv: env, desk: svc, hub: s.DesktopHub, write: write}, ids
}

func (e *dashShortcutEnv) post(t *testing.T, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(http.MethodPost, "/api/desktop/shortcuts", strings.NewReader(string(raw)))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handleDesktopShortcuts(e.s).ServeHTTP(w, r)
	return w
}

func (e *dashShortcutEnv) bootstrap(t *testing.T, token string) desktop.BootstrapPayload {
	t.Helper()
	r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	handleDesktopBootstrap(e.s).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("bootstrap = %d %s", w.Code, w.Body.String())
	}
	var p desktop.BootstrapPayload
	hdDecode(t, w, &p)
	return p
}

// dashboardShortcuts reads the stored rows (not the projection).
func (e *dashShortcutEnv) storedDashboardShortcuts(t *testing.T) []desktop.Shortcut {
	t.Helper()
	p, err := e.desk.Bootstrap(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var out []desktop.Shortcut
	for _, sc := range p.Shortcuts {
		if sc.TargetType == desktop.ShortcutTargetDashboard {
			out = append(out, sc)
		}
	}
	return out
}

// drainEvents returns the shortcut operations broadcast so far.
func drainShortcutOps(ch <-chan desktop.Event) []string {
	var ops []string
	for {
		select {
		case ev := <-ch:
			if ev.Type != "desktop_changed" {
				continue
			}
			pl, _ := ev.Payload.(map[string]interface{})
			op, _ := pl["operation"].(string)
			id, _ := pl["shortcut_id"].(string)
			if _, hasName := pl["name"]; hasName {
				op += "+name"
			}
			ops = append(ops, op+":"+id)
		case <-time.After(60 * time.Millisecond):
			return ops
		}
	}
}

func TestDesktopShortcutsPostDashboard(t *testing.T) {
	cases := []struct {
		name     string
		readOnly bool
		token    func(e *dashShortcutEnv) string
		body     func(id string) map[string]any
		prepare  func(e *dashShortcutEnv)
		want     int
	}{
		{name: "ok", token: func(e *dashShortcutEnv) string { return e.admin }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id, "name": "ignored"} }, want: 200},
		{name: "malformed id", token: func(e *dashShortcutEnv) string { return e.admin }, body: func(string) map[string]any { return map[string]any{"dashboard_id": "../x"} }, want: 400},
		{name: "unknown", token: func(e *dashShortcutEnv) string { return e.admin }, body: func(string) map[string]any { return map[string]any{"dashboard_id": "d_aaaaaaaaaaaa"} }, want: 404},
		{name: "both ids", token: func(e *dashShortcutEnv) string { return e.admin }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id, "app_id": "files"} }, want: 400},
		{name: "read token", token: func(e *dashShortcutEnv) string { return e.read }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id} }, want: 403},
		{name: "write token", token: func(e *dashShortcutEnv) string { return e.write }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id} }, want: 403},
		{name: "homedash disabled", token: func(e *dashShortcutEnv) string { return e.admin }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id} },
			prepare: func(e *dashShortcutEnv) { e.s.Cfg.HomeDash.Enabled = false }, want: 503},
		{name: "read-only desktop", readOnly: true, token: func(e *dashShortcutEnv) string { return e.admin }, body: func(id string) map[string]any { return map[string]any{"dashboard_id": id} }, want: 403},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, ids := newDashShortcutEnv(t, tc.readOnly, "Wohnzimmer")
			id := ids[0]
			events, cancel, err := e.hub.Subscribe()
			if err != nil {
				t.Fatal(err)
			}
			defer cancel()
			if tc.prepare != nil {
				tc.prepare(e)
			}
			w := e.post(t, tc.token(e), tc.body(id))
			if w.Code != tc.want {
				t.Fatalf("status = %d, want %d: %s", w.Code, tc.want, w.Body.String())
			}
			rows := e.storedDashboardShortcuts(t)
			if tc.want != 200 {
				if len(rows) != 0 {
					t.Fatalf("a failed add left rows: %+v", rows)
				}
				if ops := drainShortcutOps(events); len(ops) != 0 {
					t.Fatalf("a failed add broadcast %v", ops)
				}
				return
			}
			var ans struct {
				Status   string           `json:"status"`
				Shortcut desktop.Shortcut `json:"shortcut"`
			}
			hdDecode(t, w, &ans)
			sc := ans.Shortcut
			if ans.Status != "ok" || sc.ID != "dashboard-"+id || sc.TargetType != "dashboard" || sc.TargetID != id || sc.Name != "Wohnzimmer" || sc.Icon != "homedash" {
				t.Fatalf("answer = %s", w.Body.String())
			}
			if w2 := e.post(t, e.admin, map[string]any{"dashboard_id": id}); w2.Code != 200 || len(e.storedDashboardShortcuts(t)) != 1 {
				t.Fatalf("second add = %d, rows %d", w2.Code, len(e.storedDashboardShortcuts(t)))
			}
			ops := drainShortcutOps(events)
			if len(ops) < 1 || ops[0] != "add_shortcut:dashboard-"+id {
				t.Fatalf("events = %v", ops)
			}
		})
	}
}

func TestDesktopBootstrapProjectsDashboardShortcuts(t *testing.T) {
	e, ids := newDashShortcutEnv(t, false, "Keep", "Gone")
	keep, gone := ids[0], ids[1]
	for _, id := range []string{keep, gone} {
		if w := e.post(t, e.admin, map[string]any{"dashboard_id": id}); w.Code != 200 {
			t.Fatalf("add = %d %s", w.Code, w.Body.String())
		}
	}
	names := func(p desktop.BootstrapPayload) map[string]string {
		out := map[string]string{}
		for _, sc := range p.Shortcuts {
			if sc.TargetType == desktop.ShortcutTargetDashboard {
				out[sc.TargetID] = sc.Name
			}
		}
		return out
	}
	// A name changed behind the desktop's back shows live; the stored row keeps the old one.
	if _, err := e.svc.store.DashboardNames(context.Background(), []string{keep}); err != nil {
		t.Fatal(err)
	}
	d, err := e.svc.store.GetDashboard(context.Background(), keep)
	if err != nil {
		t.Fatal(err)
	}
	d.Name = "Renamed"
	if _, err := e.svc.store.UpdateDashboard(context.Background(), d, d.Revision, "user"); err != nil {
		t.Fatal(err)
	}
	if got := names(e.bootstrap(t, e.admin)); got[keep] != "Renamed" || got[gone] != "Gone" {
		t.Fatalf("admin names = %v", got)
	}
	if stored := e.storedDashboardShortcuts(t); len(stored) != 2 || stored[0].Name == "Renamed" && stored[1].Name == "Renamed" {
		t.Fatalf("stored rows were rewritten: %+v", stored)
	}
	// The cached slice is never written to, also under concurrent requests.
	var wg sync.WaitGroup
	for i := 0; i < 6; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			p, err := e.desk.Bootstrap(context.Background())
			if err != nil {
				return
			}
			r := httptest.NewRequest(http.MethodGet, "/api/desktop/bootstrap", nil)
			r.Header.Set("Authorization", "Bearer "+e.admin)
			e.s.projectDashboardShortcuts(r, &p)
		}()
	}
	wg.Wait()
	for _, sc := range e.storedDashboardShortcuts(t) {
		if sc.Name == "Renamed" {
			t.Fatalf("projection mutated the cached slice: %+v", sc)
		}
	}
	// A deleted dashboard disappears from the projection while its row stays.
	if _, err := e.svc.store.DeleteDashboard(context.Background(), gone, 1); err != nil {
		t.Fatal(err)
	}
	if got := names(e.bootstrap(t, e.admin)); len(got) != 1 || got[keep] != "Renamed" {
		t.Fatalf("after delete = %v", got)
	}
	if len(e.storedDashboardShortcuts(t)) != 2 {
		t.Fatal("the stored row of a deleted dashboard was touched by the read path")
	}
	// Non-admin tokens never see dashboard shortcuts; the list stays an array.
	for _, tok := range []string{e.read} {
		p := e.bootstrap(t, tok)
		if len(names(p)) != 0 || p.Shortcuts == nil {
			t.Fatalf("non-admin shortcuts = %+v", p.Shortcuts)
		}
	}
	// HomeDash off: omitted for everyone.
	e.s.Cfg.HomeDash.Enabled = false
	if got := names(e.bootstrap(t, e.admin)); len(got) != 0 {
		t.Fatalf("disabled = %v", got)
	}
	e.s.Cfg.HomeDash.Enabled = true
	// A missing store drops them too.
	saved := e.s.homeDash.Swap(nil)
	if got := names(e.bootstrap(t, e.admin)); len(got) != 0 {
		t.Fatalf("no store = %v", got)
	}
	e.s.homeDash.Store(saved)
	// A store error keeps the stored name.
	if err := e.svc.store.Close(); err != nil {
		t.Fatal(err)
	}
	if got := names(e.bootstrap(t, e.admin)); len(got) != 2 {
		t.Fatalf("store error must keep the rows, got %v", got)
	}
}

func TestHomeDashSaveAndDeleteSyncDesktopShortcut(t *testing.T) {
	e, ids := newDashShortcutEnv(t, false, "Alpha", "Beta")
	id, other := ids[0], ids[1]
	if w := e.post(t, e.admin, map[string]any{"dashboard_id": id}); w.Code != 200 {
		t.Fatalf("add = %d", w.Code)
	}
	events, cancel, err := e.hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	put := func(dashID, name string, rev int64) *httptest.ResponseRecorder {
		w := e.do(t, e.admin, http.MethodGet, "dashboard?id="+dashID, nil)
		var got struct {
			Dashboard map[string]any `json:"dashboard"`
		}
		hdDecode(t, w, &got)
		got.Dashboard["name"] = name
		return e.do(t, e.admin, http.MethodPut, "dashboard?id="+dashID, map[string]any{"revision": rev, "document": got.Dashboard})
	}
	if w := put(id, "Alpha 2", 1); w.Code != 200 {
		t.Fatalf("put = %d %s", w.Code, w.Body.String())
	}
	if ops := drainShortcutOps(events); len(ops) != 1 || ops[0] != "update_shortcut:dashboard-"+id {
		t.Fatalf("rename events = %v", ops)
	}
	if rows := e.storedDashboardShortcuts(t); len(rows) != 1 || rows[0].Name != "Alpha 2" {
		t.Fatalf("stored = %+v", rows)
	}
	if w := put(id, "Alpha 2", 2); w.Code != 200 {
		t.Fatalf("same-name put = %d", w.Code)
	}
	if ops := drainShortcutOps(events); len(ops) != 0 {
		t.Fatalf("same name broadcast %v", ops)
	}
	if w := put(other, "Beta 2", 1); w.Code != 200 {
		t.Fatalf("unpinned put = %d", w.Code)
	}
	if ops := drainShortcutOps(events); len(ops) != 0 {
		t.Fatalf("unpinned dashboard broadcast %v", ops)
	}
	// Restore revision 1 ("Alpha") syncs the name back.
	if w := e.do(t, e.admin, http.MethodPost, "dashboard/restore?id="+id+"&revision=1", nil); w.Code != 200 {
		t.Fatalf("restore = %d %s", w.Code, w.Body.String())
	}
	if ops := drainShortcutOps(events); len(ops) != 1 || ops[0] != "update_shortcut:dashboard-"+id {
		t.Fatalf("restore events = %v", ops)
	}
	if rows := e.storedDashboardShortcuts(t); len(rows) != 1 || rows[0].Name != "Alpha" {
		t.Fatalf("after restore = %+v", rows)
	}
	w := e.do(t, e.admin, http.MethodGet, "dashboard?id="+id, nil)
	var cur hdSaved
	hdDecode(t, w, &cur)
	if w := e.do(t, e.admin, http.MethodDelete, "dashboard?id="+id+"&revision="+itoa64(cur.Dashboard.Revision), nil); w.Code != 200 {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if ops := drainShortcutOps(events); len(ops) != 1 || ops[0] != "remove_shortcut:dashboard-"+id {
		t.Fatalf("delete events = %v", ops)
	}
	if rows := e.storedDashboardShortcuts(t); len(rows) != 0 {
		t.Fatalf("row survived delete: %+v", rows)
	}
}

func itoa64(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}

func TestLiveDesktopServiceNeverCreates(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if svc, hub := env.s.liveDesktopService(); svc != nil || hub != nil {
		t.Fatal("liveDesktopService created or returned a service")
	}
	id := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("X")})
	if id.Code != http.StatusCreated {
		t.Fatalf("create = %d", id.Code)
	}
	if env.s.DesktopService != nil {
		t.Fatal("a HomeDash save created a desktop service")
	}
	if _, err := os.Stat("virtual_desktop"); err == nil {
		t.Fatal("a virtual_desktop directory appeared in the package directory")
	}
}
