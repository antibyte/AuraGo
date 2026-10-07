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

	"aurago/internal/agent"
	"aurago/internal/desktop"
	"aurago/internal/flows"
	"aurago/internal/memory"
	"aurago/internal/security"
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
	var nilNode *flows.NodeError
	var nilInvalid *flows.ValidationError
	var nilTooLarge *http.MaxBytesError
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
		{flows.ErrFlowDisabled, http.StatusConflict, "FLOW_DISABLED"},
		{flows.ErrFlowMissionMissing, http.StatusConflict, "FLOW_MISSION_MISSING"},
		{flows.ErrMissionControlUnavailable, http.StatusServiceUnavailable, "FLOWS_DISABLED"},
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
		// Typed nils (a producer's bug) must not panic the mapper.
		{nilNode, http.StatusInternalServerError, "FLOW_INTERNAL"},
		{nilInvalid, http.StatusInternalServerError, "FLOW_INTERNAL"},
		{nilTooLarge, http.StatusInternalServerError, "FLOW_INTERNAL"},
		{fmt.Errorf("engine: %w", error(nilNode)), http.StatusInternalServerError, "FLOW_INTERNAL"},
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

// TestC17CancelledRequestGets503 covers a context the server cancels (the shutdown drain)
// while the client still waits: the answer must not be an implicit 200.
func TestC17CancelledRequestGets503(t *testing.T) {
	s, token := newFlowsTestServer(t)
	logs := c17CaptureLogs(s)
	rec := createTestFlow(t, s, greetFlowJSON)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, req := range []struct{ method, path, body string }{
		{http.MethodDelete, "/api/desktop/flows/" + rec.ID, ""},
		{http.MethodPut, "/api/desktop/flows/" + rec.ID, `{"doc":` + greetFlowJSON + `,"base_revision":1}`},
	} {
		r := httptest.NewRequest(req.method, req.path, strings.NewReader(req.body)).WithContext(ctx)
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		if body := flowsBody(t, w); w.Code != http.StatusServiceUnavailable || body["code"] != "FLOWS_DISABLED" ||
			body["error"] != "the request was cancelled" {
			t.Fatalf("cancelled %s = %d %s", req.method, w.Code, w.Body.String())
		}
	}
	if !strings.Contains(logs.String(), "was cancelled") || strings.Contains(logs.String(), "level=WARN") {
		t.Fatalf("log = %s", logs.String())
	}
	if got, err := s.Flows.GetFlow(context.Background(), rec.ID); err != nil || got.DraftRevision != 1 {
		t.Fatalf("the cancelled requests changed the flow: %+v %v", got, err)
	}
	// A real failure with an ended context keeps its own answer.
	w := httptest.NewRecorder()
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

// c17AgentVault runs the agent's secrets_vault tool through the real dispatcher.
func c17AgentVault(t *testing.T, s *Server, operation, key, value string) string {
	t.Helper()
	cfg := *s.Cfg
	cfg.Tools.SecretsVault.Enabled = true
	dc := &agent.DispatchContext{Cfg: &cfg, Logger: s.Logger, Vault: s.Vault, SessionID: "c17-agent"}
	tc := &agent.ToolCall{Action: "secrets_vault", Operation: operation, Key: key, Value: value}
	return agent.DispatchToolCallResult(context.Background(), tc, dc, "").Output
}

func TestC17FlowSecretsStayOutOfTheAgent(t *testing.T) {
	s, token := newFlowsTestServer(t)
	const value = "c17-agent-secret-value-0123"
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_agent", token, `{"value":"`+value+`"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d %s", w.Code, w.Body.String())
	}
	key := flowSecretPrefix + "c17_agent"
	if _, err := s.Vault.ReadSecretForAgent(key); !errors.Is(err, security.ErrSecretAgentAccessDenied) {
		t.Fatalf("ReadSecretForAgent = %v, want access denied", err)
	}
	if ok, err := s.Vault.AgentCanReadSecret(key); ok || err != nil {
		t.Fatalf("AgentCanReadSecret = %v %v", ok, err)
	}
	if tools.IsPythonAccessibleSecret(key) || tools.IsPythonAccessibleSecret("EASYDRAG_C17_AGENT") {
		t.Fatal("flow secrets must be blocked for Python, skills and the agent's vault tool")
	}
	if resolved, rejected, err := tools.ResolveVaultSecrets(s.Vault, []string{key}); err != nil || len(resolved) != 0 || len(rejected) != 1 {
		t.Fatalf("ResolveVaultSecrets = %v %v %v", resolved, rejected, err)
	}

	list := c17AgentVault(t, s, "", "", "")
	if !strings.Contains(list, `"status":"success"`) || strings.Contains(list, "easydrag_") {
		t.Fatalf("the agent's key list = %s", list)
	}
	if out := c17AgentVault(t, s, "get", key, ""); strings.Contains(out, value) || !strings.Contains(out, "Access denied") {
		t.Fatalf("the agent read a flow secret: %s", out)
	}
	if out := c17AgentVault(t, s, "delete", key, ""); !strings.Contains(out, "Access denied") {
		t.Fatalf("the agent deleted a flow secret: %s", out)
	}
	if out := c17AgentVault(t, s, "store", flowSecretPrefix+"c17_planted", "agent-chosen-value-123"); !strings.Contains(out, "Access denied") {
		t.Fatalf("the agent created a flow secret: %s", out)
	}
	if _, err := s.Vault.ReadSecret(flowSecretPrefix + "c17_planted"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("the planted flow secret exists: %v", err)
	}
	if v, err := (flowSecrets{s: s}).ReadSecret("c17_agent"); err != nil || v != value {
		t.Fatalf("flows lost the secret: %q %v", v, err)
	}
	// The vault's own user list (the skills dialog) hides flow secrets as well.
	w := httptest.NewRecorder()
	handleListVaultSecrets(s, w, httptest.NewRequest(http.MethodGet, "/api/vault/secrets?filter=user", nil))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "easydrag_") {
		t.Fatalf("vault user list = %d %s", w.Code, w.Body.String())
	}
}

