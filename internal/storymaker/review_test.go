package storymaker

import (
	"context"
	"io"
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
