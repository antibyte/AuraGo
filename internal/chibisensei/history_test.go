package chibisensei

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func seedRows(t *testing.T, st *Store, inputs ...RowInput) Project {
	t.Helper()
	p, err := st.CreateProject(context.Background(), ProjectInput{Name: "P", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddRows(context.Background(), p.ID, inputs, AddOptions{MaxRows: 50000}); err != nil {
		t.Fatal(err)
	}
	return p
}

func allRows(t *testing.T, st *Store, id string) []Row {
	t.Helper()
	var out []Row
	if err := st.EachRow(context.Background(), id, func(r Row) error { out = append(out, r); return nil }); err != nil {
		t.Fatal(err)
	}
	return out
}

func TestBulkDeleteAndUndo(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	var in []RowInput
	for _, q := range []string{"one", "two", "three", "four", "five"} {
		in = append(in, chatInput(q, "a long enough answer"))
	}
	p := seedRows(t, st, in...)
	before := allRows(t, st, p.ID)
	res, err := st.Bulk(ctx, p.ID, []int64{before[0].ID, before[1].ID, before[2].ID}, "delete", "")
	if err != nil || res.Affected != 3 {
		t.Fatalf("bulk: %+v, %v", res, err)
	}
	if n, _ := st.CountRows(ctx, p.ID); n != 2 {
		t.Fatalf("count = %d", n)
	}
	h, _ := st.History(ctx, p.ID, 0)
	if h[0].Kind != "bulk_delete" || h[0].Rows != 3 {
		t.Fatalf("history[0] = %+v", h[0])
	}
	u, err := st.Undo(ctx, p.ID, res.BatchID)
	if err != nil || u.Restored != 3 {
		t.Fatalf("undo: %+v, %v", u, err)
	}
	after := allRows(t, st, p.ID)
	for i := range before {
		if after[i].ID != before[i].ID || after[i].Seq != before[i].Seq {
			t.Errorf("row %d: id/seq %d/%d, want %d/%d", i, after[i].ID, after[i].Seq, before[i].ID, before[i].Seq)
		}
	}
	if _, err := st.Undo(ctx, p.ID, res.BatchID); !errors.Is(err, ErrAlreadyUndone) {
		t.Errorf("second undo: %v", err)
	}
	if _, err := st.Undo(ctx, p.ID, "00000000-0000-4000-8000-000000000000"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown batch: %v", err)
	}
}

func TestDedupeKeepsFirstAndUndoes(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st,
		chatInput("Hello world", "a long enough answer"),
		chatInput("Other", "a long enough answer"),
		chatInput("hello   WORLD", "A long enough answer"),
		chatInput("Hello world", "a long enough answer"))
	res, err := st.Dedupe(ctx, p.ID)
	if err != nil || res.Affected != 2 {
		t.Fatalf("dedupe: %+v, %v", res, err)
	}
	var seqs []int64
	for _, r := range allRows(t, st, p.ID) {
		seqs = append(seqs, r.Seq)
	}
	if !reflect.DeepEqual(seqs, []int64{1, 2}) {
		t.Fatalf("remaining seq = %v", seqs)
	}
	if u, err := st.Undo(ctx, p.ID, res.BatchID); err != nil || u.Restored != 2 {
		t.Fatalf("undo: %+v, %v", u, err)
	}
	if n, _ := st.CountRows(ctx, p.ID); n != 4 {
		t.Fatalf("rows after undo = %d, want 4", n)
	}
}

func TestBulkEditAndDeleteTokens(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("a", "a long enough answer"), chatInput("b", "a long enough answer"), chatInput("c", "a long enough answer"))
	rows := allRows(t, st, p.ID)
	ids := []int64{rows[0].ID, rows[1].ID}
	res, err := st.Bulk(ctx, p.ID, ids, "set_split", "eval")
	if err != nil || res.Affected != 2 {
		t.Fatalf("set_split: %+v, %v", res, err)
	}
	if again, _ := st.Bulk(ctx, p.ID, ids, "set_split", "eval"); again.Affected != 0 {
		t.Errorf("repeat affected = %d", again.Affected)
	}
	if _, err := st.Undo(ctx, p.ID, res.BatchID); err != nil {
		t.Fatal(err)
	}
	if r, _ := st.GetRow(ctx, p.ID, ids[0]); r.Split != "auto" {
		t.Errorf("split after undo = %q", r.Split)
	}

	now := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	tokens := NewDeleteTokens(func() time.Time { return now })
	tok, ttl := tokens.Issue("p", Select{IDs: []int64{3, 1}}, 4)
	if ttl != DeleteTokenTTL || len(tok) != 32 {
		t.Fatalf("token %q ttl %v", tok, ttl)
	}
	if !tokens.Consume(tok, "p", Select{IDs: []int64{1, 3}}, 4) {
		t.Error("matching consume failed")
	}
	if tokens.Consume(tok, "p", Select{IDs: []int64{1, 3}}, 4) {
		t.Error("second consume succeeded")
	}
	tok, _ = tokens.Issue("p", Select{IDs: []int64{1, 3}}, 4)
	if tokens.Consume(tok, "p", Select{IDs: []int64{1, 3}}, 5) {
		t.Error("wrong data_rev accepted")
	}
	tok, _ = tokens.Issue("p", Select{IDs: []int64{1, 3}}, 4)
	now = now.Add(6 * time.Minute)
	if tokens.Consume(tok, "p", Select{IDs: []int64{1, 3}}, 4) {
		t.Error("expired token accepted")
	}
}