func TestC17SecretWritesAreBoundedAuditedByNameAndScrubbedOnUse(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	tooLong := strings.Repeat("v", flowSecretValueMaxBytes+1)
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_big", token, `{"value":"`+tooLong+`"}`); w.Code != http.StatusRequestEntityTooLarge ||
		flowsBody(t, w)["code"] != "FLOW_TOO_LARGE" {
		t.Fatalf("oversized secret = %d %s", w.Code, flowBoundRunes(w.Body.String(), 200))
	}
	if _, err := s.Vault.ReadSecret(flowSecretPrefix + "c17_big"); !errors.Is(err, security.ErrSecretNotFound) {
		t.Fatalf("the oversized secret was stored: %v", err)
	}
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_max", token, `{"value":"`+tooLong[1:]+`"}`); w.Code != http.StatusOK {
		t.Fatalf("secret at the limit = %d %s", w.Code, flowBoundRunes(w.Body.String(), 200))
	}

	trimmed := fmt.Sprintf("c17-scrub-%d", time.Now().UnixNano())
	raw := "  " + trimmed + "\\n"
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_scrub", token, `{"value":"`+raw+`"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d %s", w.Code, w.Body.String())
	}
	// A write does not grow the global scrubber; the run that reads the secret registers it.
	if out := security.Scrub("Authorization: Bearer " + trimmed); !strings.Contains(out, trimmed) {
		t.Fatalf("the value was registered on write: %q", out)
	}
	audit := c17AuditEvents(t, stm, "flow_secret_set")
	var entry *memory.AuditEvent
	for i := range audit {
		if audit[i].TargetName == "c17_scrub" {
			entry = &audit[i]
		}
	}
	if entry == nil || entry.Summary != "Flow secret c17_scrub saved" || entry.Detail != "" {
		t.Fatalf("audit = %+v", audit)
	}
	if blob, _ := json.Marshal(audit); strings.Contains(string(blob), trimmed) || strings.Contains(string(blob), security.RedactedText("")) {
		t.Fatalf("the audit timeline holds the value: %s", blob)
	}
	if v, err := (flowSecrets{s: s}).ReadSecret("c17_scrub"); err != nil || v != "  "+trimmed+"\n" {
		t.Fatalf("ReadSecret = %q %v", v, err)
	}
	if out := security.Scrub("Authorization: Bearer " + trimmed); strings.Contains(out, trimmed) {
		t.Fatalf("the trimmed value is not scrubbed after use: %q", out)
	}
	if out := security.Scrub("raw: " + "  " + trimmed + "\n"); strings.Contains(out, trimmed) {
		t.Fatalf("the stored value is not scrubbed after use: %q", out)
	}
}

