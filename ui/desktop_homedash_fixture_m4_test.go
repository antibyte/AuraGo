package ui

// M4 additions to the HomeDash fixture (plan 2026-11-26-homedash-m4-designer.md, contract C18): PUT dashboard with the revision check (409
// revision_conflict + current_revision), POST dashboards/validate, POST dashboard/duplicate, entity filters and paging, a lint that mirrors the
// server's strict decoder (unknown field -> 422 invalid_document/bad_json) and the checks a designer can get wrong (ids, names, required bindings,
// lg layout), scripted PUT failures (answers, dropped connections, delay) and a foreign save that sends dashboard_changed.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

func init() {
	homeDashRoutes["dashboard"][http.MethodPut] = true
	homeDashRoutes["dashboards/validate"] = map[string]bool{http.MethodPost: false}
	homeDashRoutes["dashboard/duplicate"] = map[string]bool{http.MethodPost: true}
}

type homeDashPut struct {
	ID       string
	Revision int
	Document map[string]any
	At       time.Time
}
type homeDashSaveAnswer struct {
	status int
	body   map[string]any
	times  int
}
type homeDashWidgetSpec struct {
	Type     string `json:"type"`
	Variants []struct {
		ID string `json:"id"`
	} `json:"variants"`
	Bindings []struct {
		Name     string `json:"name"`
		Required bool   `json:"required"`
	} `json:"bindings"`
}
type homeDashM4 struct {
	answers []*homeDashSaveAnswer
	drops   int
	delay   time.Duration
	puts    []homeDashPut
	types   map[string]homeDashWidgetSpec
}

