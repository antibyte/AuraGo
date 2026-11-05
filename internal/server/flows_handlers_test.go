package server

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"aurago/internal/flows"
	"aurago/internal/tools"
)

func flowsCall(t *testing.T, s *Server, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, path, reader)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	return w
}

func flowsBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("body %q: %v", w.Body.String(), err)
	}
	return out
}

func flowsHTTPMux(s *Server) *http.ServeMux {
	mux := http.NewServeMux()
	registerFlowsRoutes(mux, s)
	return mux
}

func flowsHTTPCall(t *testing.T, mux *http.ServeMux, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	r := httptest.NewRequest(method, path, reader)
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	return w
}

func TestFlowsMissionStateHTTPChecksStoredOwnerAndReadScope(t *testing.T) {
	s, token := newFlowsTestServer(t)
	mux := flowsHTTPMux(s)
	create := flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows", token, `{"name":"Erster"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	first := flowsBody(t, create)["flow"].(map[string]any)
	flowID := first["id"].(string)
	create = flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows", token, `{"name":"Zweiter"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("second create = %d %s", create.Code, create.Body.String())
	}
	second := flowsBody(t, create)["flow"].(map[string]any)
	secondFlowID := second["id"].(string)
	secondMissionID := second["mission_id"].(string)
	if err := s.MissionManagerV2.SetFlowMissionEnabled(secondMissionID, secondFlowID, true); err != nil {
		t.Fatal(err)
	}
	assertState := func(id, token, want string) {
		t.Helper()
		w := flowsHTTPCall(t, mux, http.MethodGet, "/api/desktop/flows/"+id+"/enabled", token, "")
		if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatalf("state for %s = %d %s headers=%v", id, w.Code, w.Body.String(), w.Header())
		}
		body := flowsBody(t, w)
		if len(body) != 1 || body["mission_state"] != want {
			t.Fatalf("state body for %s = %+v, want mission_state %q only", id, body, want)
		}
	}
	assertState(secondFlowID, token, "enabled")
	assertState(flowID, token, "disabled")
	readToken, _, err := s.TokenManager.Create("flow state reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	assertState(flowID, readToken, "disabled")
	if w := flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows/"+flowID+"/enabled", readToken, `{"enabled":false}`); w.Code != http.StatusForbidden {
		t.Fatalf("read token write = %d %s", w.Code, w.Body.String())
	}

	store, err := flows.OpenStore(s.flowsDBPath(), s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetMissionID(context.Background(), flowID, secondMissionID); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertState(flowID, token, "mismatch")
	if w := flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows/"+flowID+"/enabled", token, `{"enabled":false}`); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_MISSION_MISMATCH" {
		t.Fatalf("foreign mission pause = %d %s", w.Code, w.Body.String())
	}
	if m, _ := s.MissionManagerV2.Get(secondMissionID); !m.Enabled {
		t.Fatal("foreign flow pause changed the other flow's mission")
	}

	store, err = flows.OpenStore(s.flowsDBPath(), s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetMissionID(context.Background(), flowID, "mission_missing"); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertState(flowID, token, "missing")
	if err := s.MissionManagerV2.Create(&tools.MissionV2{ID: "mission_ordinary", Name: "Ordinary", Prompt: "run", ExecutionType: tools.ExecutionManual}); err != nil {
		t.Fatal(err)
	}
	store, err = flows.OpenStore(s.flowsDBPath(), s.Logger)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetMissionID(context.Background(), flowID, "mission_ordinary"); err != nil {
		_ = store.Close()
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	assertState(flowID, token, "mismatch")
	if state := (flowMissionBridge{s: &Server{}}).FlowMissionState("mission_unavailable", flowID); state != flows.MissionStateUnavailable {
		t.Fatalf("unavailable manager state = %q", state)
	}
	if w := flowsHTTPCall(t, mux, http.MethodGet, "/api/desktop/flows/flow_missing/enabled", token, ""); w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_NOT_FOUND" {
		t.Fatalf("missing flow = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowsEnabledStatusRejectsCorruptBlockIDWithoutReadingItsDocument(t *testing.T) {
	s, token := newFlowsTestServer(t)
	mux := flowsHTTPMux(s)
	block, err := s.Flows.CreateBlock(context.Background(), "Block")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := sql.Open("sqlite", s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	if _, err := raw.Exec(`UPDATE flows SET draft_json=? WHERE id=?`, "not-json", block.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Flows.GetBlock(context.Background(), block.ID); err == nil {
		t.Fatal("corrupt block document unexpectedly parsed")
	}
	w := flowsHTTPCall(t, mux, http.MethodGet, "/api/desktop/flows/"+block.ID+"/enabled", token, "")
	if w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_NOT_FOUND" {
		t.Fatalf("block id status = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowsMissionStateHTTPObservesCurrentStateAfterFailedSave(t *testing.T) {
	s, token := newFlowsTestServer(t)
	mux := flowsHTTPMux(s)
	create := flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows", token, `{"name":"State"}`)
	if create.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", create.Code, create.Body.String())
	}
	created := flowsBody(t, create)["flow"].(map[string]any)
	flowID, missionID := created["id"].(string), created["mission_id"].(string)
	if err := s.MissionManagerV2.SetFlowMissionEnabled(missionID, flowID, true); err != nil {
		t.Fatal(err)
	}
	before, err := s.Flows.GetFlow(context.Background(), flowID)
	if err != nil {
		t.Fatal(err)
	}
	blockedTemp := filepath.Join(s.Cfg.Directories.DataDir, "missions_v2.json.tmp")
	if err := os.Mkdir(blockedTemp, 0o700); err != nil {
		t.Fatal(err)
	}
	failed := flowsHTTPCall(t, mux, http.MethodPost, "/api/desktop/flows/"+flowID+"/enabled", token, `{"enabled":false}`)
	if failed.Code != http.StatusInternalServerError || flowsBody(t, failed)["code"] != "FLOW_INTERNAL" {
		t.Fatalf("failed save = %d %s", failed.Code, failed.Body.String())
	}
	readback := flowsHTTPCall(t, mux, http.MethodGet, "/api/desktop/flows/"+flowID+"/enabled", token, "")
	if readback.Code != http.StatusOK || flowsBody(t, readback)["mission_state"] != "disabled" {
		t.Fatalf("readback after failed save = %d %s", readback.Code, readback.Body.String())
	}
	after, err := s.Flows.GetFlow(context.Background(), flowID)
	if err != nil {
		t.Fatal(err)
	}
	beforeDraft, _ := json.Marshal(before.Draft)
	afterDraft, _ := json.Marshal(after.Draft)
	if before.DraftRevision != after.DraftRevision || before.LiveRevision != after.LiveRevision || !bytes.Equal(beforeDraft, afterDraft) || before.MissionID != after.MissionID {
		t.Fatalf("failed pause changed flow data: before=%+v after=%+v", before, after)
	}
}

func TestFlowsAPIGates(t *testing.T) {
	s, token := newFlowsTestServer(t)
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""); w.Code != http.StatusOK {
		t.Fatalf("list = %d %s", w.Code, w.Body.String())
	}
	s.Cfg.Tools.Missions.ReadOnly = true
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"x"}`); w.Code != http.StatusForbidden ||
		flowsBody(t, w)["code"] != "FLOW_PERMISSION_DENIED" {
		t.Fatalf("read-only create = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""); w.Code != http.StatusOK {
		t.Fatalf("read-only list = %d", w.Code)
	}
	s.Cfg.Tools.Missions.ReadOnly = false
	s.Cfg.Flows.Enabled = false
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""); w.Code != http.StatusServiceUnavailable ||
		flowsBody(t, w)["code"] != "FLOWS_DISABLED" {
		t.Fatalf("disabled = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowsAPIDocumentLifecycle(t *testing.T) {
	s, token := newFlowsTestServer(t)
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"name":"Gruss"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create = %d %s", w.Code, w.Body.String())
	}
	flow := flowsBody(t, w)["flow"].(map[string]any)
	id, missionID := flow["id"].(string), flow["mission_id"].(string)
	if m, ok := s.MissionManagerV2.Get(missionID); !ok || m.ExecutionType != tools.ExecutionFlow {
		t.Fatalf("mission = %+v %v", m, ok)
	}
	if list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows", token, ""))["flows"].([]any); len(list) != 1 {
		t.Fatalf("list = %+v", list)
	}
	save := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, `{"doc":`+greetFlowJSON+`,"base_revision":1}`)
	if save.Code != http.StatusOK || flowsBody(t, save)["draft_revision"].(float64) != 2 {
		t.Fatalf("save = %d %s", save.Code, save.Body.String())
	}
	stale := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, `{"doc":`+greetFlowJSON+`,"base_revision":1}`)
	if stale.Code != http.StatusConflict || flowsBody(t, stale)["code"] != "FLOW_REVISION_CONFLICT" {
		t.Fatalf("stale save = %d %s", stale.Code, stale.Body.String())
	}
	bad := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, `{"doc":{"schema":1,"name":""},"base_revision":2}`)
	if bad.Code != http.StatusUnprocessableEntity || flowsBody(t, bad)["code"] != "FLOW_INVALID" {
		t.Fatalf("invalid save = %d %s", bad.Code, bad.Body.String())
	}
	if preview := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id+"/publish-preview", token, "")); preview["can_publish"] != true {
		t.Fatalf("preview = %+v", preview)
	}
	if pub := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/publish", token, `{"base_revision":2}`); pub.Code != http.StatusOK {
		t.Fatalf("publish = %d %s", pub.Code, pub.Body.String())
	}
	if m, _ := s.MissionManagerV2.Get(missionID); !m.FlowPublished || len(m.FlowTriggers) != 1 || m.FlowTriggers[0].TriggerType != tools.FlowTriggerManual {
		t.Fatalf("published mission = %+v", m)
	}
	if en := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/enabled", token, `{"enabled":true}`); en.Code != http.StatusOK {
		t.Fatalf("enable = %d %s", en.Code, en.Body.String())
	}
	if detail := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id, token, "")); detail["enabled"] != true {
		t.Fatalf("detail = %+v", detail)
	}
	exp := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id+"/export", token, "")
	if exp.Code != http.StatusOK || !strings.Contains(exp.Header().Get("Content-Disposition"), `filename="gruss.easydrag.json"`) {
		t.Fatalf("export = %d %q", exp.Code, exp.Header().Get("Content-Disposition"))
	}
	if _, err := flows.ParseFlow(exp.Body.Bytes()); err != nil {
		t.Fatalf("export does not parse: %v", err)
	}
	if del := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/"+id, token, ""); del.Code != http.StatusOK {
		t.Fatalf("delete = %d %s", del.Code, del.Body.String())
	}
	if _, ok := s.MissionManagerV2.Get(missionID); ok {
		t.Fatal("deleting the flow must delete its mission")
	}
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id, token, ""); w.Code != http.StatusNotFound || flowsBody(t, w)["code"] != "FLOW_NOT_FOUND" {
		t.Fatalf("deleted flow = %d %s", w.Code, w.Body.String())
	}
}

func TestFlowsAPIImportAndSecrets(t *testing.T) {
	s, token := newFlowsTestServer(t)
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+greetFlowJSON+`}`)
	if w.Code != http.StatusCreated || flowsBody(t, w)["flow"].(map[string]any)["name"] != "Gruss" {
		t.Fatalf("import = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":{"schema":99}}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad import = %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/api_token", token, `{"value":"tok-123456789012"}`); w.Code != http.StatusOK {
		t.Fatalf("put secret = %d %s", w.Code, w.Body.String())
	}
	list := flowsBody(t, flowsCall(t, s, http.MethodGet, "/api/desktop/flows/secrets", token, ""))["secrets"].([]any)
	if len(list) != 1 || list[0] != "api_token" {
		t.Fatalf("secrets = %+v", list)
	}
	if v, err := (flowSecrets{s: s}).ReadSecret("api_token"); err != nil || v != "tok-123456789012" {
		t.Fatalf("ReadSecret = %q, %v", v, err)
	}
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/secrets/Bad-Name", token, `{"value":"x"}`); w.Code != http.StatusBadRequest {
		t.Fatalf("bad name = %d", w.Code)
	}
	if w := flowsCall(t, s, http.MethodDelete, "/api/desktop/flows/secrets/api_token", token, ""); w.Code != http.StatusOK {
		t.Fatalf("delete secret = %d %s", w.Code, w.Body.String())
	}
	if _, err := (flowSecrets{s: s}).ReadSecret("api_token"); err == nil {
		t.Fatal("the deleted secret is still readable")
	}
}
