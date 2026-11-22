package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	st, err := OpenStore(filepath.Join(t.TempDir(), "chibi_sensei.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	fixed := time.Date(2026, 11, 22, 12, 0, 0, 0, time.UTC)
	st.now = func() time.Time { return fixed }
	return st
}

func chatInput(user, assistant string) RowInput {
	return RowInput{Messages: []Message{{"user", user}, {"assistant", assistant}}}
}

func TestProjectLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rev != 1 || p.DataRev != 0 || p.Preset != "balanced" || p.SplitSeed < 1 || p.SplitSeed > 1<<31-1 {
		t.Fatalf("bad new project: %+v", p)
	}
	if want := (Steps{"in_progress", "todo", "todo", "todo", "todo"}); p.Steps != want {
		t.Fatalf("steps = %+v", p.Steps)
	}
	_, err = st.CreateProject(ctx, ProjectInput{Name: "X", Recipe: RecipeSpecialist})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Issues[0].Field != "task_kind" {
		t.Fatalf("specialist without task_kind: %v", err)
	}
	goal, profile := "Answer politely", "qwen35-0p8b"
	if _, err := st.UpdateProject(ctx, p.ID, 2, ProjectPatch{GoalText: &goal}); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale rev: %v", err)
	}
	p, err = st.UpdateProject(ctx, p.ID, 1, ProjectPatch{GoalText: &goal, ProfileID: &profile})
	if err != nil {
		t.Fatal(err)
	}
	if p.Rev != 2 || p.DataRev != 1 || p.Steps.Goal != "done" {
		t.Fatalf("after update: %+v", p)
	}
	if err := st.DeleteProject(ctx, p.ID, "Bot"); !errors.Is(err, ErrNotArchived) {
		t.Fatalf("delete active: %v", err)
	}
	if _, err := st.SetArchived(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteProject(ctx, p.ID, "bot"); !errors.As(err, &ve) {
		t.Fatalf("wrong confirm name: %v", err)
	}
	if err := st.DeleteProject(ctx, p.ID, "Bot"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetProject(ctx, p.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("get deleted: %v", err)
	}
	if n, err := st.CountRows(ctx, p.ID); err != nil || n != 0 {
		t.Fatalf("count = %d, %v", n, err)
	}
}

func TestRowsPagingFiltersAndLimits(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "P", Recipe: RecipeAssistant})
	var in []RowInput
	for i := 0; i < 250; i++ {
		q := strings.Repeat("w ", i+1)
		if i < 10 {
			q = "apple " + q
		}
		in = append(in, chatInput(q, "a long enough answer"))
	}
	res, err := st.AddRows(ctx, p.ID, in, AddOptions{MaxRows: 50000})
	if err != nil || res.Added != 250 || res.DataRev != 1 {
		t.Fatalf("add: %+v, %v", res, err)
	}
	page, err := st.ListRows(ctx, p.ID, RowQuery{Offset: 200, Limit: 50})
	if err != nil || len(page.Rows) != 50 || page.Total != 250 || page.Rows[0].Seq != 201 {
		t.Fatalf("page: %d rows total %d, %v", len(page.Rows), page.Total, err)
	}
	if page, _ = st.ListRows(ctx, p.ID, RowQuery{RowFilter: RowFilter{Q: "apple"}}); page.Total != 10 {
		t.Errorf("apple total = %d", page.Total)
	}
	if page, _ = st.ListRows(ctx, p.ID, RowQuery{RowFilter: RowFilter{Q: "50%"}}); page.Total != 0 {
		t.Errorf("50%% total = %d", page.Total)
	}
	if page, _ = st.ListRows(ctx, p.ID, RowQuery{Sort: "est_tokens", Dir: "desc", Limit: 1}); page.Rows[0].Seq != 250 {
		t.Errorf("longest seq = %d", page.Rows[0].Seq)
	}
	if page, _ = st.ListRows(ctx, p.ID, RowQuery{Limit: 500}); page.Limit != 200 {
		t.Errorf("limit = %d", page.Limit)
	}
	more := make([]RowInput, 10)
	for i := range more {
		more[i] = chatInput(fmt.Sprintf("extra %d", i), "a long enough answer")
	}
	if _, err := st.AddRows(ctx, p.ID, more, AddOptions{MaxRows: 255}); !errors.Is(err, ErrDatasetFull) {
		t.Fatalf("full: %v", err)
	}
	res, err = st.AddRows(ctx, p.ID, more, AddOptions{MaxRows: 255, Partial: true})
	if err != nil || res.Added != 5 || !res.LimitReached {
		t.Fatalf("partial: %+v, %v", res, err)
	}
	row := page.Rows[0]
	if _, _, err := st.UpdateRow(ctx, p.ID, row.ID, 2, chatInput("changed", "a long enough answer")); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale row rev: %v", err)
	}
	upd, rev, err := st.UpdateRow(ctx, p.ID, row.ID, 1, chatInput("changed", "a long enough answer"))
	if err != nil || upd.Rev != 2 || rev != 3 {
		t.Fatalf("update: rev %d data_rev %d, %v", upd.Rev, rev, err)
	}
	pr, _ := st.GetProject(ctx, p.ID)
	if pr.DataRev != 3 {
		t.Errorf("project data_rev = %d", pr.DataRev)
	}
	txt := "plain"
	res, err = st.AddRows(ctx, p.ID, []RowInput{{Text: &txt}}, AddOptions{MaxRows: 50000})
	if err != nil || !reflect.DeepEqual(res.Rows[0].Flags, []string{"structure"}) {
		t.Fatalf("text row: %+v, %v", res, err)
	}
}
