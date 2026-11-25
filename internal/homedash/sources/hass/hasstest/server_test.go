package hasstest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func get(t *testing.T, s *Server, path, token string) *http.Response {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, s.URL+path, nil)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	return resp
}

func post(t *testing.T, s *Server, path string, body any) (*http.Response, []State) {
	t.Helper()
	raw, _ := json.Marshal(body)
	req, _ := http.NewRequest(http.MethodPost, s.URL+path, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+s.Token)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out []State
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp, out
}

func TestFakeServesRESTAndRecordsServiceCalls(t *testing.T) {
	s := New(t, DemoOptions())
	if r := get(t, s, "/api/states", ""); r.StatusCode != http.StatusUnauthorized {
		t.Fatalf("no token = %d", r.StatusCode)
	}
	var states []State
	if err := json.NewDecoder(get(t, s, "/api/states", s.Token).Body).Decode(&states); err != nil || len(states) != 15 || s.RESTStateRequests() != 1 {
		t.Fatalf("states = %d %v", len(states), err)
	}
	r, changed := post(t, s, "/api/services/switch/toggle", map[string]any{"entity_id": "switch.coffee"})
	if r.StatusCode != 200 || len(changed) != 1 || changed[0].State != "on" {
		t.Fatalf("toggle = %d %+v", r.StatusCode, changed)
	}
	if c := s.Calls(); len(c) != 1 || c[0].Domain != "switch" || c[0].Service != "toggle" || c[0].Data["entity_id"] != "switch.coffee" {
		t.Fatalf("calls = %+v", c)
	}
	_, changed = post(t, s, "/api/services/light/turn_on", map[string]any{"entity_id": "light.kitchen", "brightness_pct": 60})
	if len(changed) != 1 || changed[0].Attributes["brightness"] != 153.0 {
		t.Fatalf("brightness = %+v", changed)
	}
	s.FailServices(http.StatusServiceUnavailable, "")
	if r, _ := post(t, s, "/api/services/switch/toggle", map[string]any{"entity_id": "switch.coffee"}); r.StatusCode != 503 {
		t.Fatalf("failure switch = %d", r.StatusCode)
	}
	s.FailServices(0, "")
	h := get(t, s, "/api/history/period/2026-11-25T06:00:00Z?filter_entity_id=switch.coffee&end_time=2026-11-25T10:00:00Z&minimal_response&no_attributes", s.Token)
	var series [][]map[string]any
	if err := json.NewDecoder(h.Body).Decode(&series); err != nil || len(series) != 1 || len(series[0]) != 2 || !strings.Contains(s.LastHistoryQuery(), "minimal_response") {
		t.Fatalf("history = %+v %v", series, err)
	}
	if c := get(t, s, "/api/camera_proxy/camera.door", s.Token); c.StatusCode != 200 || c.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatalf("camera = %d %s", c.StatusCode, c.Header.Get("Content-Type"))
	}
}

func TestFakeWebsocketAuthStatesAndEvents(t *testing.T) {
	s := New(t, DemoOptions())
	url := "ws" + strings.TrimPrefix(s.URL, "http") + "/api/websocket"
	dial := func(token string) (*websocket.Conn, string) {
		c, _, err := websocket.DefaultDialer.Dial(url, nil)
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		_ = c.ReadJSON(&m)
		_ = c.WriteJSON(map[string]any{"type": "auth", "access_token": token})
		_ = c.ReadJSON(&m)
		return c, m["type"].(string)
	}
	if c, typ := dial("wrong"); typ != "auth_invalid" {
		t.Fatalf("wrong token = %s", typ)
	} else {
		c.Close()
	}
	c, typ := dial(s.Token)
	defer c.Close()
	if typ != "auth_ok" {
		t.Fatalf("auth = %s", typ)
	}
	_ = c.WriteJSON(map[string]any{"id": 1, "type": "get_states"})
	var res struct {
		ID     int64   `json:"id"`
		Result []State `json:"result"`
	}
	if err := c.ReadJSON(&res); err != nil || res.ID != 1 || len(res.Result) != 15 {
		t.Fatalf("get_states = %+v %v", res, err)
	}
	_ = c.WriteJSON(map[string]any{"id": 2, "type": "subscribe_events", "event_type": "state_changed"})
	_ = c.ReadJSON(&map[string]any{})
	s.SetState(State{EntityID: "switch.coffee", State: "on"}, true)
	var ev struct {
		ID    int64 `json:"id"`
		Event struct {
			EventType string `json:"event_type"`
			Data      struct {
				NewState State `json:"new_state"`
			} `json:"data"`
		} `json:"event"`
	}
	if err := c.ReadJSON(&ev); err != nil || ev.ID != 2 || ev.Event.EventType != "state_changed" || ev.Event.Data.NewState.State != "on" {
		t.Fatalf("event = %+v %v", ev, err)
	}
	s.BlockWebsocket(true)
	if _, _, err := websocket.DefaultDialer.Dial(url, nil); err == nil {
		t.Fatal("blocked websocket accepted a handshake")
	}
}

