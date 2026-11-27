// Package hasstest is a scripted Home Assistant for tests. It speaks the websocket API HomeDash uses
// (auth, get_states, subscribe_events, the three registry lists, ping) and the REST endpoints
// (GET /api/states, POST /api/services/<domain>/<service>, GET /api/history/period/<from>,
// GET /api/camera_proxy/<entity_id>). Service calls are recorded and applied to the stored states
// the way Home Assistant would, and pushed as state_changed events.
//
// It is a normal (non-test) package so that the server's end-to-end test and the frontend's
// live-handler browser test (package ui) can import it. It must not import package hass.
package hasstest

import (
	"encoding/json"
	"maps"
	"math"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"aurago/internal/testutil"
)

// UpstreamSecret is the text error answers of the fake carry by default. Tests assert that it never
// reaches a HomeDash client.
const UpstreamSecret = "hasstest-upstream-detail-7f3c"

// State is one entity state as Home Assistant reports it.
type State struct {
	EntityID    string         `json:"entity_id"`
	State       string         `json:"state"`
	Attributes  map[string]any `json:"attributes"`
	LastChanged time.Time      `json:"last_changed"`
	LastUpdated time.Time      `json:"last_updated"`
}

// Area, Device and EntityEntry are registry rows; empty strings are sent as null.
type Area struct{ AreaID, Name string }
type Device struct{ ID, AreaID string }
type EntityEntry struct{ EntityID, DeviceID, AreaID, HiddenBy, DisabledBy string }

// HistoryPoint is one recorder entry.
type HistoryPoint struct {
	State string
	At    time.Time
}

// Call is one recorded service call.
type Call struct {
	Domain, Service string
	Data            map[string]any
}

// Options script the fake.
type Options struct {
	Token             string // default "hasstest-token"
	States            []State
	Areas             []Area
	Devices           []Device
	Entities          []EntityEntry
	History           map[string][]HistoryPoint
	Cameras           map[string][]byte
	CameraContentType string // default "image/jpeg"
	NoEvents          bool   // service calls push no state_changed event
	NoChangedStates   bool   // service calls answer [] instead of the changed states
	// FailStates makes the websocket get_states command answer success:false with a Home Assistant
	// shaped error object {"code":"unknown_error","message":"..."}. REST /api/states is unaffected.
	FailStates bool
	// FailRegistries makes the three websocket registry list commands (areas, devices, entities)
	// answer success:false with the same error shape.
	FailRegistries bool
	// ChangedStatesWrapped makes service calls answer the newer Home Assistant shape
	// {"changed_states":[...],"service_response":null} instead of the bare state list. With
	// NoChangedStates the wrapped list is empty.
	ChangedStatesWrapped bool
}

// Server is a running fake Home Assistant. URL and Token are set by New and never change.
type Server struct {
	URL   string
	Token string

	// Immutable after New (copied from the options there), so handlers read them without s.mu.
	noEvents        bool
	noChangedStates bool
	failStates      bool
	failRegistries  bool
	wrapChanged     bool
	history         map[string][]HistoryPoint
	cameras         map[string][]byte
	cameraType      string

	srv       *httptest.Server
	up        websocket.Upgrader
	closeOnce sync.Once
	handlers  sync.WaitGroup // running websocket handlers; Add only under mu while !closed

	mu          sync.Mutex
	closed      bool
	states      map[string]State
	order       []string
	areas       []Area
	devices     []Device
	entities    []EntityEntry
	conns       map[*wsConn]struct{}
	calls       []Call
	wsBlocked   bool
	failStatus  int
	failBody    string
	restStates  int
	wsConnects  int
	historyReqs int
	lastHistory string
}

type wsConn struct {
	mu   sync.Mutex // serialises writes and guards subs
	c    *websocket.Conn
	subs map[string]int64 // event type -> subscription id
}

// send writes one JSON message. Writes of one connection are serialised by c.mu; a failed write
// (closed connection) is ignored, the read loop notices the close.
func (c *wsConn) send(v any) {
	c.mu.Lock()
	defer c.mu.Unlock()
	_ = c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_ = c.c.WriteJSON(v)
}

// sendFailure answers a command with a Home Assistant shaped failure.
func (c *wsConn) sendFailure(id int64) {
	c.send(map[string]any{"id": id, "type": "result", "success": false,
		"error": map[string]any{"code": "unknown_error", "message": "Scripted failure."}})
}

