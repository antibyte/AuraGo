package server

import (
	"net/http"
	"strings"
	"testing"
)

const fb4HeaderFlowJSON = `{"schema":1,"name":"Header","nodes":[
 {"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"label":"Start","position":{"x":0,"y":0},"params":{}},
 {"id":"n_bbbbbbbb","key":"call","type":"http.request","type_version":1,"label":"Call","position":{"x":300,"y":0},
  "params":{"method":"GET","url":"https://api.example/x","headers":{"X-Api-Key":"fb4-literal-key"}}}],
 "edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`

func TestFB4ExportRedactsCredentialHeaders(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, fb4HeaderFlowJSON)
	w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+rec.ID+"/export", token, "")
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "fb4-literal-key") || !strings.Contains(w.Body.String(), "[redacted]") {
		t.Fatalf("export = %d %s", w.Code, w.Body.String())
	}
}
