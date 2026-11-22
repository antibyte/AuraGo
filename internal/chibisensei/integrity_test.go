package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

const longAnswer = "a long enough answer"

func TestUndoRefusesRowsChangedLater(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)

	// A set_split, then an edit of the same row: undoing A would drop the edit.
	p := seedRows(t, st, chatInput("one", longAnswer), chatInput("two", longAnswer))
	rows := allRows(t, st, p.ID)
	a, err := st.Bulk(ctx, p.ID, []int64{rows[0].ID}, "set_split", "eval")
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.UpdateRow(ctx, p.ID, rows[0].ID, 2, chatInput("edited", longAnswer)); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Undo(ctx, p.ID, a.BatchID); !errors.Is(err, ErrUndoConflict) {
		t.Fatalf("undo after edit: %v", err)
	}
	got, _ := st.GetRow(ctx, p.ID, rows[0].ID)
	if got.Split != "eval" || got.Messages[0].Content != "edited" {
		t.Fatalf("row changed by a refused undo: %+v", got)
	}

	// A set_split, then a delete of the row: undoing A would resurrect it.
	b, err := st.Bulk(ctx, p.ID, []int64{rows[1].ID}, "set_split", "eval")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.Bulk(ctx, p.ID, []int64{rows[1].ID}, "delete", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Undo(ctx, p.ID, b.BatchID); !errors.Is(err, ErrUndoConflict) {
		t.Fatalf("undo after delete: %v", err)
	}
	if n, _ := st.CountRows(ctx, p.ID); n != 1 {
		t.Fatalf("rows after refused undo = %d", n)
	}

	// Without a later change the undo still works.
	q := seedRows(t, st, chatInput("three", longAnswer))
	r := allRows(t, st, q.ID)
	c, _ := st.Bulk(ctx, q.ID, []int64{r[0].ID}, "set_split", "eval")
	if u, err := st.Undo(ctx, q.ID, c.BatchID); err != nil || u.Restored != 1 {
		t.Fatalf("clean undo: %+v %v", u, err)
	}
	if got, _ := st.GetRow(ctx, q.ID, r[0].ID); got.Split != "auto" {
		t.Fatalf("split after undo = %s", got.Split)
	}
}

func TestRowIDsAreNotReused(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("one", longAnswer), chatInput("two", longAnswer))
	rows := allRows(t, st, p.ID)
	top := rows[len(rows)-1].ID
	if _, err := st.Bulk(ctx, p.ID, []int64{top}, "delete", ""); err != nil {
		t.Fatal(err)
	}
	res, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("three", longAnswer)}, AddOptions{})
	if err != nil || len(res.Rows) != 1 || res.Rows[0].ID <= top {
		t.Fatalf("new id after delete: %+v %v (deleted %d)", res.Rows, err, top)
	}
}

func TestDeleteSelectedRefusesChangedData(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("one", longAnswer), chatInput("two", longAnswer))
	p, _ = st.GetProject(ctx, p.ID)
	sel := Select{Filter: &RowFilter{}}
	tokens := NewDeleteTokens(nil)
	token, _ := tokens.Issue(p.ID, sel, p.DataRev)
	if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("three", longAnswer)}, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	rev, ok := tokens.Lookup(token, p.ID, sel)
	if !ok {
		t.Fatal("token not found")
	}
	if _, err := st.DeleteSelected(ctx, p.ID, sel, rev); !errors.Is(err, ErrDataChanged) {
		t.Fatalf("delete after a write: %v", err)
	}
	if n, _ := st.CountRows(ctx, p.ID); n != 3 {
		t.Fatalf("rows = %d, nothing may be deleted", n)
	}
}

func TestPruneKeepsBatchOfRunningImport(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("seed", longAnswer))
	opts := AddOptions{BatchID: "import-batch", BatchKind: "import", Source: "file"}
	if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("i1", longAnswer), chatInput("i2", longAnswer)}, opts); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput(fmt.Sprintf("other %d", i), longAnswer)}, AddOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("i3", longAnswer)}, opts); err != nil {
		t.Fatal(err)
	}
	u, err := st.Undo(ctx, p.ID, "import-batch")
	if err != nil || u.Restored != 3 {
		t.Fatalf("undo of the import: %+v %v", u, err)
	}
	if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("late", longAnswer)}, opts); !errors.Is(err, ErrAlreadyUndone) {
		t.Fatalf("rows into an undone batch: %v", err)
	}
}

