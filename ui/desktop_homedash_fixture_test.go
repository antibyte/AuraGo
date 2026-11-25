package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

var homeDashIDPattern = regexp.MustCompile(`^d_[a-z2-7]{12}$`)

// homeDashEntityPattern is the entity id grammar <source>:<native>.
var homeDashEntityPattern = regexp.MustCompile(`^[a-z][a-z0-9_]{0,31}:[^\s:][^\s]{0,199}$`)

// homeDashTemplateID is the contract's example id; the first template creation answers it.
const homeDashTemplateID = "d_m3x7q2k5a4bc"

// homeDashFixture answers /api/desktop/homedash/ like the M2 contract (docs/superpowers/plans/2026-11-25-homedash-m2-backend.md, "API contract
// (frozen for M2)"): bootstrap, widget-catalog (golden JSON, ETag, 304), entities, entity, history, dashboards with the system template and
// the empty document, dashboard GET and DELETE, the SSE stream (hello, snapshot on every PUT that adds ids, state, dashboard_changed, hb) and
// PUT stream/subscriptions. Controls let a test push values, change a dashboard, drop the streams, refuse reconnects and go read-only.
type homeDashFixture struct {
	t          *testing.T
	quit       chan struct{}
	quitOnce   sync.Once
	mu         sync.Mutex
	catalog    []byte
	hash       string
	starter    map[string]any
	entities   []map[string]any
	values     map[string]float64
	dashboards map[string]map[string]any
	seq        int
	streams    map[string]*homeDashStream
	opened     int
	refuse     int
	refused    int
	readonly   bool
	disabled   bool
	templated  bool
	requests   []string
	bodies     map[string]string
	m3         *homeDashM3 // nil until enableM3 (desktop_homedash_fixture_m3_test.go)
}

// homeDashCreateBody is the body of POST dashboards: a template (with the optional name and, for home_assistant, unassigned_title) or a document.
type homeDashCreateBody struct {
	Template        string         `json:"template"`
	Name            string         `json:"name"`
	UnassignedTitle string         `json:"unassigned_title"`
	Document        map[string]any `json:"document"`
}

type homeDashStream struct {
	id       string
	gen      int
	frames   chan string
	done     chan struct{}
	gone     chan struct{}
	goneOnce sync.Once
	subs     map[string]bool
}

// send queues a frame and never drops it: when the buffer is full it waits for the stream to drain, end or the fixture to shut down, and
// fails the test loudly when the stream neither drains nor ends within 10 s.
func (f *homeDashFixture) send(s *homeDashStream, frame string) {
	select {
	case s.frames <- frame:
		return
	default:
	}
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case s.frames <- frame:
	case <-s.done:
	case <-s.gone:
	case <-f.quit:
	case <-timer.C:
		f.t.Errorf("homedash fixture: stream %s did not drain a frame within 10 s: %.60q", s.id, frame)
	}
}

// homeDashRoutes lists the implemented routes with their methods; the value is true for a write route (refused on a read-only desktop).
var homeDashRoutes = map[string]map[string]bool{
	"bootstrap":            {http.MethodGet: false},
	"widget-catalog":       {http.MethodGet: false},
	"entities":             {http.MethodGet: false},
	"entity":               {http.MethodGet: false},
	"history":              {http.MethodGet: false},
	"stream":               {http.MethodGet: false},
	"stream/subscriptions": {http.MethodPut: false},
	"dashboards":           {http.MethodGet: false, http.MethodPost: true},
	"dashboard":            {http.MethodGet: false, http.MethodDelete: true},
	"commands":             {http.MethodPost: true},
}

func newHomeDashFixture(t *testing.T) *homeDashFixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "internal", "homedash", "testdata", "widget_catalog.json"))
	if err != nil {
		t.Fatal(err)
	}
	var head struct {
		Hash string `json:"hash"`
	}
	if err := json.Unmarshal(raw, &head); err != nil || head.Hash == "" {
		t.Fatalf("golden catalog: %v", err)
	}
	f := &homeDashFixture{t: t, quit: make(chan struct{}), catalog: raw, hash: head.Hash, values: map[string]float64{}, dashboards: map[string]map[string]any{}, streams: map[string]*homeDashStream{}, bodies: map[string]string{}}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m2/starter-dashboard.json")), &f.starter); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m2/system-entities.json")), &f.entities); err != nil {
		t.Fatal(err)
	}
	for _, e := range f.entities {
		if st, ok := e["state"].(map[string]any); ok {
			if v, ok := st["value"].(float64); ok {
				f.values[e["id"].(string)] = v
			}
		}
	}
	t.Cleanup(f.shutdown)
	return f
}

