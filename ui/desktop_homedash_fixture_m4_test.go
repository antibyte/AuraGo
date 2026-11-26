package ui

// M4 additions to the HomeDash fixture (plan 2026-11-26-homedash-m4-designer.md, contract C18): PUT dashboard with the revision check (409
// revision_conflict + current_revision), POST dashboards/validate, POST dashboard/duplicate, entity filters and paging, document checks that run the
// server's own code (homedash.DecodeDashboard, Migrate, Normalize, Autopack, Validate against the fixture's entities; the same status codes and
// bodies as internal/server/homedash_api_dashboards.go), scripted PUT failures (answers, dropped connections, delay) and a foreign save that sends
// dashboard_changed.

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"aurago/internal/homedash"
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

// homeDashHTTPStatus is the server's code -> status table (internal/server/homedash_routes.go homeDashStatus) for the codes the save routes produce.
func homeDashHTTPStatus(code string) int {
	switch code {
	case homedash.CodeInvalidDocument, homedash.CodeLimitExceeded:
		return http.StatusUnprocessableEntity
	case homedash.CodeRevisionConflict, homedash.CodeSchemaTooNew, homedash.CodeSlugTaken:
		return http.StatusConflict
	}
	return http.StatusInternalServerError
}

// homeDashFail answers a *homedash.Error like hdRequest.fail: its code's status, status/code/message, current_revision and issues.
func homeDashFail(w http.ResponseWriter, he *homedash.Error) {
	body := map[string]any{"status": "error", "code": he.Code, "message": he.Message}
	if he.CurrentRevision > 0 {
		body["current_revision"] = he.CurrentRevision
	}
	if len(he.Issues) > 0 {
		body["issues"] = he.Issues
	}
	homeDashJSON(w, homeDashHTTPStatus(he.Code), body)
}

// knownLocked looks up an entity the fixture serves (values, or the Home Assistant ones once M3 is on) for Validate's entity warnings.
func (f *homeDashFixture) knownLocked(id homedash.EntityID) (homedash.Entity, bool) {
	_, listed := f.values[string(id)]
	if f.m3 != nil && f.m3.ha[string(id)] != nil {
		listed = true
	}
	if !listed {
		return homedash.Entity{}, false
	}
	var e homedash.Entity
	raw, _ := json.Marshal(f.entityLocked(string(id)))
	_ = json.Unmarshal(raw, &e)
	return e, true
}

// prepareLocked is the server's save pipeline (svc.prepare behind DecodeDashboard): decode strictly, give the document a slug (slug may be nil),
// Migrate, Normalize, Autopack, Validate. A decode or migrate failure and a report with issues come back as the server's *homedash.Error; the
// report is returned either way so the validate route can answer it.
func (f *homeDashFixture) prepareLocked(raw []byte, slug func(d homedash.Dashboard) string) (homedash.Dashboard, homedash.Report, *homedash.Error) {
	doc, err := homedash.DecodeDashboard(raw)
	if err != nil {
		he, _ := err.(*homedash.Error)
		return homedash.Dashboard{}, homedash.Report{}, he
	}
	if slug != nil {
		doc.Slug = slug(doc)
	}
	if err := homedash.Migrate(&doc); err != nil {
		he, _ := err.(*homedash.Error)
		return doc, homedash.Report{}, he
	}
	homedash.Normalize(&doc)
	homedash.Autopack(&doc, homedash.Catalog())
	report := homedash.Validate(doc, homedash.Catalog(), f.knownLocked)
	if !report.OK() {
		return doc, report, &homedash.Error{Code: homedash.CodeInvalidDocument, Message: "the dashboard is not valid", Issues: report.Issues}
	}
	return doc, report, nil
}

