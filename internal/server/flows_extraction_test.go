package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/flows"
)

const extractionHTTPFlow = `{"schema":1,"kind":"flow","name":"Extraction example","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},
 {"id":"n_bbbbbbbb","key":"first","type":"logic.set","type_version":1,"params":{"keep_input":false,"fields":[{"name":"text","type":"text","value":"{{trigger.data.name}}"}]}},
 {"id":"n_cccccccc","key":"second","type":"logic.set","type_version":1,"params":{"keep_input":false,"fields":[{"name":"text","type":"text","value":"{{first.text}}"}]}},
 {"id":"n_dddddddd","key":"after","type":"logic.set","type_version":1,"params":{"keep_input":false,"fields":[{"name":"text","type":"text","value":"{{second.text}}"}]}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}},{"id":"e_bbbbbbbb","source":{"node":"n_bbbbbbbb","port":"out"},"target":{"node":"n_cccccccc","port":"in"}},{"id":"e_cccccccc","source":{"node":"n_cccccccc","port":"out"},"target":{"node":"n_dddddddd","port":"in"}}]}`

func c10FlowCall(t *testing.T, s *Server, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var raw []byte
	if body != nil {
		var err error
		raw, err = json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
	}
	return flowsCall(t, s, method, path, token, string(raw))
}

