package chibisensei

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"unicode/utf8"
)

// fakeWS is the workspace of the tests: a map from path to content.
type fakeWS struct{ files map[string][]byte }

func newFakeWS() *fakeWS { return &fakeWS{files: map[string][]byte{}} }

func (w *fakeWS) ReadFile(ctx context.Context, path string, maxBytes int64) ([]byte, error) {
	b, ok := w.files[path]
	if !ok {
		return nil, os.ErrNotExist
	}
	if int64(len(b)) > maxBytes {
		return nil, ErrFileTooLarge
	}
	return b, nil
}

func (w *fakeWS) WriteFile(ctx context.Context, path string, r io.Reader, maxBytes int64, overwrite bool) error {
	if _, ok := w.files[path]; ok && !overwrite {
		return ErrFileExists
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	w.files[path] = b
	return nil
}

func parseAll(t *testing.T, data, format, recipe string) ([]ImportRow, ImportStats) {
	t.Helper()
	var rows []ImportRow
	stats, _, _, err := ParseImport([]byte(data), format, Mapping{}, recipe, 0, func(r ImportRow) bool {
		rows = append(rows, r)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	return rows, stats
}

func msgs(t *testing.T, r ImportRow) string {
	t.Helper()
	var sb strings.Builder
	for _, m := range r.Input.Messages {
		sb.WriteString(m.Role[:1] + ":" + m.Content + "|")
	}
	return sb.String()
}

func TestJSONLShapesMap(t *testing.T) {
	cases := []struct{ line, want string }{
		{`{"messages":[{"role":"user","content":"q"},{"role":"assistant","content":"a"}]}`, "u:q|a:a|"},
		{`{"conversations":[{"from":"human","value":"q"},{"from":"gpt","value":"a"}]}`, "u:q|a:a|"},
		{`{"instruction":"Do","input":"x","output":"y"}`, "u:Do\n\nx|a:y|"},
		{`{"prompt":"p","completion":"c"}`, "u:p|a:c|"},
		{`{"text":"t","label":1}`, "u:t|a:1|"},
	}
	for _, c := range cases {
		rows, stats := parseAll(t, c.line+"\n", "jsonl", RecipeAssistant)
		if len(rows) != 1 || msgs(t, rows[0]) != c.want {
			t.Fatalf("%s -> %v %+v", c.line, rows, stats)
		}
	}
	rows, _ := parseAll(t, `{"text":"t"}`, "jsonl", RecipeFree)
	if len(rows) != 1 || rows[0].Input.Text == nil || *rows[0].Input.Text != "t" {
		t.Fatalf("free text: %+v", rows)
	}
	_, stats := parseAll(t, `{"text":"t"}`, "jsonl", RecipeAssistant)
	if stats.Valid != 0 || len(stats.Issues) != 1 || stats.Issues[0].MessageKey != "chibi_sensei.import.text_needs_free" {
		t.Fatalf("assistant text: %+v", stats)
	}
	var got []ImportRow
	_, _, _, err := ParseImport([]byte(`{"prompt":"p","completion":"c"}`), "jsonl", Mapping{SystemText: "Be brief"}, RecipeAssistant, 0,
		func(r ImportRow) bool { got = append(got, r); return true })
	if err != nil || len(got) != 1 || msgs(t, got[0]) != "s:Be brief|u:p|a:c|" {
		t.Fatalf("system text: %v %v", got, err)
	}
	_, stats = parseAll(t, "{bad\n", "jsonl", RecipeAssistant)
	if len(stats.Issues) != 1 || stats.Issues[0].MessageKey != "chibi_sensei.import.bad_json" || stats.Issues[0].Line != 1 {
		t.Fatalf("bad json: %+v", stats)
	}
	_, stats = parseAll(t, `{"messages":[{"role":"tool","content":"x"}]}`, "jsonl", RecipeAssistant)
	if len(stats.Issues) != 1 || stats.Issues[0].MessageKey != "chibi_sensei.import.bad_role" {
		t.Fatalf("bad role: %+v", stats)
	}
}

func TestCSVAutoMapAndIssues(t *testing.T) {
	data := "\xef\xbb\xbfQuestion;Answer\nq1;a1\nq2;\n"
	var rows []ImportRow
	stats, m, cols, err := ParseImport([]byte(data), "csv", Mapping{}, RecipeAssistant, 0, func(r ImportRow) bool {
		rows = append(rows, r)
		return true
	})
	if err != nil {
		t.Fatal(err)
	}
	if m.Shape != "columns" || m.User != "question" || m.Assistant != "answer" || len(cols) != 2 {
		t.Fatalf("mapping %+v cols %v", m, cols)
	}
	if len(rows) != 1 || rows[0].Line != 2 || msgs(t, rows[0]) != "u:q1|a:a1|" {
		t.Fatalf("rows %+v", rows)
	}
	if len(stats.Issues) != 1 || stats.Issues[0].MessageKey != "chibi_sensei.import.invalid_row" || stats.Issues[0].Line != 3 {
		t.Fatalf("issues %+v", stats.Issues)
	}
	if got := AutoMapCSV([]string{"Text", "Label"}); got.Shape != "text_label" || got.Text != "text" || got.Label != "label" {
		t.Fatalf("text,label -> %+v", got)
	}
	pv, err := PreviewImport([]byte(data), "csv", nil, RecipeAssistant)
	if err != nil || len(pv.Sample) != 1 || pv.Mapping.Shape != "columns" {
		t.Fatalf("preview %+v %v", pv, err)
	}
}

func TestSplitParagraphs(t *testing.T) {
	got := SplitParagraphs("a\n\n b \r\n\r\nc", 2000)
	if fmt.Sprint(got) != "[a b c]" {
		t.Fatalf("got %q", got)
	}
	long := strings.TrimSpace(strings.Repeat("wort ", 900)) // 4499 runes
	parts := SplitParagraphs(long, 2000)
	if len(parts) != 3 {
		t.Fatalf("parts = %d", len(parts))
	}
	for _, p := range parts {
		if utf8.RuneCountInString(p) > 2000 {
			t.Fatalf("part of %d runes", utf8.RuneCountInString(p))
		}
	}
}

func TestRunImportStopsAtLimits(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	ws := newFakeWS()
	var sb strings.Builder
	for i := 0; i < 30; i++ {
		fmt.Fprintf(&sb, `{"prompt":"question %d","completion":"a sufficiently long answer %d"}`+"\n", i, i)
	}
	ws.files["Imports/d.jsonl"] = []byte(sb.String())
	run := func(importMax, datasetMax int) JobResult {
		p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
		if err != nil {
			t.Fatal(err)
		}
		res, err := RunImport(ctx, st, ws, p.ID, ImportParams{WorkspacePath: "Imports/d.jsonl", Format: "auto"}, 1<<20, importMax, datasetMax, func(Progress) {})
		if err != nil {
			t.Fatal(err)
		}
		if n, _ := st.CountRows(ctx, p.ID); n != res.RowsAdded || res.BatchID == "" {
			t.Fatalf("rows %d result %+v", n, res)
		}
		return res
	}
	if res := run(20, 1000); res.RowsAdded != 20 {
		t.Fatalf("import limit: %+v", res)
	}
	if res := run(1000, 10); res.RowsAdded != 10 {
		t.Fatalf("dataset limit: %+v", res)
	}
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "Big", Recipe: RecipeAssistant})
	_, err := RunImport(ctx, st, ws, p.ID, ImportParams{WorkspacePath: "Imports/d.jsonl"}, 10, 100, 100, func(Progress) {})
	if je, ok := err.(*JobError); !ok || je.Code != "file_too_large" {
		t.Fatalf("too large: %v", err)
	}
	if !bytes.Contains(ws.files["Imports/d.jsonl"], []byte("question 29")) {
		t.Fatal("fixture")
	}
}
