package server

import (
	"aurago/internal/security"
	"aurago/internal/systemworld"
	"aurago/internal/tools"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestSystemWorldReducerUnknownZeroResetAndPrivacy(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	w := s.worldRuntime()
	w.observe(EventSystemMetrics, map[string]any{"cpu": map[string]any{"usage_percent": 0}, "uptime_seconds": 7})
	snap := w.snapshot()
	if v, ok := snap.Metrics["cpu"]; !ok || v != 0 {
		t.Fatal("measured zero lost")
	}
	if _, ok := snap.Metrics["ram"]; ok {
		t.Fatal("missing RAM fabricated")
	}
	w.observe(EventSystemMetrics, map[string]any{"cpu": map[string]any{"usage_percent": 0}, "available": map[string]bool{"cpu": false}})
	if _, ok := w.snapshot().Metrics["cpu"]; ok {
		t.Fatal("failed measurement became zero")
	}
	w.mu.Lock()
	w.netAt = time.Now().Add(-10 * time.Second).UnixMilli()
	w.previousNet = [2]float64{1000, 2000}
	w.mu.Unlock()
	w.observe(EventSystemMetrics, map[string]any{"network": map[string]int{"bytes_sent": 100, "bytes_recv": 200}})
	if _, ok := w.snapshot().Metrics["network_sent"]; ok {
		t.Fatal("counter reset emitted rate")
	}
	w.mu.Lock()
	w.netAt = time.Now().Add(-10 * time.Second).UnixMilli()
	w.mu.Unlock()
	w.observe(EventSystemMetrics, map[string]any{"network": map[string]int{"bytes_sent": 200, "bytes_recv": 400}})
	if w.snapshot().Metrics["network_sent"] < 9 || w.snapshot().Metrics["network_sent"] > 11 {
		t.Fatal("rate not derived from elapsed time")
	}
	secret := "world-private-credential"
	security.RegisterSensitive(secret)
	w.observe(EventContainerUpdate, []map[string]any{{"id": "demo", "name": secret, "state": "running", "logs": "PRIVATE OUTPUT"}})
	w.observe(EventAgentAction, map[string]any{"tool_name": "docker", "state": "started", "result": "PRIVATE OUTPUT", "summary": "PRIVATE OUTPUT", "reasoning": "PRIVATE OUTPUT"})
	w.observe(SSEEventType("llm_stream_delta"), map[string]any{"content": "PRIVATE OUTPUT"})
	encoded, _ := json.Marshal(w.snapshot())
	if strings.Contains(string(encoded), secret) || strings.Contains(string(encoded), "PRIVATE OUTPUT") {
		t.Fatal("private payload persisted")
	}
	if len(w.snapshot().Entities) != 9 {
		t.Fatalf("missing districts: %+v", w.snapshot())
	}
}

func TestSystemWorldPermissionsAndActionsIdempotent(t *testing.T) {
	tools.ConfigureRuntimePermissions(tools.RuntimePermissions{DockerEnabled: true})
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	gated, read, write := testDesktopPermissionServer(t)
	for _, token := range []string{read, write} {
		for _, kind := range []string{"snapshot", "history", "events", "entity", "actions"} {
			method := "GET"
			if kind == "actions" {
				method = "POST"
			}
			r := httptest.NewRequest(method, "/api/desktop/system-world/"+kind, strings.NewReader(`{}`))
			r.Header.Set("Authorization", "Bearer "+token)
			rec := httptest.NewRecorder()
			if kind == "actions" {
				handleSystemWorldAction(gated)(rec, r)
			} else {
				handleSystemWorldRead(gated, kind)(rec, r)
			}
			if rec.Code != 403 {
				t.Fatalf("%s allowed a restricted desktop token: %d", kind, rec.Code)
			}
		}
	}
	s := newDesktopOfficeTestServer(t)
	s.Cfg.Docker.Enabled = true
	var calls atomic.Int32
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" && strings.HasSuffix(r.URL.Path, "/restart") {
			calls.Add(1)
			w.WriteHeader(204)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/version") {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"ApiVersion":"1.45"}`)
			return
		}
		fmt.Fprint(w, "OK")
	}))
	defer docker.Close()
	s.Cfg.Docker.Host = strings.Replace(docker.URL, "http://", "tcp://", 1)
	runtime := s.worldRuntime()
	runtime.mu.Lock()
	runtime.set(systemworld.Entity{ID: "container:demo", Kind: "container", District: "infra", State: "running", At: time.Now().UnixMilli()})
	runtime.mu.Unlock()
	request := func(body string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handleSystemWorldAction(s)(rec, httptest.NewRequest("POST", "/api/desktop/system-world/actions", strings.NewReader(body)))
		return rec
	}
	payload := `{"entity":"container:demo","action":"restart","request_id":"test-request-012345","confirmed":true}`
	noConfirm := request(strings.ReplaceAll(payload, `true`, `false`))
	if noConfirm.Code != 400 || calls.Load() != 0 {
		t.Fatal("confirmation bypass")
	}
	first := request(payload)
	if first.Code != 200 || !strings.Contains(first.Body.String(), `"completed"`) {
		t.Fatalf("action: %d %s", first.Code, first.Body.String())
	}
	second := request(payload)
	if second.Body.String() != first.Body.String() || calls.Load() != 1 {
		t.Fatal("duplicate action was executed")
	}
	collision := request(strings.Replace(payload, `"restart"`, `"stop"`, 1))
	if collision.Code != 409 {
		t.Fatal("idempotency key reused for another action")
	}
	s.Cfg.VirtualDesktop.ReadOnly = true
	if request(payload).Code != 403 {
		t.Fatal("desktop read-only bypass")
	}
	s.Cfg.VirtualDesktop.ReadOnly = false
	s.Cfg.Docker.ReadOnly = true
	if request(strings.Replace(payload, "012345", "012346", 1)).Code != 409 || calls.Load() != 1 {
		t.Fatal("docker read-only bypass")
	}
	s.Cfg.Docker.ReadOnly = false
	runtime.mu.Lock()
	e := runtime.entities["container:demo"]
	e.At = time.Now().Add(-time.Minute).UnixMilli()
	runtime.entities[e.ID] = e
	runtime.mu.Unlock()
	if request(strings.Replace(payload, "012345", "012347", 1)).Code != 409 {
		t.Fatal("stale target actionable")
	}
}

