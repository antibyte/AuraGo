package ui

import (
	"encoding/json"
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
	"unicode/utf8"
)

// M3 additions to the HomeDash fixture (contract "API contract (frozen for M3)" of docs/superpowers/plans/2026-11-25-homedash-m3-backend.md):
// the ha demo entities of hasstest.DemoOptions(), the commands route with expected states and state frames, the home_assistant template,
// source statuses, latency, and switches for every P1 path (delayed, dropped, duplicated or skewed confirmation; error answers).

// homeDashCommand is one command request the fixture received (logged before any answer, queued answers included).
type homeDashCommand struct {
	Entity, Command, RequestID, DashboardID, WidgetID string
	Args                                              map[string]any
	Confirmed                                         bool
	At                                                time.Time
}

// homeDashAnswer is a queued error answer: the next `times` commands for `entity` ("" = any entity) get status and body instead of the normal path.
type homeDashAnswer struct {
	status int
	body   map[string]any
	times  int
	entity string
}

// homeDashConfirm shapes the state frame that follows a 202: after delay; none when drop; twice when duplicate; skew attributes are merged into the frame.
type homeDashConfirm struct {
	delay           time.Duration
	drop, duplicate bool
	skew            map[string]any
}

type homeDashM3 struct {
	ha        map[string]map[string]any // id -> entity (wire shape, current state included)
	haDoc     map[string]any            // the dashboard object of the 201 example
	listed    bool                      // bootstrap.templates lists home_assistant
	omitted   int                       // "omitted" of the 201 answer
	mode      string                    // "", "empty" (422 template_empty), "down" (503 source_unavailable)
	sources   map[string]map[string]any
	commands  []homeDashCommand
	answers   []*homeDashAnswer
	confirm   homeDashConfirm
	latency   time.Duration
	snapDelay time.Duration
}

// enableM3 loads ha-entities.json and ha-dashboard.json; the ha source is idle, the confirmation delay is 80 ms, home_assistant is not listed.
func (f *homeDashFixture) enableM3(t *testing.T) {
	t.Helper()
	var list []map[string]any
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m3/ha-entities.json")), &list); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal([]byte(readDesktopAssetText(t, "testdata/homedash-m3/ha-dashboard.json")), &doc); err != nil {
		t.Fatal(err)
	}
	m := &homeDashM3{ha: map[string]map[string]any{}, haDoc: doc, sources: map[string]map[string]any{}, confirm: homeDashConfirm{delay: 80 * time.Millisecond}}
	for _, e := range list {
		m.ha[e["id"].(string)] = e
	}
	m.sources["ha"] = map[string]any{"source": "ha", "state": "idle", "at": homeDashNow()}
	f.mu.Lock()
	f.m3 = m
	f.mu.Unlock()
}

func homeDashM3Error(w http.ResponseWriter, status int, code, message string, extra map[string]any) {
	body := map[string]any{"status": "error", "code": code, "message": message}
	for k, v := range extra {
		body[k] = v
	}
	homeDashJSON(w, status, body)
}

func homeDashNum(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, !math.IsNaN(n) && !math.IsInf(n, 0)
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	}
	return 0, false
}

// homeDashCap finds a capability by name in an entity.
func homeDashCap(e map[string]any, name string) map[string]any {
	caps, _ := e["capabilities"].([]any)
	for _, c := range caps {
		if m, _ := c.(map[string]any); m != nil && m["name"] == name {
			return m
		}
	}
	return nil
}

