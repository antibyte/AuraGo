package ui

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Unit tests of fixture M3: no browser, the fixture is driven through httptest and a hand-made stream.

func homeDashM3Do(t *testing.T, f *homeDashFixture, method, path string, body any) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	var raw string
	if body != nil {
		b, _ := json.Marshal(body)
		raw = string(b)
	}
	rec := httptest.NewRecorder()
	f.ServeHTTP(rec, httptest.NewRequest(method, "/api/desktop/homedash/"+path, strings.NewReader(raw)))
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec, out
}

func homeDashM3Cmd(entity, command, id string, args map[string]any, confirmed bool) map[string]any {
	return map[string]any{"entity": entity, "command": command, "args": args, "request_id": id, "dashboard_id": "d_m3x7q2k5a4bc", "widget_id": "w2", "confirmed": confirmed}
}

// homeDashM3Stream registers a stream subscribed to ids without going through the SSE handler.
func homeDashM3Stream(f *homeDashFixture, ids ...string) *homeDashStream {
	s := &homeDashStream{id: "s1", gen: 1, frames: make(chan string, 16), done: make(chan struct{}), gone: make(chan struct{}), subs: map[string]bool{}}
	for _, id := range ids {
		s.subs[id] = true
	}
	f.mu.Lock()
	f.streams[s.id] = s
	f.mu.Unlock()
	return s
}

func homeDashM3Frame(t *testing.T, s *homeDashStream, within time.Duration) string {
	t.Helper()
	select {
	case fr := <-s.frames:
		return fr
	case <-time.After(within):
		return ""
	}
}

func homeDashM3Fixture(t *testing.T) *homeDashFixture {
	t.Helper()
	f := newHomeDashFixture(t)
	f.enableM3(t)
	return f
}

func TestHomeDashFixtureM3Command202AndStateFrame(t *testing.T) {
	f := homeDashM3Fixture(t)
	s := homeDashM3Stream(f, "ha:switch.coffee")
	start := time.Now()
	rec, body := homeDashM3Do(t, f, http.MethodPost, "commands", homeDashM3Cmd("ha:switch.coffee", "turn_on", "r_1", map[string]any{}, false))
	if rec.Code != 202 || body["accepted"] != true || body["superseded"] != false {
		t.Fatalf("answer %d %v", rec.Code, body)
	}
	exp, _ := body["expected"].(map[string]any)
	if exp["value"] != true || exp["text"] != "on" || exp["available"] != true || exp["updated_at"] == nil {
		t.Fatalf("expected %v", exp)
	}
	fr := homeDashM3Frame(t, s, 2*time.Second)
	if !strings.HasPrefix(fr, "event: state\n") || !strings.Contains(fr, `"ha:switch.coffee":{`) || !strings.Contains(fr, `"value":true`) {
		t.Fatalf("frame %q", fr)
	}
	if d := time.Since(start); d < 70*time.Millisecond {
		t.Fatalf("frame after %v, want the 80 ms confirm delay", d)
	}
	if log := f.commandLog(); len(log) != 1 || log[0].Command != "turn_on" || log[0].RequestID != "r_1" || log[0].WidgetID != "w2" {
		t.Fatalf("log %+v", log)
	}
	_, ent := homeDashM3Do(t, f, http.MethodGet, "entity?id=ha:switch.coffee", nil)
	if st := ent["entity"].(map[string]any)["state"].(map[string]any); st["value"] != true {
		t.Fatalf("stored state %v", st)
	}
}