func (c *wsConn) sub(event string) (int64, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	id, ok := c.subs[event]
	return id, ok
}

type change struct{ old, new State }

// New starts the fake on IPv4 loopback; it stops when the test ends.
func New(t testing.TB, o Options) *Server {
	t.Helper()
	if o.Token == "" {
		o.Token = "hasstest-token"
	}
	if o.CameraContentType == "" {
		o.CameraContentType = "image/jpeg"
	}
	s := &Server{Token: o.Token, noEvents: o.NoEvents, noChangedStates: o.NoChangedStates, failStates: o.FailStates, failRegistries: o.FailRegistries, wrapChanged: o.ChangedStatesWrapped, cameraType: o.CameraContentType,
		history: map[string][]HistoryPoint{}, cameras: map[string][]byte{},
		states: map[string]State{}, conns: map[*wsConn]struct{}{},
		areas: slices.Clone(o.Areas), devices: slices.Clone(o.Devices), entities: slices.Clone(o.Entities)}
	for id, pts := range o.History {
		s.history[id] = slices.Clone(pts)
	}
	for id, img := range o.Cameras {
		s.cameras[id] = slices.Clone(img)
	}
	s.up = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	for _, st := range o.States {
		s.putLocked(st)
	}
	s.srv = testutil.NewHTTPServer(t, http.HandlerFunc(s.serve))
	s.URL = s.srv.URL
	t.Cleanup(s.Close)
	return s
}

// putLocked stores a copy of st (the caller's attribute map is never kept). The caller holds s.mu or
// is New.
func (s *Server) putLocked(st State) {
	if st.LastUpdated.IsZero() {
		st.LastUpdated = time.Now().UTC()
	}
	if st.LastChanged.IsZero() {
		st.LastChanged = st.LastUpdated
	}
	if st.Attributes == nil {
		st.Attributes = map[string]any{}
	} else {
		st.Attributes = maps.Clone(st.Attributes)
	}
	if _, ok := s.states[st.EntityID]; !ok {
		s.order = append(s.order, st.EntityID)
	}
	s.states[st.EntityID] = st
}

// Close drops every websocket, stops the server (Home Assistant is gone) and waits until every
// websocket handler has returned. It is safe to call more than once and concurrently; every caller
// returns after the shutdown is complete.
func (s *Server) Close() {
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true // no new websocket registers from here on
		s.mu.Unlock()
		s.DropWebsockets()
		s.srv.Close()
		s.handlers.Wait()
	})
}

// DropWebsockets closes every websocket connection (a network blip).
func (s *Server) DropWebsockets() {
	for _, c := range s.connList() {
		_ = c.c.Close()
	}
}

