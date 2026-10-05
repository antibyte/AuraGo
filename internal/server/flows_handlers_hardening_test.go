package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/memory"
	"aurago/internal/tools"
)

// c17LogBuffer collects log output from the handler and background goroutines.
type c17LogBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *c17LogBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *c17LogBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// c17CaptureLogs routes s.Logger, Debug included, into a buffer.
func c17CaptureLogs(s *Server) *c17LogBuffer {
	logs := &c17LogBuffer{}
	s.Logger = slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug}))
	return logs
}

// c17Audit gives the server an audit timeline.
func c17Audit(t *testing.T, s *Server) *memory.SQLiteMemory {
	t.Helper()
	stm, err := memory.NewSQLiteMemory(":memory:", slog.New(slog.NewTextHandler(&c17LogBuffer{}, nil)))
	if err != nil {
		t.Fatalf("NewSQLiteMemory: %v", err)
	}
	t.Cleanup(func() { _ = stm.Close() })
	s.ShortTermMem = stm
	return stm
}

func c17AuditEvents(t *testing.T, stm *memory.SQLiteMemory, eventType string) []memory.AuditEvent {
	t.Helper()
	page, err := stm.SearchAuditEvents(memory.AuditFilter{Type: eventType, Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	return page.Entries
}

// c17Events subscribes to the desktop events the server broadcasts.
func c17Events(t *testing.T, s *Server) <-chan desktop.Event {
	t.Helper()
	hub := desktop.NewHub(4)
	events, cancel, err := hub.Subscribe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(cancel)
	s.DesktopHub = hub
	return events
}

func c17Drain(events <-chan desktop.Event) []desktop.Event {
	var out []desktop.Event
	for {
		select {
		case ev := <-events:
			out = append(out, ev)
		default:
			return out
		}
	}
}

// c17FlakyBridge is the server's bridge with SyncFlowMission failing a given number of times.
type c17FlakyBridge struct {
	flowMissionBridge
	syncFailures atomic.Int32
}

func (b *c17FlakyBridge) SyncFlowMission(missionID, name string, bindings []flows.TriggerBinding) error {
	if b.syncFailures.Add(-1) >= 0 {
		return errors.New(`mission store C:\data\missions.json is busy`)
	}
	return b.flowMissionBridge.SyncFlowMission(missionID, name, bindings)
}

// c17SwapBridge replaces the server's flow service with one over the same store and
// registry that uses bridge. The test server's cleanup shuts the new one down.
func c17SwapBridge(t *testing.T, s *Server, bridge flows.MissionBridge) {
	t.Helper()
	ctx := context.Background()
	old := s.Flows
	if err := old.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	svc := flows.NewService(old.Store(), old.Registry(), &flows.Services{Clock: flows.RealClock(), Location: time.Local},
		bridge, flows.ServiceConfig{}, s.Logger)
	if err := svc.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}
	s.Flows = svc
}

func TestC17ErrorMapCoversEveryFlowSentinel(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	cases := []struct {
		err    error
		status int
		code   string
	}{
		{flows.ErrNotFound, http.StatusNotFound, "FLOW_NOT_FOUND"},
		{flows.ErrRunNotFound, http.StatusNotFound, "FLOW_RUN_NOT_FOUND"},
		{flows.ErrRevisionConflict, http.StatusConflict, "FLOW_REVISION_CONFLICT"},
		{flows.ErrNotPublished, http.StatusConflict, "FLOW_NOT_PUBLISHED"},
		{flows.ErrNoTrigger, http.StatusConflict, "FLOW_NO_TRIGGER"},
		{fmt.Errorf("%w: %q", flows.ErrFlowExists, "flow_x"), http.StatusConflict, "FLOW_EXISTS"},
		{fmt.Errorf("lookup: %w", flows.ErrMissionAmbiguous), http.StatusConflict, "FLOW_MISSION_AMBIGUOUS"},
		{tools.ErrMissionLocked, http.StatusConflict, "FLOW_LOCKED"},
		{fmt.Errorf("delete: %w", tools.ErrMissionLocked), http.StatusConflict, "FLOW_LOCKED"},
		{flows.ErrQueueFull, http.StatusTooManyRequests, "FLOW_RUN_LIMIT"},
		{flows.ErrRunnerClosed, http.StatusServiceUnavailable, "FLOWS_DISABLED"},
		{flows.ErrDocumentTooLarge, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE"},
		{flows.ErrTestDataTooLarge, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE"},
		{&http.MaxBytesError{Limit: 4096}, http.StatusRequestEntityTooLarge, "FLOW_TOO_LARGE"},
		{fmt.Errorf("%w: 99", flows.ErrUnsupportedSchema), http.StatusBadRequest, "FLOW_BAD_REQUEST"},
		{fmt.Errorf("%w: %q", flows.ErrUnknownTemplate, "nope"), http.StatusBadRequest, "FLOW_BAD_REQUEST"},
		{&flows.ValidationError{}, http.StatusUnprocessableEntity, "FLOW_INVALID"},
		{flows.NewNodeError("FLOW_PARAM_INVALID", "bad"), http.StatusBadRequest, "FLOW_PARAM_INVALID"},
		{&flows.NodeError{Message: "no code"}, http.StatusBadRequest, "FLOW_NODE_FAILED"},
	}
	for _, tc := range cases {
		w := httptest.NewRecorder()
		s.flowsErrorFrom(w, httptest.NewRequest(http.MethodGet, "/api/desktop/flows/flow_x", nil), tc.err)
		body := flowsBody(t, w)
		if msg, _ := body["error"].(string); w.Code != tc.status || body["code"] != tc.code || msg == "" {
			t.Errorf("%v = %d %v, want %d %s", tc.err, w.Code, body, tc.status, tc.code)
		}
	}
	w := httptest.NewRecorder()
	s.flowsErrorFrom(w, httptest.NewRequest(http.MethodGet, "/", nil), &flows.ValidationError{})
	if issues, ok := flowsBody(t, w)["issues"].([]any); !ok || issues == nil {
		t.Fatalf("FLOW_INVALID needs an issues list: %s", w.Body.String())
	}
}

func TestC17InternalErrorsStayInTheLog(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	logs := c17CaptureLogs(s)
	cause := `open C:\Users\me\data\flows.db: sql: database disk image is malformed ` + strings.Repeat("x", 1000)
	w := httptest.NewRecorder()
	s.flowsErrorFrom(w, httptest.NewRequest(http.MethodPost, "/api/desktop/flows/flow_abc/publish", nil), errors.New(cause))
	body := flowsBody(t, w)
	if w.Code != http.StatusInternalServerError || body["code"] != "FLOW_INTERNAL" || body["error"] != flowsInternalMessage {
		t.Fatalf("internal error = %d %s", w.Code, w.Body.String())
	}
	if strings.Contains(w.Body.String(), "flows.db") || strings.Contains(w.Body.String(), "sql:") {
		t.Fatalf("the answer leaks the cause: %s", w.Body.String())
	}
	out := logs.String()
	if !strings.Contains(out, "level=WARN") || !strings.Contains(out, "flows.db") || !strings.Contains(out, "flow_id=flow_abc") ||
		!strings.Contains(out, "/api/desktop/flows/flow_abc/publish") {
		t.Fatalf("log = %s", out)
	}
	if strings.Contains(out, strings.Repeat("x", flowErrorRunes)) {
		t.Fatal("the logged cause is not bounded")
	}
}

func TestC17CancelledRequestGetsNoAnswer(t *testing.T) {
	s, token := newFlowsTestServer(t)
	logs := c17CaptureLogs(s)
	rec := createTestFlow(t, s, greetFlowJSON)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodDelete, "/api/desktop/flows/"+rec.ID, nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	if w.Body.Len() != 0 || w.Header().Get("Content-Type") != "" {
		t.Fatalf("a cancelled request was answered: %d %s", w.Code, w.Body.String())
	}
	if !strings.Contains(logs.String(), "ended by the client") {
		t.Fatalf("log = %s", logs.String())
	}
	if _, err := s.Flows.GetFlow(context.Background(), rec.ID); err != nil {
		t.Fatalf("the cancelled delete changed the flow: %v", err)
	}
	// A real failure with an ended context still answers.
	w = httptest.NewRecorder()
	s.flowsErrorFrom(w, httptest.NewRequest(http.MethodGet, "/", nil).WithContext(ctx), flows.ErrNotFound)
	if w.Code != http.StatusNotFound {
		t.Fatalf("not found with an ended context = %d", w.Code)
	}
}

func TestC17OversizedBodiesAre413(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	big := strings.Repeat("a", flows.MaxDocumentBytes)
	hugeDoc := `{"schema":1,"name":"Gruss","description":"` + big + `"}`
	cases := []struct{ method, path, body string }{
		{http.MethodPost, "/api/desktop/flows", `{"name":"` + strings.Repeat("b", flowsDocBodyLimit) + `"}`},
		{http.MethodPost, "/api/desktop/flows", `{"import":` + hugeDoc + `}`},
		{http.MethodPut, "/api/desktop/flows/" + rec.ID, `{"doc":` + hugeDoc + `,"base_revision":1}`},
		{http.MethodPost, "/api/desktop/flows/" + rec.ID + "/publish", `{"base_revision":1,"pad":"` + strings.Repeat("c", flowsSmallBodyLimit) + `"}`},
	}
	for _, tc := range cases {
		w := flowsCall(t, s, tc.method, tc.path, token, tc.body)
		if w.Code != http.StatusRequestEntityTooLarge || flowsBody(t, w)["code"] != "FLOW_TOO_LARGE" {
			t.Errorf("%s %s = %d %s", tc.method, tc.path, w.Code, flowBoundRunes(w.Body.String(), 200))
		}
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":`); w.Code != http.StatusBadRequest {
		t.Fatalf("broken JSON = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"template":"no_such_template"}`); w.Code != http.StatusBadRequest ||
		flowsBody(t, w)["code"] != "FLOW_BAD_REQUEST" {
		t.Fatalf("unknown template = %d %s", w.Code, w.Body.String())
	}
}

func TestC17PartialPublishIsReportedAndHeals(t *testing.T) {
	s, token := newFlowsTestServer(t)
	logs := c17CaptureLogs(s)
	stm := c17Audit(t, s)
	events := c17Events(t, s)
	bridge := &c17FlakyBridge{flowMissionBridge: flowMissionBridge{s: s}}
	bridge.syncFailures.Store(1)
	c17SwapBridge(t, s, bridge)
	rec := createTestFlow(t, s, greetFlowJSON)
	c17Drain(events)

	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`)
	body := flowsBody(t, w)
	if w.Code != http.StatusOK || body["partial"] != true || body["code"] != "FLOW_PUBLISH_INCOMPLETE" ||
		body["error"] != flowPublishIncompleteMessage {
		t.Fatalf("partial publish = %d %s", w.Code, w.Body.String())
	}
	flow, _ := body["flow"].(map[string]any)
	if flow["live_revision"] != float64(1) || flow["id"] != rec.ID {
		t.Fatalf("partial publish record = %+v", flow)
	}
	if _, ok := body["issues"].([]any); !ok {
		t.Fatalf("partial publish issues = %s", w.Body.String())
	}
	if strings.Contains(w.Body.String(), "missions.json") {
		t.Fatalf("the cause leaked into the answer: %s", w.Body.String())
	}
	if !strings.Contains(logs.String(), "missions.json") {
		t.Fatalf("the cause is not logged: %s", logs.String())
	}
	if m, _ := s.MissionManagerV2.Get(rec.MissionID); m.FlowPublished {
		t.Fatal("the failing sync still published the mission")
	}
	published := 0
	for _, ev := range c17Drain(events) {
		if payload, _ := ev.Payload.(map[string]interface{}); ev.Type == "flows_changed" && payload["reason"] == "published" {
			published++
		}
	}
	if published != 1 {
		t.Fatalf("flows_changed published events = %d", published)
	}
	audit := c17AuditEvents(t, stm, "flow_publish")
	if len(audit) != 1 || audit[0].Status != memory.AuditStatusWarning || !strings.Contains(audit[0].Summary, "Mission Control update failed") {
		t.Fatalf("audit = %+v", audit)
	}
	stored, err := s.Flows.GetFlow(context.Background(), rec.ID)
	if err != nil || stored.HasUnpublishedChanges() {
		t.Fatalf("after a partial publish the draft counts as published: %v %v", stored, err)
	}

	// Publishing the same draft revision again finishes the update.
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`)
	if body := flowsBody(t, w); w.Code != http.StatusOK || body["partial"] != nil || body["code"] != nil {
		t.Fatalf("healing publish = %d %s", w.Code, w.Body.String())
	}
	if m, _ := s.MissionManagerV2.Get(rec.MissionID); !m.FlowPublished || len(m.FlowTriggers) != 1 {
		t.Fatalf("the healing publish did not sync the mission: %+v", m)
	}
	if audit := c17AuditEvents(t, stm, "flow_publish"); len(audit) != 2 || audit[0].Status != memory.AuditStatusSuccess {
		t.Fatalf("audit after heal = %+v", audit)
	}
}

func TestC17EnableAuditNamesTheFlow(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	rec := createTestFlow(t, s, greetFlowJSON)
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", w.Code, w.Body.String())
	}
	for _, step := range []struct {
		body, event, summary string
	}{
		{`{"enabled":true}`, "flow_enable", "Flow Gruss enabled"},
		{`{"enabled":false}`, "flow_disable", "Flow Gruss disabled"},
	} {
		if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, step.body); w.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", step.body, w.Code, w.Body.String())
		}
		audit := c17AuditEvents(t, stm, step.event)
		if len(audit) != 1 || audit[0].TargetName != "Gruss" || audit[0].TargetID != rec.ID || audit[0].Summary != step.summary {
			t.Fatalf("%s audit = %+v", step.event, audit)
		}
	}
}

// TestC17AuditTypesAreRegisteredInTheDashboard drives every audited flow action through
// the API and checks that the dashboard's audit type filter lists each recorded type and
// every dashboard locale labels it.
func TestC17AuditTypesAreRegisteredInTheDashboard(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	created := flowsBody(t, flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"A"}`))["flow"].(map[string]any)
	flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+greetFlowJSON+`}`)
	id := created["id"].(string)
	steps := []struct{ method, path, body string }{
		{http.MethodPut, "/api/desktop/flows/" + id, `{"doc":` + greetFlowJSON + `,"base_revision":1}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/publish", `{"base_revision":2}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/enabled", `{"enabled":true}`},
		{http.MethodPost, "/api/desktop/flows/" + id + "/enabled", `{"enabled":false}`},
		{http.MethodPut, "/api/desktop/flows/secrets/c17_audit", `{"value":"c17-audit-value-123"}`},
		{http.MethodDelete, "/api/desktop/flows/secrets/c17_audit", ""},
		{http.MethodDelete, "/api/desktop/flows/" + id, ""},
	}
	for _, step := range steps {
		if w := flowsCall(t, s, step.method, step.path, token, step.body); w.Code != http.StatusOK {
			t.Fatalf("%s %s = %d %s", step.method, step.path, w.Code, w.Body.String())
		}
	}
	page, err := stm.SearchAuditEvents(memory.AuditFilter{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	types := map[string]bool{}
	for _, ev := range page.Entries {
		types[ev.EventType] = true
	}
	if len(types) != 8 {
		t.Fatalf("audited types = %v, want the 8 flow actions", types)
	}
	ui := filepath.Join("..", "..", "ui")
	html, err := os.ReadFile(filepath.Join(ui, "dashboard.html"))
	if err != nil {
		t.Fatal(err)
	}
	locales, err := filepath.Glob(filepath.Join(ui, "lang", "dashboard", "*.json"))
	if err != nil || len(locales) != 16 {
		t.Fatalf("dashboard locales = %v %v", locales, err)
	}
	for typ := range types {
		if !strings.Contains(string(html), `<option value="`+typ+`" data-i18n="dashboard.audit_type_`+typ+`">`) {
			t.Errorf("the dashboard audit filter does not list %s", typ)
		}
		for _, path := range locales {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var labels map[string]string
			if err := json.Unmarshal(data, &labels); err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if strings.TrimSpace(labels["dashboard.audit_type_"+typ]) == "" {
				t.Errorf("%s has no label for %s", filepath.Base(path), typ)
			}
		}
	}
}

func TestC17ListCarriesNoDocuments(t *testing.T) {
	s, token := newFlowsTestServer(t)
	createTestFlow(t, s, greetFlowJSON)
	list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""))["flows"].([]any)
	if len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	card := list[0].(map[string]any)
	for _, key := range []string{"draft", "live", "nodes", "edges"} {
		if _, ok := card[key]; ok {
			t.Errorf("the list card carries %q", key)
		}
	}
}
