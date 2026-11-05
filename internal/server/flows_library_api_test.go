package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"aurago/internal/security"
)

func flowFolderCall(t *testing.T, s *Server, method, path, token, body string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	s.handleFlowFolders(w, r)
	return w
}

func TestFlowLibraryPartialCASAndPrivacy(t *testing.T) {
	s, token := newFlowsTestServer(t)
	stm := c17Audit(t, s)
	rec := createTestFlow(t, s, greetFlowJSON)
	path := "/api/desktop/flows/" + rec.ID + "/library"
	initial := flowsCall(t, s, http.MethodGet, path, token, "")
	if initial.Code != http.StatusOK {
		t.Fatalf("read library: %d %s", initial.Code, initial.Body.String())
	}
	meta := flowsBody(t, initial)["library"].(map[string]any)
	if meta["revision"] != float64(0) || meta["folder_id"] != nil || len(meta["tags"].([]any)) != 0 {
		t.Fatalf("initial metadata = %+v", meta)
	}
	created := flowFolderCall(t, s, http.MethodPost, flowFoldersPath, token, `{"name":"Saved folder"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create folder: %d %s", created.Code, created.Body.String())
	}
	folder := flowsBody(t, created)["folder"].(map[string]any)
	folderID := folder["id"].(string)
	const private = "library_tag_private_canary"
	data, _ := json.Marshal(map[string]any{"base_revision": 0, "folder_id": folderID, "tags": []string{private, "public"}})
	updated := flowsCall(t, s, http.MethodPut, path, token, string(data))
	if updated.Code != http.StatusOK || flowsBody(t, updated)["library"].(map[string]any)["revision"] != float64(1) {
		t.Fatalf("update: %d %s", updated.Code, updated.Body.String())
	}
	if w := flowFolderCall(t, s, http.MethodDelete, flowFoldersPath+"/"+folderID, token, `{"base_revision":1}`); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_FOLDER_NOT_EMPTY" {
		t.Fatalf("delete occupied: %d %s", w.Code, w.Body.String())
	}
	release := security.RegisterScopedSensitiveExact(private)
	defer release()
	masked := flowsCall(t, s, http.MethodGet, path, token, "")
	maskedMeta := flowsBody(t, masked)["library"].(map[string]any)
	if masked.Code != http.StatusOK || strings.Contains(masked.Body.String(), private) || maskedMeta["tags_redacted"] != true || maskedMeta["tags_editable"] != false {
		t.Fatalf("masked metadata: %d %s", masked.Code, masked.Body.String())
	}
	clearFolder := flowsCall(t, s, http.MethodPut, path, token, `{"base_revision":1,"folder_id":null}`)
	if clearFolder.Code != http.StatusOK || strings.Contains(clearFolder.Body.String(), private) {
		t.Fatalf("clear folder: %d %s", clearFolder.Code, clearFolder.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPut, path, token, `{"base_revision":1,"tags":[]}`); w.Code != http.StatusConflict || flowsBody(t, w)["code"] != "FLOW_LIBRARY_CONFLICT" {
		t.Fatalf("stale patch: %d %s", w.Code, w.Body.String())
	}
	if w := flowsCall(t, s, http.MethodPut, path, token, `{"base_revision":2,"tags":["prefix [redacted] suffix"]}`); w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_LIBRARY_SENSITIVE" {
		t.Fatalf("redaction marker: %d %s", w.Code, w.Body.String())
	}
	release()
	unmasked := flowsCall(t, s, http.MethodGet, path, token, "")
	if !strings.Contains(unmasked.Body.String(), private) {
		t.Fatalf("omitted tags were not preserved: %s", unmasked.Body.String())
	}
	if w := flowFolderCall(t, s, http.MethodDelete, flowFoldersPath+"/"+folderID, token, `{"base_revision":1}`); w.Code != http.StatusOK {
		t.Fatalf("delete empty folder: %d %s", w.Code, w.Body.String())
	}
	after, err := s.Flows.GetFlow(t.Context(), rec.ID)
	if err != nil || after.DraftRevision != rec.DraftRevision || after.LiveRevision != rec.LiveRevision {
		t.Fatalf("library changed flow revisions: %+v %v", after, err)
	}
	for _, event := range c17AuditEvents(t, stm, "flow_library_change") {
		encoded, _ := json.Marshal(event)
		if strings.Contains(string(encoded), private) || strings.Contains(string(encoded), "Saved folder") {
			t.Fatal("library audit disclosed user text")
		}
	}
}

func TestFlowAndBlockVersionNotes(t *testing.T) {
	s, token := newFlowsTestServer(t)
	flow := createTestFlow(t, s, greetFlowJSON)
	blockResponse := flowBlockCall(t, s, http.MethodPost, flowBlocksPath, token, map[string]any{"name": "Note block"})
	if blockResponse.Code != http.StatusCreated {
		t.Fatalf("block: %d %s", blockResponse.Code, blockResponse.Body.String())
	}
	block := flowsBody(t, blockResponse)["block"].(map[string]any)
	for _, tc := range []struct {
		name, path string
		block      bool
	}{
		{"flow", "/api/desktop/flows/" + flow.ID, false},
		{"block", flowBlocksPath + "/" + block["id"].(string), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			call := func(method, path, body string) *httptest.ResponseRecorder {
				if tc.block {
					var data any
					if body != "" {
						if err := json.Unmarshal([]byte(body), &data); err != nil {
							t.Fatal(err)
						}
					}
					return flowBlockCall(t, s, method, path, token, data)
				}
				return flowsCall(t, s, method, path, token, body)
			}
			publish := call(http.MethodPost, tc.path+"/publish", `{"base_revision":1,"note":"original note"}`)
			if publish.Code != http.StatusOK {
				t.Fatalf("publish: %d %s", publish.Code, publish.Body.String())
			}
			notePath := tc.path + "/versions/1/note"
			note := call(http.MethodGet, notePath, "")
			if note.Code != http.StatusOK || flowsBody(t, note)["note"] != "original note" || flowsBody(t, note)["revision"] != float64(1) {
				t.Fatalf("note: %d %s", note.Code, note.Body.String())
			}
			edit := call(http.MethodPut, notePath, `{"base_revision":1,"note":"updated note"}`)
			if edit.Code != http.StatusOK || flowsBody(t, edit)["revision"] != float64(2) {
				t.Fatalf("edit note: %d %s", edit.Code, edit.Body.String())
			}
			conflict := call(http.MethodPost, tc.path+"/publish", `{"base_revision":1,"note":"original note"}`)
			if conflict.Code != http.StatusConflict || flowsBody(t, conflict)["code"] != "FLOW_VERSION_NOTE_CONFLICT" {
				t.Fatalf("republish conflict: %d %s", conflict.Code, conflict.Body.String())
			}
			if w := call(http.MethodPost, tc.path+"/publish", `{"base_revision":1}`); w.Code != http.StatusOK {
				t.Fatalf("repair publish: %d %s", w.Code, w.Body.String())
			}
			if w := call(http.MethodGet, notePath, ""); flowsBody(t, w)["note"] != "updated note" {
				t.Fatalf("repair changed note: %d %s", w.Code, w.Body.String())
			}
			if w := call(http.MethodPut, notePath, `{"base_revision":1,"note":"stale"}`); w.Code != http.StatusConflict {
				t.Fatalf("stale note: %d %s", w.Code, w.Body.String())
			}
			if w := call(http.MethodPost, tc.path+"/publish", `{"base_revision":1,"note":null}`); w.Code != http.StatusBadRequest {
				t.Fatalf("null note: %d %s", w.Code, w.Body.String())
			}
		})
	}
}

func TestFlowVersionNotePrivacyBeforePreviewTruncation(t *testing.T) {
	s, token := newFlowsTestServer(t)
	rec := createTestFlow(t, s, greetFlowJSON)
	path := "/api/desktop/flows/" + rec.ID
	const secret = "later_registered_private_secret"
	text := strings.Repeat("x", 113) + secret + " end"
	body, _ := json.Marshal(map[string]any{"base_revision": 1, "note": text})
	if w := flowsCall(t, s, http.MethodPost, path+"/publish", token, string(body)); w.Code != http.StatusOK {
		t.Fatalf("publish: %d %s", w.Code, w.Body.String())
	}
	release := security.RegisterScopedSensitiveExact(secret)
	defer release()
	full := flowsCall(t, s, http.MethodGet, path+"/versions/1/note", token, "")
	if full.Code != http.StatusOK || strings.Contains(full.Body.String(), secret) || flowsBody(t, full)["redacted"] != true || flowsBody(t, full)["editable"] != false {
		t.Fatalf("masked note: %d %s", full.Code, full.Body.String())
	}
	list := flowsCall(t, s, http.MethodGet, path+"/versions", token, "")
	versions := flowsBody(t, list)["versions"].([]any)
	if list.Code != http.StatusOK || len(versions) != 1 {
		t.Fatalf("versions: %d %s", list.Code, list.Body.String())
	}
	item := versions[0].(map[string]any)
	preview, _ := item["note_preview"].(string)
	if strings.Contains(preview, secret[:7]) || item["note_redacted"] != true || item["note_editable"] != false {
		t.Fatalf("unsafe preview: %+v", item)
	}
	masked, _ := json.Marshal(map[string]any{"base_revision": 1, "note": flowsBody(t, full)["note"]})
	if w := flowsCall(t, s, http.MethodPut, path+"/versions/1/note", token, string(masked)); w.Code != http.StatusBadRequest || flowsBody(t, w)["code"] != "FLOW_LIBRARY_SENSITIVE" {
		t.Fatalf("masked roundtrip: %d %s", w.Code, w.Body.String())
	}
	release()
	if w := flowsCall(t, s, http.MethodGet, path+"/versions/1/note", token, ""); flowsBody(t, w)["note"] != text {
		t.Fatalf("note was altered: %d %s", w.Code, w.Body.String())
	}
}