// homeDashDocMap turns a prepared document into the JSON object the fixture stores.
func homeDashDocMap(doc homedash.Dashboard) map[string]any {
	raw, _ := json.Marshal(doc)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

// homeDashWarnings is the answer's warnings array ([] when none); homeDashIssues the same for issues.
func homeDashWarnings(r homedash.Report) []homedash.Issue {
	if r.Warnings == nil {
		return []homedash.Issue{}
	}
	return r.Warnings
}
func homeDashIssues(r homedash.Report) []homedash.Issue {
	if r.Issues == nil {
		return []homedash.Issue{}
	}
	return r.Issues
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
		Revision int             `json:"revision"`
		Document json.RawMessage `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil {
		homeDashError(w, 400, "bad_request", "the body is not valid JSON for this endpoint")
		return
	}
	if body.Revision < 1 || len(body.Document) == 0 {
		homeDashError(w, 400, "bad_request", "revision and document are required")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	cur := f.dashboards[id]
	// The server validates before the store checks the revision, so an invalid document answers 422 even when the revision is stale.
	doc, report, he := f.prepareLocked(body.Document, func(d homedash.Dashboard) string {
		if d.Slug == "" && cur != nil {
			s, _ := cur["slug"].(string)
			return s
		}
		return d.Slug
	})
	if he != nil {
		homeDashFail(w, he)
		return
	}
	if cur == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	have := homeDashInt(cur["revision"])
	if body.Revision != have {
		homeDashFail(w, &homedash.Error{Code: homedash.CodeRevisionConflict, Message: "stale revision", CurrentRevision: int64(have)})
		return
	}
	next := homeDashDocMap(doc)
	rev := have + 1
	next["id"], next["revision"], next["approved_revision"], next["updated_at"], next["saved_by"] = id, rev, rev, homeDashNow(), "user"
	f.dashboards[id] = next
	var sent map[string]any
	_ = json.Unmarshal(body.Document, &sent)
	f.m4Locked().puts = append(f.m4Locked().puts, homeDashPut{ID: id, Revision: body.Revision, Document: sent, At: time.Now()})
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboard": homeDashClone(next), "warnings": homeDashWarnings(report)})
}

// validate answers like hdRequest.validateDashboard: 200 {valid, issues, warnings, document} for a decodable document (an empty slug is derived from
// the name), 422 with the decoder's issue when it is not.
func (f *homeDashFixture) validate(w http.ResponseWriter, raw []byte) {
	var body struct {
		Document json.RawMessage `json:"document"`
	}
	if json.Unmarshal(raw, &body) != nil {
		homeDashError(w, 400, "bad_request", "the body is not valid JSON for this endpoint")
		return
	}
	if len(body.Document) == 0 {
		homeDashError(w, 400, "bad_request", "document is required")
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	doc, report, he := f.prepareLocked(body.Document, func(d homedash.Dashboard) string {
		if d.Slug == "" {
			return homeDashSlug(d.Name)
		}
		return d.Slug
	})
	if he != nil && he.Code != homedash.CodeInvalidDocument || he != nil && len(he.Issues) == 1 && he.Issues[0].Code == "bad_json" {
		homeDashFail(w, he)
		return
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "valid": report.OK(), "issues": homeDashIssues(report), "warnings": homeDashWarnings(report), "document": doc})
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
	// The checks are the server's own (homedash.Validate): an md item under the variant minimum, a URL in a title and a fractional layout position.
	docWith := func(widgetExtra, lg, md string) string {
		return `{"document":{"name":"x","pages":[{"id":"a","name":"A","widgets":[{"id":"w1","type":"sensor","variant":"sparkline",` + widgetExtra + `"bindings":{"primary":"system:cpu"}}],"layouts":{"lg":[` + lg + `],"md":` + md + `}}]}}`
	}
	okLG := `{"id":"w1","x":0,"y":0,"w":2,"h":2}`
	if status, out := call("POST", "dashboards/validate", docWith("", okLG, "null")); status != 200 || out["valid"] != true {
		t.Fatalf("the baseline document is valid: %d %v", status, out)
	}
	issueAt := func(body string) (string, string) {
		status, out := call("POST", "dashboards/validate", body)
		if (status == 422) != (out["code"] == "invalid_document") {
			t.Fatalf("validate answers 422 only with invalid_document: %d %v", status, out)
		}
		list, _ := out["issues"].([]any)
		if len(list) == 0 {
			return "", ""
		}
		m, _ := list[0].(map[string]any)
		path, _ := m["path"].(string)
		code, _ := m["code"].(string)
		return path, code
	}
	for _, c := range []struct{ name, body, path, want string }{
		{"md below the minimum size", docWith("", okLG, `[{"id":"w1","x":0,"y":0,"w":1,"h":1}]`), "/pages/0/layouts/md/0", "size_out_of_range"},
		{"title with a URL", docWith(`"title":"see http://x.y",`, okLG, "null"), "/pages/0/widgets/0/title", "url_not_allowed"},
		{"fractional x", docWith("", `{"id":"w1","x":0.5,"y":0,"w":2,"h":2}`, "null"), "", "bad_json"}, // the strict decoder refuses 0.5 for an int: 422 invalid_document, not a validate report
	} {
		path, code := issueAt(c.body)
		if code == "" {
			t.Fatalf("%s: no issue", c.name)
		}
		if code != c.want || path != c.path {
			t.Fatalf("%s: issue %q at %q, want %q at %q", c.name, code, path, c.want, c.path)
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
