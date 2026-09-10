package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopNotesVersionsAndTrash(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	h := handleDesktopNotes(s)
	request := func(method, url, header, version string, body interface{}) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(method, url, bytes.NewReader(data))
		if header != "" {
			r.Header.Set(header, version)
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}
	url := "/api/desktop/notes"
	created := request("POST", url, "", "", map[string]interface{}{"title": "Test", "folder": "Documents/Notes/Work", "content": "# Test"})
	if created.Code != 200 {
		t.Fatal(created.Code, created.Body.String())
	}
	var note desktop.Note
	json.Unmarshal(created.Body.Bytes(), &note)
	old := created.Header().Get("ETag")
	body := map[string]interface{}{"path": note.Path, "content": "# Edited"}
	if w := request("PUT", url, "", "", body); w.Code != 428 {
		t.Fatal("unguarded", w.Code)
	}
	saved := request("PUT", url, "If-Match", old, body)
	if saved.Code != 200 {
		t.Fatal(saved.Code, saved.Body.String())
	}
	var savedNote desktop.Note
	if err := json.Unmarshal(saved.Body.Bytes(), &savedNote); err != nil || savedNote.Content != "# Edited" || savedNote.Modified.IsZero() || savedNote.Version == "" {
		t.Fatal("incomplete saved snapshot", saved.Body.String())
	}
	if w := request("PUT", url, "If-Match", old, body); w.Code != 412 {
		t.Fatal("stale", w.Code)
	}
	if w := request("PUT", url, "If-None-Match", "*", body); w.Code != 412 {
		t.Fatal("collision", w.Code)
	}
	moved := request("PATCH", url, "If-Match", saved.Header().Get("ETag"), map[string]interface{}{"path": note.Path, "operation": "trash"})
	if moved.Code != 200 {
		t.Fatal(moved.Code, moved.Body.String())
	}
	if w := request("GET", url+"?q=Edited", "", "", nil); w.Code != 200 {
		t.Fatal(w.Code)
	}
	var trashed desktop.Note
	json.Unmarshal(moved.Body.Bytes(), &trashed)
	restored := request("PATCH", url, "If-Match", trashed.Version, map[string]interface{}{"path": trashed.Path, "operation": "restore"})
	if restored.Code != 200 {
		t.Fatal(restored.Code, restored.Body.String())
	}
	var restoredNote desktop.Note
	json.Unmarshal(restored.Body.Bytes(), &restoredNote)
	if restoredNote.Path != note.Path || restoredNote.Content != "# Edited" {
		t.Fatal("restore lost folder or content", restored.Body.String())
	}
}