var homeDashDocKeys = map[string][]string{
	"dashboard": {"schema_version", "id", "slug", "name", "icon", "revision", "approved_revision", "updated_at", "saved_by", "look", "grid", "pages", "kiosk"},
	"look": {"family", "palette", "backdrop", "density", "radius_scale"}, "palette": {"mode", "custom", "snapshot"}, "custom": {"accent", "surface_hue", "scheme"},
	"backdrop": {"kind"}, "grid": {"row_unit", "gap", "fit"}, "page": {"id", "name", "icon", "compact", "widgets", "layouts"},
	"widget": {"id", "type", "variant", "family", "title", "icon", "bindings", "options", "actions", "confirm"}, "actions": {"tap", "hold"}, "action": {"do", "page"},
	"layouts": {"lg", "md", "sm"}, "item": {"id", "x", "y", "w", "h"},
	"kiosk": {"start_page", "idle_dim_seconds", "idle_dim_level", "page_rotation_seconds", "screen_saver", "show_clock"},
}
var homeDashItemID = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,31}$`)

func homeDashIssue(path, code, msg string) map[string]any {
	return map[string]any{"path": path, "code": code, "message": msg}
}

func (f *homeDashFixture) m4Locked() *homeDashM4 {
	if f.m4 == nil {
		var cat struct {
			Widgets []homeDashWidgetSpec `json:"widgets"`
		}
		_ = json.Unmarshal(f.catalog, &cat)
		f.m4 = &homeDashM4{types: map[string]homeDashWidgetSpec{}}
		for _, w := range cat.Widgets {
			f.m4.types[w.Type] = w
		}
	}
	return f.m4
}

// lintLocked returns ("bad_json", issues) for a field the strict decoder refuses, ("", issues) for document issues and ("", nil) when clean.
func (f *homeDashFixture) lintLocked(doc map[string]any) (string, []map[string]any) {
	var bad []map[string]any
	unknown := func(path, kind string, v any) {
		m, _ := v.(map[string]any)
		for k := range m {
			if !slices.Contains(homeDashDocKeys[kind], k) {
				bad = append(bad, homeDashIssue(path+"/"+k, "bad_json", `json: unknown field "`+k+`"`))
			}
		}
	}
	unknown("", "dashboard", doc)
	if look, _ := doc["look"].(map[string]any); look != nil {
		unknown("/look", "look", look)
		pal, _ := look["palette"].(map[string]any)
		unknown("/look/palette", "palette", pal)
		if pal != nil {
			unknown("/look/palette/custom", "custom", pal["custom"])
		}
		unknown("/look/backdrop", "backdrop", look["backdrop"])
	}
	unknown("/grid", "grid", doc["grid"])
	unknown("/kiosk", "kiosk", doc["kiosk"])
	pages, _ := doc["pages"].([]any)
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		pp := fmt.Sprintf("/pages/%d", i)
		unknown(pp, "page", p)
		lay, _ := p["layouts"].(map[string]any)
		unknown(pp+"/layouts", "layouts", lay)
		for _, bp := range []string{"lg", "md", "sm"} {
			items, _ := lay[bp].([]any)
			for j, it := range items {
				unknown(fmt.Sprintf("%s/layouts/%s/%d", pp, bp, j), "item", it)
			}
		}
		ws, _ := p["widgets"].([]any)
		for j, rw := range ws {
			wp := fmt.Sprintf("%s/widgets/%d", pp, j)
			unknown(wp, "widget", rw)
			w, _ := rw.(map[string]any)
			acts, _ := w["actions"].(map[string]any)
			unknown(wp+"/actions", "actions", acts)
			for _, k := range []string{"tap", "hold"} {
				if acts != nil {
					unknown(wp+"/actions/"+k, "action", acts[k])
				}
			}
		}
	}
	if len(bad) > 0 {
		return "bad_json", bad
	}
	var issues []map[string]any
	// Codes follow validator.text of the server: fewer characters than the minimum is "required", more than the maximum "too_long".
	textIssue := func(path string, v any, min, max int) {
		n := 0
		if s, ok := v.(string); ok {
			n = utf8.RuneCountInString(s)
		}
		switch {
		case n < min:
			issues = append(issues, homeDashIssue(path, "required", "required"))
		case n > max:
			issues = append(issues, homeDashIssue(path, "too_long", fmt.Sprintf("at most %d characters", max)))
		}
	}
	textIssue("/name", doc["name"], 1, 80)
	switch {
	case len(pages) == 0:
		issues = append(issues, homeDashIssue("/pages", "required", "at least one page"))
	case len(pages) > 12:
		issues = append(issues, homeDashIssue("/pages", "too_many", "at most 12 pages"))
	}
	seen := map[string]bool{}
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		pp := fmt.Sprintf("/pages/%d", i)
		if id, _ := p["id"].(string); !homeDashItemID.MatchString(id) {
			issues = append(issues, homeDashIssue(pp+"/id", "bad_format", "invalid page id"))
		}
		textIssue(pp+"/name", p["name"], 1, 40)
		ws, _ := p["widgets"].([]any)
		ids := map[string]bool{}
		for j, rw := range ws {
			w, _ := rw.(map[string]any)
			wp := fmt.Sprintf("%s/widgets/%d", pp, j)
			id, _ := w["id"].(string)
			switch {
			case !homeDashItemID.MatchString(id):
				issues = append(issues, homeDashIssue(wp+"/id", "bad_format", "invalid widget id"))
			case seen[id]:
				issues = append(issues, homeDashIssue(wp+"/id", "duplicate_id", "widget id used twice"))
			}
			seen[id], ids[id] = true, true
			spec, ok := f.m4Locked().types[fmt.Sprint(w["type"])]
			if !ok {
				issues = append(issues, homeDashIssue(wp+"/type", "unknown_type", "unknown widget type"))
				continue
			}
			if !slices.ContainsFunc(spec.Variants, func(v struct {
				ID string `json:"id"`
			}) bool {
				return v.ID == w["variant"]
			}) {
				issues = append(issues, homeDashIssue(wp+"/variant", "unknown_variant", "unknown variant"))
			}
			bind, _ := w["bindings"].(map[string]any)
			for _, b := range spec.Bindings {
				if v, _ := bind[b.Name].(string); b.Required && v == "" {
					issues = append(issues, homeDashIssue(wp+"/bindings/"+b.Name, "binding_required", "this binding is required"))
				}
			}
		}
		lay, _ := p["layouts"].(map[string]any)
		lg, _ := lay["lg"].([]any)
		placed, cells := map[string]bool{}, map[[2]int]bool{}
		for j, ri := range lg {
			it, _ := ri.(map[string]any)
			id := fmt.Sprint(it["id"])
			x, y, wd, ht := homeDashInt(it["x"]), homeDashInt(it["y"]), homeDashInt(it["w"]), homeDashInt(it["h"])
			lp := fmt.Sprintf("%s/layouts/lg/%d", pp, j)
			switch {
			case !ids[id]:
				issues = append(issues, homeDashIssue(lp, "layout_unknown_widget", "no such widget"))
			case placed[id]:
				issues = append(issues, homeDashIssue(lp, "layout_duplicate", "placed twice"))
			case x < 0 || wd < 1 || ht < 1 || x+wd > 12 || y < 0:
				issues = append(issues, homeDashIssue(lp, "layout_out_of_bounds", "outside the grid"))
			}
			placed[id] = true
			for cx := x; cx < x+wd && cx < 12; cx++ {
				for cy := y; cy < y+ht && cy < 500; cy++ {
					if cells[[2]int{cx, cy}] {
						issues = append(issues, homeDashIssue(lp, "layout_overlap", "overlaps another widget"))
						cx, cy = 12, 500
					}
					cells[[2]int{cx, cy}] = true
				}
			}
		}
		for id := range ids {
			if !placed[id] {
				issues = append(issues, homeDashIssue(pp+"/layouts/lg", "layout_missing", "widget "+id+" has no place"))
			}
		}
	}
	return "", issues
}

// slugTakenLocked reports whether a stored dashboard already uses the slug (used by create in desktop_homedash_fixture_test.go).
func (f *homeDashFixture) slugTakenLocked(slug string) bool {
	for _, d := range f.dashboards {
		if s, _ := d["slug"].(string); s == slug {
			return true
		}
	}
	return false
}

func (f *homeDashFixture) update(w http.ResponseWriter, r *http.Request, raw []byte) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	if len(raw) > 512<<10 {
		homeDashError(w, 413, "body_too_large", "request body too large")
		return
	}
	f.mu.Lock()
	m := f.m4Locked()
	delay, drop := m.delay, m.drops > 0
	if drop {
		m.drops--
	}
	var answer *homeDashSaveAnswer
	if len(m.answers) > 0 && !drop {
		answer = m.answers[0]
		if answer.times--; answer.times <= 0 {
			m.answers = m.answers[1:]
		}
	}
	f.mu.Unlock()
	if delay > 0 {
		select {
		case <-time.After(delay):
		case <-f.quit:
		}
	}
	if drop {
		if hj, ok := w.(http.Hijacker); ok {
			if conn, _, err := hj.Hijack(); err == nil {
				_ = conn.Close()
				return
			}
		}
		homeDashError(w, 502, "bad_gateway", "connection dropped")
		return
	}
	if answer != nil {
		homeDashJSON(w, answer.status, answer.body)
		return
	}
	var body struct {
		Revision int            `json:"revision"`
		Document map[string]any `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Revision < 1 || body.Document == nil {
		homeDashError(w, 400, "bad_request", "send {revision, document}")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cur := f.dashboards[id]
	if cur == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	have := homeDashInt(cur["revision"])
	if body.Revision != have {
		homeDashJSON(w, 409, map[string]any{"status": "error", "code": "revision_conflict", "message": "stale revision", "current_revision": have})
		return
	}
	if code, issues := f.lintLocked(body.Document); code != "" || len(issues) > 0 {
		homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": issues})
		return
	}
	next := homeDashClone(body.Document)
	if s, _ := next["slug"].(string); s == "" {
		next["slug"] = cur["slug"]
	}
	rev := have + 1
	next["id"], next["revision"], next["approved_revision"], next["updated_at"], next["saved_by"] = id, rev, rev, homeDashNow(), "user"
	f.dashboards[id] = next
	f.m4Locked().puts = append(f.m4Locked().puts, homeDashPut{ID: id, Revision: body.Revision, Document: homeDashClone(body.Document), At: time.Now()})
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboard": homeDashClone(next), "warnings": []any{}})
}