func TestUpdateProjectRevalidatesOverrides(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant, ProfileID: "gemma3-270m"})
	if err != nil {
		t.Fatal(err)
	}
	full := Overrides{Method: "full"}
	p, err = st.UpdateProject(ctx, p.ID, p.Rev, ProjectPatch{Overrides: &full})
	if err != nil {
		t.Fatal(err)
	}
	qwen := "qwen35-0p8b"
	_, err = st.UpdateProject(ctx, p.ID, p.Rev, ProjectPatch{ProfileID: &qwen})
	var ve *ValidationError
	if !errors.As(err, &ve) || ve.Issues[0].Field != "overrides.method" {
		t.Fatalf("switch to a LoRA-only profile: %v", err)
	}
}

func TestExportFailsWhenRowsChange(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("one", longAnswer), chatInput("two", longAnswer))
	ws := newFakeWS()
	changed := false
	_, err := RunExport(ctx, st, ws, p.ID, ExportParams{WorkspacePath: "Exports/a.jsonl", Which: "all"}, func(Progress) {
		if changed {
			return
		}
		changed = true
		if _, err := st.AddRows(ctx, p.ID, []RowInput{chatInput("late", longAnswer)}, AddOptions{}); err != nil {
			t.Error(err)
		}
	})
	var je *JobError
	if !errors.As(err, &je) || je.Code != "data_changed" {
		t.Fatalf("export with a concurrent write: %v", err)
	}
	if _, ok := ws.files["Exports/a.jsonl"]; ok {
		t.Fatal("a file was published although the rows changed")
	}
}

func TestLabelBalanceArgsAreBounded(t *testing.T) {
	cls := Project{ProjectSummary: ProjectSummary{Recipe: RecipeSpecialist, TaskKind: "classify"}, Labels: []string{"a", "b"}}
	rows := make([]CheckRow, 500)
	for i := range rows {
		rows[i] = chatCheckRow(int64(i+1), fmt.Sprintf("question %d", i), fmt.Sprintf("answer-%d", i), 20)
	}
	res, _ := runCheck(t, CheckInput{Project: cls, Rows: rows})
	c := findCheck(res, "label_balance")
	labels, _ := c.Args["labels"].(map[string]int)
	if c == nil || len(labels) > 2+maxOtherLabels || c.Args["other_labels"] != 500-maxOtherLabels {
		t.Fatalf("label_balance args: %d labels, other_labels %v", len(labels), c.Args["other_labels"])
	}
}

func TestRunnerCloseIsBounded(t *testing.T) {
	st := newTestStore(t)
	p, _ := st.CreateProject(context.Background(), ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	old := cancelWait
	cancelWait = 100 * time.Millisecond
	defer func() { cancelWait = old }()
	r := NewRunner(st, nil)
	stop := make(chan struct{})
	defer close(stop)
	if _, err := r.Start(context.Background(), nil, p.ID, "export", nil, 0, func(context.Context, func(Progress)) (JobResult, error) {
		<-stop // ignores its context
		return JobResult{}, nil
	}); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { r.Close(); close(done) }()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not return")
	}
}

func TestDeleteProjectRemovesRunData(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p := seedRows(t, st, chatInput("one", longAnswer))
	for _, q := range []string{
		`INSERT INTO runs(id,project_id) VALUES('r1','` + p.ID + `')`,
		`INSERT INTO run_events(run_id) VALUES('r1')`,
		`INSERT INTO metrics(run_id,step,kind) VALUES('r1',1,'train')`,
		`INSERT INTO artifacts(id,run_id,project_id) VALUES('a1','r1','` + p.ID + `')`,
		`INSERT INTO eval_reports(run_id,kind) VALUES('r1','x')`,
	} {
		if _, err := st.db.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.SetArchived(ctx, p.ID, true); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteProject(ctx, p.ID, "P"); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"runs", "run_events", "metrics", "artifacts", "eval_reports", "dataset_rows", "row_history", "batches"} {
		var n int
		if err := st.db.QueryRow(`SELECT COUNT(*) FROM ` + table).Scan(&n); err != nil || n != 0 {
			t.Errorf("%s keeps %d rows (%v)", table, n, err)
		}
	}
}