func (s *Server) connList() []*wsConn {
	s.mu.Lock()
	defer s.mu.Unlock()
	conns := make([]*wsConn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	return conns
}

// BlockWebsocket makes the websocket handshake answer 404 (a proxy that blocks websockets).
func (s *Server) BlockWebsocket(block bool) { s.mu.Lock(); s.wsBlocked = block; s.mu.Unlock() }

// FailServices makes service calls answer status with {"message": body} (0 = normal; an empty body
// is UpstreamSecret).
func (s *Server) FailServices(status int, body string) {
	if body == "" {
		body = UpstreamSecret
	}
	s.mu.Lock()
	s.failStatus, s.failBody = status, body
	s.mu.Unlock()
}

func (s *Server) Calls() []Call            { s.mu.Lock(); defer s.mu.Unlock(); return slices.Clone(s.calls) }
func (s *Server) WebsocketConnects() int   { s.mu.Lock(); defer s.mu.Unlock(); return s.wsConnects }
func (s *Server) OpenWebsockets() int      { s.mu.Lock(); defer s.mu.Unlock(); return len(s.conns) }
func (s *Server) RESTStateRequests() int   { s.mu.Lock(); defer s.mu.Unlock(); return s.restStates }
func (s *Server) HistoryRequests() int     { s.mu.Lock(); defer s.mu.Unlock(); return s.historyReqs }
func (s *Server) LastHistoryQuery() string { s.mu.Lock(); defer s.mu.Unlock(); return s.lastHistory }

// State returns one stored state.
func (s *Server) State(id string) (State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, ok := s.states[id]
	st.Attributes = maps.Clone(st.Attributes)
	return st, ok
}

// SetState stores a state as an integration update would and, with push, sends state_changed to
// every websocket subscribed to it.
func (s *Server) SetState(st State, push bool) {
	if st.LastUpdated.IsZero() {
		st.LastUpdated = time.Now().UTC()
	}
	s.mu.Lock()
	old, had := s.states[st.EntityID]
	s.putLocked(st)
	st = s.states[st.EntityID]
	s.mu.Unlock()
	if push {
		var prev *State
		if had {
			prev = &old
		}
		s.pushEvent("state_changed", map[string]any{"entity_id": st.EntityID, "old_state": prev, "new_state": &st})
	}
}

// SetEntityArea sets the area of one entity registry entry (adding it when missing) and sends
// entity_registry_updated.
func (s *Server) SetEntityArea(entityID, areaID string) {
	s.mu.Lock()
	i := slices.IndexFunc(s.entities, func(e EntityEntry) bool { return e.EntityID == entityID })
	if i < 0 {
		s.entities = append(s.entities, EntityEntry{EntityID: entityID})
		i = len(s.entities) - 1
	}
	s.entities[i].AreaID = areaID
	s.mu.Unlock()
	s.pushEvent("entity_registry_updated", map[string]any{"action": "update", "entity_id": entityID})
}

// pushEvent sends one event to every connection subscribed to its type. Payloads hold only values
// that are never mutated after they are stored (attribute maps are cloned on write).
func (s *Server) pushEvent(event string, data any) {
	for _, c := range s.connList() {
		if id, ok := c.sub(event); ok {
			c.send(map[string]any{"id": id, "type": "event",
				"event": map[string]any{"event_type": event, "data": data, "origin": "LOCAL", "time_fired": time.Now().UTC()}})
		}
	}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/api/websocket" {
		s.serveWebsocket(w, r)
		return
	}
	if r.Header.Get("Authorization") != "Bearer "+s.Token {
		writeJSON(w, http.StatusUnauthorized, map[string]any{"message": "Unauthorized"})
		return
	}
	p := r.URL.Path
	switch {
	case r.Method == http.MethodGet && p == "/api/states":
		s.mu.Lock()
		s.restStates++
		s.mu.Unlock()
		writeJSON(w, http.StatusOK, s.stateList())
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/states/"):
		st, ok := s.State(strings.TrimPrefix(p, "/api/states/"))
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]any{"message": "Entity not found."})
			return
		}
		writeJSON(w, http.StatusOK, st)
	case r.Method == http.MethodPost && strings.HasPrefix(p, "/api/services/"):
		s.serveService(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/history/period/"):
		s.serveHistory(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "/api/camera_proxy/"):
		img, ok := s.cameras[strings.TrimPrefix(p, "/api/camera_proxy/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", s.cameraType)
		_, _ = w.Write(img)
	default:
		http.NotFound(w, r)
	}
}

func (s *Server) stateList() []State {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]State, 0, len(s.order))
	for _, id := range s.order {
		if st, ok := s.states[id]; ok {
			out = append(out, st)
		}
	}
	return out
}

func nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func (s *Server) registry(kind string) []map[string]any {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := []map[string]any{}
	switch kind {
	case "areas":
		for _, a := range s.areas {
			out = append(out, map[string]any{"area_id": a.AreaID, "name": a.Name})
		}
	case "devices":
		for _, d := range s.devices {
			out = append(out, map[string]any{"id": d.ID, "area_id": nullable(d.AreaID)})
		}
	default:
		for _, e := range s.entities {
			out = append(out, map[string]any{"entity_id": e.EntityID, "device_id": nullable(e.DeviceID), "area_id": nullable(e.AreaID),
				"hidden_by": nullable(e.HiddenBy), "disabled_by": nullable(e.DisabledBy)})
		}
	}
	return out
}

