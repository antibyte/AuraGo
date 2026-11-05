package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"aurago/internal/flows"
)

// TestFlowBatchHTTP exercises the real handlers, block store and Runner with a pinned pure block.
func TestFlowBatchHTTP(t *testing.T) {
	s, token := newFlowsTestServer(t)
	created := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Batch echo"})
	if created.Code != http.StatusCreated {
		t.Fatalf("create block: %d %s", created.Code, created.Body.String())
	}
	block := flowsBody(t, created)["block"].(map[string]any)
	blockID := block["id"].(string)
	blockPath := flowBlocksPath + "/" + blockID
	doc := block["draft"].(map[string]any)
	for _, raw := range doc["nodes"].([]any) {
		node := raw.(map[string]any)
		params := node["params"].(map[string]any)
		switch node["type"] {
		case "block.input":
			params["fields"] = []any{map[string]any{"name": "value", "type": "string", "required": true, "nullable": false}}
		case "block.output":
			params["fields"] = []any{map[string]any{"name": "result", "type": "string", "required": true, "nullable": false}}
			params["values"] = map[string]any{"result": "v1: {{block_input.data.value}}"}
		}
	}
	if w := flowBlockCall(t, s, http.MethodPut, blockPath, token, map[string]any{"doc": doc, "base_revision": 1}); w.Code != http.StatusOK {
		t.Fatalf("save block: %d %s", w.Code, w.Body.String())
	}
	if w := flowBlockCall(t, s, http.MethodPost, blockPath+"/publish", token, map[string]any{"base_revision": 2}); w.Code != http.StatusOK {
		t.Fatalf("publish block: %d %s", w.Code, w.Body.String())
	}
	parent := fmt.Sprintf(`{"schema":1,"kind":"flow","name":"Batch parent","nodes":[
		{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},
		{"id":"n_bbbbbbbb","key":"batch","type":"component.each","type_version":1,"params":{"flow_id":%q,"revision":1,"items":"{{trigger.data.items}}","item_field":"value","inputs":{},"max_items":4,"concurrency":1,"on_item_error":"collect"}}
	],"edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`, blockID)
	parentRes := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+parent+`}`)
	if parentRes.Code != http.StatusCreated {
		t.Fatalf("create parent: %d %s", parentRes.Code, parentRes.Body.String())
	}
	parentID := flowsBody(t, parentRes)["flow"].(map[string]any)["id"].(string)
	parentPath := "/api/desktop/flows/" + parentID
	if w := flowsCall(t, s, http.MethodPost, parentPath+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("publish parent: %d %s", w.Code, w.Body.String())
	}
	deps := flowBlockCall(t, s, http.MethodGet, blockPath+"/dependents", token, nil)
	if deps.Code != http.StatusOK || !strings.Contains(deps.Body.String(), parentID) {
		t.Fatalf("batch dependency missing: %d %s", deps.Code, deps.Body.String())
	}
	if w := flowBlockCall(t, s, http.MethodDelete, blockPath, token, map[string]any{"base_revision": 2, "base_live_revision": 1}); w.Code != http.StatusConflict {
		t.Fatalf("delete pinned block: %d %s", w.Code, w.Body.String())
	}
	// A newer block cannot change the parent's selected immutable revision.
	for _, raw := range doc["nodes"].([]any) {
		node := raw.(map[string]any)
		if node["type"] == "block.output" {
			node["params"].(map[string]any)["values"] = map[string]any{"result": "v2: {{block_input.data.value}}"}
		}
	}
	if w := flowBlockCall(t, s, http.MethodPut, blockPath, token, map[string]any{"doc": doc, "base_revision": 2}); w.Code != http.StatusOK {
		t.Fatalf("save block v2: %d %s", w.Code, w.Body.String())
	}
	if w := flowBlockCall(t, s, http.MethodPost, blockPath+"/publish", token, map[string]any{"base_revision": 3}); w.Code != http.StatusOK {
		t.Fatalf("publish block v2: %d %s", w.Code, w.Body.String())
	}
	const input = `{"base_revision":1,"trigger_data":{"items":["alpha","{{run.started_at}}",42]}}`
	checkOutput := func(body map[string]any) {
		t.Helper()
		for _, raw := range body["steps"].([]any) {
			step := raw.(map[string]any)
			if step["node_id"] != "n_bbbbbbbb" {
				continue
			}
			output := step["output"].(map[string]any)
			items := output["items"].([]any)
			if output["count"] != float64(3) || output["succeeded"] != float64(2) || output["failed"] != float64(1) || len(items) != 3 {
				t.Fatalf("batch summary: %+v", output)
			}
			for i, expected := range []string{"v1: alpha", "v1: {{run.started_at}}"} {
				item := items[i].(map[string]any)
				if item["index"] != float64(i) || item["status"] != "success" || item["output"].(map[string]any)["result"] != expected {
					t.Fatalf("batch item %d: %+v", i, item)
				}
			}
			failed := items[2].(map[string]any)
			code, ok := failed["error_code"].(string)
			if failed["index"] != float64(2) || failed["status"] != "error" || !ok || code == "" || failed["output"] != nil {
				t.Fatalf("failed batch item: %+v", failed)
			}
			return
		}
		t.Fatalf("batch step missing: %+v", body)
	}
	sim := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", token, input)
	if sim.Code != http.StatusOK || flowsBody(t, sim)["status"] != "success" {
		t.Fatalf("simulation: %d %s", sim.Code, sim.Body.String())
	}
	checkOutput(flowsBody(t, sim))
	empty := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", token, `{"base_revision":1,"trigger_data":{"items":[]}}`)
	if empty.Code != http.StatusOK || flowsBody(t, empty)["status"] != "success" {
		t.Fatalf("empty batch: %d %s", empty.Code, empty.Body.String())
	}
	emptyStep := false
	for _, raw := range flowsBody(t, empty)["steps"].([]any) {
		step := raw.(map[string]any)
		if step["node_id"] == "n_bbbbbbbb" {
			emptyStep = true
			output := step["output"].(map[string]any)
			if output["count"] != float64(0) || len(output["items"].([]any)) != 0 {
				t.Fatalf("empty batch output: %+v", output)
			}
		}
	}
	if !emptyStep {
		t.Fatalf("empty batch step missing: %s", empty.Body.String())
	}
	refused := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", token, `{"base_revision":1,"fixtures":{"n_bbbbbbbb":{"output":{}}}}`)
	if refused.Code == http.StatusOK {
		t.Fatalf("batch fixture unexpectedly accepted: %s", refused.Body.String())
	}
	readToken, _, err := s.TokenManager.Create("batch reader", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", readToken, input); w.Code != http.StatusForbidden {
		t.Fatalf("read-only token simulated: %d %s", w.Code, w.Body.String())
	}
	started := flowsCall(t, s, http.MethodPost, parentPath+"/test", token, input)
	if started.Code != http.StatusAccepted {
		t.Fatalf("test run: %d %s", started.Code, started.Body.String())
	}
	runID := flowsBody(t, started)["run_id"].(string)
	checkOutput(waitFlowRunStatus(t, s, token, runID, "success"))
	tooMany := flowsCall(t, s, http.MethodPost, parentPath+"/simulate", token, `{"base_revision":1,"trigger_data":{"items":["a","b","c","d","e"]}}`)
	if tooMany.Code != http.StatusOK || flowsBody(t, tooMany)["error_code"] != "FLOW_PARAM_INVALID" {
		t.Fatalf("batch over limit: %d %s", tooMany.Code, tooMany.Body.String())
	}
	stopParent := strings.Replace(parent, `"on_item_error":"collect"`, `"on_item_error":"stop"`, 1)
	stopRes := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+stopParent+`}`)
	if stopRes.Code != http.StatusCreated {
		t.Fatalf("create stop parent: %d %s", stopRes.Code, stopRes.Body.String())
	}
	stopID := flowsBody(t, stopRes)["flow"].(map[string]any)["id"].(string)
	stop := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+stopID+"/simulate", token, input)
	if stop.Code != http.StatusOK || flowsBody(t, stop)["error_code"] != "FLOW_BATCH_ITEM_FAILED" {
		t.Fatalf("stop failure: %d %s", stop.Code, stop.Body.String())
	}
	foundStop := false
	for _, raw := range flowsBody(t, stop)["steps"].([]any) {
		step := raw.(map[string]any)
		if step["node_id"] == "n_bbbbbbbb" {
			foundStop = true
			message, _ := step["error_message"].(string)
			if !strings.HasPrefix(message, "batch item 2 failed (") || strings.Contains(message, "alpha") || step["output"] != nil {
				t.Fatalf("stop leaked item details or returned partial output: %+v", step)
			}
		}
	}
	if !foundStop {
		t.Fatalf("stop step missing: %s", stop.Body.String())
	}
	// Direct publication gives the same actionable budget issue as preview.
	nodes := []string{`{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}}`}
	edges := make([]string, 0, 6)
	for i := range 6 {
		nodeID := fmt.Sprintf("n_aaaaaaa%c", 'b'+i)
		nodes = append(nodes, fmt.Sprintf(`{"id":%q,"key":"batch_%d","type":"component.each","type_version":1,"params":{"flow_id":%q,"revision":1,"items":[],"item_field":"value","inputs":{},"max_items":1000,"concurrency":1,"on_item_error":"collect"}}`, nodeID, i, blockID))
		edges = append(edges, fmt.Sprintf(`{"id":"e_aaaaaaa%c","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":%q,"port":"in"}}`, 'a'+i, nodeID))
	}
	budgetDoc := `{"schema":1,"kind":"flow","name":"Too much work","nodes":[` + strings.Join(nodes, ",") + `],"edges":[` + strings.Join(edges, ",") + `]}`
	budgetCreated := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+budgetDoc+`}`)
	if budgetCreated.Code != http.StatusCreated {
		t.Fatalf("create budget draft: %d %s", budgetCreated.Code, budgetCreated.Body.String())
	}
	budgetID := flowsBody(t, budgetCreated)["flow"].(map[string]any)["id"].(string)
	budgetPath := "/api/desktop/flows/" + budgetID
	budgetPreview := flowsCall(t, s, http.MethodGet, budgetPath+"/publish-preview", token, "")
	if budgetPreview.Code != http.StatusOK {
		t.Fatalf("over-budget preview: %d %s", budgetPreview.Code, budgetPreview.Body.String())
	}
	var previewIssue map[string]any
	for _, raw := range flowsBody(t, budgetPreview)["issues"].([]any) {
		issue := raw.(map[string]any)
		if issue["code"] == "BATCH_WORK_LIMIT" {
			previewIssue = issue
		}
	}
	budgetPublish := flowsCall(t, s, http.MethodPost, budgetPath+"/publish", token, `{"base_revision":1}`)
	if budgetPublish.Code != http.StatusUnprocessableEntity || flowsBody(t, budgetPublish)["code"] != "FLOW_INVALID" {
		t.Fatalf("direct over-budget publish: %d %s", budgetPublish.Code, budgetPublish.Body.String())
	}
	foundBudget := false
	for _, raw := range flowsBody(t, budgetPublish)["issues"].([]any) {
		issue := raw.(map[string]any)
		if issue["code"] == "BATCH_WORK_LIMIT" && issue["param"] == "max_items" && issue["node_id"] != "" && reflect.DeepEqual(issue, previewIssue) {
			foundBudget = true
		}
	}
	if !foundBudget {
		t.Fatalf("missing actionable batch budget issue: %s", budgetPublish.Body.String())
	}
}

func TestFlowBatchAIProposalRemainsExcluded(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	stepper := proposalStepper(func(_ context.Context, request flows.LLMRequest) (flows.LLMResponse, error) {
		if strings.Contains(request.System, `"type":"component.each"`) {
			t.Fatal("goal-only catalog exposed batch references")
		}
		return flows.LLMResponse{Text: strings.ReplaceAll(greetFlowJSON, "logic.set", "component.each")}, nil
	})
	_, err := s.makeNewFlowProposal(context.Background(), "create a batch", stepper)
	var nodeErr *flows.NodeError
	if !errors.As(err, &nodeErr) || nodeErr.Code != "FLOW_AI_OUTPUT_INVALID" {
		t.Fatalf("batch proposal error: %v", err)
	}
	// This is a response-only proposal: no flow is created as a side effect.
	list, err := s.Flows.ListFlows(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("proposal created %d flows", len(list))
	}
}
