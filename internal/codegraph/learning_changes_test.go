package codegraph

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestGraphNotifiesFingerprintWhenChangedFileCannotParse(t *testing.T) {
	p := Project{ID: "test", Root: t.TempDir()}
	var changes sync.Map
	m := New(t.Context(), nil, nil, func(_ context.Context, _ Project, name, hash string) { changes.Store(name, hash) })
	defer m.Close()
	for _, source := range []string{"package demo\nfunc Original() {}\n", "package demo\nfunc Broken( {"} {
		if err := os.WriteFile(filepath.Join(p.Root, "main.go"), []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if err := m.Rescan(t.Context(), p); err != nil {
			t.Fatal(err)
		}
		want, err := Fingerprint(p, "main.go")
		if got, _ := changes.Load("main.go"); err != nil || got != want {
			t.Errorf("changed fingerprint not reported: got %v, want %s, err %v", got, want, err)
		}
	}
	result, err := m.Query(t.Context(), p, Request{Operation: "outline", File: "main.go"})
	if err != nil || result.Status.Complete || result.Status.Indexed != 0 || result.Status.SkippedCount != 1 || len(result.Symbols) != 0 {
		t.Fatal("failed parse published facts or complete coverage", result, err)
	}
}
