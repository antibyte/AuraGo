package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/dbutil"
	"aurago/internal/security"
)

// TestFlowChildHistoryHTTP exercises the real handler, engine, journal and pins.
func TestFlowChildHistoryHTTP(t *testing.T) {
	s, admin := newFlowsTestServer(t)
	createBlock := func(name, nested string) string {
		t.Helper()
		w := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, admin, map[string]any{"name": name})
		if w.Code != http.StatusCreated {
			t.Fatalf("create block: %d %s", w.Code, w.Body.String())
		}
		block := flowsBody(t, w)["block"].(map[string]any)
		id := block["id"].(string)
		doc := block["draft"].(map[string]any)
		nodes := doc["nodes"].([]any)
		var in, out string
		for _, raw := range nodes {
			n := raw.(map[string]any)
			params := n["params"].(map[string]any)
			switch n["type"] {
			case "block.input":
				in = n["id"].(string)
				params["fields"] = []any{map[string]any{"name": "value", "type": "string", "required": true}}
			case "block.output":
				out = n["id"].(string)
				params["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true}}
				value := "{{block_input.data.value}}"
				if nested != "" {
					value = "{{nested.result}}"
				}
				params["values"] = map[string]any{"result": value}
			}
		}
		if nested != "" {
			nodes = append(nodes, map[string]any{"id": "n_nestedaa", "key": "nested", "type": "component.call", "type_version": 1,
				"params": map[string]any{"flow_id": nested, "revision": 1, "inputs": map[string]any{"value": "{{block_input.data.value}}"}}})
			doc["nodes"] = nodes
			doc["edges"] = []any{
				map[string]any{"id": "e_nestedaa", "source": map[string]any{"node": in, "port": "out"}, "target": map[string]any{"node": "n_nestedaa", "port": "in"}},
				map[string]any{"id": "e_nestedab", "source": map[string]any{"node": "n_nestedaa", "port": "out"}, "target": map[string]any{"node": out, "port": "in"}},
			}
		}
		path := flowBlocksPath + "/" + id
		if w = flowBlockCall(t, s, http.MethodPut, path, admin, map[string]any{"doc": doc, "base_revision": 1}); w.Code != http.StatusOK {
			t.Fatalf("save block: %d %s", w.Code, w.Body.String())
		}
		if w = flowBlockCall(t, s, http.MethodPost, path+"/publish", admin, map[string]any{"base_revision": 2}); w.Code != http.StatusOK {
			t.Fatalf("publish block: %d %s", w.Code, w.Body.String())
		}
		return id
	}
	leaf := createBlock("Leaf", "")
	outer := createBlock("Outer", leaf)
	doc := fmt.Sprintf(`{"schema":1,"kind":"flow","name":"Parent","nodes":[{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},{"id":"n_bbbbbbbb","key":"call","type":"component.call","type_version":1,"params":{"flow_id":%q,"revision":1,"inputs":{"value":"{{trigger.data.value}}"}}}],"edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`, outer)
	w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", admin, `{"import":`+doc+`}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("create parent: %d %s", w.Code, w.Body.String())
	}
	flowID := flowsBody(t, w)["flow"].(map[string]any)["id"].(string)
	const canary = "child-history-later-sensitive-canary"
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+flowID+"/test", admin, `{"base_revision":1,"trigger_data":{"value":"`+canary+`"}}`)
	if w.Code != http.StatusAccepted {
		t.Fatalf("start parent: %d %s", w.Code, w.Body.String())
	}
	runID := flowsBody(t, w)["run_id"].(string)
	detail := waitFlowRunStatus(t, s, admin, runID, "success")
	var parentSeq int
	for _, raw := range detail["steps"].([]any) {
		step := raw.(map[string]any)
		if step["node_id"] == "n_bbbbbbbb" {
			if summary, ok := step["child_history"].(map[string]any); ok {
				parentSeq = int(summary["parent_seq"].(float64))
			}
		}
	}
	if parentSeq <= 0 {
		t.Fatalf("missing committed parent sequence: %+v", detail)
	}
	path := fmt.Sprintf("/api/desktop/flows/runs/%s/steps/%d/children", runID, parentSeq)
	reader, _, err := s.TokenManager.Create("child history reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	writer, _, err := s.TokenManager.Create("child history writer", []string{desktopScopeWrite}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w = flowsCall(t, s, http.MethodGet, path, "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous read = %d", w.Code)
	}
	if w = flowsCall(t, s, http.MethodGet, path, writer, ""); w.Code != http.StatusForbidden {
		t.Fatalf("write-only read = %d", w.Code)
	}
	for _, invalidPath := range []string{
		fmt.Sprintf("/api/desktop/flows/runs/%s/steps/0/children", runID),
		fmt.Sprintf("/api/desktop/flows/runs/%s/steps/01/children", runID),
		fmt.Sprintf("/api/desktop/flows/runs/%s/steps/999999999999999999999999999999/children", runID),
		path + "//document", path + "/i_0/document", path + "/i_01/document", path + "/i_1001/document",
	} {
		if w = flowsCall(t, s, http.MethodGet, invalidPath, reader, ""); w.Code != http.StatusNotFound {
			t.Fatalf("invalid child path %q = %d", invalidPath, w.Code)
		}
	}
	if w = flowsCall(t, s, http.MethodPost, path, admin, ""); w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("child trace mutation = %d", w.Code)
	}
	s.Cfg.Tools.Missions.ReadOnly = true
	if w = flowsCall(t, s, http.MethodGet, path, reader, ""); w.Code != http.StatusOK {
		t.Fatalf("read-only child history = %d", w.Code)
	}
	s.Cfg.Tools.Missions.ReadOnly = false
	w = flowsCall(t, s, http.MethodGet, path, reader, "")
	if w.Code != http.StatusOK || w.Header().Get("Cache-Control") != "no-store" || w.Body.Len() > 256<<10 || !json.Valid(w.Body.Bytes()) {
		t.Fatalf("trace = %d %s", w.Code, w.Body.String())
	}
	trace := flowsBody(t, w)
	frames, ok := trace["frames"].([]any)
	if trace["schema"] != float64(1) || trace["run_id"] != runID || int(trace["parent_seq"].(float64)) != parentSeq || !ok || len(frames) < 2 {
		t.Fatalf("nested trace = %+v", trace)
	}
	outerFrame := frames[0].(map[string]any)
	invocation := outerFrame["invocation_id"].(string)
	if outerFrame["block_id"] != outer || outerFrame["revision"] != float64(1) || len(outerFrame["steps"].([]any)) == 0 {
		t.Fatalf("outer frame = %+v", outerFrame)
	}
	docPath := path + "/" + invocation + "/document"
	if w = flowsCall(t, s, http.MethodGet, docPath, reader, ""); w.Code != http.StatusOK || flowsBody(t, w)["block_id"] != outer {
		t.Fatalf("pinned document = %d %s", w.Code, w.Body.String())
	}
	// Simulations retain inline history without creating another stored run.
	runsPath := "/api/desktop/flows/" + flowID + "/runs"
	w = flowsCall(t, s, http.MethodGet, runsPath, reader, "")
	if w.Code != http.StatusOK {
		t.Fatalf("list runs: %d", w.Code)
	}
	runsBefore := len(flowsBody(t, w)["runs"].([]any))
	assertSimulation := func(w *httptest.ResponseRecorder, wantFrames int) {
		t.Helper()
		if w.Code != http.StatusOK {
			t.Fatalf("simulation = %d %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		traces, ok := body["child_traces"].([]any)
		docs, docsOK := body["child_documents"].([]any)
		if !ok || len(traces) == 0 || !docsOK || len(docs) == 0 {
			t.Fatalf("inline child history missing: %+v", body)
		}
		trace := traces[0].(map[string]any)
		if _, hasRun := trace["run_id"]; hasRun || len(trace["frames"].([]any)) < wantFrames || trace["parent_seq"].(float64) <= 0 {
			t.Fatalf("invalid simulation trace: %+v", trace)
		}
	}
	w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+flowID+"/simulate", admin, `{"base_revision":1,"trigger_node":"n_aaaaaaaa","trigger_data":{"value":"Ada"}}`)
	assertSimulation(w, 2)
	w = flowBlockCall(t, s, http.MethodPost, flowBlocksPath+"/"+outer+"/simulate", admin, map[string]any{"base_revision": 2, "inputs": map[string]any{"value": "Ada"}})
	assertSimulation(w, 1)
	w = flowsCall(t, s, http.MethodGet, runsPath, reader, "")
	if w.Code != http.StatusOK || len(flowsBody(t, w)["runs"].([]any)) != runsBefore {
		t.Fatalf("simulation changed stored runs: %d", w.Code)
	}
	// Publishing a later block version must not change this run's pinned document.
	blockPath := flowBlocksPath + "/" + outer
	blockResponse := flowBlockCall(t, s, http.MethodGet, blockPath, admin, nil)
	if blockResponse.Code != http.StatusOK {
		t.Fatalf("read block: %d", blockResponse.Code)
	}
	blockDoc := flowsBody(t, blockResponse)["block"].(map[string]any)["draft"].(map[string]any)
	for _, raw := range blockDoc["nodes"].([]any) {
		n := raw.(map[string]any)
		if n["type"] == "block.output" {
			n["params"].(map[string]any)["values"] = map[string]any{"result": "v2: {{nested.result}}"}
		}
	}
	if w = flowBlockCall(t, s, http.MethodPut, blockPath, admin, map[string]any{"base_revision": 2, "doc": blockDoc}); w.Code != http.StatusOK {
		t.Fatalf("save block v2: %d %s", w.Code, w.Body.String())
	}
	if w = flowBlockCall(t, s, http.MethodPost, blockPath+"/publish", admin, map[string]any{"base_revision": 3}); w.Code != http.StatusOK {
		t.Fatalf("publish block v2: %d %s", w.Code, w.Body.String())
	}
	if w = flowsCall(t, s, http.MethodGet, docPath, reader, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), "v2:") {
		t.Fatalf("pinned v1 was replaced: %d %s", w.Code, w.Body.String())
	}
	if w = flowsCall(t, s, http.MethodGet, path+"/i_1001/document", reader, ""); w.Code < 400 {
		t.Fatalf("invalid invocation accepted: %d", w.Code)
	}
	release := security.RegisterScopedSensitiveExact(canary)
	t.Cleanup(release)
	if w = flowsCall(t, s, http.MethodGet, path, reader, ""); w.Code != http.StatusOK || strings.Contains(w.Body.String(), canary) {
		t.Fatalf("late registered secret in trace (status %d)", w.Code)
	}
	// A same-number replacement must fail hash verification without hiding trace data.
	db, err := dbutil.Open(s.flowsDBPath())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	var version string
	if err := db.QueryRow(`SELECT json FROM flow_versions WHERE flow_id=? AND revision=1`, outer).Scan(&version); err != nil {
		t.Fatal(err)
	}
	var rawDoc map[string]any
	if err := json.Unmarshal([]byte(version), &rawDoc); err != nil {
		t.Fatal(err)
	}
	rawDoc["description"] = "replaced"
	tampered, err := json.Marshal(rawDoc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`UPDATE flow_versions SET json=? WHERE flow_id=? AND revision=1`, string(tampered), outer); err != nil {
		t.Fatal(err)
	}
	if w = flowsCall(t, s, http.MethodGet, docPath, reader, ""); w.Code != http.StatusOK || flowsBody(t, w)["doc_missing"] != "unavailable" {
		t.Fatalf("replaced pinned document not withheld: %d %s", w.Code, w.Body.String())
	}
	if w = flowsCall(t, s, http.MethodGet, path, reader, ""); w.Code != http.StatusOK {
		t.Fatalf("trace unavailable when its document is corrupt: %d", w.Code)
	}
	if _, err := db.Exec(`UPDATE flow_child_traces SET trace_json=? WHERE run_id=? AND parent_seq=?`, `{bad`, runID, parentSeq); err != nil {
		t.Fatal(err)
	}
	if w = flowsCall(t, s, http.MethodGet, path, reader, ""); w.Code != http.StatusServiceUnavailable || flowsBody(t, w)["code"] != "FLOW_CHILD_HISTORY_UNAVAILABLE" || strings.Contains(w.Body.String(), "{bad") {
		t.Fatalf("corrupt trace must fail closed: %d %s", w.Code, w.Body.String())
	}
}
