package server

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// The parent holds the selected immutable block version until its published
// revision is removed; a simulation executes that version without a fixture.
func TestFlowBlockParentSimulationAndDependency(t *testing.T) {
	s, token := newFlowsTestServer(t)
	created := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Echo block"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create block: %d %s", created.Code, created.Body.String())
	}
	block := flowsBody(t, created)["block"].(map[string]any)
	blockID := block["id"].(string)
	doc := block["draft"].(map[string]any)
	for _, raw := range doc["nodes"].([]any) {
		node := raw.(map[string]any)
		params := node["params"].(map[string]any)
		switch node["type"] {
		case "block.input":
			params["fields"] = []any{map[string]any{"name": "message", "type": "string", "required": true, "nullable": false}}
		case "block.output":
			params["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true, "nullable": false}}
			params["values"] = map[string]any{"result": "{{block_input.data.message}}"}
		}
	}
	path := flowBlocksPath + "/" + blockID
	if w := flowBlockCall(t, s, http.MethodPut, path, token, map[string]any{"doc": doc, "base_revision": 1}); w.Code != http.StatusOK {
		t.Fatalf("save block: %d %s", w.Code, w.Body.String())
	}
	published := flowBlockCall(t, s, http.MethodPost, path+"/publish", token, map[string]any{"base_revision": 2})
	if published.Code != http.StatusOK {
		t.Fatalf("publish block: %d %s", published.Code, published.Body.String())
	}
	blockRevision := int(flowsBody(t, published)["block"].(map[string]any)["live_revision"].(float64))
	const private = "parent_input_canary_private"
	parent := fmt.Sprintf(`{"schema":1,"kind":"flow","name":"Calls block","nodes":[
		{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},
		{"id":"n_bbbbbbbb","key":"call_block","type":"component.call","type_version":1,"params":{"flow_id":%q,"revision":%d,"inputs":{"message":"{{trigger.data.message}}"}}}
	],"edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`, blockID, blockRevision)
	newParent := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+parent+`}`)
	if newParent.Code != http.StatusCreated {
		t.Fatalf("create parent: %d %s", newParent.Code, newParent.Body.String())
	}
	parentID := flowsBody(t, newParent)["flow"].(map[string]any)["id"].(string)
	parentPath := "/api/desktop/flows/" + parentID
	if w := flowsCall(t, s, http.MethodPost, parentPath+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish parent: %d %s", w.Code, w.Body.String())
	}
	deps := flowBlockCall(t, s, http.MethodGet, path+"/dependents", token, nil)
	if deps.Code != http.StatusOK || strings.Contains(deps.Body.String(), private) {
		t.Fatalf("dependents: %d %s", deps.Code, deps.Body.String())
	}
	items := flowsBody(t, deps)["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["flow_id"] != parentID || items[0].(map[string]any)["kind"] != "published" {
		t.Fatalf("dependents = %+v", items)
	}
	blocked := flowBlockCall(t, s, http.MethodDelete, path, token, map[string]any{"base_revision": 2, "base_live_revision": blockRevision})
	if blocked.Code != http.StatusConflict || flowsBody(t, blocked)["code"] != "FLOW_BLOCK_IN_USE" {
		t.Fatalf("delete pinned block: %d %s", blocked.Code, blocked.Body.String())
	}
	sim := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", token, `{"base_revision":1,"trigger_data":{"message":"`+private+`"}}`)
	if sim.Code != http.StatusOK {
		t.Fatalf("simulate parent: %d %s", sim.Code, sim.Body.String())
	}
	result := flowsBody(t, sim)
	if result["mode"] != "simulation" || result["status"] != "success" {
		t.Fatalf("simulation = %+v", result)
	}
	matched := false
	for _, raw := range result["steps"].([]any) {
		step := raw.(map[string]any)
		if step["node_id"] == "n_bbbbbbbb" {
			out, ok := step["output"].(map[string]any)
			matched = ok && out["result"] == private
		}
	}
	if !matched {
		t.Fatalf("block result missing: %+v", result)
	}
	if w := flowsCall(t, s, http.MethodDelete, parentPath, token, ""); w.Code != http.StatusOK {
		t.Fatalf("delete parent: %d %s", w.Code, w.Body.String())
	}
	if w := flowBlockCall(t, s, http.MethodDelete, path, token, map[string]any{"base_revision": 2, "base_live_revision": blockRevision}); w.Code != http.StatusOK {
		t.Fatalf("delete unpinned block: %d %s", w.Code, w.Body.String())
	}
}
