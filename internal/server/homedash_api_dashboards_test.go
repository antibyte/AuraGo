package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
	"slices"
	"strings"
	"testing"

	"aurago/internal/config"
	"aurago/internal/homedash"
)

type hdSaved struct {
	Dashboard homedash.Dashboard `json:"dashboard"`
	Warnings  []homedash.Issue   `json:"warnings"`
}

func minimalDoc(name string) map[string]any {
	return map[string]any{"name": name, "pages": []any{map[string]any{"id": "main", "name": "Main"}}}
}

func TestHomeDashDashboardCRUDAndRevisionConflict(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	if w := env.do(t, env.admin, http.MethodGet, "dashboards", nil); w.Code != 200 || !strings.Contains(w.Body.String(), `"dashboards":[]`) {
		t.Fatalf("empty list = %d %s", w.Code, w.Body.String())
	}
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("Living Room")})
	if w.Code != http.StatusCreated || !strings.Contains(w.Body.String(), `"warnings":[]`) {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	var c hdSaved
	hdDecode(t, w, &c)
	d := c.Dashboard
	if d.Slug != "living-room" || d.Revision != 1 || d.ApprovedRevision != 1 || d.SavedBy != "user" || d.Look.Family != "glass" || d.Kiosk == nil {
		t.Fatalf("created = %+v", d)
	}
	path := "dashboard?id=" + d.ID

	if w := env.do(t, env.admin, http.MethodGet, "dashboards", nil); w.Code != 200 {
		t.Fatalf("list = %d", w.Code)
	} else {

		var l struct{ Dashboards []homedash.DashboardSummary }
		hdDecode(t, w, &l)
		if len(l.Dashboards) != 1 || l.Dashboards[0].ID != d.ID {
			t.Fatalf("list = %+v", l)
		}
	}
	if w := env.do(t, env.admin, http.MethodGet, path, nil); w.Code != 200 {
		t.Fatalf("get = %d %s", w.Code, w.Body.String())
	}

	d.Name = "Lounge"
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 1, "document": d})
	var u hdSaved
	hdDecode(t, w, &u)
	if !strings.Contains(w.Body.String(), `"warnings":[]`) {
		t.Fatalf("update warnings not an array: %s", w.Body.String())
	}
	if w.Code != 200 || u.Dashboard.Revision != 2 || u.Dashboard.Name != "Lounge" || u.Dashboard.ApprovedRevision != 2 {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 1, "document": d})
	var conflict struct {
		Code            string `json:"code"`
		CurrentRevision int64  `json:"current_revision"`
	}
	hdDecode(t, w, &conflict)
	if w.Code != http.StatusConflict || conflict.Code != "revision_conflict" || conflict.CurrentRevision != 2 {
		t.Fatalf("stale update = %d %s", w.Code, w.Body.String())
	}
	d.Pages = nil
	w = env.do(t, env.admin, http.MethodPut, path, map[string]any{"revision": 2, "document": d})
	var invalid struct {
		Code   string           `json:"code"`
		Issues []homedash.Issue `json:"issues"`
	}
	hdDecode(t, w, &invalid)
	if w.Code != http.StatusUnprocessableEntity || invalid.Code != "invalid_document" ||
		!slices.ContainsFunc(invalid.Issues, func(i homedash.Issue) bool { return i.Path == "/pages" }) {
		t.Fatalf("invalid update = %d %s", w.Code, w.Body.String())
	}

	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("Living Room")})
	var second hdSaved
	hdDecode(t, w, &second)
	if second.Dashboard.Slug != "living-room-2" {
		t.Fatalf("second slug = %q", second.Dashboard.Slug)
	}
	taken := minimalDoc("Other")
	taken["slug"] = "living-room"
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": taken}); w.Code != http.StatusConflict || hdCode(t, w) != "slug_taken" {
		t.Fatalf("taken slug = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", `{"document":{"name":"X","bogus":1}}`); w.Code != http.StatusUnprocessableEntity || hdCode(t, w) != "invalid_document" {
		t.Fatalf("unknown field = %d %s", w.Code, w.Body.String())
	}

	if w := env.do(t, env.admin, http.MethodDelete, path+"&revision=1", nil); w.Code != http.StatusConflict {
		t.Fatalf("stale delete = %d", w.Code)
	}
	if w := env.do(t, env.admin, http.MethodDelete, path+"&revision=2", nil); w.Code != 200 {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, path, nil); w.Code != http.StatusNotFound || hdCode(t, w) != "dashboard_not_found" {
		t.Fatalf("get deleted = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "dashboard?id=nope", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad id = %d", w.Code)
	}
}