// shutdown ends every stream handler so the test server can close; idempotent.
func (f *homeDashFixture) shutdown() { f.quitOnce.Do(func() { close(f.quit) }) }

func homeDashJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func homeDashError(w http.ResponseWriter, status int, code, message string) {
	homeDashJSON(w, status, map[string]any{"status": "error", "code": code, "message": message})
}

func homeDashReadonly(w http.ResponseWriter) {
	homeDashJSON(w, http.StatusForbidden, map[string]any{"error": "desktop_readonly", "code": "desktop_readonly", "message": "the desktop is read-only"})
}

func homeDashFrame(event string, data any) string {
	raw, _ := json.Marshal(data)
	return "event: " + event + "\ndata: " + string(raw) + "\n\n"
}

func homeDashNow() string { return time.Now().UTC().Format(time.RFC3339Nano) }

func homeDashSource(state string) map[string]any {
	return map[string]any{"source": "system", "state": state, "at": homeDashNow()}
}

func homeDashID(n int) string {
	const abc = "abcdefghijklmnopqrstuvwxyz234567"
	b := []byte("d_aaaaaaaaaaaa")
	for i := len(b) - 1; n > 0 && i >= 2; i-- {
		b[i] = abc[n%32]
		n /= 32
	}
	return string(b)
}

func homeDashSlug(name string) string {
	var b strings.Builder
	dash := false
	for _, r := range strings.ToLower(name) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			dash = false
			continue
		}
		if !dash && b.Len() > 0 {
			b.WriteByte('-')
			dash = true
		}
	}
	if s := strings.TrimRight(b.String(), "-"); s != "" {
		return s
	}
	return "dashboard"
}

func homeDashClone(v map[string]any) map[string]any {
	raw, _ := json.Marshal(v)
	out := map[string]any{}
	_ = json.Unmarshal(raw, &out)
	return out
}

func homeDashInt(v any) int {
	switch n := v.(type) {
	case int:
		return n
	case float64:
		return int(n)
	}
	return 0
}

func (f *homeDashFixture) uniqueSlugLocked(base string) string {
	used := map[string]bool{}
	for _, d := range f.dashboards {
		if s, ok := d["slug"].(string); ok {
			used[s] = true
		}
	}
	if !used[base] {
		return base
	}
	for i := 2; ; i++ {
		if s := fmt.Sprintf("%s-%d", base, i); !used[s] {
			return s
		}
	}
}

func (f *homeDashFixture) entityLocked(id string) map[string]any {
	if f.m3 != nil {
		if e := f.m3.ha[id]; e != nil {
			return homeDashClone(e)
		}
	}
	for _, e := range f.entities {
		if e["id"] == id {
			out := map[string]any{}
			for k, v := range e {
				out[k] = v
			}
			out["state"] = map[string]any{"value": f.values[id], "available": true, "updated_at": homeDashNow()}
			return out
		}
	}
	return map[string]any{"id": id, "source": strings.SplitN(id, ":", 2)[0], "kind": "generic", "name": id, "capabilities": []any{}, "readonly": true,
		"state": map[string]any{"value": nil, "available": false, "updated_at": homeDashNow()}}
}

func (f *homeDashFixture) broadcastLocked(frame string) {
	for _, s := range f.streams {
		f.send(s, frame)
	}
}