func TestHomeDashFixtureM3ConfirmShapes(t *testing.T) {
	f := homeDashM3Fixture(t)
	s := homeDashM3Stream(f, "ha:light.kitchen")
	f.setConfirm(homeDashConfirm{delay: 10 * time.Millisecond, duplicate: true, skew: map[string]any{"color_temp_kelvin": 6300.0}})
	rec, body := homeDashM3Do(t, f, http.MethodPost, "commands", homeDashM3Cmd("ha:light.kitchen", "color_temp", "r_2", map[string]any{"kelvin": 6502}, false))
	if rec.Code != 202 {
		t.Fatalf("%d %v", rec.Code, body)
	}
	if k := body["expected"].(map[string]any)["attributes"].(map[string]any)["color_temp_kelvin"]; k != 6502.0 {
		t.Fatalf("expected kelvin %v", k)
	}
	first, second := homeDashM3Frame(t, s, 2*time.Second), homeDashM3Frame(t, s, 2*time.Second)
	if first == "" || first != second || !strings.Contains(first, `"color_temp_kelvin":6300`) {
		t.Fatalf("duplicate skewed frames %q / %q", first, second)
	}
	f.setConfirm(homeDashConfirm{delay: 10 * time.Millisecond, drop: true})
	homeDashM3Do(t, f, http.MethodPost, "commands", homeDashM3Cmd("ha:light.kitchen", "turn_off", "r_3", map[string]any{}, false))
	if fr := homeDashM3Frame(t, s, 200*time.Millisecond); fr != "" {
		t.Fatalf("dropped confirmation sent %q", fr)
	}
}

func TestHomeDashFixtureM3ErrorBodies(t *testing.T) {
	f := homeDashM3Fixture(t)
	post := func(c map[string]any) (*httptest.ResponseRecorder, map[string]any) {
		return homeDashM3Do(t, f, http.MethodPost, "commands", c)
	}
	check := func(name string, rec *httptest.ResponseRecorder, body map[string]any, status int, code string) {
		t.Helper()
		if rec.Code != status || body["code"] != code || body["status"] != "error" || body["message"] == "" {
			t.Fatalf("%s: %d %v", name, rec.Code, body)
		}
	}
	rec, body := post(homeDashM3Cmd("ha:switch.coffee", "turn_on", "", map[string]any{}, false))
	check("request_id", rec, body, 400, "bad_request")
	if body["message"] != "request_id must be 1 to 64 characters" {
		t.Fatalf("message %v", body["message"])
	}
	rec, body = post(homeDashM3Cmd("ha:light.nope", "turn_on", "r_1", nil, false))
	check("unknown entity", rec, body, 404, "entity_not_found")
	rec, body = post(homeDashM3Cmd("ha:scene.movie", "brightness", "r_2", map[string]any{"value": 5}, false))
	check("unsupported", rec, body, 400, "unsupported_command")
	rec, body = post(homeDashM3Cmd("ha:script.goodnight", "activate", "r_3", nil, false))
	check("read_only", rec, body, 403, "read_only")
	if body["blocked_by"] != "home_assistant.allowed_services" {
		t.Fatalf("blocked_by %v", body["blocked_by"])
	}
	f.setScriptAllowed(true)
	rec, body = post(homeDashM3Cmd("ha:script.goodnight", "activate", "r_4", nil, false))
	check("confirmation", rec, body, 409, "confirmation_required")
	rec, body = post(homeDashM3Cmd("ha:script.goodnight", "activate", "r_5", nil, true))
	if rec.Code != 202 || body["expected"] != nil {
		t.Fatalf("confirmed activate %d %v", rec.Code, body)
	}
	rec, body = post(homeDashM3Cmd("ha:light.kitchen", "color_temp", "r_6", map[string]any{"kelvin": 3000}, false))
	check("grid", rec, body, 400, "invalid_args")
	if body["message"] != "kelvin is not a multiple of 50" {
		t.Fatalf("message %v", body["message"])
	}
	for name, c := range map[string]map[string]any{
		"brightness": homeDashM3Cmd("ha:light.kitchen", "brightness", "r_7", map[string]any{"value": 101}, false),
		"option":     homeDashM3Cmd("ha:select.heater_mode", "select_option", "r_8", map[string]any{"option": "turbo"}, false),
		"step":       homeDashM3Cmd("ha:input_number.volume", "set_value", "r_9", map[string]any{"value": 33}, false),
	} {
		rec, body = post(c)
		check(name, rec, body, 400, "invalid_args")
	}
	rec, _ = post(homeDashM3Cmd("ha:input_number.volume", "set_value", "r_10", map[string]any{"value": 40}, false))
	if rec.Code != 202 {
		t.Fatalf("on-grid set_value %d", rec.Code)
	}
	// Queued answers: 429 with Retry-After, 503 with source, then the normal path again.
	f.answerNext(429, map[string]any{"status": "error", "code": "rate_limited", "message": "too many commands", "retry_after_ms": 3200}, 1, "ha:light.kitchen")
	f.answerNext(503, map[string]any{"status": "error", "code": "source_unavailable", "message": "Home Assistant is not reachable", "source": "ha"}, 1, "ha:switch.coffee")
	rec, body = post(homeDashM3Cmd("ha:light.kitchen", "brightness", "r_11", map[string]any{"value": 50}, false))
	check("429", rec, body, 429, "rate_limited")
	if rec.Header().Get("Retry-After") != "4" || body["retry_after_ms"] != 3200.0 {
		t.Fatalf("429 %v %v", rec.Header(), body)
	}
	rec, body = post(homeDashM3Cmd("ha:switch.coffee", "turn_on", "r_12", nil, false))
	check("503", rec, body, 503, "source_unavailable")
	if body["source"] != "ha" {
		t.Fatalf("source %v", body["source"])
	}
	if rec, _ = post(homeDashM3Cmd("ha:light.kitchen", "brightness", "r_13", map[string]any{"value": 50}, false)); rec.Code != 202 {
		t.Fatalf("queued answer not consumed: %d", rec.Code)
	}
	f.setReadonly(true)
	if rec, body = post(homeDashM3Cmd("ha:switch.coffee", "turn_on", "r_14", nil, false)); rec.Code != 403 || body["code"] != "desktop_readonly" {
		t.Fatalf("desktop read-only %d %v", rec.Code, body)
	}
}