func dialAuthed(t *testing.T, s *Server) *websocket.Conn {
	t.Helper()
	c, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(s.URL, "http")+"/api/websocket", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Close() })
	_ = c.SetReadDeadline(time.Now().Add(5 * time.Second))
	var m map[string]any
	_ = c.ReadJSON(&m)
	_ = c.WriteJSON(map[string]any{"type": "auth", "access_token": s.Token})
	if err := c.ReadJSON(&m); err != nil || m["type"] != "auth_ok" {
		t.Fatalf("auth = %v %v", m, err)
	}
	return c
}

func TestFakeWebsocketRegistriesPingAndRegistryEvents(t *testing.T) {
	s := New(t, DemoOptions())
	c := dialAuthed(t, s)
	type reg struct {
		ID      int64            `json:"id"`
		Type    string           `json:"type"`
		Success bool             `json:"success"`
		Result  []map[string]any `json:"result"`
	}
	ask := func(id int64, typ string) reg {
		_ = c.WriteJSON(map[string]any{"id": id, "type": typ})
		var r reg
		if err := c.ReadJSON(&r); err != nil {
			t.Fatal(err)
		}
		return r
	}
	if r := ask(1, "config/area_registry/list"); !r.Success || len(r.Result) != 3 || r.Result[0]["name"] != "Kitchen" {
		t.Fatalf("areas = %+v", r)
	}
	if r := ask(2, "config/device_registry/list"); !r.Success || len(r.Result) != 2 {
		t.Fatalf("devices = %+v", r)
	}
	r := ask(3, "config/entity_registry/list")
	if !r.Success || len(r.Result) != 8 {
		t.Fatalf("entities = %+v", r)
	}
	for _, e := range r.Result {
		if e["entity_id"] == "input_boolean.guest_mode" && e["hidden_by"] != "user" {
			t.Fatalf("guest_mode hidden_by = %v", e["hidden_by"])
		}
		if e["entity_id"] == "switch.coffee" && (e["hidden_by"] != nil || e["device_id"] != nil) {
			t.Fatalf("coffee row = %v", e)
		}
	}
	if r := ask(4, "ping"); r.Type != "pong" || r.ID != 4 {
		t.Fatalf("ping = %+v", r)
	}
	if r := ask(5, "bogus/command"); r.Success || r.Type != "result" {
		t.Fatalf("unknown command = %+v", r)
	}
	_ = c.WriteJSON(map[string]any{"id": 6, "type": "subscribe_events", "event_type": "entity_registry_updated"})
	_ = c.ReadJSON(&map[string]any{})
	s.SetEntityArea("light.sofa", "kitchen")
	var ev struct {
		ID    int64 `json:"id"`
		Event struct {
			EventType string `json:"event_type"`
		} `json:"event"`
	}
	if err := c.ReadJSON(&ev); err != nil || ev.ID != 6 || ev.Event.EventType != "entity_registry_updated" {
		t.Fatalf("registry event = %+v %v", ev, err)
	}
	r = ask(7, "config/entity_registry/list")
	for _, e := range r.Result {
		if e["entity_id"] == "light.sofa" && e["area_id"] != "kitchen" {
			t.Fatalf("sofa area after SetEntityArea = %v", e["area_id"])
		}
	}
}

func TestFakeCloseAndDropAreRaceFreeAndIdempotent(t *testing.T) {
	s := New(t, DemoOptions())
	conns := make([]*websocket.Conn, 0, 3)
	for range 3 {
		conns = append(conns, dialAuthed(t, s))
	}
	if n := s.OpenWebsockets(); n != 3 {
		t.Fatalf("open websockets = %d", n)
	}
	var wg sync.WaitGroup
	for i := range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			switch i % 3 {
			case 0:
				s.DropWebsockets()
			case 1:
				s.SetState(State{EntityID: "switch.coffee", State: "on"}, true)
			default:
				s.Close()
			}
		}()
	}
	wg.Wait()
	s.Close() // again, plus the test cleanup
	if n := s.OpenWebsockets(); n != 0 {
		t.Fatalf("open websockets after Close = %d", n)
	}
	for _, c := range conns {
		_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
		if _, _, err := c.ReadMessage(); err == nil {
			t.Fatal("connection still readable after Close")
		}
	}
}
