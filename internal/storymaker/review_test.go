package storymaker

import (
	"context"
	"errors"
	"io"
	"io/fs"
	"testing"
)

func TestReviewSheetRoundTrips(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	project, _, _, err := ps.Create(ctx, "Review", "16:9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := ps.OpenReviewSheet(ctx, project.ID); err == nil {
		t.Fatal("no sheet yet")
	}
	if err := ps.SaveReviewSheet(ctx, project.ID, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := ps.SaveReviewSheet(ctx, project.ID, []byte("second")); err != nil {
		t.Fatal(err)
	}
	r, _, err := ps.OpenReviewSheet(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	data, _ := io.ReadAll(r)
	if string(data) != "second" {
		t.Fatalf("sheet = %q: a new sheet replaces the old one", data)
	}
	if err := ps.SaveReviewSheet(ctx, "../evil", []byte("x")); err == nil {
		t.Fatal("a bad id is refused")
	}
}

// A check that ends after its project was deleted must not bring the folder back: the store creates the folders of a file it
// writes.
func TestSaveReviewSheetRefusesADeletedProject(t *testing.T) {
	ctx := context.Background()
	ps, _, _ := newTestStore(t)
	project, _, _, err := ps.Create(ctx, "Deleted", "16:9", 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := ps.Delete(ctx, project.ID); err != nil {
		t.Fatal(err)
	}
	if err := ps.SaveReviewSheet(ctx, project.ID, []byte("late")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("SaveReviewSheet after the delete = %v, want fs.ErrNotExist", err)
	}
	if names, err := ps.st.List(ctx, ps.dir(project.ID)); err != nil || len(names) != 0 {
		t.Fatalf("the deleted project folder came back: %v, %v", names, err)
	}
	// A project that never existed is the same case.
	if err := ps.SaveReviewSheet(ctx, "00000000-0000-4000-8000-999999999999", []byte("x")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("SaveReviewSheet of an unknown project = %v", err)
	}
}