func TestC17SecretWritesAreRateLimitedPerClient(t *testing.T) {
	s, token := newFlowsTestServer(t)
	call := func(method, path, body, remote string) *httptest.ResponseRecorder {
		var r *http.Request
		if body == "" {
			r = httptest.NewRequest(method, path, nil)
		} else {
			r = httptest.NewRequest(method, path, strings.NewReader(body))
		}
		r.RemoteAddr = remote
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		s.handleFlows(w, r)
		return w
	}
	const first, second = "192.0.2.10:1000", "192.0.2.11:1000"
	for i := 0; i < flowSecretWritesPerWindow; i++ {
		method, body := http.MethodPut, `{"value":"c17-rate-value-123"}`
		if i%2 == 1 {
			method, body = http.MethodDelete, ""
		}
		if w := call(method, "/api/desktop/flows/secrets/c17_rate", body, first); w.Code != http.StatusOK {
			t.Fatalf("write %d = %d %s", i, w.Code, w.Body.String())
		}
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete} {
		w := call(method, "/api/desktop/flows/secrets/c17_rate", `{"value":"c17-rate-value-123"}`, first)
		if w.Code != http.StatusTooManyRequests || flowsBody(t, w)["code"] != "FLOW_RATE_LIMITED" || w.Header().Get("Retry-After") == "" {
			t.Fatalf("%s over the limit = %d %s", method, w.Code, w.Body.String())
		}
	}
	if w := call(http.MethodGet, "/api/desktop/flows/secrets", "", first); w.Code != http.StatusOK {
		t.Fatalf("listing is not limited: %d", w.Code)
	}
	if w := call(http.MethodPut, "/api/desktop/flows/secrets/c17_rate", `{"value":"c17-rate-value-123"}`, second); w.Code != http.StatusOK {
		t.Fatalf("another client = %d %s", w.Code, w.Body.String())
	}

	var l flowRateLimiter
	start := time.Now()
	for i := 0; i < 3; i++ {
		if !l.allow("a", start.Add(time.Duration(i)*time.Second), 3, time.Minute) {
			t.Fatalf("event %d refused", i)
		}
	}
	if l.allow("a", start.Add(59*time.Second), 3, time.Minute) {
		t.Fatal("a fourth event within the window was allowed")
	}
	if !l.allow("a", start.Add(61*time.Second), 3, time.Minute) {
		t.Fatal("the window does not slide")
	}
	for i := 0; i < 2*flowRateKeysSweep; i++ {
		l.allow(fmt.Sprintf("k%d", i), start, 3, time.Minute)
	}
	l.allow("late", start.Add(2*time.Minute), 3, time.Minute)
	if n := len(l.windows); n > flowRateKeysSweep+2 {
		t.Fatalf("idle keys are kept: %d", n)
	}
}

// c17SecretFlowJSON is a flow whose http.request node uses the flow secret named in it.
func c17SecretFlowJSON(name, secret string, disabled bool) string {
	return `{"schema":1,"name":"` + name + `","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"call","type":"http.request","type_version":1,"label":"Call","position":{"x":300,"y":0},
  "settings":{"disabled":` + fmt.Sprint(disabled) + `},"params":{"url":"https://example.com/api","auth_secret":" ` + secret + ` "}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`
}

