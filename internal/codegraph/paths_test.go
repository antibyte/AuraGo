package codegraph

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGraphRejectsLinkedFilesAndResources(t *testing.T) {
	dir, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "private.js"), []byte("function Private(){}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(outside, "private.js"), filepath.Join(dir, "linked.js")); err != nil {
		t.Skipf("symlink creation unavailable: %v", err)
	}
	m := New(t.Context(), nil, nil, nil)
	defer m.Close()
	p := Project{ID: "jail", Root: dir}
	r, err := m.Query(t.Context(), p, Request{Operation: "search", Query: "Private"})
	if err != nil || r.Status.Complete || r.Status.Skipped["linked.js"] != "symlink" || len(r.Symbols) != 0 {
		t.Fatal("linked source escaped scope", r, err)
	}
	for _, resource := range []string{"linked.js", "../private.js", "/private.js", "C:/private.js", "a/../../private.js"} {
		if _, err := Fingerprint(p, resource); err == nil {
			t.Fatalf("resource %q escaped scope", resource)
		}
	}
}
