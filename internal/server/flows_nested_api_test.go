package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/flows"
)

// TestFlowNestedBlocksHTTP checks immutable transitive composition through real handlers and Runner.
func TestFlowNestedBlocksHTTP(t *testing.T) {
	s, token := newFlowsTestServer(t)
	missionsBefore := s.MissionManagerV2.List()
	type block struct {
		id, path string
		doc      map[string]any
	}
	createBlock := func(name, child string) block {
		t.Helper()
		w := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": name})
		if w.Code != http.StatusCreated {
			t.Fatalf("create %s: %d %s", name, w.Code, w.Body.String())
		}
		rec := flowsBody(t, w)["block"].(map[string]any)
		b := block{id: rec["id"].(string), doc: rec["draft"].(map[string]any)}
		b.path = flowBlocksPath + "/" + b.id
		nodes := b.doc["nodes"].([]any)
		var inputID, outputID string
		for _, raw := range nodes {
			n := raw.(map[string]any)
			p := n["params"].(map[string]any)
			switch n["type"] {
			case "block.input":
				inputID = n["id"].(string)
				p["fields"] = []any{map[string]any{"name": "value", "type": "string", "required": true}}
			case "block.output":
				outputID = n["id"].(string)
				p["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true}}
				value := "v1: {{block_input.data.value}}"
				if child != "" {
					value = "{{nested.result}}"
				}
				p["values"] = map[string]any{"result": value}
			}
		}
		if child != "" {
			// Deliberately identical call IDs in A and B: identity belongs to the frame.
			nodes = append(nodes, map[string]any{"id": "n_nestedaa", "key": "nested", "type": "component.call", "type_version": 1, "params": map[string]any{"flow_id": child, "revision": 1, "inputs": map[string]any{"value": "{{block_input.data.value}}"}}})
			b.doc["nodes"] = nodes
			b.doc["edges"] = []any{
				map[string]any{"id": "e_nestedaa", "source": map[string]any{"node": inputID, "port": "out"}, "target": map[string]any{"node": "n_nestedaa", "port": "in"}},
				map[string]any{"id": "e_nestedab", "source": map[string]any{"node": "n_nestedaa", "port": "out"}, "target": map[string]any{"node": outputID, "port": "in"}},
			}
		}
		w = flowBlockCall(t, s, http.MethodPut, b.path, token, map[string]any{"doc": b.doc, "base_revision": 1})
		if w.Code != http.StatusOK {
			t.Fatalf("save %s: %d %s", name, w.Code, w.Body.String())
		}
		preview := flowBlockCall(t, s, http.MethodGet, b.path+"/publish-preview", token, nil)
		if preview.Code != http.StatusOK || flowsBody(t, preview)["draft_revision"] != float64(2) {
			t.Fatalf("preview %s: %d %s", name, preview.Code, preview.Body.String())
		}
		w = flowBlockCall(t, s, http.MethodPost, b.path+"/publish", token, map[string]any{"base_revision": 2})
		if w.Code != http.StatusOK {
			t.Fatalf("publish %s: %d %s", name, w.Code, w.Body.String())
		}
		return b
	}
	c := createBlock("C", "")
	b := createBlock("B", c.id)
	a := createBlock("A", b.id)
	if !reflect.DeepEqual(missionsBefore, s.MissionManagerV2.List()) {
		t.Fatal("block creation or publication changed missions")
	}
	sim := flowBlockCall(t, s, http.MethodPost, a.path+"/simulate", token, map[string]any{"base_revision": 2, "inputs": map[string]any{"value": "Ada"}, "at": "2026-01-02T03:04:05Z"})
	if sim.Code != http.StatusOK {
		t.Fatalf("nested block simulation: %d %s", sim.Code, sim.Body.String())
	}
	result := flowsBody(t, sim)
	if result["status"] != "success" || result["mode"] != "simulation" || result["outputs"].(map[string]any)["result"] != "v1: Ada" {
		t.Fatalf("nested simulation: %+v", result)
	}
	for _, child := range []block{a, b, c} {
		runs, err := s.Flows.Store().ListRuns(context.Background(), child.id, flows.RunFilter{Limit: 10})
		if err != nil || len(runs) != 0 {
			t.Fatalf("nested simulation persisted child runs: %+v %v", runs, err)
		}
	}
	if !reflect.DeepEqual(missionsBefore, s.MissionManagerV2.List()) {
		t.Fatal("nested simulation changed missions")
	}
	if stale := flowBlockCall(t, s, http.MethodPost, a.path+"/simulate", token, map[string]any{"base_revision": 1, "inputs": map[string]any{"value": "Ada"}}); stale.Code != http.StatusConflict {
		t.Fatalf("stale simulation: %d %s", stale.Code, stale.Body.String())
	}
	reader, _, err := s.TokenManager.Create("nested read", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if denied := flowBlockCall(t, s, http.MethodPost, a.path+"/simulate", reader, map[string]any{"base_revision": 2, "inputs": map[string]any{"value": "Ada"}}); denied.Code != http.StatusForbidden {
		t.Fatalf("reader simulated: %d %s", denied.Code, denied.Body.String())
	}

	parentDoc := fmt.Sprintf(`{"schema":1,"kind":"flow","name":"Nested parent","nodes":[{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},{"id":"n_bbbbbbbb","key":"call","type":"component.call","type_version":1,"params":{"flow_id":%q,"revision":1,"inputs":{"value":"{{trigger.data.value}}"}}}],"edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`, a.id)
	parents := make([]string, 0, 2)
	parentDocs := make([]map[string]any, 0, 2)
	for range 2 {
		w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+parentDoc+`}`)
		if w.Code != http.StatusCreated {
			t.Fatalf("create parent: %d %s", w.Code, w.Body.String())
		}
		rec := flowsBody(t, w)["flow"].(map[string]any)
		id := rec["id"].(string)
		parents = append(parents, id)
		parentDocs = append(parentDocs, rec["draft"].(map[string]any))
		w = flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/publish", token, `{"base_revision":1}`)
		if w.Code != http.StatusOK {
			t.Fatalf("publish parent: %d %s", w.Code, w.Body.String())
		}
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < 20; page++ {
		w := flowBlockCall(t, s, http.MethodGet, c.path+"/dependents?limit=1&cursor="+url.QueryEscape(cursor), token, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("dependents: %d %s", w.Code, w.Body.String())
		}
		body := flowsBody(t, w)
		for _, raw := range body["items"].([]any) {
			item := raw.(map[string]any)
			if item["kind"] == "published" {
				seen[item["flow_id"].(string)] = true
			}
			if strings.Contains(fmt.Sprint(item), "Ada") {
				t.Fatalf("dependency leaked payload: %+v", item)
			}
		}
		cursor, _ = body["next_cursor"].(string)
		if cursor == "" {
			break
		}
		if page == 19 {
			t.Fatal("dependency pagination did not finish")
		}
	}
	for _, id := range []string{a.id, b.id, parents[0], parents[1]} {
		if !seen[id] {
			t.Errorf("transitive dependent %s missing: %+v", id, seen)
		}
	}
	if w := flowBlockCall(t, s, http.MethodDelete, c.path, token, map[string]any{"base_revision": 2, "base_live_revision": 1}); w.Code != http.StatusConflict {
		t.Fatalf("delete pinned grandchild: %d %s", w.Code, w.Body.String())
	}

	// Publish C v2; all existing callers continue using the original transitive pin.
	for _, raw := range c.doc["nodes"].([]any) {
		n := raw.(map[string]any)
		if n["type"] == "block.output" {
			n["params"].(map[string]any)["values"] = map[string]any{"result": "v2: {{block_input.data.value}}"}
		}
	}
	if w := flowBlockCall(t, s, http.MethodPut, c.path, token, map[string]any{"doc": c.doc, "base_revision": 2}); w.Code != http.StatusOK {
		t.Fatalf("save C v2: %d %s", w.Code, w.Body.String())
	}
	if w := flowBlockCall(t, s, http.MethodPost, c.path+"/publish", token, map[string]any{"base_revision": 3}); w.Code != http.StatusOK {
		t.Fatalf("publish C v2: %d %s", w.Code, w.Body.String())
	}
	assertRun := func(id string, draft int, expected string) {
		t.Helper()
		w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/test", token, fmt.Sprintf(`{"base_revision":%d,"trigger_data":{"value":"Ada"}}`, draft))
		if w.Code != http.StatusAccepted {
			t.Fatalf("start parent: %d %s", w.Code, w.Body.String())
		}
		run := waitFlowRunStatus(t, s, token, flowsBody(t, w)["run_id"].(string), "success")
		found := false
		for _, raw := range run["steps"].([]any) {
			step := raw.(map[string]any)
			if step["node_id"] == "n_bbbbbbbb" {
				out, ok := step["output"].(map[string]any)
				found = ok && out["result"] == expected
			}
		}
		if !found {
			t.Fatalf("parent result wanted %q: %+v", expected, run)
		}
	}
	for _, id := range parents {
		assertRun(id, 1, "v1: Ada")
	}
	// Upgrades are explicit at each edge; parent two stays on A v1.
	for _, item := range []block{b, a} {
		for _, raw := range item.doc["nodes"].([]any) {
			n := raw.(map[string]any)
			if n["type"] == "component.call" {
				n["params"].(map[string]any)["revision"] = 2
			}
		}
		if w := flowBlockCall(t, s, http.MethodPut, item.path, token, map[string]any{"doc": item.doc, "base_revision": 2}); w.Code != http.StatusOK {
			t.Fatalf("upgrade nested draft: %d %s", w.Code, w.Body.String())
		}
		if w := flowBlockCall(t, s, http.MethodPost, item.path+"/publish", token, map[string]any{"base_revision": 3}); w.Code != http.StatusOK {
			t.Fatalf("publish nested upgrade: %d %s", w.Code, w.Body.String())
		}
	}
	assertRun(parents[0], 1, "v1: Ada")
	for _, raw := range parentDocs[0]["nodes"].([]any) {
		n := raw.(map[string]any)
		if n["type"] == "component.call" {
			n["params"].(map[string]any)["revision"] = 2
		}
	}
	parentPath := "/api/desktop/flows/" + parents[0]
	if w := c9FlowPut(t, s, parentPath, token, parentDocs[0]); w.Code != http.StatusOK {
		t.Fatalf("upgrade parent: %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPost, parentPath+"/publish", token, `{"base_revision":2}`); w.Code != http.StatusOK {
		t.Fatalf("publish parent upgrade: %d %s", w.Code, w.Body.String())
	}
	assertRun(parents[0], 2, "v2: Ada")
	assertRun(parents[1], 1, "v1: Ada")
	// A missing nested revision remains an editable draft, but cannot be simulated or published.
	for _, raw := range a.doc["nodes"].([]any) {
		n := raw.(map[string]any)
		if n["type"] == "component.call" {
			n["params"].(map[string]any)["flow_id"] = "flow_missing"
		}
	}
	if w := flowBlockCall(t, s, http.MethodPut, a.path, token, map[string]any{"doc": a.doc, "base_revision": 3}); w.Code != http.StatusOK {
		t.Fatalf("save unresolved draft: %d %s", w.Code, w.Body.String())
	}
	badPreview := flowBlockCall(t, s, http.MethodGet, a.path+"/publish-preview", token, nil)
	if badPreview.Code != http.StatusOK || flowsBody(t, badPreview)["can_publish"] != false {
		t.Fatalf("unresolved preview: %d %s", badPreview.Code, badPreview.Body.String())
	}
	actionable := false
	for _, raw := range flowsBody(t, badPreview)["issues"].([]any) {
		issue := raw.(map[string]any)
		if issue["code"] == "PARAM_INVALID" && issue["node_id"] == "n_nestedaa" && issue["param"] == "flow_id" {
			actionable = true
		}
	}
	if !actionable {
		t.Fatalf("unresolved preview missing actionable call issue: %s", badPreview.Body.String())
	}
	badSim := flowBlockCall(t, s, http.MethodPost, a.path+"/simulate", token, map[string]any{"base_revision": 4, "inputs": map[string]any{"value": "Ada"}})
	if badSim.Code < 400 {
		t.Fatalf("unresolved simulation executed: %d %s", badSim.Code, badSim.Body.String())
	}
	badPublish := flowBlockCall(t, s, http.MethodPost, a.path+"/publish", token, map[string]any{"base_revision": 4})
	if badPublish.Code < 400 {
		t.Fatalf("unresolved draft published: %d %s", badPublish.Code, badPublish.Body.String())
	}
	assertRun(parents[1], 1, "v1: Ada")
}

func c9FlowPut(t *testing.T, s *Server, path, token string, doc map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"doc": doc, "base_revision": 1})
	if err != nil {
		t.Fatal(err)
	}
	return flowsCall(t, s, http.MethodPut, path, token, string(raw))
}