func TestHomeDashStarterTemplate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil) // no source registered: every binding warns entity_unknown
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"})
	if w.Code != http.StatusCreated {
		t.Fatalf("template = %d %s", w.Code, w.Body.String())
	}
	var c hdSaved
	hdDecode(t, w, &c)
	d := c.Dashboard
	assertStarterMatchesContract(t, w.Body.Bytes())
	if d.Name != "System" || d.Slug != "system" || d.Look.Family != "glass" || d.Look.Palette.Mode != "theme" ||
		d.Look.RadiusScale != 1 || !d.Grid.RowUnit.Auto || len(d.Pages) != 1 || d.ApprovedRevision != 1 {
		t.Fatalf("template dashboard = %+v", d)
	}
	assertStarterMatchesContract(t, w.Body.Bytes())
	got := []string{}
	for _, wd := range d.Pages[0].Widgets {
		got = append(got, fmt.Sprintf("%s:%s/%s=%s", wd.ID, wd.Type, wd.Variant, wd.Bindings["primary"]))
	}
	want := []string{"cpu:gauge/arc=system:cpu", "memory:gauge/needle=system:memory", "disk:gauge/arc=system:disk",
		"net-rx:sensor/sparkline=system:net_rx", "net-tx:sensor/sparkline=system:net_tx", "uptime:entity/tile=system:uptime"}
	if !slices.Equal(got, want) {
		t.Fatalf("widgets = %v", got)
	}
	wantLG := []homedash.LayoutItem{{ID: "cpu", X: 0, Y: 0, W: 4, H: 4}, {ID: "memory", X: 4, Y: 0, W: 4, H: 4}, {ID: "disk", X: 8, Y: 0, W: 4, H: 4},
		{ID: "net-rx", X: 0, Y: 4, W: 5, H: 2}, {ID: "net-tx", X: 5, Y: 4, W: 5, H: 2}, {ID: "uptime", X: 10, Y: 4, W: 2, H: 2}}
	if !slices.Equal(d.Pages[0].Layouts.LG, wantLG) {
		t.Fatalf("layout = %+v", d.Pages[0].Layouts.LG)
	}
	if len(c.Warnings) != 6 || c.Warnings[0].Code != "entity_unknown" {
		t.Fatalf("warnings = %+v", c.Warnings)
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system", "name": "Server"})
	hdDecode(t, w, &c)
	if c.Dashboard.Slug != "system-2" || c.Dashboard.Name != "Server" {
		t.Fatalf("second template = %+v", c.Dashboard)
	}
	for _, body := range []any{map[string]any{"template": "nope"}, map[string]any{"template": "system", "document": minimalDoc("A")}, map[string]any{}} {
		if w := env.do(t, env.admin, http.MethodPost, "dashboards", body); w.Code != http.StatusBadRequest {
			t.Fatalf("%v = %d %s", body, w.Code, w.Body.String())
		}
	}
}

func TestHomeDashDashboardWritesRefusedOnReadOnlyDesktop(t *testing.T) {
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true })
	if w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"template": "system"}); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" {
		t.Fatalf("create on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodGet, "dashboards", nil); w.Code != http.StatusOK {
		t.Fatalf("list on a read-only desktop = %d", w.Code)
	}
}

