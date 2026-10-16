package services

import "testing"

func TestWriteTargetsOverlapLifecycle(t *testing.T) {
	var w WriteTargets
	a := WriteTarget{Run: "a", Lineage: "a", Project: "p", Resource: "main.ts"}
	if len(w.Note(a)) != 0 {
		t.Fatal("first write overlaps")
	}
	child := a
	child.Run = "child"
	if len(w.Note(child)) != 0 {
		t.Fatal("own child conflicts")
	}
	b := a
	b.Run = "b"
	b.Lineage = "b"
	if len(w.Note(b)) != 2 {
		t.Fatal("parallel overlap missing")
	}
	w.Release("a")
	w.Release("child")
	if len(w.Snapshot("p")) != 1 {
		t.Fatal("release failed")
	}
	c := b
	c.Project = "q"
	if len(w.Note(c)) != 0 {
		t.Fatal("project leaked")
	}
	w.Clear()
	if len(w.Snapshot("p")) != 0 {
		t.Fatal("shutdown leaked")
	}
}