func homeDashStrings(v any) []string {
	out := []string{}
	switch l := v.(type) {
	case []string:
		return l
	case []any:
		for _, x := range l {
			if s, ok := x.(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// command answers POST commands in the contract's order: body (400 bad_request), queued answer, entity (404), capability (400 unsupported_command),
// allowed (403 read_only + blocked_by), sensitive without confirmed (409), args (400 invalid_args), then 202 with expected and, after confirm.delay,
// a state frame (none when drop; twice when duplicate; skew attributes merged into the frame). The desktop read-only 403 is the route table's job.
func (f *homeDashFixture) command(w http.ResponseWriter, raw []byte) {
	var req struct {
		Entity      string         `json:"entity"`
		Command     string         `json:"command"`
		Args        map[string]any `json:"args"`
		RequestID   string         `json:"request_id"`
		DashboardID string         `json:"dashboard_id"`
		WidgetID    string         `json:"widget_id"`
		Confirmed   bool           `json:"confirmed"`
	}
	if json.Unmarshal(raw, &req) != nil || req.Entity == "" || req.Command == "" {
		homeDashM3Error(w, 400, "bad_request", "entity and command are required", nil)
		return
	}
	if n := utf8.RuneCountInString(req.RequestID); n < 1 || n > 64 {
		homeDashM3Error(w, 400, "bad_request", "request_id must be 1 to 64 characters", nil)
		return
	}
	if !homeDashEntityPattern.MatchString(req.Entity) {
		homeDashM3Error(w, 400, "bad_request", "malformed entity id", nil)
		return
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	m := f.m3
	if m == nil {
		homeDashM3Error(w, 404, "entity_not_found", "unknown entity", nil)
		return
	}
	m.commands = append(m.commands, homeDashCommand{Entity: req.Entity, Command: req.Command, RequestID: req.RequestID, DashboardID: req.DashboardID, WidgetID: req.WidgetID,
		Args: req.Args, Confirmed: req.Confirmed, At: time.Now()})
	for i, a := range m.answers {
		if a.times > 0 && (a.entity == "" || a.entity == req.Entity) {
			if a.times--; a.times == 0 {
				m.answers = append(m.answers[:i], m.answers[i+1:]...)
			}
			if ms, ok := homeDashNum(a.body["retry_after_ms"]); ok && a.status == http.StatusTooManyRequests {
				w.Header().Set("Retry-After", strconv.Itoa(int(math.Ceil(ms/1000))))
			}
			homeDashJSON(w, a.status, a.body)
			return
		}
	}
	e := m.ha[req.Entity]
	if e == nil {
		if _, ok := f.values[req.Entity]; ok { // a system entity: known, offers no command
			homeDashM3Error(w, 400, "unsupported_command", "the entity does not offer this command", nil)
			return
		}
		homeDashM3Error(w, 404, "entity_not_found", "unknown entity", nil)
		return
	}
	cp := homeDashCap(e, req.Command)
	if cp == nil {
		homeDashM3Error(w, 400, "unsupported_command", "the entity does not offer this command", nil)
		return
	}
	if allowed, _ := cp["allowed"].(bool); !allowed {
		by, _ := cp["blocked_by"].(string)
		if by == "" {
			by = "home_assistant.readonly"
		}
		homeDashM3Error(w, 403, "read_only", "commands are switched off", map[string]any{"blocked_by": by})
		return
	}
	if sensitive, _ := cp["sensitive"].(bool); sensitive && !req.Confirmed {
		homeDashM3Error(w, 409, "confirmation_required", "this command needs a confirmation", nil)
		return
	}
	if msg := homeDashArgsError(cp, req.Command, req.Args); msg != "" {
		homeDashM3Error(w, 400, "invalid_args", msg, nil)
		return
	}
	st, _ := e["state"].(map[string]any)
	next := homeDashApply(st, req.Command, req.Args)
	body := map[string]any{"status": "ok", "accepted": true, "superseded": false}
	if exp := homeDashExpected(st, req.Command, req.Args); exp != nil {
		body["expected"] = exp
	}
	f.scheduleConfirmLocked(req.Entity, next, m.confirm)
	homeDashJSON(w, http.StatusAccepted, body)
}

// scheduleConfirmLocked sends the state frame for next after c.delay unless c.drop. The timer takes f.mu itself; it never fires after f.quit.
func (f *homeDashFixture) scheduleConfirmLocked(id string, next map[string]any, c homeDashConfirm) {
	if c.drop {
		return
	}
	if len(c.skew) > 0 {
		attrs, _ := next["attributes"].(map[string]any)
		if attrs == nil {
			attrs = map[string]any{}
		}
		for k, v := range c.skew {
			attrs[k] = v
		}
		next["attributes"] = attrs
	}
	time.AfterFunc(c.delay, func() {
		select {
		case <-f.quit:
			return
		default:
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.m3 == nil || f.m3.ha[id] == nil {
			return
		}
		st := homeDashClone(next)
		st["updated_at"] = homeDashNow()
		f.m3.ha[id]["state"] = st
		frame := homeDashFrame("state", map[string]any{"changes": map[string]any{id: st}})
		times := 1
		if c.duplicate {
			times = 2
		}
		for i := 0; i < times; i++ {
			f.sendToSubscribersLocked(id, frame)
		}
	})
}

func (f *homeDashFixture) sendToSubscribersLocked(id, frame string) {
	for _, s := range f.streams {
		if s.subs[id] {
			f.send(s, frame)
		}
	}
}

// snapshotLater sends the snapshot of ids to stream s after d, when the stream still holds them (called with f.mu held).
func (f *homeDashFixture) snapshotLater(s *homeDashStream, ids []string, d time.Duration) {
	time.AfterFunc(d, func() {
		select {
		case <-f.quit:
			return
		default:
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.streams[s.id] != s {
			return
		}
		ents := []any{}
		for _, id := range ids {
			if s.subs[id] {
				ents = append(ents, f.entityLocked(id))
			}
		}
		if len(ents) == 0 {
			return
		}
		sources := []any{}
		if f.m3 != nil && f.m3.sources["ha"] != nil {
			sources = append(sources, homeDashClone(f.m3.sources["ha"]))
		}
		f.send(s, homeDashFrame("snapshot", map[string]any{"entities": ents, "sources": sources}))
	})
}

// createHA answers POST dashboards for the home_assistant template (called with f.mu held, like create). mode "down" -> 503 source_unavailable
// {"source":"ha"}; mode "empty" -> 422 template_empty; else the template doc with name (default "Home Assistant"), last page name = unassigned_title
// (default "Unassigned"), slug home-assistant(-n) and 201 with "omitted": m3.omitted.
func (f *homeDashFixture) createHA(w http.ResponseWriter, body homeDashCreateBody) {
	m := f.m3
	switch m.mode {
	case "down":
		homeDashM3Error(w, 503, "source_unavailable", "Home Assistant is not reachable", map[string]any{"source": "ha"})
		return
	case "empty":
		homeDashM3Error(w, 422, "template_empty", "Home Assistant has no entity this template can control", nil)
		return
	}
	name, title := body.Name, body.UnassignedTitle
	if name == "" {
		name = "Home Assistant"
	}
	if title == "" {
		title = "Unassigned"
	}
	for field, v := range map[string]string{"name": name, "unassigned_title": title} {
		limit := 80
		if field == "unassigned_title" {
			limit = 40
		}
		bad := utf8.RuneCountInString(v) > limit || strings.Contains(v, "://")
		for _, r := range v {
			bad = bad || r < 0x20 || r == 0x7f
		}
		if bad {
			homeDashJSON(w, 422, map[string]any{"status": "error", "code": "invalid_document", "message": "invalid document",
				"issues": []any{map[string]any{"path": "/" + field, "code": "bad_format", "message": field + " is not valid"}}})
			return
		}
	}
	doc := homeDashClone(m.haDoc)
	doc["name"] = name
	doc["slug"] = f.uniqueSlugLocked("home-assistant")
	if pages, _ := doc["pages"].([]any); len(pages) > 0 {
		if last, _ := pages[len(pages)-1].(map[string]any); last != nil {
			last["name"] = title
		}
	}
	f.seq++
	id := homeDashID(f.seq)
	if !f.templated {
		id, f.templated = homeDashTemplateID, true
	}
	doc["id"], doc["revision"], doc["approved_revision"], doc["updated_at"], doc["saved_by"] = id, 1, 1, homeDashNow(), "user"
	f.dashboards[id] = doc
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": 1, "approved_revision": 1}))
	homeDashJSON(w, 201, map[string]any{"status": "ok", "omitted": m.omitted, "warnings": []any{}, "dashboard": doc})
}

func homeDashOnOff(st map[string]any) (value, ok bool) {
	value, ok = st["value"].(bool)
	return
}

func homeDashExpState(value any, text string, attrs map[string]any) map[string]any {
	out := map[string]any{"value": value, "text": text, "available": true, "updated_at": homeDashNow()}
	if attrs != nil {
		out["attributes"] = attrs
	}
	return out
}

func homeDashOnText(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// homeDashExpected is the contract's expected table (nil for activate/press and for toggle from a state that is neither on nor off).
func homeDashExpected(st map[string]any, command string, args map[string]any) map[string]any {
	switch command {
	case "toggle":
		cur, ok := homeDashOnOff(st)
		if !ok {
			return nil
		}
		return homeDashExpState(!cur, homeDashOnText(!cur), nil)
	case "turn_on":
		return homeDashExpState(true, "on", nil)
	case "turn_off":
		return homeDashExpState(false, "off", nil)
	case "brightness":
		n, _ := homeDashNum(args["value"])
		if n <= 0 {
			return homeDashExpState(false, "off", nil)
		}
		return homeDashExpState(true, "on", map[string]any{"brightness_pct": n})
	case "color_temp":
		k, _ := homeDashNum(args["kelvin"])
		return homeDashExpState(true, "on", map[string]any{"color_temp_kelvin": k})
	case "color":
		h, _ := homeDashNum(args["h"])
		s, _ := homeDashNum(args["s"])
		return homeDashExpState(true, "on", map[string]any{"hs_color": []any{h, s}})
	case "effect":
		o, _ := args["option"].(string)
		return homeDashExpState(true, "on", map[string]any{"effect": o})
	case "select_option":
		o, _ := args["option"].(string)
		return homeDashExpState(o, o, nil)
	case "set_value":
		n, _ := homeDashNum(args["value"])
		return homeDashExpState(n, strconv.FormatFloat(n, 'f', -1, 64), nil)
	}
	return nil
}

// homeDashApply is the next state of a successful command: a clone of st with the command's effect (brightness 0 and turn_off clear brightness_pct;
// scene/button value = now). The input is not changed.
func homeDashApply(st map[string]any, command string, args map[string]any) map[string]any {
	next := homeDashClone(st)
	if next == nil {
		next = map[string]any{}
	}
	attrs, _ := next["attributes"].(map[string]any)
	setAttr := func(k string, v any) {
		if attrs == nil {
			attrs = map[string]any{}
		}
		attrs[k] = v
		next["attributes"] = attrs
	}
	off := func() {
		next["value"], next["text"] = false, "off"
		if attrs != nil {
			delete(attrs, "brightness_pct")
		}
	}
	on := func() { next["value"], next["text"] = true, "on" }
	next["available"] = true
	switch command {
	case "toggle":
		if cur, ok := homeDashOnOff(st); ok {
			if cur {
				off()
			} else {
				on()
			}
		}
	case "turn_on":
		if cur, _ := homeDashOnOff(st); !cur {
			on()
			if modes := homeDashStrings(attrs["supported_color_modes"]); len(modes) > 0 && !(len(modes) == 1 && modes[0] == "onoff") {
				if _, has := attrs["brightness_pct"]; !has {
					setAttr("brightness_pct", float64(100))
				}
			}
		}
	case "turn_off":
		off()
	case "brightness":
		n, _ := homeDashNum(args["value"])
		if n <= 0 {
			off()
		} else {
			on()
			setAttr("brightness_pct", n)
		}
	case "color_temp":
		k, _ := homeDashNum(args["kelvin"])
		on()
		setAttr("color_temp_kelvin", k)
		setAttr("color_mode", "color_temp")
	case "color":
		h, _ := homeDashNum(args["h"])
		s, _ := homeDashNum(args["s"])
		on()
		setAttr("hs_color", []any{h, s})
		setAttr("color_mode", "hs")
	case "effect":
		o, _ := args["option"].(string)
		on()
		setAttr("effect", o)
	case "select_option":
		o, _ := args["option"].(string)
		next["value"], next["text"] = o, o
	case "set_value":
		n, _ := homeDashNum(args["value"])
		next["value"], next["text"] = n, strconv.FormatFloat(n, 'f', -1, 64)
	case "activate", "press":
		now := homeDashNow()
		next["value"], next["text"] = now, now
	}
	next["updated_at"] = homeDashNow()
	return next
}

// homeDashArgsError checks the W1 arguments against the capability cp; "" when they are fine.
func homeDashArgsError(cp map[string]any, command string, args map[string]any) string {
	capNum := func(k string, def float64) float64 {
		if n, ok := homeDashNum(cp[k]); ok {
			return n
		}
		return def
	}
	switch command {
	case "brightness":
		n, ok := homeDashNum(args["value"])
		if !ok || n != math.Trunc(n) || n < 0 || n > 100 {
			return "value must be an integer from 0 to 100"
		}
	case "color_temp":
		k, ok := homeDashNum(args["kelvin"])
		if !ok {
			return "kelvin must be a number"
		}
		lo, hi, step := capNum("min", 2000), capNum("max", 6500), capNum("step", 50)
		if k < lo || k > hi {
			return "kelvin is out of range"
		}
		if q := (k - lo) / step; math.Abs(q-math.Round(q))*step > 1e-6*step {
			return "kelvin is not a multiple of " + strconv.FormatFloat(step, 'f', -1, 64)
		}
	case "color":
		h, hok := homeDashNum(args["h"])
		s, sok := homeDashNum(args["s"])
		if !hok || !sok || h < 0 || h > 360 || s < 0 || s > 100 {
			return "h must be 0 to 360 and s 0 to 100"
		}
	case "effect", "select_option":
		o, ok := args["option"].(string)
		if !ok {
			return "option must be a string"
		}
		for _, x := range homeDashStrings(cp["options"]) {
			if x == o {
				return ""
			}
		}
		return "option is not offered"
	case "set_value":
		v, ok := homeDashNum(args["value"])
		if !ok {
			return "value must be a number"
		}
		lo, hi, step := capNum("min", 0), capNum("max", 100), capNum("step", 1)
		if v < lo || v > hi {
			return "value is out of range"
		}
		if q := math.Round((v - lo) / step); math.Abs(v-(lo+q*step)) > 1e-6*step {
			return "value is not a multiple of " + strconv.FormatFloat(step, 'f', -1, 64)
		}
	}
	return ""
}

// Controls.

// answerNext makes the next `times` commands for entity ("" = any) answer status and body (a 429 body with retry_after_ms also sets Retry-After).
func (f *homeDashFixture) answerNext(status int, body map[string]any, times int, entity string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.m3.answers = append(f.m3.answers, &homeDashAnswer{status: status, body: body, times: times, entity: entity})
}

func (f *homeDashFixture) setConfirm(c homeDashConfirm) {
	f.mu.Lock()
	f.m3.confirm = c
	f.mu.Unlock()
}

func (f *homeDashFixture) commandLog() []homeDashCommand {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]homeDashCommand, len(f.m3.commands))
	copy(out, f.m3.commands)
	return out
}

// pushHAState stores st as the entity's state and sends a state frame to the streams subscribed to it.
func (f *homeDashFixture) pushHAState(id string, st map[string]any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	e := f.m3.ha[id]
	if e == nil {
		f.t.Errorf("pushHAState: unknown entity %s", id)
		return
	}
	st = homeDashClone(st)
	if _, ok := st["updated_at"]; !ok {
		st["updated_at"] = homeDashNow()
	}
	e["state"] = st
	f.sendToSubscribersLocked(id, homeDashFrame("state", map[string]any{"changes": map[string]any{id: st}}))
}

// setSource sets a source's status (bootstrap, snapshots) and sends a source frame to the streams that hold ids of src.
func (f *homeDashFixture) setSource(src, state, reason string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := map[string]any{"source": src, "state": state, "at": homeDashNow()}
	if reason != "" {
		s["reason"] = reason
	}
	f.m3.sources[src] = s
	frame := homeDashFrame("source", s)
	for _, st := range f.streams {
		for id := range st.subs {
			if strings.HasPrefix(id, src+":") {
				f.send(st, frame)
				break
			}
		}
	}
}

// setHATemplate sets whether bootstrap lists home_assistant, the "omitted" count of the 201 answer and the mode ("", "empty", "down").
func (f *homeDashFixture) setHATemplate(listed bool, omitted int, mode string) {
	f.mu.Lock()
	f.m3.listed, f.m3.omitted, f.m3.mode = listed, omitted, mode
	f.mu.Unlock()
}

func (f *homeDashFixture) setLatency(d time.Duration) { f.mu.Lock(); f.m3.latency = d; f.mu.Unlock() }

func (f *homeDashFixture) setSnapshotDelay(d time.Duration) {
	f.mu.Lock()
	f.m3.snapDelay = d
	f.mu.Unlock()
}

// setWidgetConfirm sets "confirm" of a widget and announces the new revision with a dashboard_changed frame.
func (f *homeDashFixture) setWidgetConfirm(t *testing.T, dashboardID, widgetID string, on bool) {
	t.Helper()
	f.mu.Lock()
	defer f.mu.Unlock()
	doc := f.dashboards[dashboardID]
	if doc == nil {
		t.Fatalf("no dashboard %s", dashboardID)
	}
	found := false
	pages, _ := doc["pages"].([]any)
	for _, p := range pages {
		page, _ := p.(map[string]any)
		widgets, _ := page["widgets"].([]any)
		for _, wd := range widgets {
			if m, _ := wd.(map[string]any); m != nil && m["id"] == widgetID {
				m["confirm"], found = on, true
			}
		}
	}
	if !found {
		t.Fatalf("no widget %s in %s", widgetID, dashboardID)
	}
	rev := homeDashInt(doc["revision"]) + 1
	doc["revision"], doc["approved_revision"], doc["updated_at"] = rev, rev, homeDashNow()
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": dashboardID, "revision": rev, "approved_revision": rev}))
}

// setScriptAllowed makes script.goodnight allowed (sensitive, not read-only) or blocked again by home_assistant.allowed_services, with an entity frame.
func (f *homeDashFixture) setScriptAllowed(on bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	const id = "ha:script.goodnight"
	e := f.m3.ha[id]
	if e == nil {
		f.t.Errorf("setScriptAllowed: %s missing", id)
		return
	}
	cp := homeDashCap(e, "activate")
	cp["allowed"], cp["sensitive"] = on, true
	if on {
		delete(cp, "blocked_by")
		e["readonly"] = false
		delete(e, "readonly_reason")
	} else {
		cp["blocked_by"] = "home_assistant.allowed_services"
		e["readonly"], e["readonly_reason"] = true, "home_assistant.allowed_services"
	}
	f.sendToSubscribersLocked(id, homeDashFrame("entity", map[string]any{"entities": []any{homeDashClone(e)}}))
}

// seedDashboard stores a copy of doc under a new id (slug made unique, revision 1 unless set) and returns the id.
func (f *homeDashFixture) seedDashboard(doc map[string]any) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	d := homeDashClone(doc)
	f.seq++
	id := homeDashID(f.seq)
	slug, _ := d["slug"].(string)
	if slug == "" {
		name, _ := d["name"].(string)
		slug = homeDashSlug(name)
	}
	rev := homeDashInt(d["revision"])
	if rev < 1 {
		rev = 1
	}
	d["id"], d["slug"], d["revision"], d["approved_revision"], d["updated_at"], d["saved_by"] = id, f.uniqueSlugLocked(slug), rev, rev, homeDashNow(), "user"
	f.dashboards[id] = d
	f.broadcastLocked(homeDashFrame("dashboard_changed", map[string]any{"id": id, "revision": rev, "approved_revision": rev}))
	return id
}

// haIDs lists the ha entity ids the fixture serves, sorted.
func (f *homeDashFixture) haIDs() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := []string{}
	for id := range f.m3.ha {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