const starterContractJSON = `{"schema_version":1,"slug":"system","name":"System","revision":1,"approved_revision":1,"saved_by":"user",
"look":{"family":"glass","palette":{"mode":"theme"},"backdrop":{"kind":"theme"},"density":"comfortable","radius_scale":1},
"grid":{"row_unit":"auto","gap":null,"fit":"scroll"},
"pages":[{"id":"main","name":"Main","compact":true,
"widgets":[
{"id":"cpu","type":"gauge","variant":"arc","icon":"cpu","bindings":{"primary":"system:cpu"},"options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":80},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
{"id":"memory","type":"gauge","variant":"needle","icon":"memory","bindings":{"primary":"system:memory"},"options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":85},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
{"id":"disk","type":"gauge","variant":"arc","icon":"disk","bindings":{"primary":"system:disk"},"options":{"alert":95,"decimals":0,"max":100,"min":0,"warn":85},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
{"id":"net-rx","type":"sensor","variant":"sparkline","icon":"download","bindings":{"primary":"system:net_rx"},"options":{"decimals":1,"hours":1},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
{"id":"net-tx","type":"sensor","variant":"sparkline","icon":"upload","bindings":{"primary":"system:net_tx"},"options":{"decimals":1,"hours":1},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false},
{"id":"uptime","type":"entity","variant":"tile","icon":"timer","bindings":{"primary":"system:uptime"},"options":{},"actions":{"tap":{"do":"default"},"hold":{"do":"more_info"}},"confirm":false}],
"layouts":{"lg":[{"id":"cpu","x":0,"y":0,"w":4,"h":4},{"id":"memory","x":4,"y":0,"w":4,"h":4},{"id":"disk","x":8,"y":0,"w":4,"h":4},
{"id":"net-rx","x":0,"y":4,"w":5,"h":2},{"id":"net-tx","x":5,"y":4,"w":5,"h":2},{"id":"uptime","x":10,"y":4,"w":2,"h":2}]}}],
"kiosk":{"start_page":"main","idle_dim_seconds":120,"idle_dim_level":0.3,"page_rotation_seconds":0,"screen_saver":"clock","show_clock":true}}`

// assertStarterMatchesContract compares the saved starter dashboard with the frozen contract
// example, field by field, ignoring the server-generated id and updated_at.
func assertStarterMatchesContract(t *testing.T, answer []byte) {
	t.Helper()
	var got struct {
		Dashboard map[string]any `json:"dashboard"`
	}
	if err := json.Unmarshal(answer, &got); err != nil {
		t.Fatal(err)
	}
	var want map[string]any
	if err := json.Unmarshal([]byte(starterContractJSON), &want); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"id", "updated_at"} {
		if _, ok := got.Dashboard[k]; !ok {
			t.Fatalf("dashboard lacks %q", k)
		}
		delete(got.Dashboard, k)
	}
	if !reflect.DeepEqual(got.Dashboard, want) {
		g, _ := json.MarshalIndent(got.Dashboard, "", " ")
		t.Fatalf("starter template differs from the contract example: %s", g)
	}
}

