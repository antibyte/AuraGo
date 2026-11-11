package server

import (
	"net/http"
	"strings"
	"testing"
)

// TestFlowTriggerSampleSource: the test-data answer says whether the sample is the built-in
// default (which the editor shows in the UI language) or a saved one (user data).
func TestFlowTriggerSampleSource(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	trigger := ""
	for _, n := range rec.Draft.Nodes {
		if strings.HasPrefix(n.Type, "trigger.") {
			trigger = n.ID
			break
		}
	}
	if trigger == "" {
		t.Fatal("the greet flow has no trigger")
	}
	path := "/api/desktop/flows/" + rec.ID + "/test-data/" + trigger
	if got := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))["source"]; got != "builtin" {
		t.Fatalf("source before saving = %v, want builtin", got)
	}
	if w := flowsCall(t, s, http.MethodPut, path, token, `{"data":{"text":"mine"}}`); w.Code != http.StatusOK {
		t.Fatalf("save = %d %s", w.Code, w.Body.String())
	}
	body := flowsBody(t, flowsCall(t, s, http.MethodGet, path, token, ""))
	data, _ := body["data"].(map[string]any)
	if body["source"] != "saved" || data["text"] != "mine" {
		t.Fatalf("after saving = %v, want source saved with the saved data", body)
	}
}