func (f *homeDashFixture) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)
	path := strings.TrimPrefix(r.URL.Path, "/api/desktop/homedash/")
	f.mu.Lock()
	f.requests = append(f.requests, r.Method+" "+r.URL.RequestURI())
	f.bodies[r.Method+" "+path] = string(raw)
	off, ro := f.disabled, f.readonly
	f.mu.Unlock()
	w.Header().Set("Cache-Control", "no-store")
	// Contract order: route, method, availability, desktop read-only (write routes), handler.
	methods, known := homeDashRoutes[path]
	if !known {
		homeDashError(w, http.StatusNotFound, "not_found", "no route "+path)
		return
	}
	write, allowed := methods[r.Method]
	if !allowed {
		allow := []string{}
		for m := range methods {
			allow = append(allow, m)
		}
		sort.Strings(allow)
		w.Header().Set("Allow", strings.Join(allow, ", "))
		homeDashError(w, http.StatusMethodNotAllowed, "method_not_allowed", "method not allowed")
		return
	}
	if off {
		homeDashError(w, http.StatusServiceUnavailable, "homedash_disabled", "HomeDash is disabled")
		return
	}
	if write && ro {
		homeDashReadonly(w)
		return
	}
	switch path {
	case "bootstrap", "widget-catalog", "dashboards", "dashboard", "stream/subscriptions":
		f.mu.Lock()
		var latency time.Duration
		if f.m3 != nil {
			latency = f.m3.latency
		}
		f.mu.Unlock()
		if latency > 0 {
			select {
			case <-time.After(latency):
			case <-f.quit:
			}
		}
	}
	switch r.Method + " " + path {
	case "GET bootstrap":
		f.mu.Lock()
		ro := f.readonly
		templates, sources := []string{"system"}, []any{homeDashSource("idle")}
		if f.m3 != nil {
			if f.m3.listed {
				templates = append(templates, "home_assistant")
			}
			if s := f.m3.sources["ha"]; s != nil {
				sources = append(sources, homeDashClone(s))
			}
		}
		f.mu.Unlock()
		homeDashJSON(w, 200, map[string]any{"status": "ok", "api_version": 1, "stream_protocol": 1, "schema_version": 1, "catalog_hash": f.hash, "readonly": false, "desktop_readonly": ro, "kiosk_enabled": false,
			"limits":    map[string]any{"dashboards": 50, "subscriptions": 300, "pages": 12, "widgets_per_page": 120, "widgets": 400, "document_bytes": 262144, "history_points": 500, "history_range_seconds": 604800},
			"templates": templates, "sources": sources})
	case "GET widget-catalog":
		w.Header().Set("ETag", `"`+f.hash+`"`)
		if strings.Contains(r.Header.Get("If-None-Match"), f.hash) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = io.WriteString(w, `{"status":"ok","catalog":`+string(f.catalog)+`}`)
	case "GET entities":
		f.mu.Lock()
		list := []any{}
		for _, e := range f.entities {
			list = append(list, f.entityLocked(e["id"].(string)))
		}
		sources := []any{homeDashSource("polling")}
		if f.m3 != nil {
			ids := []string{}
			for id := range f.m3.ha {
				ids = append(ids, id)
			}
			sort.Strings(ids)
			for _, id := range ids {
				list = append(list, f.entityLocked(id))
			}
			if s := f.m3.sources["ha"]; s != nil {
				sources = append(sources, homeDashClone(s))
			}
		}
		f.mu.Unlock()
		homeDashJSON(w, 200, map[string]any{"status": "ok", "entities": list, "total": len(list), "limit": 100, "offset": 0, "sources": sources})
	case "GET entity":
		id := r.URL.Query().Get("id")
		if !homeDashEntityPattern.MatchString(id) {
			homeDashError(w, 400, "bad_request", "bad entity id")
			return
		}
		f.mu.Lock()
		_, known := f.values[id]
		if f.m3 != nil && f.m3.ha[id] != nil {
			known = true
		}
		e := f.entityLocked(id)
		f.mu.Unlock()
		if !known {
			homeDashError(w, 404, "entity_not_found", "unknown entity")
			return
		}
		homeDashJSON(w, 200, map[string]any{"status": "ok", "entity": e})
	case "GET history":
		f.history(w, r)
	case "GET stream":
		f.stream(w, r)
	case "PUT stream/subscriptions":
		f.subscribe(w, raw)
	case "GET dashboards":
		f.list(w)
	case "POST dashboards":
		f.create(w, raw)
	case "GET dashboard":
		f.get(w, r)
	case "DELETE dashboard":
		f.remove(w, r)
	case "POST commands":
		f.command(w, raw)
	}
}

