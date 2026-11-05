package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
)

func flowBlockCall(t *testing.T, s *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.handleFlowBlocks(w, r)
	return w
}

func TestFlowBlockLifecycleAndAuditPrivacy(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	events := c17Events(t, s)
	create := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Reusable"})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	block := flowsBody(t, create)["block"].(map[string]any)
	id := block["id"].(string)
	if block["mission_id"] != "" || block["kind"] != "block" {
		t.Fatalf("created block = %+v", block)
	}
	doc := block["draft"].(map[string]any)
	const inputCanary = "block_input_canary_private"
	const outputCanary = "block_output_canary_private"
	const secretCanary = "block_secret_canary_private"
	doc["description"] = secretCanary
	for _, raw := range doc["nodes"].([]any) {
		node := raw.(map[string]any)
		params := node["params"].(map[string]any)
		switch node["type"] {
		case "block.input":
			params["fields"] = []any{map[string]any{"name": "message", "type": "string", "required": true, "nullable": false}}
		case "block.output":
			params["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true, "nullable": false}, map[string]any{"name": "other", "type": "string", "required": true, "nullable": false}}
			params["values"] = map[string]any{"result": "{{block_input.data.message}}", "other": outputCanary}
		}
	}
	path := flowBlocksPath + "/" + id
	save := flowBlockCall(t, s, http.MethodPut, path, token, map[string]any{"doc": doc, "base_revision": 1})
	if save.Code != http.StatusOK {
		t.Fatalf("save: %d %s", save.Code, save.Body.String())
	}
	if stale := flowBlockCall(t, s, http.MethodPost, path+"/publish", token, map[string]any{"base_revision": 1}); stale.Code != http.StatusConflict {
		t.Fatalf("stale publish: %d %s", stale.Code, stale.Body.String())
	}
	preview := flowBlockCall(t, s, http.MethodGet, path+"/publish-preview", token, nil)
	if preview.Code != http.StatusOK || flowsBody(t, preview)["draft_revision"] != float64(2) {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	contract := flowsBody(t, preview)["contract"].(map[string]any)
	if len(contract["inputs"].([]any)) != 1 || len(contract["outputs"].([]any)) != 2 {
		t.Fatalf("preview contract = %+v", contract)
	}
	sim := flowBlockCall(t, s, http.MethodPost, path+"/simulate", token, map[string]any{"base_revision": flowsBody(t, preview)["draft_revision"], "inputs": map[string]any{"message": inputCanary}, "at": "2026-01-02T03:04:05Z"})
	if sim.Code != http.StatusOK {
		t.Fatalf("simulation: %d %s", sim.Code, sim.Body.String())
	}
	result := flowsBody(t, sim)
	outputs, ok := result["outputs"].(map[string]any)
	if !ok || result["mode"] != "simulation" || result["status"] != "success" || result["started_at"] != "2026-01-02T03:04:05Z" || outputs["result"] != inputCanary || outputs["other"] != outputCanary {
		t.Fatalf("simulation result = %+v", result)
	}
	for _, body := range []map[string]any{
		{"base_revision": 1, "inputs": map[string]any{"message": inputCanary}},
		{"base_revision": 2, "inputs": map[string]any{"message": false}},
	} {
		w := flowBlockCall(t, s, http.MethodPost, path+"/simulate", token, body)
		if w.Code < 400 {
			t.Errorf("bad simulation accepted: %d %s", w.Code, w.Body.String())
		}
	}
	c18Secret(t, secretCanary)
	redacted := flowBlockCall(t, s, http.MethodPost, path+"/simulate", token, map[string]any{"base_revision": 2, "inputs": map[string]any{"message": secretCanary}})
	if redacted.Code != http.StatusOK || strings.Contains(redacted.Body.String(), secretCanary) {
		t.Fatalf("simulation did not scrub registered sensitive input: %d %s", redacted.Code, redacted.Body.String())
	}
	runs, err := s.Flows.Store().ListRuns(context.Background(), id, flows.RunFilter{Limit: 10})
	if err != nil || len(runs) != 0 {
		t.Fatalf("simulation persisted runs: %+v %v", runs, err)
	}
	pub := flowBlockCall(t, s, http.MethodPost, path+"/publish", token, map[string]any{"base_revision": 2})
	if pub.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", pub.Code, pub.Body.String())
	}
	published := flowsBody(t, pub)["block"].(map[string]any)
	live := int(published["live_revision"].(float64))
	version := flowBlockCall(t, s, http.MethodGet, fmt.Sprintf("%s/versions/%d", path, live), token, nil)
	if version.Code != http.StatusOK || flowsBody(t, version)["contract"] == nil {
		t.Fatalf("version: %d %s", version.Code, version.Body.String())
	}
	dependents := flowBlockCall(t, s, http.MethodGet, path+"/dependents?limit=1", token, nil)
	if dependents.Code != http.StatusOK || len(flowsBody(t, dependents)["items"].([]any)) != 0 {
		t.Fatalf("dependents: %d %s", dependents.Code, dependents.Body.String())
	}
	for _, query := range []string{"revision=-1", "limit=0", "limit=101", "revision=abc"} {
		w := flowBlockCall(t, s, http.MethodGet, path+"/dependents?"+query, token, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("dependents %s: %d %s", query, w.Code, w.Body.String())
		}
	}
	if stale := flowBlockCall(t, s, http.MethodDelete, path, token, map[string]any{"base_revision": 1, "base_live_revision": live}); stale.Code != http.StatusConflict {
		t.Fatalf("stale delete: %d %s", stale.Code, stale.Body.String())
	}
	deleted := flowBlockCall(t, s, http.MethodDelete, path, token, map[string]any{"base_revision": 2, "base_live_revision": live})
	if deleted.Code != http.StatusOK {
		t.Fatalf("delete: %d %s", deleted.Code, deleted.Body.String())
	}
	for _, action := range []string{"flow_block_create", "flow_block_publish", "flow_block_delete"} {
		events := c17AuditEvents(t, stm, action)
		if len(events) != 1 || events[0].TargetID != id {
			t.Fatalf("%s events = %+v", action, events)
		}
		encoded, err := json.Marshal(events[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, canary := range []string{inputCanary, outputCanary, secretCanary} {
			if bytes.Contains(encoded, []byte(canary)) {
				t.Errorf("%s audit contains private data", action)
			}
		}
	}
	broadcasts := c17Drain(events)
	if len(broadcasts) != 4 {
		t.Fatalf("block broadcasts = %+v", broadcasts)
	}
	for i, reason := range []string{"created", "saved", "published", "deleted"} {
		payload, ok := broadcasts[i].Payload.(map[string]interface{})
		if broadcasts[i].Type != "flows_changed" || !ok || len(payload) != 2 || payload["flow_id"] != id || payload["reason"] != reason {
			t.Fatalf("broadcast %d = %+v", i, broadcasts[i])
		}
	}
}

func TestFlowBlockRoutesRejectWrongKindAndUnknownPaths(t *testing.T) {
	s, token := newFlowsTestServer(t)
	create := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Block"})
	if create.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	block := flowsBody(t, create)["block"].(map[string]any)
	id := block["id"].(string)
	flow := createTestFlow(t, s, greetFlowJSON)
	for _, path := range []string{
		flowBlocksPath + "//" + id, flowBlocksPath + "/" + id + "//", flowBlocksPath + "/" + id + "/unknown", flowBlocksPath + "/" + id + "/run", flowBlocksPath + "/" + id + "/enabled",
		flowBlocksPath + "/" + flow.ID, flowBlocksPath + "/" + flow.ID + "/versions",
	} {
		w := flowBlockCall(t, s, http.MethodGet, path, token, nil)
		if w.Code != http.StatusNotFound {
			t.Errorf("GET %s = %d %s", path, w.Code, w.Body.String())
		}
	}
	for _, suffix := range []string{"", "/publish-preview", "/versions", "/scenarios", "/test-data/n_aaaaaaaa"} {
		w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id+suffix, token, "")
		if w.Code != http.StatusNotFound {
			t.Errorf("legacy flow route %s: %d %s", suffix, w.Code, w.Body.String())
		}
	}
	for _, tc := range []struct{ method, suffix, body string }{
		{http.MethodPost, "/publish", `{"base_revision":1}`},
		{http.MethodPost, "/enabled", `{"enabled":true}`},
		{http.MethodPost, "/run", `{}`},
		{http.MethodPost, "/test", `{"base_revision":1}`},
		{http.MethodPost, "/simulate", `{"base_revision":1}`},
		{http.MethodPost, "/repair-mission", `{"base_revision":1,"base_live_revision":0}`},
		{http.MethodPost, "/versions", `{"revision":1,"base_revision":1}`},
		{http.MethodPut, "/test-data/n_aaaaaaaa", `{"data":{"test":"value"}}`},
	} {
		w := flowsCall(t, s, tc.method, "/api/desktop/flows/"+id+tc.suffix, token, tc.body)
		if w.Code != http.StatusNotFound {
			t.Errorf("legacy %s %s: %d %s", tc.method, tc.suffix, w.Code, w.Body.String())
		}
	}
	blockJSON, err := json.Marshal(block["draft"])
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/desktop/flows", `{"import":` + string(blockJSON) + `}`},
		{http.MethodPut, "/api/desktop/flows/" + flow.ID, `{"doc":` + string(blockJSON) + `,"base_revision":1}`},
	} {
		w := flowsCall(t, s, tc.method, tc.path, token, tc.body)
		if w.Code >= 200 && w.Code < 300 {
			t.Errorf("legacy %s accepted block document: %d %s", tc.method, w.Code, w.Body.String())
		}
	}
}

func TestFlowBlockCatalogAndValidation(t *testing.T) {
	s, token := newFlowsTestServer(t)
	answer := flowBlockCall(t, s, http.MethodGet, flowBlocksPath+"/node-types", token, nil)
	if answer.Code != http.StatusOK {
		t.Fatalf("catalog: %d %s", answer.Code, answer.Body.String())
	}
	allowed := map[string]bool{"block.input": true, "block.output": true, "logic.set": true, "logic.if": true, "logic.switch": true, "logic.merge": true, "component.call": true}
	seen := map[string]bool{}
	for _, raw := range flowsBody(t, answer)["node_types"].([]any) {
		node := raw.(map[string]any)
		id := node["type"].(string)
		if !allowed[id] || seen[id] {
			t.Errorf("unexpected or duplicate catalog node %q", id)
		}
		seen[id] = true
	}
	if len(seen) != len(allowed) {
		t.Errorf("catalog nodes = %v", seen)
	}
	created := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Catalog"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	doc := flowsBody(t, created)["block"].(map[string]any)["draft"]
	valid := flowBlockCall(t, s, http.MethodPost, flowBlocksPath+"/validate", token, map[string]any{"doc": doc, "mode": "publish"})
	if valid.Code != http.StatusOK || flowsBody(t, valid)["valid"] != true {
		t.Fatalf("validate: %d %s", valid.Code, valid.Body.String())
	}
	invalid := flowBlockCall(t, s, http.MethodPost, flowBlocksPath+"/validate", token, map[string]any{"doc": doc, "mode": "unknown"})
	if invalid.Code != http.StatusBadRequest {
		t.Fatalf("invalid mode: %d %s", invalid.Code, invalid.Body.String())
	}
}

func TestFlowBlockMuxRedirectDoesNotMutate(t *testing.T) {
	s, token := newFlowsTestServer(t)
	mux := http.NewServeMux()
	registerFlowsRoutes(mux, s)
	read := httptest.NewRequest(http.MethodGet, flowBlocksPath, nil)
	read.Header.Set("Authorization", "Bearer "+token)
	readResponse := httptest.NewRecorder()
	mux.ServeHTTP(readResponse, read)
	if readResponse.Code != http.StatusOK {
		t.Fatalf("registered block route: %d %s", readResponse.Code, readResponse.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, flowBlocksPath+"//", bytes.NewBufferString(`{"name":"redirect"}`))
	r.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, r)
	if w.Code < 300 || w.Code >= 400 {
		t.Fatalf("expected canonical redirect: %d %s", w.Code, w.Body.String())
	}
	list := flowBlockCall(t, s, http.MethodGet, flowBlocksPath, token, nil)
	if list.Code != http.StatusOK || len(flowsBody(t, list)["blocks"].([]any)) != 0 {
		t.Fatalf("redirect changed the collection: %d %s", list.Code, list.Body.String())
	}
}

func TestFlowBlockListPagination(t *testing.T) {
	s, token := newFlowsTestServer(t)
	for _, name := range []string{"First", "Second", "Third"} {
		w := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": name})
		if w.Code != http.StatusCreated {
			t.Fatalf("create: %d %s", w.Code, w.Body.String())
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 3; page++ {
		w := flowBlockCall(t, s, http.MethodGet, flowBlocksPath+"?limit=1&cursor="+cursor, token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("list page: %d %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		items := body["blocks"].([]any)
		if len(items) != 1 {
			t.Fatalf("page = %+v", body)
		}
		item := items[0].(map[string]any)
		id := item["id"].(string)
		if seen[id] || item["draft"] != nil || item["live"] != nil {
			t.Fatalf("list item = %+v", item)
		}
		seen[id] = true
		cursor, _ = body["next_cursor"].(string)
		if (page < 2) != (cursor != "") {
			t.Fatalf("page %d cursor = %q", page, cursor)
		}
	}
	for _, query := range []string{"limit=0", "limit=101", "limit=bad", "cursor=%21"} {
		w := flowBlockCall(t, s, http.MethodGet, flowBlocksPath+"?"+query, token, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("list %s: %d %s", query, w.Code, w.Body.String())
		}
	}
}

func TestFlowBlockVersionPagination(t *testing.T) {
	s, token := newFlowsTestServer(t)
	created := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Versioned"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	block := flowsBody(t, created)["block"].(map[string]any)
	path := flowBlocksPath + "/" + block["id"].(string)
	doc := block["draft"]
	for draft := 1; draft <= 3; draft++ {
		if draft > 1 {
			w := flowBlockCall(t, s, http.MethodPut, path, token, map[string]any{"doc": doc, "base_revision": draft - 1})
			if w.Code != http.StatusOK {
				t.Fatalf("save: %d %s", w.Code, w.Body.String())
			}
		}
		w := flowBlockCall(t, s, http.MethodPost, path+"/publish", token, map[string]any{"base_revision": draft})
		if w.Code != http.StatusOK {
			t.Fatalf("publish: %d %s", w.Code, w.Body.String())
		}
	}
	cursor := 0
	last := int(^uint(0) >> 1)
	for page := 0; page < 3; page++ {
		w := flowBlockCall(t, s, http.MethodGet, fmt.Sprintf("%s/versions?limit=1&before_revision=%d", path, cursor), token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("version page: %d %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		items := body["versions"].([]any)
		if len(items) != 1 {
			t.Fatalf("page = %+v", body)
		}
		revision := int(items[0].(map[string]any)["revision"].(float64))
		if revision >= last {
			t.Fatalf("version order %d after %d", revision, last)
		}
		last = revision
		value, ok := body["next_cursor"].(float64)
		if (page < 2) != ok {
			t.Fatalf("page %d cursor = %v", page, body["next_cursor"])
		}
		cursor = int(value)
	}
	for _, query := range []string{"before_revision=-1", "limit=0", "limit=101"} {
		w := flowBlockCall(t, s, http.MethodGet, path+"/versions?"+query, token, nil)
		if w.Code != http.StatusBadRequest {
			t.Errorf("versions %s: %d %s", query, w.Code, w.Body.String())
		}
	}
}

func TestFlowBlocksPathAndBearerScope(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	read := ff1Token(t, s, "block read", desktopScopeRead)
	write := ff1Token(t, s, "block write", desktopScopeRead, desktopScopeWrite)
	admin := ff1Token(t, s, "block admin", desktopScopeAdmin)
	for _, path := range []string{flowBlocksPath, flowBlocksPath + "/block_1/versions/1", flowBlocksPath + "/block_1/simulate"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			if !validRouteBearer(s, read, path, method) {
				t.Errorf("read token denied %s %s", method, path)
			}
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
			if validRouteBearer(s, read, path, method) || validRouteBearer(s, write, path, method) || !validRouteBearer(s, admin, path, method) {
				t.Errorf("incorrect mutation scope for %s %s", method, path)
			}
		}
	}
	if isFlowBlocksAPIPath(flowBlocksPath+"-other") || isFlowBlocksAPIPath("/api/desktop/flows/blocks") {
		t.Fatal("block namespace matched an unrelated path")
	}
	for _, path := range []string{flowBlocksPath + "//x", flowBlocksPath + "/x//", flowBlocksPath + "//"} {
		if validFlowBlockPath(path) {
			t.Errorf("repeated path separator accepted: %q", path)
		}
	}
	if got := flowBlockPathParts(flowBlocksPath + "/block_1/versions/2"); !reflect.DeepEqual(got, []string{"block_1", "versions", "2"}) {
		t.Fatalf("path parts = %#v", got)
	}
}

func TestFlowBlockErrorMapping(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{flows.ErrWrongFlowKind, http.StatusNotFound, "FLOW_NOT_FOUND"},
		{flows.ErrBlockUnavailable, http.StatusConflict, "FLOW_BLOCK_UNAVAILABLE"},
		{flows.ErrBlockInvalid, http.StatusBadRequest, "FLOW_BAD_REQUEST"},
	} {
		w := httptest.NewRecorder()
		r := httptest.NewRequest(http.MethodPost, "/api/desktop/flows/flow_example/publish", nil)
		s.flowsErrorFrom(w, r, tc.err)
		if w.Code != tc.status || flowsBody(t, w)["code"] != tc.code {
			t.Errorf("%v: %d %s", tc.err, w.Code, w.Body.String())
		}
	}
	w := httptest.NewRecorder()
	s.flowBlockError(w, httptest.NewRequest(http.MethodDelete, flowBlocksPath+"/flow_example", nil), flows.ErrBlockInUse)
	if w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_BLOCK_IN_USE" {
		t.Fatalf("block in use: %d %s", w.Code, w.Body.String())
	}
}

func TestFlowBlockAuditTypesAreRegistered(t *testing.T) {
	actions := []string{"flow_block_create", "flow_block_publish", "flow_block_delete"}
	markup := readDashboardMarkupForTest(t)
	for _, action := range actions {
		if !strings.Contains(markup, `<option value="`+action+`" data-i18n="dashboard.audit_type_`+action+`">`) {
			t.Errorf("dashboard audit filter is missing %s", action)
		}
	}
	locales, err := filepath.Glob(filepath.Join("..", "..", "ui", "lang", "dashboard", "*.json"))
	if err != nil || len(locales) != 16 {
		t.Fatalf("dashboard locales = %v, %v", locales, err)
	}
	for _, path := range locales {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var labels map[string]string
		if err := json.Unmarshal(data, &labels); err != nil {
			t.Fatal(err)
		}
		for _, action := range actions {
			if strings.TrimSpace(labels["dashboard.audit_type_"+action]) == "" {
				t.Errorf("%s has no audit label for %s", path, action)
			}
		}
	}
}

func TestFlowBlockAdmissionGates(t *testing.T) {
	s, token := newFlowsTestServer(t)
	call := func(method, path, credential string) (bool, *httptest.ResponseRecorder) {
		t.Helper()
		r := httptest.NewRequest(method, path, nil)
		if credential != "" {
			r.Header.Set("Authorization", "Bearer "+credential)
		}
		w := httptest.NewRecorder()
		return s.admitFlowBlockRequest(w, r, flowBlockPathParts(path)), w
	}
	if ok, w := call(http.MethodGet, flowBlocksPath, ""); ok || w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read: admitted=%v status=%d", ok, w.Code)
	}
	for _, tc := range []struct {
		origin string
		allow  bool
	}{{"http://example.com", true}, {"https://evil.example", false}} {
		r := httptest.NewRequest(http.MethodPost, flowBlocksPath+"/validate", nil)
		r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
		r.Header.Set("Origin", tc.origin)
		w := httptest.NewRecorder()
		if ok := s.admitFlowBlockRequest(w, r, flowBlockPathParts(r.URL.Path)); ok != tc.allow {
			t.Fatalf("origin %q: admitted=%v status=%d", tc.origin, ok, w.Code)
		}
	}
	for _, setReadOnly := range []func(bool){
		func(v bool) { s.Cfg.Tools.Missions.ReadOnly = v },
		func(v bool) { s.Cfg.VirtualDesktop.ReadOnly = v },
	} {
		setReadOnly(true)
		if ok, w := call(http.MethodPost, flowBlocksPath, token); ok || w.Code != http.StatusForbidden {
			t.Fatalf("read-only create: admitted=%v status=%d", ok, w.Code)
		}
		if ok, w := call(http.MethodPost, flowBlocksPath+"/validate", token); !ok {
			t.Fatalf("read-only validation: status=%d body=%s", w.Code, w.Body.String())
		}
		if ok, w := call(http.MethodPost, flowBlocksPath+"/flow_x/simulate", token); ok || w.Code != http.StatusForbidden {
			t.Fatalf("read-only simulation: admitted=%v status=%d", ok, w.Code)
		}
		setReadOnly(false)
	}
	s.Cfg.Flows.Enabled = false
	if ok, w := call(http.MethodGet, flowBlocksPath, token); ok || w.Code != http.StatusServiceUnavailable {
		t.Fatalf("disabled read: admitted=%v status=%d", ok, w.Code)
	}
}