func TestC17SecretDeleteNamesThePublishedFlowsUsingIt(t *testing.T) {
	s, token := newFlowsTestServer(t)
	ctx := context.Background()
	publish := func(rec *flows.FlowRecord) {
		t.Helper()
		// The store publish skips the availability rules; only the live document matters here.
		if _, err := s.Flows.Store().Publish(ctx, rec.ID, rec.DraftRevision, time.Now()); err != nil {
			t.Fatalf("store publish: %v", err)
		}
	}
	publish(createTestFlow(t, s, c17SecretFlowJSON("Zeta uses it", "c17_used", false)))
	publish(createTestFlow(t, s, c17SecretFlowJSON("Alpha uses it", "c17_used", false)))
	publish(createTestFlow(t, s, c17SecretFlowJSON("Disabled node", "c17_used", true)))
	publish(createTestFlow(t, s, c17SecretFlowJSON("Other secret", "c17_other", false)))
	createTestFlow(t, s, c17SecretFlowJSON("Draft only", "c17_used", false))

	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_used", token, `{"value":"c17-used-value-123"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d %s", w.Code, w.Body.String())
	}
	w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/secrets/c17_used", token, "")
	body := flowsBody(t, w)
	users, _ := body["used_by"].([]any)
	if w.Code != http.StatusOK || body["status"] != "deleted" || len(users) != 2 || users[0] != "Alpha uses it" || users[1] != "Zeta uses it" {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	w = flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/secrets/c17_unused", token, "")
	if users, ok := flowsBody(t, w)["used_by"].([]any); w.Code != http.StatusOK || !ok || len(users) != 0 {
		t.Fatalf("delete of an unused secret = %d %s", w.Code, w.Body.String())
	}
}

func TestC17ScrubbedJSONWalksTheValues(t *testing.T) {
	secret := fmt.Sprintf("c17 \"quoted\" \\ secret\n\t<%d>", time.Now().UnixNano())
	security.RegisterSensitive(secret)
	payload := map[string]any{
		"run":  map[string]any{"output": "token=" + secret + " end", "list": []any{secret, 42, true, nil}},
		secret: "as a key",
	}
	s := &Server{Logger: slog.New(slog.NewTextHandler(&c17LogBuffer{}, nil))}
	w := httptest.NewRecorder()
	s.flowsJSONScrubbed(w, http.StatusOK, payload)
	if !json.Valid(w.Body.Bytes()) {
		t.Fatalf("not JSON: %s", w.Body.String())
	}
	escaped, _ := json.Marshal(secret)
	if strings.Contains(w.Body.String(), strings.Trim(string(escaped), `"`)) {
		t.Fatalf("the escaped secret is in the answer: %s", w.Body.String())
	}
	var decoded map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &decoded); err != nil {
		t.Fatal(err)
	}
	run := decoded["run"].(map[string]any)
	if out := run["output"].(string); strings.Contains(out, secret) || !strings.Contains(out, security.RedactedText("")) || !strings.HasSuffix(out, " end") {
		t.Fatalf("output = %q", out)
	}
	if list := run["list"].([]any); len(list) != 4 || list[1] != float64(42) || list[2] != true || list[3] != nil {
		t.Fatalf("list = %+v", list)
	}
	for key := range decoded {
		if strings.Contains(key, secret) {
			t.Fatalf("the secret is a key: %q", key)
		}
	}
	if w.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("scrubbed answers must not be cached")
	}
}

func TestC17UnencodableAnswersAre500(t *testing.T) {
	logs := &c17LogBuffer{}
	s := &Server{Logger: slog.New(slog.NewTextHandler(logs, nil))}
	for name, write := range map[string]func(http.ResponseWriter){
		"flowsJSON":         func(w http.ResponseWriter) { flowsJSON(w, http.StatusOK, map[string]any{"bad": make(chan int)}) },
		"flowsJSONScrubbed": func(w http.ResponseWriter) { s.flowsJSONScrubbed(w, http.StatusOK, map[string]any{"bad": func() {}}) },
	} {
		w := httptest.NewRecorder()
		write(w)
		if body := flowsBody(t, w); w.Code != http.StatusInternalServerError || body["code"] != "FLOW_INTERNAL" {
			t.Fatalf("%s with an unencodable value = %d %s", name, w.Code, w.Body.String())
		}
	}
	if !strings.Contains(logs.String(), "could not be encoded") {
		t.Fatalf("the scrubbed encode failure is not logged: %s", logs.String())
	}
}