func TestSystemWorldProducerBoundsStormAndCounts(t *testing.T) {
	w := newDesktopOfficeTestServer(t).worldRuntime()
	rows := make([]map[string]any, 1000)
	for i := range rows {
		rows[i] = map[string]any{"id": fmt.Sprintf("c%04d", i), "name": "Container", "state": "running"}
	}
	w.observe(EventContainerUpdate, rows)
	if got := w.snapshot(); got.Metrics["container_count"] != 1000 || len(got.Entities) != 807 {
		t.Fatalf("unbounded or inaccurate aggregate: %d", len(got.Entities))
	}
	for i := 0; i < 2100; i++ {
		w.observe(EventAgentAction, map[string]any{"tool_name": "docker", "state": []string{"started", "succeeded"}[i%2]})
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) != 1000 || !w.overflow {
		t.Fatal("producer did not mark dropped events")
	}
}

func TestSystemWorldRecordChangedFactsWithoutHeartbeatStorm(t *testing.T) {
	w := newDesktopOfficeTestServer(t).worldRuntime()
	w.mu.Lock()
	defer w.mu.Unlock()
	e := systemworld.Entity{ID: "graph", Kind: "district", District: "graph", State: "idle", At: 60000, Values: map[string]float64{"nodes": 100}}
	w.set(e)
	w.pending = nil
	e.At = 120000
	w.set(e)
	if len(w.pending) != 0 || w.metrics["observed:system-world/district"] != 120000 {
		t.Fatal("unchanged poll must update freshness without an event storm")
	}
	e.Values = map[string]float64{"nodes": 101}
	w.set(e)
	if len(w.pending) != 1 || w.pending[0].Values["nodes"] != 101 {
		t.Fatal("changed fact was lost until the next checkpoint")
	}
	w.pending = nil
	e.Model = "model-updated"
	w.set(e)
	if len(w.pending) != 1 || w.pending[0].Model != "model-updated" {
		t.Fatal("route change was lost")
	}
}