func TestHomeDashDuplicateRestoreApproveRevisionsValidate(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	var c hdSaved
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("A")}), &c)
	d := c.Dashboard
	q := "?id=" + d.ID
	d.Name = "B"
	if w := env.do(t, env.admin, http.MethodPut, "dashboard"+q, map[string]any{"revision": 1, "document": d}); w.Code != 200 {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	var revs struct{ Revisions []homedash.RevisionInfo }
	hdDecode(t, env.do(t, env.admin, http.MethodGet, "dashboard/revisions"+q, nil), &revs)
	if len(revs.Revisions) != 2 || revs.Revisions[0].Revision != 2 || revs.Revisions[1].Revision != 1 {
		t.Fatalf("revisions = %+v", revs)
	}
	w := env.do(t, env.admin, http.MethodPost, "dashboard/restore"+q+"&revision=1", nil)
	var r hdSaved
	hdDecode(t, w, &r)
	if w.Code != 200 || r.Dashboard.Revision != 3 || r.Dashboard.Name != "A" || r.Dashboard.ApprovedRevision != 3 {
		t.Fatalf("restore = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboard/approve"+q+"&revision=2", nil)
	var a hdSaved
	hdDecode(t, w, &a)
	if w.Code != 200 || a.Dashboard.ApprovedRevision != 2 || a.Dashboard.Revision != 3 {
		t.Fatalf("approve = %d %s", w.Code, w.Body.String())
	}
	for _, p := range []string{"dashboard/approve", "dashboard/restore"} {
		if w := env.do(t, env.admin, http.MethodPost, p+q+"&revision=99", nil); w.Code != http.StatusNotFound || hdCode(t, w) != "revision_not_found" {
			t.Fatalf("%s 99 = %d %s", p, w.Code, w.Body.String())
		}
	}
	w = env.do(t, env.admin, http.MethodPost, "dashboard/duplicate"+q, nil)
	var dup hdSaved
	hdDecode(t, w, &dup)
	if w.Code != http.StatusCreated || dup.Dashboard.Name != "A (copy)" || dup.Dashboard.Slug != "a-copy" || dup.Dashboard.Revision != 1 || dup.Dashboard.ID == d.ID {
		t.Fatalf("duplicate = %d %s", w.Code, w.Body.String())
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboard/duplicate"+q, map[string]any{"name": "Second"}), &dup)
	if dup.Dashboard.Name != "Second" || dup.Dashboard.Slug != "a-copy-2" {
		t.Fatalf("named duplicate = %+v", dup.Dashboard)
	}
	var v struct {
		Valid    bool               `json:"valid"`
		Issues   []homedash.Issue   `json:"issues"`
		Document homedash.Dashboard `json:"document"`
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": minimalDoc("V")}), &v)
	if !v.Valid || v.Document.Slug != "v" || v.Document.Kiosk == nil || v.Document.Kiosk.StartPage != "main" {
		t.Fatalf("validate valid = %+v", v)
	}
	hdDecode(t, env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": map[string]any{"name": "V"}}), &v)
	if v.Valid || !slices.ContainsFunc(v.Issues, func(i homedash.Issue) bool { return i.Path == "/pages" }) {
		t.Fatalf("validate invalid = %+v", v)
	}
}

func TestHomeDashValidateWorksOnReadOnlyDesktop(t *testing.T) {
	env := newHomeDashTestEnv(t, func(c *config.Config) { c.VirtualDesktop.ReadOnly = true })
	if w := env.do(t, env.admin, http.MethodPost, "dashboards/validate", map[string]any{"document": minimalDoc("V")}); w.Code != http.StatusOK {
		t.Fatalf("validate on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
	if w := env.do(t, env.admin, http.MethodPost, "dashboard/duplicate?id=d_aaaaaaaaaaaa", nil); w.Code != http.StatusForbidden || hdCode(t, w) != "desktop_readonly" {
		t.Fatalf("duplicate on a read-only desktop = %d %s", w.Code, w.Body.String())
	}
}

func TestHomeDashRestoreHonoursExpectedRevision(t *testing.T) {
	env := newHomeDashTestEnv(t, nil)
	w := env.do(t, env.admin, http.MethodPost, "dashboards", map[string]any{"document": minimalDoc("Hall")})
	var c hdSaved
	hdDecode(t, w, &c)
	d := c.Dashboard
	d.Name = "Hall v2"
	if w := env.do(t, env.admin, http.MethodPut, "dashboard?id="+d.ID, map[string]any{"revision": 1, "document": d}); w.Code != 200 {
		t.Fatalf("update = %d %s", w.Code, w.Body.String())
	}
	base := "dashboard/restore?id=" + d.ID + "&revision=1"
	if w := env.do(t, env.admin, http.MethodPost, base+"&expected_revision=abc", nil); w.Code != http.StatusBadRequest {
		t.Fatalf("bad expected_revision = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPost, base+"&expected_revision=1", nil)
	if w.Code != http.StatusConflict || hdCode(t, w) != "revision_conflict" {
		t.Fatalf("stale restore = %d %s", w.Code, w.Body.String())
	}
	w = env.do(t, env.admin, http.MethodPost, base+"&expected_revision=2", nil)
	var ok hdSaved
	hdDecode(t, w, &ok)
	if w.Code != 200 || ok.Dashboard.Revision != 3 || ok.Dashboard.Name != "Hall" {
		t.Fatalf("matching restore = %d %s", w.Code, w.Body.String())
	}
	// Without expected_revision the restore keeps applying on top of the current head.
	if w := env.do(t, env.admin, http.MethodPost, base, nil); w.Code != 200 {
		t.Fatalf("unconditional restore = %d %s", w.Code, w.Body.String())
	}
}