func (f *homeDashFixture) history(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	id, to, points := q.Get("id"), time.Now().UTC(), 120
	from := to.Add(-24 * time.Hour)
	var err error
	if !homeDashEntityPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad entity id")
		return
	}
	if v := q.Get("to"); v != "" {
		if to, err = time.Parse(time.RFC3339Nano, v); err != nil {
			homeDashError(w, 400, "bad_request", "bad to")
			return
		}
	}
	if v := q.Get("from"); v != "" {
		if from, err = time.Parse(time.RFC3339Nano, v); err != nil {
			homeDashError(w, 400, "bad_request", "bad from")
			return
		}
	}
	if v := q.Get("points"); v != "" {
		if points, err = strconv.Atoi(v); err != nil || points < 1 || points > 500 {
			homeDashError(w, 400, "bad_request", "bad points")
			return
		}
	}
	if !to.After(from) || to.Sub(from) > 7*24*time.Hour {
		homeDashError(w, 400, "bad_request", "bad range")
		return
	}
	f.mu.Lock()
	base, known := f.values[id]
	f.mu.Unlock()
	if !known {
		homeDashError(w, 404, "entity_not_found", "unknown entity")
		return
	}
	out := []map[string]any{}
	if id != "system:uptime" {
		step := to.Sub(from) / time.Duration(points)
		for i := 0; i < points; i++ {
			v := math.Round((base+base*0.35*math.Sin(float64(i)/7)+base*0.1*math.Sin(float64(i)*1.7))*10) / 10
			out = append(out, map[string]any{"t": from.Add(step*time.Duration(i) + step/2).Format(time.RFC3339Nano), "v": math.Max(0, v)})
		}
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "id": id, "origin": "source", "points": out})
}

func (f *homeDashFixture) stream(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	if f.refuse > 0 {
		f.refuse--
		f.refused++
		f.mu.Unlock()
		homeDashError(w, http.StatusTooManyRequests, "too_many_streams", "too many streams")
		return
	}
	f.opened++
	s := &homeDashStream{id: fmt.Sprintf("%032x", f.opened), gen: f.opened, frames: make(chan string, 64), done: make(chan struct{}), gone: make(chan struct{}), subs: map[string]bool{}}
	f.streams[s.id] = s
	f.mu.Unlock()
	defer func() {
		s.goneOnce.Do(func() { close(s.gone) })
		f.mu.Lock()
		if f.streams[s.id] == s {
			delete(f.streams, s.id)
		}
		f.mu.Unlock()
	}()
	flusher, ok := w.(http.Flusher)
	if !ok {
		homeDashError(w, 500, "internal_error", "no flusher")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	write := func(frame string) bool {
		if _, err := io.WriteString(w, frame); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	if !write(homeDashFrame("hello", map[string]any{"stream_id": s.id, "protocol": 1, "heartbeat_ms": 15000, "limits": map[string]any{"entities": 300}, "server_time": homeDashNow(), "principal": "session"})) {
		return
	}
	hb := time.NewTicker(15 * time.Second)
	defer hb.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case <-s.done:
			return
		case <-f.quit:
			return
		case frame := <-s.frames:
			if !write(frame) || strings.HasPrefix(frame, "event: revoked\n") {
				return
			}
		case <-hb.C:
			if !write(homeDashFrame("hb", map[string]any{"server_time": homeDashNow()})) {
				return
			}
		}
	}
}

func (f *homeDashFixture) subscribe(w http.ResponseWriter, raw []byte) {
	var body struct {
		StreamID string   `json:"stream_id"`
		Entities []string `json:"entities"`
	}
	if err := json.Unmarshal(raw, &body); err != nil {
		homeDashError(w, 400, "bad_request", "bad json")
		return
	}
	f.mu.Lock()
	s := f.streams[body.StreamID]
	if s == nil {
		f.mu.Unlock()
		homeDashError(w, 404, "stream_not_found", "unknown stream")
		return
	}
	accepted, rejected, added, next := []string{}, []map[string]any{}, []any{}, map[string]bool{}
	haLate := []string{} // ha ids whose snapshot waits m3.snapDelay
	for _, id := range body.Entities {
		if next[id] {
			continue
		}
		if !homeDashEntityPattern.MatchString(id) {
			rejected = append(rejected, map[string]any{"id": id, "reason": "invalid_id"})
			continue
		}
		isHA := f.m3 != nil && strings.HasPrefix(id, "ha:")
		if !strings.HasPrefix(id, "system:") && !isHA {
			rejected = append(rejected, map[string]any{"id": id, "reason": "unknown_source"})
			continue
		}
		if isHA && f.m3.ha[id] == nil {
			rejected = append(rejected, map[string]any{"id": id, "reason": "unknown_entity"})
			continue
		}
		next[id] = true
		accepted = append(accepted, id)
		if s.subs[id] {
			continue
		}
		if isHA && f.m3.snapDelay > 0 {
			haLate = append(haLate, id)
			continue
		}
		added = append(added, f.entityLocked(id))
	}
	s.subs = next
	if len(added) > 0 {
		sources := []any{homeDashSource("polling")}
		if f.m3 != nil && f.m3.sources["ha"] != nil {
			for _, e := range added {
				if m, _ := e.(map[string]any); m != nil && m["source"] == "ha" {
					sources = append(sources, homeDashClone(f.m3.sources["ha"]))
					break
				}
			}
		}
		f.send(s, homeDashFrame("snapshot", map[string]any{"entities": added, "sources": sources}))
	}
	if len(haLate) > 0 {
		f.snapshotLater(s, haLate, f.m3.snapDelay)
	}
	f.mu.Unlock()
	homeDashJSON(w, 200, map[string]any{"status": "ok", "accepted": accepted, "rejected": rejected})
}