func (s *Server) serveService(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.NotFound(w, r)
		return
	}
	var data map[string]any
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid JSON"})
		return
	}
	s.mu.Lock()
	s.calls = append(s.calls, Call{Domain: parts[0], Service: parts[1], Data: data})
	if s.failStatus != 0 {
		code, body := s.failStatus, s.failBody
		s.mu.Unlock()
		writeJSON(w, code, map[string]any{"message": body})
		return
	}
	changes, code := s.applyLocked(parts[0], parts[1], data, time.Now().UTC())
	s.mu.Unlock()
	if code != http.StatusOK {
		writeJSON(w, code, map[string]any{"message": UpstreamSecret})
		return
	}
	out := []State{}
	for _, c := range changes {
		if !s.noEvents {
			s.pushEvent("state_changed", map[string]any{"entity_id": c.new.EntityID, "old_state": &c.old, "new_state": &c.new})
		}
		if !s.noChangedStates {
			out = append(out, c.new)
		}
	}
	if s.wrapChanged {
		writeJSON(w, http.StatusOK, map[string]any{"changed_states": out, "service_response": nil})
		return
	}
	writeJSON(w, http.StatusOK, out)
}

func stringsOf(v any) []string {
	raw, _ := v.([]any)
	out := []string{}
	for _, e := range raw {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func number(v any) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	}
	return 0, false
}

// applyLocked applies a service call like Home Assistant does for the domains HomeDash controls.
// The caller holds s.mu. Stored attribute maps are never mutated: the new state gets a clone.
func (s *Server) applyLocked(domain, service string, data map[string]any, now time.Time) ([]change, int) {
	id, _ := data["entity_id"].(string)
	old, ok := s.states[id]
	if !ok || !strings.HasPrefix(id, domain+".") {
		return nil, http.StatusBadRequest
	}
	st := old
	st.Attributes = maps.Clone(old.Attributes)
	if st.Attributes == nil {
		st.Attributes = map[string]any{}
	}
	switch service {
	case "turn_on":
		switch domain {
		case "scene":
			st.State = now.Format(time.RFC3339Nano)
		case "script": // runs and ends at once: the state stays "off"
		default:
			st.State = "on"
		}
		if domain == "light" {
			applyLight(st.Attributes, data, &st.State)
		}
	case "turn_off":
		st.State = "off"
		if domain == "light" {
			st.Attributes["brightness"] = nil
		}
	case "toggle":
		if st.State == "on" {
			st.State = "off"
			if domain == "light" {
				st.Attributes["brightness"] = nil
			}
		} else {
			st.State = "on"
		}
	case "select_option":
		opt, _ := data["option"].(string)
		if !slices.Contains(stringsOf(st.Attributes["options"]), opt) {
			return nil, http.StatusBadRequest
		}
		st.State = opt
	case "set_value":
		v, ok := number(data["value"])
		if !ok {
			return nil, http.StatusBadRequest
		}
		st.State = strconv.FormatFloat(v, 'f', 1, 64)
	case "set_temperature":
		if domain != "climate" {
			return nil, http.StatusBadRequest
		}
		if v, ok := number(data["temperature"]); ok {
			st.Attributes["temperature"] = v
		}
		if v, ok := number(data["target_temp_low"]); ok {
			st.Attributes["target_temp_low"] = v
		}
		if v, ok := number(data["target_temp_high"]); ok {
			st.Attributes["target_temp_high"] = v
		}
	case "set_hvac_mode":
		mode, _ := data["hvac_mode"].(string)
		if domain != "climate" || !slices.Contains(stringsOf(st.Attributes["hvac_modes"]), mode) {
			return nil, http.StatusBadRequest
		}
		st.State = mode
	case "press":
		st.State = now.Format(time.RFC3339Nano)
	default:
		return nil, http.StatusBadRequest
	}
	st.LastUpdated = now
	if st.State != old.State {
		st.LastChanged = now
	}
	s.states[id] = st
	return []change{{old: old, new: st}}, http.StatusOK
}

func applyLight(attrs, data map[string]any, state *string) {
	if v, ok := number(data["brightness_pct"]); ok {
		if v <= 0 {
			*state, attrs["brightness"] = "off", nil
			return
		}
		attrs["brightness"] = math.Round(v * 255 / 100)
	} else if attrs["brightness"] == nil && slices.ContainsFunc(stringsOf(attrs["supported_color_modes"]), func(m string) bool { return m != "onoff" }) {
		attrs["brightness"] = 255.0
	}
	if v, ok := number(data["color_temp_kelvin"]); ok {
		attrs["color_temp_kelvin"], attrs["color_mode"] = v, "color_temp"
	}
	if v, ok := data["hs_color"].([]any); ok && len(v) == 2 {
		attrs["hs_color"], attrs["color_mode"] = v, "hs"
	}
	if v, ok := data["effect"].(string); ok {
		attrs["effect"] = v
	}
}