func TestFlowExtractionHTTP(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm, events := c17Audit(t, s), c17Events(t, s)
	created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+extractionHTTPFlow+`}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	rec := flowsBody(t, created)["flow"].(map[string]any)
	id := rec["id"].(string)
	path := "/api/desktop/flows/" + id
	request := map[string]any{"base_revision": 1, "node_ids": []string{"n_bbbbbbbb", "n_cccccccc"}, "name": "Extracted logic"}
	missions := s.MissionManagerV2.List()
	simulate := func(revision int) any {
		t.Helper()
		w := flowsCall(t, s, http.MethodPost, path+"/simulate", token, fmt.Sprintf(`{"base_revision":%d,"trigger_data":{"name":"Ada"}}`, revision))
		if w.Code != http.StatusOK || flowsBody(t, w)["status"] != "success" {
			t.Fatalf("simulate: %d %s", w.Code, w.Body.String())
		}
		for _, raw := range flowsBody(t, w)["steps"].([]any) {
			step := raw.(map[string]any)
			if step["node_id"] == "n_dddddddd" {
				return step["output"]
			}
		}
		t.Fatalf("successor missing: %s", w.Body.String())
		return nil
	}
	before := simulate(1)
	preview := c10FlowCall(t, s, http.MethodPost, path+"/extract/preview", token, request)
	if preview.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", preview.Code, preview.Body.String())
	}
	p := flowsBody(t, preview)
	ids := p["ids"].(map[string]any)
	blockID := ids["block_id"].(string)
	if p["base_revision"] != float64(1) || p["block_revision"] != float64(1) || p["digest"] == "" || p["parent_hash"] == "" || p["block_hash"] == "" {
		t.Fatalf("preview DTO: %+v", p)
	}
	review := p["review"].(map[string]any)
	if review["budget_note"] != true || strings.Contains(preview.Body.String(), "Ada") {
		t.Fatalf("unsafe review: %s", preview.Body.String())
	}
	if blocks := flowBlockCall(t, s, http.MethodGet, flowBlocksPath, token, nil); blocks.Code != http.StatusOK || len(flowsBody(t, blocks)["blocks"].([]any)) != 0 {
		t.Fatalf("preview created block: %d %s", blocks.Code, blocks.Body.String())
	}
	if !reflect.DeepEqual(missions, s.MissionManagerV2.List()) {
		t.Fatal("preview changed missions")
	}
	reconcile := map[string]any{"base_revision": 1, "block_id": blockID, "parent_hash": p["parent_hash"], "block_hash": p["block_hash"]}
	unknown := c10FlowCall(t, s, http.MethodPost, path+"/extract/reconcile", token, reconcile)
	if unknown.Code != http.StatusOK || flowsBody(t, unknown)["status"] != "unknown_or_conflict" || flowsBody(t, unknown)["flow"] != nil || flowsBody(t, unknown)["block"] != nil {
		t.Fatalf("pre-apply reconcile: %d %s", unknown.Code, unknown.Body.String())
	}
	apply := map[string]any{"base_revision": 1, "node_ids": request["node_ids"], "name": request["name"], "ids": ids, "digest": p["digest"]}
	invalid := make(map[string]any, len(apply))
	for k, v := range apply {
		invalid[k] = v
	}
	invalid["digest"] = strings.Repeat("0", 64)
	denied := c10FlowCall(t, s, http.MethodPost, path+"/extract/apply", token, invalid)
	if denied.Code != http.StatusConflict {
		t.Fatalf("digest tampering: %d %s", denied.Code, denied.Body.String())
	}
	_ = c17Drain(events)
	applied := c10FlowCall(t, s, http.MethodPost, path+"/extract/apply", token, apply)
	if applied.Code != http.StatusOK {
		t.Fatalf("apply: %d %s", applied.Code, applied.Body.String())
	}
	result := flowsBody(t, applied)
	flow := result["flow"].(map[string]any)
	block := result["block"].(map[string]any)
	if flow["id"] != id || flow["draft_revision"] != float64(2) || flow["mission_id"] != rec["mission_id"] || block["id"] != blockID || block["live_revision"] != float64(1) || block["mission_id"] != "" {
		t.Fatalf("apply result: %+v", result)
	}
	if !reflect.DeepEqual(missions, s.MissionManagerV2.List()) {
		t.Fatal("apply changed missions")
	}
	for _, action := range []string{"flow_block_create", "flow_block_publish"} {
		rows := c17AuditEvents(t, stm, action)
		if len(rows) != 1 || rows[0].TargetID != blockID {
			t.Fatalf("%s audit = %+v", action, rows)
		}
		encoded, err := json.Marshal(rows[0])
		if err != nil {
			t.Fatal(err)
		}
		for _, private := range []string{"Ada", "Extracted logic", "n_bbbbbbbb"} {
			if strings.Contains(string(encoded), private) {
				t.Fatalf("%s audit leaked data", action)
			}
		}
	}
	broadcasts := c17Drain(events)
	if len(broadcasts) != 3 {
		t.Fatalf("apply broadcasts = %+v", broadcasts)
	}
	for i, expected := range []struct{ id, reason string }{{blockID, "created"}, {blockID, "published"}, {id, "saved"}} {
		payload, ok := broadcasts[i].Payload.(map[string]interface{})
		if broadcasts[i].Type != "flows_changed" || !ok || len(payload) != 2 || payload["flow_id"] != expected.id || payload["reason"] != expected.reason {
			t.Fatalf("broadcast %d = %+v", i, broadcasts[i])
		}
	}
	if after := simulate(2); !reflect.DeepEqual(before, after) {
		t.Fatalf("successor changed: before=%+v after=%+v", before, after)
	}
	repeated := c10FlowCall(t, s, http.MethodPost, path+"/extract/apply", token, apply)
	if repeated.Code != http.StatusConflict {
		t.Fatalf("repeat did not conflict: %d %s", repeated.Code, repeated.Body.String())
	}
	confirmed := c10FlowCall(t, s, http.MethodPost, path+"/extract/reconcile", token, reconcile)
	if confirmed.Code != http.StatusOK || flowsBody(t, confirmed)["status"] != "applied_state_confirmed" || !reflect.DeepEqual(flowsBody(t, confirmed)["flow"], flow) || !reflect.DeepEqual(flowsBody(t, confirmed)["block"], block) {
		t.Fatalf("reconcile: %d %s", confirmed.Code, confirmed.Body.String())
	}
	changed := flow["draft"].(map[string]any)
	changed["description"] = "later edit"
	if w := c10FlowCall(t, s, http.MethodPut, path, token, map[string]any{"doc": changed, "base_revision": 2}); w.Code != http.StatusOK {
		t.Fatalf("later edit: %d %s", w.Code, w.Body.String())
	}
	later := c10FlowCall(t, s, http.MethodPost, path+"/extract/reconcile", token, reconcile)
	if later.Code != http.StatusOK || flowsBody(t, later)["status"] != "unknown_or_conflict" || flowsBody(t, later)["flow"] != nil || flowsBody(t, later)["block"] != nil {
		t.Fatalf("changed reconcile: %d %s", later.Code, later.Body.String())
	}
}

func TestFlowExtractionHTTPGuards(t *testing.T) {
	s, token := newFlowsTestServer(t)
	created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+extractionHTTPFlow+`}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	path := "/api/desktop/flows/" + flowsBody(t, created)["flow"].(map[string]any)["id"].(string) + "/extract/preview"
	valid := `{"base_revision":1,"node_ids":["n_bbbbbbbb"],"name":"Extract"}`
	unsafe := c10FlowCall(t, s, http.MethodPost, path, token, map[string]any{"base_revision": 1, "node_ids": []string{"n_bbbbbbbb", "n_dddddddd"}, "name": "Extract"})
	if unsafe.Code != http.StatusUnprocessableEntity || flowsBody(t, unsafe)["code"] != "FLOW_EXTRACTION_UNSUPPORTED" || flowsBody(t, unsafe)["reason"] == nil || strings.Contains(unsafe.Body.String(), "n_bbbbbbbb") {
		t.Fatalf("unsupported selection: %d %s", unsafe.Code, unsafe.Body.String())
	}
	block := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "A block"})
	if block.Code != http.StatusCreated {
		t.Fatalf("create block: %d %s", block.Code, block.Body.String())
	}
	blockID := flowsBody(t, block)["block"].(map[string]any)["id"].(string)
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+blockID+"/extract/preview", token, valid); w.Code != http.StatusNotFound {
		t.Fatalf("block used as flow: %d %s", w.Code, w.Body.String())
	}
	reader, _, err := s.TokenManager.Create("extraction read", []string{desktopScopeRead}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w := flowsCall(t, s, http.MethodPost, path, reader, valid); w.Code != http.StatusForbidden {
		t.Fatalf("reader: %d %s", w.Code, w.Body.String())
	}
	for _, raw := range []string{valid + ` {}`, `{"base_revision":1,"node_ids":["n_bbbbbbbb"],"name":"\ud800"}`, `{"base_revision":1,"node_ids":[],"name":"x","extra":true}`} {
		if w := flowsCall(t, s, http.MethodPost, path, token, raw); w.Code != http.StatusBadRequest {
			t.Errorf("bad body accepted: %d %s", w.Code, w.Body.String())
		}
	}
	for _, name := range []string{"bad\x00name", "bad\nname", "bad\x7fname", "bad\u0085name"} {
		w := c10FlowCall(t, s, http.MethodPost, path, token, map[string]any{"base_revision": 1, "node_ids": []string{"n_bbbbbbbb"}, "name": name})
		if w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_EXTRACTION_INVALID" {
			t.Errorf("control in extraction name accepted: %d %s", w.Code, w.Body.String())
		}
	}
	oversized := `{"name":"` + strings.Repeat("a", int(flowsSmallBodyLimit)) + `"}`
	if w := flowsCall(t, s, http.MethodPost, path, token, oversized); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("large body: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(valid))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	r.Header.Set("Origin", "https://evil.example")
	w := httptest.NewRecorder()
	s.handleFlows(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross origin: %d %s", w.Code, w.Body.String())
	}
	s.Cfg.Tools.Missions.ReadOnly = true
	if w := flowsCall(t, s, http.MethodPost, path, token, valid); w.Code != http.StatusForbidden {
		t.Fatalf("readonly: %d %s", w.Code, w.Body.String())
	}
}

