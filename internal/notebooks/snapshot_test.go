package notebooks

import (
	"encoding/json"
	"testing"
)

func TestSessionSnapshotPreservesPublicStateWithoutRuntime(t *testing.T) {
	s := &Session{ID: "session", Project: "project", File: "book.ipynb", Status: "busy", Error: "message", Internet: true, Execution: "execution", Revision: "revision", Cell: "cell", container: "private-container", files: []string{"private-file"}}
	k := &Kernels{sessions: map[string]*Session{s.ID: s}}
	snapshot := k.List()[0]
	s.Status = "idle"
	data, err := json.Marshal(snapshot)
	want := `{"id":"session","project_id":"project","file":"book.ipynb","status":"busy","error":"message","internet":true,"execution_id":"execution","revision":"revision","cell_id":"cell"}`
	if err != nil || string(data) != want {
		t.Fatalf("snapshot wire contract changed: %s, %v", data, err)
	}
}
