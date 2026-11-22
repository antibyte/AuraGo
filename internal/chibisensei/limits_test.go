package chibisensei

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestRunnerTimeoutFailsAndCancelStaysCancelled(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	r := NewRunner(st, nil)
	defer r.Close()
	waitCtx := func(ctx context.Context, _ func(Progress)) (JobResult, error) {
		<-ctx.Done()
		return JobResult{RowsAdded: 2}, ctx.Err()
	}

	limited, cancelLimit := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancelLimit()
	j, err := r.Start(limited, nil, p.ID, "export", nil, 0, waitCtx)
	if err != nil {
		t.Fatal(err)
	}
	if got := waitJob(t, st, j.ID); got.State != JobFailed || got.ErrorCode != "timeout" || got.Result.RowsAdded != 2 {
		t.Fatalf("timeout: %+v", got)
	}

	j, err = r.Start(ctx, nil, p.ID, "export", nil, 0, waitCtx)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := r.Cancel(ctx, j.ID); err != nil || got.State != JobCancelled || got.ErrorCode != "" {
		t.Fatalf("cancel: %+v %v", got, err)
	}
}

func TestHubRefusesTheSeventeenthSubscriber(t *testing.T) {
	hub := NewHub()
	var cancels []func()
	for i := 0; i < maxSubsPerProject; i++ {
		_, cancel, err := hub.Subscribe("p1")
		if err != nil {
			t.Fatalf("subscriber %d: %v", i+1, err)
		}
		cancels = append(cancels, cancel)
	}
	if _, _, err := hub.Subscribe("p1"); !errors.Is(err, ErrTooManyStreams) {
		t.Fatalf("17th subscriber: %v", err)
	}
	if _, cancel, err := hub.Subscribe("p2"); err != nil { // another project has its own 16
		t.Fatalf("other project: %v", err)
	} else {
		cancel()
	}
	cancels[0]()
	if _, cancel, err := hub.Subscribe("p1"); err != nil { // a slot freed by a leaving stream
		t.Fatalf("after a leave: %v", err)
	} else {
		cancel()
	}
	for _, c := range cancels {
		c()
	}
}

