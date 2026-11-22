package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestExportLinesAndSplit(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	ws := newFakeWS()
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Free", Recipe: RecipeFree})
	if err != nil {
		t.Fatal(err)
	}
	x := "x"
	in := []RowInput{{Messages: []Message{{"user", "<b>"}, {"assistant", "ok"}}}, {Text: &x}}
	if _, err := st.AddRows(ctx, p.ID, in, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err := RunExport(ctx, st, ws, p.ID, ExportParams{WorkspacePath: "Exports/a.jsonl", Which: "all"}, func(Progress) {})
	if err != nil || res.Rows != 2 || res.Path != "Exports/a.jsonl" {
		t.Fatalf("export all: %+v %v", res, err)
	}
	want := `{"messages":[{"role":"user","content":"<b>"},{"role":"assistant","content":"ok"}]}` + "\n" + `{"text":"x"}` + "\n"
	if got := string(ws.files["Exports/a.jsonl"]); got != want {
		t.Fatalf("lines:\n%s\nwant:\n%s", got, want)
	}
	_, err = RunExport(ctx, st, ws, p.ID, ExportParams{WorkspacePath: "Exports/a.jsonl", Which: "all"}, func(Progress) {})
	var je *JobError
	if !errors.As(err, &je) || je.Code != "file_exists" {
		t.Fatalf("existing file: %v", err)
	}
	var ve *ValidationError
	if _, err = RunExport(ctx, st, ws, p.ID, ExportParams{WorkspacePath: "Exports/a.txt", Which: "all"}, func(Progress) {}); !errors.As(err, &ve) {
		t.Fatalf("extension: %v", err)
	}

	big, _ := st.CreateProject(ctx, ProjectInput{Name: "Big", Recipe: RecipeAssistant})
	rows := make([]RowInput, 200)
	for i := range rows {
		rows[i] = chatInput(fmt.Sprintf("question %d", i), fmt.Sprintf("a long enough answer %d", i))
	}
	if _, err := st.AddRows(ctx, big.ID, rows, AddOptions{}); err != nil {
		t.Fatal(err)
	}
	res, err = RunExport(ctx, st, ws, big.ID, ExportParams{WorkspacePath: "Exports/eval.jsonl", Which: "eval"}, func(Progress) {})
	if err != nil || res.Rows != 10 || strings.Count(string(ws.files["Exports/eval.jsonl"]), "\n") != 10 {
		t.Fatalf("eval: %+v %v", res, err)
	}
}