func (f *homeDashFixture) list(w http.ResponseWriter) {
	f.mu.Lock()
	out := []map[string]any{}
	for _, d := range f.dashboards {
		out = append(out, map[string]any{"id": d["id"], "slug": d["slug"], "name": d["name"], "revision": d["revision"], "approved_revision": d["approved_revision"], "updated_at": d["updated_at"], "saved_by": d["saved_by"]})
	}
	f.mu.Unlock()
	sort.Slice(out, func(i, j int) bool {
		a, b := strings.ToLower(out[i]["name"].(string)), strings.ToLower(out[j]["name"].(string))
		if a != b {
			return a < b
		}
		return out[i]["id"].(string) < out[j]["id"].(string)
	})
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboards": out})
}

func (f *homeDashFixture) create(w http.ResponseWriter, raw []byte) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var body homeDashCreateBody
	if json.Unmarshal(raw, &body) != nil || (body.Template == "") == (body.Document == nil) {
		homeDashError(w, 400, "bad_request", "send a template or a document")
		return
	}
	if body.UnassignedTitle != "" && body.Template != "home_assistant" {
		homeDashError(w, 400, "bad_request", "unassigned_title belongs to the home_assistant template")
		return
	}
	if body.Template == "home_assistant" && f.m3 != nil {
		f.createHA(w, body)
		return
	}
	var doc map[string]any
	if body.Template != "" {
		if body.Template != "system" {
			homeDashError(w, 400, "bad_request", "unknown template")
			return
		}
		doc = homeDashClone(f.starter)
		doc["name"] = "System"
		if body.Name != "" {
			doc["name"] = body.Name
		}
		doc["slug"] = f.uniqueSlugLocked("system")
	} else {
		name, _ := body.Document["name"].(string)
		pages, _ := body.Document["pages"].([]any)
		if name == "" || len(pages) == 0 {
			homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document", "issues": []any{map[string]any{"path": "/pages", "code": "required", "message": "at least one page"}}})
			return
		}
		page, _ := pages[0].(map[string]any)
		doc = map[string]any{"schema_version": 1, "name": name, "slug": f.uniqueSlugLocked(homeDashSlug(name)),
			"look":  map[string]any{"family": "glass", "palette": map[string]any{"mode": "theme"}, "backdrop": map[string]any{"kind": "theme"}, "density": "comfortable", "radius_scale": 1},
			"grid":  map[string]any{"row_unit": "auto", "gap": nil, "fit": "scroll"},
			"pages": []any{map[string]any{"id": page["id"], "name": page["name"], "compact": true, "widgets": []any{}, "layouts": map[string]any{"lg": []any{}}}},
			"kiosk": map[string]any{"start_page": page["id"], "idle_dim_seconds": 120, "idle_dim_level": 0.3, "page_rotation_seconds": 0, "screen_saver": "clock", "show_clock": true}}
	}
	f.seq++
	id := homeDashID(f.seq)
	if body.Template != "" && !f.templated {
		id, f.templated = homeDashTemplateID, true
	}
	doc["id"], doc["revision"], doc["approved_revision"], doc["updated_at"], doc["saved_by"] = id, 1, 1, homeDashNow(), "user"
	f.dashboards[id] = doc
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": 1, "approved_revision": 1}))
	homeDashJSON(w, 201, map[string]any{"status": "ok", "dashboard": doc, "warnings": []any{}})
}

