package server

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"aurago/internal/desktop"
)

func TestDesktopNotesMetadataFirstUse(t *testing.T) {
	s := newDesktopOfficeTestServer(t)
	h := handleDesktopNotes(s)
	request := func(method, path, header, version, content string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"path": path, "content": content})
		r := httptest.NewRequest(method, "/api/desktop/notes?path="+url.QueryEscape(path), bytes.NewReader(body))
		if header != "" {
			r.Header.Set(header, version)
		}
		w := httptest.NewRecorder()
		h(w, r)
		return w
	}
	path := desktop.NotesDirectory + "/notes.meta.json"
	// The secure file reader wraps fs.ErrNotExist with path context. The client
	// must receive 404 so it can initialize the optional sidecar conditionally.
	for _, missing := range []string{path, desktop.NotesDirectory + "/absent.md"} {
		if w := request("GET", missing, "", "", ""); w.Code != 404 {
			t.Fatalf("missing %s: status=%d body=%s", missing, w.Code, w.Body.String())
		}
	}
	initial := `{"version":1,"pinned":[],"sort":"modified","last_note":"Documents/Notes/example.md"}`
	created := request("PUT", path, "If-None-Match", "*", initial)
	if created.Code != 200 {
		t.Fatalf("create sidecar: %d %s", created.Code, created.Body.String())
	}
	read := request("GET", path, "", "", "")
	var saved struct{ Content, Version string }
	if err := json.Unmarshal(read.Body.Bytes(), &saved); err != nil || read.Code != 200 || saved.Content != initial || saved.Version != created.Header().Get("ETag") {
		t.Fatalf("read sidecar: %d %s, error=%v", read.Code, read.Body.String(), err)
	}
	updated := request("PUT", path, "If-Match", saved.Version, `{"version":1,"pinned":["Documents/Notes/example.md"],"sort":"name"}`)
	if updated.Code != 200 {
		t.Fatalf("update sidecar: %d %s", updated.Code, updated.Body.String())
	}
	for _, precondition := range [][2]string{{"If-Match", saved.Version}, {"If-None-Match", "*"}} {
		if w := request("PUT", path, precondition[0], precondition[1], initial); w.Code != 412 {
			t.Fatalf("sidecar overwrite protection: %d %s", w.Code, w.Body.String())
		}
	}
	if w := request("GET", "Documents/Notes/not-a-note.json", "", "", ""); w.Code != 400 {
		t.Fatalf("invalid paths must stay invalid: %d %s", w.Code, w.Body.String())
	}
	listed := request("GET", "", "", "", "")
	var result desktop.NotesResult
	if err := json.Unmarshal(listed.Body.Bytes(), &result); err != nil || listed.Code != 200 || result.Total != 0 {
		t.Fatalf("metadata leaked into note list: %d %s", listed.Code, listed.Body.String())
	}
}

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

func TestDesktopNotesIndexMkdirAndLinkUpdates(t *testing.T) {
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
	const url = "/api/desktop/notes"
	linking := request("POST", url, "", "", map[string]interface{}{"title": "Linking", "content": "# Linking\n[[Target]] #idx"})
	target := request("POST", url, "", "", map[string]interface{}{"title": "Target", "content": "# Target"})
	if linking.Code != 200 || target.Code != 200 {
		t.Fatal(linking.Code, target.Code, target.Body.String())
	}
	var targetNote desktop.Note
	json.Unmarshal(target.Body.Bytes(), &targetNote)

	index := request("GET", url+"?index=1", "", "", nil)
	var indexed desktop.NotesIndexResult
	if err := json.Unmarshal(index.Body.Bytes(), &indexed); err != nil || index.Code != 200 || len(indexed.Notes) != 2 {
		t.Fatalf("index: %d %s", index.Code, index.Body.String())
	}
	if !strings.Contains(index.Body.String(), `"notes"`) || !strings.Contains(index.Body.String(), `"attachments"`) {
		t.Fatalf("index fields: %s", index.Body.String())
	}

	if w := request("POST", url, "", "", map[string]interface{}{"operation": "mkdir", "folder": "Documents/Notes/Neu"}); w.Code != 200 || !strings.Contains(w.Body.String(), `"path":"Documents/Notes/Neu"`) {
		t.Fatalf("mkdir: %d %s", w.Code, w.Body.String())
	}
	after := request("GET", url+"?index=1", "", "", nil)
	if !strings.Contains(after.Body.String(), `"Documents/Notes/Neu"`) {
		t.Fatalf("folder missing from index: %s", after.Body.String())
	}
	for _, bad := range []string{"Documents/Notes/../x", "Documents/Other", "Documents/Notes/.attachments/x", "Documents/Notes", "Documents/Notes/a/../b", ""} {
		if w := request("POST", url, "", "", map[string]interface{}{"operation": "mkdir", "folder": bad}); w.Code != 400 {
			t.Fatalf("mkdir %q: %d %s", bad, w.Code, w.Body.String())
		}
	}

	moved := request("PATCH", url, "If-Match", targetNote.Version, map[string]interface{}{"path": targetNote.Path, "operation": "move", "new_path": "Documents/Notes/Neu/Renamed.md", "update_links": true})
	if moved.Code != 200 || !strings.Contains(moved.Body.String(), `"updated_links":1`) {
		t.Fatalf("move with links: %d %s", moved.Code, moved.Body.String())
	}
	var linkingNote desktop.Note
	json.Unmarshal(linking.Body.Bytes(), &linkingNote)
	read := request("GET", url+"?path="+linkingNote.Path, "", "", nil)
	if !strings.Contains(read.Body.String(), "[[Renamed]]") {
		t.Fatalf("link not rewritten: %s", read.Body.String())
	}
}
