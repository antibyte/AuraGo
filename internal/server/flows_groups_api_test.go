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
)

// TestFlowGroupsHTTP tests compatibility and version replacement through both public APIs.
func TestFlowGroupsHTTP(t *testing.T) {
	for _, kind := range []string{"flow", "block"} {
		t.Run(kind, func(t *testing.T) {
			s, token := newFlowsTestServer(t)
			readToken, _, err := s.TokenManager.Create("group reader", []string{desktopScopeRead}, nil)
			if err != nil {
				t.Fatal(err)
			}
			var id, path, key string
			var doc map[string]any
			if kind == "flow" {
				w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+greetFlowJSON+`}`)
				if w.Code != http.StatusCreated {
					t.Fatalf("create: %d %s", w.Code, w.Body.String())
				}
				rec := flowsBody(t, w)["flow"].(map[string]any)
				id, key, doc = rec["id"].(string), "flow", rec["draft"].(map[string]any)
				path = "/api/desktop/flows/" + id
			} else {
				w := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Grouped block"})
				if w.Code != http.StatusCreated {
					t.Fatalf("create block: %d %s", w.Code, w.Body.String())
				}
				rec := flowsBody(t, w)["block"].(map[string]any)
				id, key, doc = rec["id"].(string), "block", rec["draft"].(map[string]any)
				path = flowBlocksPath + "/" + id
			}
			call := func(method, route, credential string, body any) *httptest.ResponseRecorder {
				t.Helper()
				if kind == "block" {
					return flowBlockCall(t, s, method, route, credential, body)
				}
				encoded := ""
				if body != nil {
					value, err := json.Marshal(body)
					if err != nil {
						t.Fatal(err)
					}
					encoded = string(value)
				}
				return flowsCall(t, s, method, route, credential, encoded)
			}
			draft := func() map[string]any {
				t.Helper()
				w := call(http.MethodGet, path, token, nil)
				if w.Code != http.StatusOK {
					t.Fatalf("get: %d %s", w.Code, w.Body.String())
				}
				return flowsBody(t, w)[key].(map[string]any)["draft"].(map[string]any)
			}
			save := func(document map[string]any, revision int) *httptest.ResponseRecorder {
				return call(http.MethodPut, path, token, map[string]any{"doc": document, "base_revision": revision})
			}
			publish := func(revision int) {
				t.Helper()
				if w := call(http.MethodPost, path+"/publish", token, map[string]any{"base_revision": revision}); w.Code != http.StatusOK {
					t.Fatalf("publish %d: %d %s", revision, w.Code, w.Body.String())
				}
			}
			restore := func(version, revision int) *httptest.ResponseRecorder {
				return call(http.MethodPost, path+"/versions", token, map[string]any{"revision": version, "base_revision": revision})
			}
			publish(1) // Version 1 predates groups.
			nodeID := doc["nodes"].([]any)[0].(map[string]any)["id"].(string)
			const title = `<img src=x onerror="not executed">`
			doc["groups"] = []any{map[string]any{"id": "g_aaaaaaaa", "title": title, "color": "blue", "note": "Notes <script>stay text</script>", "members": []any{nodeID}}}
			if w := save(doc, 1); w.Code != http.StatusOK {
				t.Fatalf("save group: %d %s", w.Code, w.Body.String())
			}
			publish(2)
			var pinMissionID string
			if kind == "block" {
				parent := fmt.Sprintf(`{"schema":1,"kind":"flow","name":"Pinned block","nodes":[{"id":"n_aaaaaaaa","key":"start","type":"trigger.manual","type_version":1,"params":{}},{"id":"n_bbbbbbbb","key":"block","type":"component.call","type_version":1,"params":{"flow_id":%q,"revision":2,"inputs":{}}}],"edges":[{"id":"e_aaaaaaaa","source":{"node":"n_aaaaaaaa","port":"out"},"target":{"node":"n_bbbbbbbb","port":"in"}}]}`, id)
				created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+parent+`}`)
				if created.Code != http.StatusCreated {
					t.Fatalf("create pin owner: %d %s", created.Code, created.Body.String())
				}
				parentRecord := flowsBody(t, created)["flow"].(map[string]any)
				parentID := parentRecord["id"].(string)
				pinMissionID = parentRecord["mission_id"].(string)
				if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+parentID+"/publish", token, `{"base_revision":1}`); w.Code != http.StatusOK {
					t.Fatalf("publish pin owner: %d %s", w.Code, w.Body.String())
				}
			}
			if got := draft()["groups"].([]any)[0].(map[string]any); got["title"] != title {
				t.Fatalf("group text did not roundtrip: %+v", got)
			}
			oldClient := draft()
			delete(oldClient, "groups")
			if w := save(oldClient, 2); w.Code != http.StatusOK {
				t.Fatalf("legacy omission: %d %s", w.Code, w.Body.String())
			}
			if len(draft()["groups"].([]any)) != 1 {
				t.Fatal("legacy omission cleared groups")
			}
			nullClient := draft()
			nullClient["groups"] = nil
			if w := save(nullClient, 3); w.Code != http.StatusOK || len(draft()["groups"].([]any)) != 1 {
				t.Fatalf("null must preserve groups: %d %s", w.Code, w.Body.String())
			}
			if w := save(nullClient, 3); w.Code != http.StatusConflict {
				t.Fatalf("stale draft accepted: %d %s", w.Code, w.Body.String())
			}
			dangling := draft()
			delete(dangling, "groups") // Simulates an older client deleting a grouped node.
			dangling["nodes"] = dangling["nodes"].([]any)[1:]
			dangling["edges"] = []any{}
			if w := save(dangling, 4); w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "GROUP_INVALID") {
				t.Fatalf("dangling preserved group: %d %s", w.Code, w.Body.String())
			}
			clear := draft()
			clear["groups"] = []any{}
			if w := save(clear, 4); w.Code != http.StatusOK {
				t.Fatalf("explicit clear: %d %s", w.Code, w.Body.String())
			}
			if groups, ok := draft()["groups"].([]any); ok && len(groups) != 0 {
				t.Fatalf("clear retained groups: %+v", groups)
			}
			if w := restore(2, 4); w.Code != http.StatusConflict {
				t.Fatalf("stale restore: %d %s", w.Code, w.Body.String())
			}
			var beforeMissions any
			if kind == "block" {
				beforeMissions = s.MissionManagerV2.List()
			}
			if w := restore(2, 5); w.Code != http.StatusOK || len(draft()["groups"].([]any)) != 1 {
				t.Fatalf("restore grouped version: %d %s", w.Code, w.Body.String())
			}
			if w := restore(1, 6); w.Code != http.StatusOK {
				t.Fatalf("restore group-free version: %d %s", w.Code, w.Body.String())
			}
			if groups, ok := draft()["groups"].([]any); ok && len(groups) != 0 {
				t.Fatalf("old version retained draft groups: %+v", groups)
			}
			history := call(http.MethodGet, path+"/versions", token, nil)
			if history.Code != http.StatusOK || len(flowsBody(t, history)["versions"].([]any)) != 2 {
				t.Fatalf("restore altered immutable history: %d %s", history.Code, history.Body.String())
			}
			if kind == "block" {
				if !reflect.DeepEqual(beforeMissions, s.MissionManagerV2.List()) || pinMissionID == "" {
					t.Fatal("block restore changed Mission Control state")
				}
				version := call(http.MethodGet, path+"/versions/2", token, nil)
				if version.Code != http.StatusOK || len(flowsBody(t, version)["doc"].(map[string]any)["groups"].([]any)) != 1 {
					t.Fatalf("restore altered block version: %d %s", version.Code, version.Body.String())
				}
				if w := call(http.MethodDelete, path, token, map[string]any{"base_revision": 7, "base_live_revision": 2}); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_BLOCK_IN_USE" {
					t.Fatalf("restore lost published block pin: %d %s", w.Code, w.Body.String())
				}
				wrongKind := flowsCall(t, s, http.MethodPost, "/api/desktop/flows/"+id+"/versions", token, `{"revision":2,"base_revision":7}`)
				if wrongKind.Code != http.StatusNotFound {
					t.Fatalf("block restored via flow API: %d %s", wrongKind.Code, wrongKind.Body.String())
				}
			}
			if w := restore(0, 7); w.Code != http.StatusBadRequest {
				t.Fatalf("invalid version: %d %s", w.Code, w.Body.String())
			}
			if w := call(http.MethodPost, path+"/versions", readToken, map[string]any{"revision": 2, "base_revision": 7}); w.Code != http.StatusForbidden {
				t.Fatalf("read-only restore: %d %s", w.Code, w.Body.String())
			}
			s.Cfg.Tools.Missions.ReadOnly = true
			if w := restore(2, 7); w.Code != http.StatusForbidden {
				t.Fatalf("readonly restore: %d %s", w.Code, w.Body.String())
			}
			s.Cfg.Tools.Missions.ReadOnly = false
			s.Cfg.Flows.Enabled = false
			if w := restore(2, 7); w.Code != http.StatusServiceUnavailable {
				t.Fatalf("disabled restore: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestFlowGroupsHTTPImportAndBoundaries(t *testing.T) {
	s, token := newFlowsTestServer(t)
	var doc map[string]any
	if err := json.Unmarshal([]byte(greetFlowJSON), &doc); err != nil {
		t.Fatal(err)
	}
	group := map[string]any{"id": "g_aaaaaaaa", "title": "Safe <em>text</em>", "color": "rose", "note": "literal <script>text</script>", "members": []any{"n_aaaaaaaa"}}
	doc["groups"] = []any{group}
	importBody, err := json.Marshal(map[string]any{"import": doc})
	if err != nil {
		t.Fatal(err)
	}
	created := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, string(importBody))
	if created.Code != http.StatusCreated {
		t.Fatalf("import: %d %s", created.Code, created.Body.String())
	}
	id := flowsBody(t, created)["flow"].(map[string]any)["id"].(string)
	exported := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id+"/export", token, "")
	if exported.Code != http.StatusOK {
		t.Fatalf("export: %d %s", exported.Code, exported.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(exported.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if got := result["groups"].([]any)[0].(map[string]any); got["title"] != group["title"] || got["note"] != group["note"] {
		t.Fatalf("import/export lost literal group text: %+v", got)
	}
	if result["schema"] != float64(2) {
		t.Fatalf("grouped export has no compatibility marker: %s", exported.Body.String())
	}
	reimport := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+exported.Body.String()+`}`)
	if reimport.Code != http.StatusCreated {
		t.Fatalf("schema 2 reimport: %d %s", reimport.Code, reimport.Body.String())
	}
	reimportDraft := flowsBody(t, reimport)["flow"].(map[string]any)["draft"].(map[string]any)
	if reimportDraft["schema"] != float64(1) || len(reimportDraft["groups"].([]any)) != 1 {
		t.Fatalf("reimport did not normalize/preserve group: %+v", reimportDraft)
	}
	putSchema2, _ := json.Marshal(map[string]any{"doc": result, "base_revision": 1})
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, string(putSchema2)); w.Code != http.StatusBadRequest {
		t.Fatalf("schema 2 accepted by draft PUT: %d %s", w.Code, w.Body.String())
	}
	result["schema"] = 99
	future, _ := json.Marshal(map[string]any{"import": result})
	if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, string(future)); w.Code != http.StatusBadRequest {
		t.Fatalf("future export schema accepted: %d %s", w.Code, w.Body.String())
	}
	plain := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, `{"import":`+greetFlowJSON+`}`)
	if plain.Code != http.StatusCreated {
		t.Fatalf("create group-free flow: %d %s", plain.Code, plain.Body.String())
	}
	plainID := flowsBody(t, plain)["flow"].(map[string]any)["id"].(string)
	if w := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+plainID+"/export", token, ""); w.Code != http.StatusOK || flowsBody(t, w)["schema"] != float64(1) {
		t.Fatalf("group-free export changed format: %d %s", w.Code, w.Body.String())
	}
	for _, tc := range []struct {
		name   string
		groups []any
	}{
		{"unknown member", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "members": []any{"n_zzzzzzzz"}}}},
		{"empty members", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "members": []any{}}}},
		{"invalid group id", []any{map[string]any{"id": "group-invalid", "title": "A", "color": "blue", "members": []any{"n_aaaaaaaa"}}}},
		{"duplicate member", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "members": []any{"n_aaaaaaaa", "n_aaaaaaaa"}}}},
		{"duplicate membership", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "members": []any{"n_aaaaaaaa"}}, map[string]any{"id": "g_bbbbbbbb", "title": "B", "color": "green", "members": []any{"n_aaaaaaaa"}}}},
		{"invalid color", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "not-a-color", "members": []any{"n_aaaaaaaa"}}}},
		{"empty title", []any{map[string]any{"id": "g_aaaaaaaa", "title": " ", "color": "blue", "members": []any{"n_aaaaaaaa"}}}},
		{"title control", []any{map[string]any{"id": "g_aaaaaaaa", "title": "bad\u0001title", "color": "blue", "members": []any{"n_aaaaaaaa"}}}},
		{"note control", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "note": "bad\u0085note", "members": []any{"n_aaaaaaaa"}}}},
		{"oversized title", []any{map[string]any{"id": "g_aaaaaaaa", "title": strings.Repeat("x", 81), "color": "blue", "members": []any{"n_aaaaaaaa"}}}},
		{"oversized note", []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "note": strings.Repeat("x", 2049), "members": []any{"n_aaaaaaaa"}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc["groups"] = tc.groups
			body, _ := json.Marshal(map[string]any{"doc": doc, "base_revision": 1})
			w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, string(body))
			if w.Code != http.StatusUnprocessableEntity || !strings.Contains(w.Body.String(), "GROUP_INVALID") {
				t.Fatalf("invalid group: %d %s", w.Code, w.Body.String())
			}
			invalidImport, _ := json.Marshal(map[string]any{"import": doc})
			if w := flowsCall(t, s, http.MethodPost, "/api/desktop/flows", token, string(invalidImport)); w.Code != http.StatusUnprocessableEntity || flowsBody(t, w)["code"] != "FLOW_INVALID" || !strings.Contains(w.Body.String(), "GROUP_INVALID") {
				t.Fatalf("invalid group import: %d %s", w.Code, w.Body.String())
			}
		})
	}
	const secret = "group-export-sensitive-canary"
	c18Secret(t, secret)
	doc["groups"] = []any{map[string]any{"id": "g_aaaaaaaa", "title": "A", "color": "blue", "note": "before " + secret + " after", "members": []any{"n_aaaaaaaa"}}}
	secretDoc, _ := json.Marshal(map[string]any{"doc": doc, "base_revision": 1})
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, string(secretDoc)); w.Code != http.StatusOK {
		t.Fatalf("save group with sensitive text: %d %s", w.Code, w.Body.String())
	}
	secretExport := flowsCall(t, s, http.MethodGet, "/api/desktop/flows/"+id+"/export", token, "")
	if secretExport.Code != http.StatusOK || strings.Contains(secretExport.Body.String(), secret) || !strings.Contains(secretExport.Body.String(), "[redacted]") {
		t.Fatalf("group export did not scrub registered secret: %d", secretExport.Code)
	}
	large := `{"doc":{"schema":1,"name":"x","description":"` + strings.Repeat("x", flowsDocBodyLimit) + `"},"base_revision":1}`
	if w := flowsCall(t, s, http.MethodPut, "/api/desktop/flows/"+id, token, large); w.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized group request: %d", w.Code)
	}
	blockPath := flowBlocksPath + "/" + id + "/versions"
	if w := flowBlockCall(t, s, http.MethodPost, blockPath, token, map[string]any{"revision": 1, "base_revision": 1}); w.Code != http.StatusNotFound {
		t.Fatalf("flow ID restored as block: %d %s", w.Code, w.Body.String())
	}
	r := httptest.NewRequest(http.MethodPost, blockPath, strings.NewReader(`{"revision":1,"base_revision":1}`))
	r.AddCookie(&http.Cookie{Name: sessionCookieName, Value: createSessionValue(s.Cfg.Auth.SessionSecret, time.Now().Add(time.Hour))})
	r.Header.Set("Origin", "https://untrusted.example")
	w := httptest.NewRecorder()
	s.handleFlowBlocks(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("cross-origin block restore: %d %s", w.Code, w.Body.String())
	}
}