func TestHomeDashFixtureM3ExpectedTable(t *testing.T) {
	on := map[string]any{"value": true, "text": "on"}
	text := map[string]any{"value": "x", "text": "x"}
	if e := homeDashExpected(on, "toggle", nil); e["value"] != false || e["text"] != "off" || e["available"] != true {
		t.Fatalf("toggle %v", e)
	}
	if homeDashExpected(text, "toggle", nil) != nil || homeDashExpected(on, "activate", nil) != nil || homeDashExpected(on, "press", nil) != nil {
		t.Fatal("toggle from text, activate and press have no expected")
	}
	if e := homeDashExpected(on, "brightness", map[string]any{"value": 0.0}); e["value"] != false || e["attributes"] != nil {
		t.Fatalf("brightness 0 %v", e)
	}
	if e := homeDashExpected(on, "set_value", map[string]any{"value": 55.0}); e["value"] != 55.0 || e["text"] != "55" {
		t.Fatalf("set_value %v", e)
	}
	next := homeDashApply(map[string]any{"value": true, "attributes": map[string]any{"brightness_pct": 40.0}}, "turn_off", nil)
	if next["value"] != false || next["attributes"].(map[string]any)["brightness_pct"] != nil {
		t.Fatalf("turn_off %v", next)
	}
}