func TestSystemWorldMissionDispatchAndCancelAcknowledgements(t *testing.T) {
	allowMissionMutationsForTest(t)
	t.Cleanup(tools.ClearRuntimePermissionsForTest)
	s := newDesktopOfficeTestServer(t)
	s.Cfg.Tools.Missions.Enabled = true
	mgr := tools.NewMissionManagerV2(t.TempDir(), nil)
	s.MissionManagerV2 = mgr
	if err := mgr.Create(&tools.MissionV2{ID: "world_mission", Name: "Review mission", Prompt: "Fixture only", ExecutionType: tools.ExecutionManual, Enabled: true}); err != nil {
		t.Fatal(err)
	}
	mgr.SetCallback(func(_, _ string) {})
	if err := mgr.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(mgr.Stop)
	observe := func(state string) {
		w := s.worldRuntime()
		w.mu.Lock()
		defer w.mu.Unlock()
		w.set(systemworld.Entity{ID: "mission:world_mission", Kind: "mission", District: "missions", State: state, At: time.Now().UnixMilli()})
	}
	post := func(verb, nonce string, confirmed bool) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]any{"entity": "mission:world_mission", "action": verb, "request_id": nonce, "confirmed": confirmed})
		rec := httptest.NewRecorder()
		handleSystemWorldAction(s)(rec, httptest.NewRequest("POST", "/api/desktop/system-world/actions", strings.NewReader(string(body))))
		return rec
	}
	observe("idle")
	s.Cfg.Tools.Missions.ReadOnly = true
	if r := post("start", "world-mission-denied", false); r.Code != 409 {
		t.Fatalf("read-only mission started: %d %s", r.Code, r.Body.String())
	}
	s.Cfg.Tools.Missions.ReadOnly = false
	first := post("start", "world-mission-start", false)
	if first.Code != 202 || !strings.Contains(first.Body.String(), `"accepted"`) {
		t.Fatalf("dispatch must be accepted, not completed: %d %s", first.Code, first.Body.String())
	}
	if repeat := post("start", "world-mission-start", false); repeat.Body.String() != first.Body.String() {
		t.Fatal("mission retry changed its receipt")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if m, ok := mgr.Get("world_mission"); ok && m.Status == tools.MissionStatusRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("mission dispatcher did not start")
		}
		time.Sleep(20 * time.Millisecond)
	}
	ctx, release := s.missionRunTracker().begin("world_mission")
	defer release()
	observe("running")
	if r := post("cancel", "world-mission-cancel", false); r.Code != 400 || ctx.Err() != nil {
		t.Fatal("cancellation bypassed target confirmation")
	}
	r := post("cancel", "world-mission-cancel", true)
	if r.Code != 202 || ctx.Err() == nil || !strings.Contains(r.Body.String(), `"accepted"`) {
		t.Fatalf("cancel did not reach run context: %d %s", r.Code, r.Body.String())
	}
	// A daemon advertised in an old sample can disappear before dispatch. Its
	// real handler must reject it rather than generating a success receipt.
	s.DaemonSupervisor = tools.NewDaemonSupervisor(tools.DaemonSupervisorConfig{LogDir: t.TempDir()}, nil, nil, nil, nil, s.Logger)
	w := s.worldRuntime()
	w.mu.Lock()
	w.set(systemworld.Entity{ID: "daemon:missing", Kind: "daemon", District: "infra", State: "stopped", At: time.Now().UnixMilli()})
	w.mu.Unlock()
	rec := httptest.NewRecorder()
	handleSystemWorldAction(s)(rec, httptest.NewRequest("POST", "/api/desktop/system-world/actions", strings.NewReader(`{"entity":"daemon:missing","action":"start","request_id":"world-daemon-absent"}`)))
	if rec.Code != 409 || !strings.Contains(rec.Body.String(), `"failed"`) {
		t.Fatalf("missing daemon reported success: %d %s", rec.Code, rec.Body.String())
	}
}