func TestFlowExtractionErrorHidesUnknownReason(t *testing.T) {
	s, _ := newFlowsTestServer(t)
	w := httptest.NewRecorder()
	s.flowExtractionError(w, httptest.NewRequest(http.MethodPost, "/api/desktop/flows/x/extract/preview", nil), &flows.ExtractionError{Reason: "private-user-value"})
	if w.Code != http.StatusUnprocessableEntity || strings.Contains(w.Body.String(), "private-user-value") || flowsBody(t, w)["reason"] != nil {
		t.Fatalf("unsafe reason exposed: %d %s", w.Code, w.Body.String())
	}
}

func TestFlowExtractionPreviewScrubsReviewOnly(t *testing.T) {
	s, token := newFlowsTestServer(t)
	const secret = "private_field_abcdef"
	doc := strings.Replace(extractionHTTPFlow, `{"name":"text","type":"text","value":"{{first.text}}"}`, `{"name":"`+secret+`","type":"text","value":"{{first.text}}"}`, 1)
	doc = strings.ReplaceAll(doc, "{{second.text}}", "{{second."+secret+"}}")
	created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+doc+`}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create: %d %s", created.Code, created.Body.String())
	}
	id := flowsBody(t, created)["flow"].(map[string]any)["id"].(string)
	c18Secret(t, secret)
	w := c10FlowCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/extract/preview", token, map[string]any{"base_revision": 1, "node_ids": []string{"n_bbbbbbbb", "n_cccccccc"}, "name": "Safe"})
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), secret) {
		t.Fatalf("review leaked: %d %s", w.Code, w.Body.String())
	}
	body := flowsBody(t, w)
	if !extractionHashPattern.MatchString(body["digest"].(string)) || !extractionBlockPattern.MatchString(body["ids"].(map[string]any)["block_id"].(string)) {
		t.Fatalf("proposal identity was altered: %+v", body)
	}
}