func TestC17MissingMissionIsReportedNotHealed(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	rec := createTestFlow(t, s, greetFlowJSON)
	// Mission Control lost the flow's mission (DeleteFlowMission does not call back).
	if err := s.MissionManagerV2.DeleteFlowMission(rec.MissionID); err != nil {
		t.Fatal(err)
	}
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/publish", token, `{"base_revision":1}`)
	body := flowsBody(t, w)
	if w.Code != http.StatusOK || body["partial"] != true || body["code"] != "FLOW_MISSION_MISSING" ||
		body["error"] != flowPublishMissionMissingMessage {
		t.Fatalf("publish without a mission = %d %s", w.Code, w.Body.String())
	}
	if audit := c17AuditEvents(t, stm, "flow_publish"); len(audit) != 1 || !strings.Contains(audit[0].Summary, "entry is missing") {
		t.Fatalf("audit = %+v", audit)
	}
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+rec.ID+"/enabled", token, `{"enabled":true}`)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_MISSION_MISSING" {
		t.Fatalf("enable without a mission = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/"+rec.ID, token, ""); w.Code != http.StatusOK {
		t.Fatalf("delete without a mission = %d %s", w.Code, w.Body.String())
	}
}

func TestC17DeletingAFlowWithALockedMissionIs409(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	// Mission Control may change only the enabled switch and the lock of a flow mission.
	if err := s.MissionManagerV2.Update(rec.MissionID, &tools.MissionV2{Locked: true}); err != nil {
		t.Fatalf("lock: %v", err)
	}
	w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/"+rec.ID, token, "")
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_LOCKED" {
		t.Fatalf("delete of a locked flow = %d %s", w.Code, w.Body.String())
	}
	if _, err := s.Flows.GetFlow(context.Background(), rec.ID); err != nil {
		t.Fatalf("the locked flow was deleted: %v", err)
	}
	if m, ok := s.MissionManagerV2.Get(rec.MissionID); !ok || !m.Locked {
		t.Fatalf("the locked mission = %+v %v", m, ok)
	}
}

func TestC17SecretDeleteAuditsOnlyRealDeletes(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	if w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/secrets/c17_never", token, ""); w.Code != http.StatusOK {
		t.Fatalf("delete of a missing secret = %d %s", w.Code, w.Body.String())
	}
	if audit := c17AuditEvents(t, stm, "flow_secret_delete"); len(audit) != 0 {
		t.Fatalf("a delete that removed nothing was audited: %+v", audit)
	}
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/c17_real", token, `{"value":"c17-real-value-123"}`); w.Code != http.StatusOK {
		t.Fatalf("put = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/secrets/c17_real", token, ""); w.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", w.Code, w.Body.String())
	}
	if audit := c17AuditEvents(t, stm, "flow_secret_delete"); len(audit) != 1 || audit[0].TargetName != "c17_real" {
		t.Fatalf("audit = %+v", audit)
	}
	// A cancelled request leaves used_by out; the vault delete itself has no context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r := httptest.NewRequest(http.MethodDelete, "/api/desktop/flows/secrets/c17_real", nil).WithContext(ctx)
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	if body := flowsBody(t, w); w.Code != http.StatusOK || body["status"] != "deleted" || body["used_by"] != nil {
		t.Fatalf("cancelled delete = %d %s", w.Code, w.Body.String())
	}
}

func TestC17MalformedFlowPathsAre404(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	paths := []string{
		"/api/desktop/flows/" + strings.Repeat("a", 5000),
		"/api/desktop/flows/a/b/c/d",
		"/api/desktop/flows/" + rec.ID + "/publish/extra",
		"/api/desktop/flows/" + rec.ID + "/export/x",
		"/api/desktop/flows/" + rec.ID + "/nope",
		"/api/desktop/flows/" + rec.ID + "//publish",
		"/api/desktop/flows/flow%20x",
		"/api/desktop/flows/" + strings.Repeat("%27", 30),
	}
	for _, path := range paths {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			w := flowsCall(t, s, method, path, token, `{}`)
			if w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_NOT_FOUND" {
				t.Errorf("%s %s = %d %s", method, flowBoundRunes(path, 60), w.Code, flowBoundRunes(w.Body.String(), 200))
			}
		}
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/flow_missing0", token, ""); w.Code != http.StatusNotFound {
		t.Fatalf("an unknown well-formed id = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID+"/", token, ""); w.Code != http.StatusOK {
		t.Fatalf("a trailing slash = %d", w.Code)
	}
}