func (f *homeDashFixture) validate(w http.ResponseWriter, raw []byte) {
	var body struct {
		Document map[string]any `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil || body.Document == nil {
		homeDashError(w, 400, "bad_request", "send {document}")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	code, issues := f.lintLocked(body.Document)
	if code == "bad_json" {
		homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": issues})
		return
	}
	if issues == nil {
		issues = []map[string]any{}
	}
	warnings := []map[string]any{}
	pages, _ := body.Document["pages"].([]any)
	for i, rp := range pages {
		p, _ := rp.(map[string]any)
		ws, _ := p["widgets"].([]any)
		for j, rw := range ws {
			wd, _ := rw.(map[string]any)
			bind, _ := wd["bindings"].(map[string]any)
			names := make([]string, 0, len(bind))
			for n := range bind {
				names = append(names, n)
			}
			slices.Sort(names)
			for _, n := range names {
				eid, _ := bind[n].(string)
				_, known := f.values[eid]
				if f.m3 != nil && f.m3.ha[eid] != nil {
					known = true
				}
				if eid != "" && !known {
					warnings = append(warnings, homeDashIssue(fmt.Sprintf("/pages/%d/widgets/%d/bindings/%s", i, j, n), "entity_unknown", "no such entity"))
				}
			}
		}
	}
	doc := homeDashClone(body.Document)
	if s, _ := doc["slug"].(string); s == "" {
		doc["slug"] = f.uniqueSlugLocked(homeDashSlug(fmt.Sprint(doc["name"])))
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "valid": len(issues) == 0, "issues": issues, "warnings": warnings, "document": doc})
}

func (f *homeDashFixture) duplicate(w http.ResponseWriter, r *http.Request, raw []byte) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	var body struct {
		Name string `json:"name"`
	}
	if len(raw) > 0 && json.Unmarshal(raw, &body) != nil {
		homeDashError(w, 400, "bad_request", "bad body")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	src := f.dashboards[id]
	if src == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	doc := homeDashClone(src)
	name := body.Name
	if name == "" {
		name = string([]rune(fmt.Sprint(src["name"]))[:min(73, utf8.RuneCountInString(fmt.Sprint(src["name"])))]) + " (copy)"
	}
	doc["name"] = name
	doc["slug"] = f.uniqueSlugLocked(fmt.Sprint(src["slug"]) + "-copy")
	f.seq++
	nid := homeDashID(f.seq)
	doc["id"], doc["revision"], doc["approved_revision"], doc["updated_at"], doc["saved_by"] = nid, 1, 1, homeDashNow(), "user"
	f.dashboards[nid] = doc
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": nid, "revision": 1, "approved_revision": 1}))
	homeDashJSON(w, 201, map[string]any{"status": "ok", "dashboard": homeDashClone(doc), "warnings": []any{}})
}

// homeDashEntityPage filters (source = id prefix, kind, area exact, q case-insensitive in name, id and area) and pages the entity list.
func homeDashEntityPage(list []any, q url.Values) (page []any, total, limit, offset int, bad string) {
	limit = 100
	if v := q.Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 1 || n > 500 {
			return nil, 0, 0, 0, "limit"
		}
		limit = n
	}
	if v := q.Get("offset"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return nil, 0, 0, 0, "offset"
		}
		offset = n
	}
	needle := strings.ToLower(strings.TrimSpace(q.Get("q")))
	var kept []any
	for _, raw := range list {
		e, _ := raw.(map[string]any)
		id, _ := e["id"].(string)
		if s := q.Get("source"); s != "" && !strings.HasPrefix(id, s+":") {
			continue
		}
		if k := q.Get("kind"); k != "" && e["kind"] != k {
			continue
		}
		if a := q.Get("area"); a != "" && e["area"] != a {
			continue
		}
		if needle != "" && !strings.Contains(strings.ToLower(fmt.Sprint(e["name"])+" "+id+" "+fmt.Sprint(e["area"])), needle) {
			continue
		}
		kept = append(kept, raw)
	}
	total = len(kept)
	if offset > total {
		offset = total
	}
	end := min(offset+limit, total)
	page = append([]any{}, kept[offset:end]...)
	return page, total, limit, offset, ""
}

// Controls.
func (f *homeDashFixture) answerPut(status int, body map[string]any, times int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.m4Locked()
	m.answers = append(m.answers, &homeDashSaveAnswer{status: status, body: body, times: max(times, 1)})
}
func (f *homeDashFixture) dropPut(n int) { f.mu.Lock(); f.m4Locked().drops = n; f.mu.Unlock() }
func (f *homeDashFixture) setPutDelay(d time.Duration) {
	f.mu.Lock()
	f.m4Locked().delay = d
	f.mu.Unlock()
}
func (f *homeDashFixture) putLog() []homeDashPut {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.m4Locked().puts)
}
func (f *homeDashFixture) dashboardDoc(id string) map[string]any {
	f.mu.Lock()
	defer f.mu.Unlock()
	if d := f.dashboards[id]; d != nil {
		return homeDashClone(d)
	}
	return nil
}

// foreignSave edits a stored dashboard like another window or the agent would: revision and updated_at move, saved_by is set (an agent save leaves
// approved_revision alone) and every stream gets dashboard_changed. It returns the new revision.
func (f *homeDashFixture) foreignSave(t *testing.T, id, savedBy string, mutate func(doc map[string]any)) int {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	doc := f.dashboards[id]
	if doc == nil {
		t.Fatalf("no dashboard %s", id)
	}
	if mutate != nil {
		mutate(doc)
	}
	rev := homeDashInt(doc["revision"]) + 1
	doc["revision"], doc["updated_at"], doc["saved_by"] = rev, homeDashNow(), savedBy
	if savedBy != "agent" {
		doc["approved_revision"] = rev
	}
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": homeDashInt(doc["approved_revision"])}))
	return rev
}

func TestHomeDashFixtureSaveRoutes(t *testing.T) {
	f := newHomeDashFixture(t)
	call := func(method, target, body string) (int, map[string]any) {
		t.Helper()
		req := httptest.NewRequest(method, "/api/desktop/homedash/"+target, strings.NewReader(body))
		rec := httptest.NewRecorder()
		f.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	status, made := call("POST", "dashboards", `{"document":{"name":"Test","pages":[{"id":"main","name":"Main"}]}}`)
	if status != 201 {
		t.Fatalf("create: %d %v", status, made)
	}
	doc := made["dashboard"].(map[string]any)
	id := doc["id"].(string)
	if _, ok := doc["kiosk"].(map[string]any); !ok {
		t.Fatalf("create keeps the posted document and adds the kiosk default: %v", doc)
	}
	put := func(rev int, mutate func(d map[string]any)) (int, map[string]any) {
		t.Helper()
		d := f.dashboardDoc(id)
		if mutate != nil {
			mutate(d)
		}
		raw, _ := json.Marshal(map[string]any{"revision": rev, "document": d})
		return call("PUT", "dashboard?id="+id, string(raw))
	}
	if status, out := put(1, nil); status != 200 || homeDashInt(out["dashboard"].(map[string]any)["revision"]) != 2 {
		t.Fatalf("first PUT: %d %v", status, out)
	}
	if status, out := put(1, nil); status != 409 || out["code"] != "revision_conflict" || homeDashInt(out["current_revision"]) != 2 {
		t.Fatalf("stale PUT: %d %v", status, out)
	}
	bad := func(d map[string]any) {
		p := d["pages"].([]any)[0].(map[string]any)
		p["widgets"] = []any{map[string]any{"id": "w1", "type": "switch", "variant": "tile", "lgSize": map[string]any{"w": 2}, "bindings": map[string]any{"primary": "system:cpu"}, "options": map[string]any{}, "actions": map[string]any{}}}
	}
	if status, out := put(2, bad); status != 422 || out["code"] != "invalid_document" || out["issues"].([]any)[0].(map[string]any)["code"] != "bad_json" {
		t.Fatalf("unknown field: %d %v", status, out)
	}
	draft := func(d map[string]any) {
		p := d["pages"].([]any)[0].(map[string]any)
		p["widgets"] = []any{map[string]any{"id": "w1", "type": "switch", "variant": "tile", "bindings": map[string]any{}, "options": map[string]any{}, "actions": map[string]any{}, "confirm": false}}
		p["layouts"] = map[string]any{"lg": []any{map[string]any{"id": "w1", "x": 0, "y": 0, "w": 2, "h": 1}}}
	}
	if status, out := put(2, draft); status != 422 || out["issues"].([]any)[0].(map[string]any)["code"] != "binding_required" {
		t.Fatalf("draft widget: %d %v", status, out)
	}
	d := f.dashboardDoc(id)
	draft(d)
	raw, _ := json.Marshal(map[string]any{"document": d})
	if status, out := call("POST", "dashboards/validate", string(raw)); status != 200 || out["valid"] != false || out["issues"].([]any)[0].(map[string]any)["code"] != "binding_required" {
		t.Fatalf("validate: %d %v", status, out)
	}
	if status, out := call("POST", "dashboards/validate", `{"document":{"name":"x","pages":[{"id":"a","name":"A","lgSize":1}]}}`); status != 422 {
		t.Fatalf("validate bad_json: %d %v", status, out)
	}
	// Lint codes follow validator.text and the pages check of the server: empty or missing is "required", over the limit "too_long" / "too_many".
	codeAt := func(body, path string) string {
		_, out := call("POST", "dashboards/validate", body)
		list, _ := out["issues"].([]any)
		for _, it := range list {
			if m, _ := it.(map[string]any); m["path"] == path {
				code, _ := m["code"].(string)
				return code
			}
		}
		return ""
	}
	tooLong := strings.Repeat("x", 81)
	for _, c := range []struct{ body, path, want string }{
		{`{"document":{"pages":[{"id":"a","name":"A"}]}}`, "/name", "required"},
		{`{"document":{"name":"","pages":[{"id":"a","name":"A"}]}}`, "/name", "required"},
		{`{"document":{"name":"` + tooLong + `","pages":[{"id":"a","name":"A"}]}}`, "/name", "too_long"},
		{`{"document":{"name":"x","pages":[]}}`, "/pages", "required"},
		{`{"document":{"name":"x","pages":[{"id":"a"}]}}`, "/pages/0/name", "required"},
		{`{"document":{"name":"x","pages":[{"id":"a","name":"` + tooLong[:41] + `"}]}}`, "/pages/0/name", "too_long"},
	} {
		if got := codeAt(c.body, c.path); got != c.want {
			t.Fatalf("lint %s: got %q, want %q", c.path, got, c.want)
		}
	}
	if status, out := call("POST", "dashboard/duplicate?id="+id, ``); status != 201 || out["dashboard"].(map[string]any)["name"] != "Test (copy)" || out["dashboard"].(map[string]any)["slug"] != "test-copy" {
		t.Fatalf("duplicate: %d %v", status, out)
	}
	f.answerPut(503, map[string]any{"status": "error", "code": "storage_unavailable", "message": "later"}, 1)
	if status, _ := put(2, nil); status != 503 {
		t.Fatalf("scripted answer: %d", status)
	}
	if status, _ := put(2, nil); status != 200 {
		t.Fatalf("after the scripted answer: %d", status)
	}
	if got := len(f.putLog()); got != 2 {
		t.Fatalf("put log holds the accepted saves: %d", got)
	}
	if status, out := call("GET", "entities?kind=sensor&limit=1", ""); status != 200 || len(out["entities"].([]any)) != 1 || homeDashInt(out["total"]) < 2 || homeDashInt(out["limit"]) != 1 {
		t.Fatalf("entity filter: %d %v", status, out)
	}
	if status, _ := call("GET", "entities?limit=900", ""); status != 400 {
		t.Fatalf("limit out of range: %d", status)
	}
	if rev := f.foreignSave(t, id, "agent", func(d map[string]any) { d["name"] = "Agent was here" }); rev != 4 || f.dashboardDoc(id)["name"] != "Agent was here" || homeDashInt(f.dashboardDoc(id)["approved_revision"]) != 3 {
		t.Fatalf("foreign save: %d %v", rev, f.dashboardDoc(id))
	}
}