func (s *Server) serveHistory(w http.ResponseWriter, r *http.Request) {
	from, err := time.Parse(time.RFC3339, strings.TrimPrefix(r.URL.Path, "/api/history/period/"))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid datetime"})
		return
	}
	q := r.URL.Query()
	end := from.Add(24 * time.Hour) // Home Assistant's default window
	if v := q.Get("end_time"); v != "" {
		if end, err = time.Parse(time.RFC3339, v); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]any{"message": "Invalid end_time"})
			return
		}
	}
	s.mu.Lock()
	s.historyReqs++
	s.lastHistory = r.URL.RawQuery
	s.mu.Unlock()
	out := []any{}
	for _, id := range strings.Split(q.Get("filter_entity_id"), ",") {
		var series []map[string]any
		for _, p := range s.history[id] {
			if p.At.Before(from) || p.At.After(end) {
				continue
			}
			e := map[string]any{"state": p.State, "last_changed": p.At.UTC().Format(time.RFC3339Nano)}
			if len(series) == 0 {
				e["entity_id"] = id
			}
			series = append(series, e)
		}
		if len(series) > 0 {
			out = append(out, series)
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) serveWebsocket(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	blocked := s.wsBlocked || s.closed
	s.mu.Unlock()
	if blocked {
		http.NotFound(w, r)
		return
	}
	c, err := s.up.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	c.SetReadLimit(1 << 20)
	conn := &wsConn{c: c, subs: map[string]int64{}}
	s.mu.Lock()
	if s.closed { // Close started between the check above and the upgrade
		s.mu.Unlock()
		_ = c.Close()
		return
	}
	s.handlers.Add(1)
	s.wsConnects++
	s.conns[conn] = struct{}{}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.conns, conn)
		s.mu.Unlock()
		_ = c.Close()
		s.handlers.Done()
	}()
	conn.send(map[string]any{"type": "auth_required", "ha_version": "2026.11.0"})
	_ = c.SetReadDeadline(time.Now().Add(10 * time.Second)) // an unauthenticated peer cannot hold the handler
	var auth struct {
		Type        string `json:"type"`
		AccessToken string `json:"access_token"`
	}
	if c.ReadJSON(&auth) != nil {
		return
	}
	if auth.Type != "auth" || auth.AccessToken != s.Token {
		conn.send(map[string]any{"type": "auth_invalid", "message": "Invalid access token or password"})
		return
	}
	_ = c.SetReadDeadline(time.Time{})
	conn.send(map[string]any{"type": "auth_ok", "ha_version": "2026.11.0"})
	for {
		var m struct {
			ID        int64  `json:"id"`
			Type      string `json:"type"`
			EventType string `json:"event_type"`
		}
		if c.ReadJSON(&m) != nil {
			return
		}
		var result any
		switch m.Type {
		case "ping":
			conn.send(map[string]any{"id": m.ID, "type": "pong"})
			continue
		case "subscribe_events":
			conn.mu.Lock()
			conn.subs[m.EventType] = m.ID
			conn.mu.Unlock()
		case "get_states":
			if s.failStates {
				conn.sendFailure(m.ID)
				continue
			}
			result = s.stateList()
		case "config/area_registry/list":
			if s.failRegistries {
				conn.sendFailure(m.ID)
				continue
			}
			result = s.registry("areas")
		case "config/device_registry/list":
			if s.failRegistries {
				conn.sendFailure(m.ID)
				continue
			}
			result = s.registry("devices")
		case "config/entity_registry/list":
			if s.failRegistries {
				conn.sendFailure(m.ID)
				continue
			}
			result = s.registry("entities")
		default:
			conn.send(map[string]any{"id": m.ID, "type": "result", "success": false, "error": map[string]any{"code": "unknown_command", "message": "Unknown command."}})
			continue
		}
		conn.send(map[string]any{"id": m.ID, "type": "result", "success": true, "result": result})
	}
}