func (f *homeDashFixture) get(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if !homeDashIDPattern.MatchString(id) {
		homeDashError(w, 400, "bad_request", "bad id")
		return
	}
	f.mu.Lock()
	var doc map[string]any
	if d := f.dashboards[id]; d != nil {
		doc = homeDashClone(d)
	}
	f.mu.Unlock()
	if doc == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	homeDashJSON(w, 200, map[string]any{"status": "ok", "dashboard": doc, "schema_too_new": false})
}

func (f *homeDashFixture) remove(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	f.mu.Lock()
	defer f.mu.Unlock()
	id := q.Get("id")
	rev, err := strconv.Atoi(q.Get("revision"))
	if !homeDashIDPattern.MatchString(id) || err != nil || rev < 1 {
		homeDashError(w, 400, "bad_request", "bad id or revision")
		return
	}
	doc := f.dashboards[id]
	if doc == nil {
		homeDashError(w, 404, "dashboard_not_found", "no such dashboard")
		return
	}
	if cur := homeDashInt(doc["revision"]); cur != rev {
		homeDashJSON(w, 409, map[string]any{"status": "error", "code": "revision_conflict", "message": "stale revision", "current_revision": cur})
		return
	}
	delete(f.dashboards, id)
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": 0, "approved_revision": 0}))
	homeDashJSON(w, 200, map[string]any{"status": "ok", "id": id})
}

// Test controls.
func (f *homeDashFixture) pushState(id string, value float64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.values[id] = value
	frame := homeDashFrame("state", map[string]any{"changes": map[string]any{id: map[string]any{"value": value, "available": true, "updated_at": homeDashNow()}}})
	for _, s := range f.streams {
		if s.subs[id] {
			f.send(s, frame)
		}
	}
}

func (f *homeDashFixture) setFamily(t *testing.T, id, family string) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	doc := f.dashboards[id]
	if doc == nil {
		t.Fatalf("no dashboard %s", id)
	}
	doc["look"].(map[string]any)["family"] = family
	rev := homeDashInt(doc["revision"]) + 1
	doc["revision"], doc["approved_revision"], doc["updated_at"] = rev, rev, homeDashNow()
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
}

func (f *homeDashFixture) dropStreams(refuseNext int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refuse = refuseNext
	for id, s := range f.streams {
		close(s.done)
		delete(f.streams, id)
	}
}

// revoke sends a revoked frame with the reason to every stream; each stream ends after writing it.
func (f *homeDashFixture) revoke(reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.broadcastLocked(homeDashFrame("revoked", map[string]any{"reason": reason}))
}

// setDisabled makes every route answer 503 homedash_disabled (new streams included).
func (f *homeDashFixture) setDisabled(on bool) { f.mu.Lock(); f.disabled = on; f.mu.Unlock() }

func (f *homeDashFixture) setReadonly(on bool) { f.mu.Lock(); f.readonly = on; f.mu.Unlock() }

// mutations counts the POST and DELETE requests the fixture received, refused ones included.
func (f *homeDashFixture) mutations() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, r := range f.requests {
		if strings.HasPrefix(r, "POST ") || strings.HasPrefix(r, "DELETE ") {
			n++
		}
	}
	return n
}

// subscribedIDs is the sorted subscription set of the newest open stream.
func (f *homeDashFixture) subscribedIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var newest *homeDashStream
	for _, s := range f.streams {
		if newest == nil || s.gen > newest.gen {
			newest = s
		}
	}
	out := []string{}
	if newest != nil {
		for id := range newest.subs {
			out = append(out, id)
		}
	}
	sort.Strings(out)
	return out
}

func (f *homeDashFixture) counts() (opened, refused int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.opened, f.refused
}

func (f *homeDashFixture) body(route string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.bodies[route]
}

func (f *homeDashFixture) findRequest(match func(string) bool) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, r := range f.requests {
		if match(r) {
			return r
		}
	}
	return ""
}

func (f *homeDashFixture) idByName(name string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for id, d := range f.dashboards {
		if d["name"] == name {
			return id
		}
	}
	return ""
}

// waitSubscribed waits until a stream opened after the first openedBefore streams holds n subscribed ids (openedBefore 0: any stream).
func (f *homeDashFixture) waitSubscribed(t *testing.T, openedBefore, n int, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		f.mu.Lock()
		for _, s := range f.streams {
			if s.gen > openedBefore && len(s.subs) == n {
				f.mu.Unlock()
				return
			}
		}
		f.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("no stream opened after #%d holds %d subscribed ids", openedBefore, n)
}