func TestHomeDashFixtureM3TemplateAndBootstrap(t *testing.T) {
	f := homeDashM3Fixture(t)
	_, boot := homeDashM3Do(t, f, http.MethodGet, "bootstrap", nil)
	if got := boot["templates"].([]any); len(got) != 1 || got[0] != "system" {
		t.Fatalf("templates %v", got)
	}
	if src := boot["sources"].([]any); len(src) != 2 || src[1].(map[string]any)["source"] != "ha" || src[1].(map[string]any)["state"] != "idle" {
		t.Fatalf("sources %v", boot["sources"])
	}
	f.setHATemplate(true, 2, "")
	_, boot = homeDashM3Do(t, f, http.MethodGet, "bootstrap", nil)
	if got := boot["templates"].([]any); len(got) != 2 || got[1] != "home_assistant" {
		t.Fatalf("templates %v", got)
	}
	rec, body := homeDashM3Do(t, f, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant", "name": "Zuhause", "unassigned_title": "Ohne Raum"})
	if rec.Code != 201 || body["omitted"] != 2.0 {
		t.Fatalf("create %d %v", rec.Code, body)
	}
	doc := body["dashboard"].(map[string]any)
	pages := doc["pages"].([]any)
	if doc["name"] != "Zuhause" || doc["slug"] != "home-assistant" || doc["id"] != homeDashTemplateID || len(pages) != 3 || pages[2].(map[string]any)["name"] != "Ohne Raum" {
		t.Fatalf("doc %v", doc)
	}
	_, body = homeDashM3Do(t, f, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"})
	doc = body["dashboard"].(map[string]any)
	if doc["name"] != "Home Assistant" || doc["slug"] != "home-assistant-2" || doc["pages"].([]any)[2].(map[string]any)["name"] != "Unassigned" {
		t.Fatalf("defaults %v", doc)
	}
	if rec, body = homeDashM3Do(t, f, http.MethodPost, "dashboards", map[string]any{"template": "system", "unassigned_title": "x"}); rec.Code != 400 || body["code"] != "bad_request" {
		t.Fatalf("unassigned_title on system %d %v", rec.Code, body)
	}
	f.setHATemplate(true, 0, "empty")
	if rec, body = homeDashM3Do(t, f, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"}); rec.Code != 422 || body["code"] != "template_empty" {
		t.Fatalf("empty %d %v", rec.Code, body)
	}
	f.setHATemplate(true, 0, "down")
	if rec, body = homeDashM3Do(t, f, http.MethodPost, "dashboards", map[string]any{"template": "home_assistant"}); rec.Code != 503 || body["code"] != "source_unavailable" || body["source"] != "ha" {
		t.Fatalf("down %d %v", rec.Code, body)
	}
}

func TestHomeDashFixtureM3SubscribeSeedAndWidgetConfirm(t *testing.T) {
	f := homeDashM3Fixture(t)
	s := homeDashM3Stream(f)
	f.setSnapshotDelay(40 * time.Millisecond)
	rec, body := homeDashM3Do(t, f, http.MethodPut, "stream/subscriptions", map[string]any{"stream_id": "s1", "entities": []string{"ha:light.kitchen", "ha:light.nope", "bad:x"}})
	if rec.Code != 200 || len(body["accepted"].([]any)) != 1 || len(body["rejected"].([]any)) != 2 {
		t.Fatalf("subscribe %d %v", rec.Code, body)
	}
	if fr := homeDashM3Frame(t, s, 10*time.Millisecond); fr != "" {
		t.Fatalf("snapshot before the delay: %q", fr)
	}
	if fr := homeDashM3Frame(t, s, 2*time.Second); !strings.HasPrefix(fr, "event: snapshot\n") || !strings.Contains(fr, `"ha:light.kitchen"`) {
		t.Fatalf("snapshot %q", fr)
	}
	f.setSource("ha", "degraded", "unreachable")
	if fr := homeDashM3Frame(t, s, time.Second); !strings.HasPrefix(fr, "event: source\n") || !strings.Contains(fr, `"reason":"unreachable"`) {
		t.Fatalf("source frame %q", fr)
	}
	var starter map[string]any
	raw := readDesktopAssetText(t, "testdata/homedash-m2/starter-dashboard.json")
	if err := json.Unmarshal([]byte(raw), &starter); err != nil {
		t.Fatal(err)
	}
	id := f.seedDashboard(starter)
	if !homeDashIDPattern.MatchString(id) || id == homeDashTemplateID {
		t.Fatalf("seed id %q", id)
	}
	<-s.frames // dashboard_changed of the seed
	f.setWidgetConfirm(t, id, "cpu", true)
	if fr := homeDashM3Frame(t, s, time.Second); !strings.HasPrefix(fr, "event: dashboard_changed\n") || !strings.Contains(fr, `"revision":2`) {
		t.Fatalf("dashboard_changed %q", fr)
	}
	_, got := homeDashM3Do(t, f, http.MethodGet, "dashboard?id="+id, nil)
	cpu := got["dashboard"].(map[string]any)["pages"].([]any)[0].(map[string]any)["widgets"].([]any)[0].(map[string]any)
	if cpu["id"] != "cpu" || cpu["confirm"] != true || cpu["icon"] != "cpu" {
		t.Fatalf("cpu widget %v", cpu)
	}
}