func TestRunnerQueuesThirdModelJobAndStorageRecheckStopsImport(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p1, _ := st.CreateProject(ctx, ProjectInput{Name: "One", Recipe: RecipeAssistant})
	p2, _ := st.CreateProject(ctx, ProjectInput{Name: "Two", Recipe: RecipeAssistant})
	r := NewRunner(st, nil)
	defer r.Close()

	started := make(chan string, 4)
	hold := func(name string) JobFunc {
		return func(ctx context.Context, _ func(Progress)) (JobResult, error) {
			if ctx.Err() != nil { // a job cancelled while queued is entered with a done context and must do no work
				return JobResult{}, ctx.Err()
			}
			started <- name
			<-ctx.Done()
			return JobResult{}, ctx.Err()
		}
	}
	a, err := r.Start(ctx, nil, p1.ID, "generate", nil, 0, hold("a"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.Start(ctx, nil, p1.ID, "clean", nil, 0, hold("b")); err != nil {
		t.Fatal(err)
	}
	c, err := r.Start(ctx, nil, p2.ID, "generate", nil, 0, hold("c"))
	if err != nil {
		t.Fatal(err)
	}
	if c.State != JobQueued {
		t.Fatalf("third job state = %s", c.State)
	}
	for i := 0; i < 2; i++ {
		select {
		case <-started:
		case <-time.After(5 * time.Second):
			t.Fatal("the first two jobs did not start")
		}
	}
	if got, _ := st.GetJob(ctx, c.ID); got.State != JobQueued {
		t.Fatalf("stored state = %s", got.State)
	}
	if got, err := r.Cancel(ctx, c.ID); err != nil || got.State != JobCancelled {
		t.Fatalf("cancel queued: %+v %v", got, err)
	}
	select {
	case name := <-started:
		t.Fatalf("a cancelled queued job ran (%s)", name)
	default:
	}
	// A queued job turns running when a slot frees.
	d, err := r.Start(ctx, nil, p2.ID, "generate", nil, 0, hold("d"))
	if err != nil || d.State != JobQueued {
		t.Fatalf("fourth job: %+v %v", d, err)
	}
	if _, err := r.Cancel(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case name := <-started:
		if name != "d" {
			t.Fatalf("started %s", name)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the queued job did not start")
	}
	if got, _ := st.GetJob(ctx, d.ID); got.State != JobRunning {
		t.Fatalf("state after the slot freed = %s", got.State)
	}

	// The storage limit is checked again before each chunk of rows; the job keeps what it added.
	var sb strings.Builder
	for i := 0; i < 600; i++ {
		fmt.Fprintf(&sb, `{"prompt":"question %d","completion":"a sufficiently long answer %d"}`+"\n", i, i)
	}
	ws := newFakeWS()
	ws.files["Imports/big.jsonl"] = []byte(sb.String())
	p3, _ := st.CreateProject(ctx, ProjectInput{Name: "Three", Recipe: RecipeAssistant})
	checks := 0
	res, err := RunImport(ctx, st, ws, p3.ID, ImportParams{WorkspacePath: "Imports/big.jsonl", Format: "auto",
		Full: func() bool { checks++; return checks > 1 }}, 1<<24, 1000, 1000, func(Progress) {})
	var je *JobError
	if !errors.As(err, &je) || je.Code != "storage_full" || res.RowsAdded != importChunk {
		t.Fatalf("import: %+v %v", res, err)
	}
	if n, _ := st.CountRows(ctx, p3.ID); n != importChunk {
		t.Fatalf("rows kept = %d", n)
	}
}

func TestImportSkipsOverlongLines(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	ws := newFakeWS()
	huge := strings.Repeat("x", 2<<20)
	ws.files["Imports/d.jsonl"] = []byte(`{"prompt":"question 1","completion":"a sufficiently long answer 1"}` + "\n" +
		`{"prompt":"` + huge + `","completion":"a sufficiently long answer"}` + "\n" +
		`{"prompt":"question 3","completion":"a sufficiently long answer 3"}` + "\n")
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	res, err := RunImport(ctx, st, ws, p.ID, ImportParams{WorkspacePath: "Imports/d.jsonl", Format: "auto"}, 1<<24, 100, 100, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsAdded != 2 || len(res.Issues) != 1 || res.Issues[0].MessageKey != "chibi_sensei.import.line_too_long" || res.Issues[0].Line != 2 {
		t.Fatalf("jsonl: %+v", res)
	}

	// A CSV record over the cap is skipped as well and does not swallow the records after it.
	csvData := "question,answer\nq1,a sufficiently long answer 1\n" + `"` + huge + `",x` + "\n" + "q3,a sufficiently long answer 3\n"
	ws.files["Imports/d.csv"] = []byte(csvData)
	p2, _ := st.CreateProject(ctx, ProjectInput{Name: "Csv", Recipe: RecipeAssistant})
	res, err = RunImport(ctx, st, ws, p2.ID, ImportParams{WorkspacePath: "Imports/d.csv", Format: "auto"}, 1<<24, 100, 100, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsAdded != 2 || len(res.Issues) != 1 || res.Issues[0].MessageKey != "chibi_sensei.import.line_too_long" || res.Issues[0].Line != 3 {
		t.Fatalf("csv: %+v", res)
	}
}

func TestImportCSVStrictQuotingSkipsBadRecords(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	ws := newFakeWS()
	huge := strings.Repeat("x", 2<<20)
	ws.files["Imports/q.csv"] = []byte("question,answer\n" +
		"q1,a sufficiently long answer 1\n" + // line 2
		`5" screen,a sufficiently long answer bare` + "\n" + // line 3: a bare quote
		"q2,a sufficiently long answer 2\n" + // line 4
		`"` + huge + `",x` + "\n" + // line 5: over 1 MiB
		"\"q3\nsecond line\",a sufficiently long answer 3\n" + // lines 6-7
		"q4,a sufficiently long answer 4\n") // line 8
	p, _ := st.CreateProject(ctx, ProjectInput{Name: "Csv", Recipe: RecipeAssistant})
	res, err := RunImport(ctx, st, ws, p.ID, ImportParams{WorkspacePath: "Imports/q.csv", Format: "auto"}, 1<<24, 100, 100, func(Progress) {})
	if err != nil {
		t.Fatal(err)
	}
	if res.RowsAdded != 4 || res.RowsSkipped != 2 || len(res.Issues) != 2 {
		t.Fatalf("result: %+v", res)
	}
	if is := res.Issues[0]; is.MessageKey != "chibi_sensei.import.bad_csv" || is.Line != 3 {
		t.Fatalf("bare quote issue: %+v", is)
	}
	if is := res.Issues[1]; is.MessageKey != "chibi_sensei.import.line_too_long" || is.Line != 5 {
		t.Fatalf("long record issue: %+v", is)
	}
	var got []string
	for _, r := range allRows(t, st, p.ID) {
		got = append(got, r.Messages[0].Content+"|"+r.Messages[1].Content)
	}
	want := []string{"q1|a sufficiently long answer 1", "q2|a sufficiently long answer 2",
		"q3\nsecond line|a sufficiently long answer 3", "q4|a sufficiently long answer 4"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("rows = %q, want %q", got, want)
	}
}
