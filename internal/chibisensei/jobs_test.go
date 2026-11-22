package chibisensei

import (
	"context"
	"errors"
	"testing"
	"time"
)

func waitJob(t *testing.T, st *Store, id string) Job {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		j, err := st.GetJob(context.Background(), id)
		if err != nil {
			t.Fatal(err)
		}
		if j.terminal() {
			return j
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("job did not finish")
	return Job{}
}

func TestRunnerLifecycleAndHub(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	p, err := st.CreateProject(ctx, ProjectInput{Name: "Bot", Recipe: RecipeAssistant})
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub()
	events, cancelSub := hub.Subscribe(p.ID)
	defer cancelSub()
	r := NewRunner(st, hub)
	defer r.Close()

	released := make(chan struct{}, 4)
	release := func() { released <- struct{}{} }
	block := make(chan struct{})
	first, err := r.Start(ctx, release, p.ID, "import_file", map[string]string{"a": "b"}, 10, func(ctx context.Context, report func(Progress)) (JobResult, error) {
		report(Progress{Done: 1, Total: 10, Phase: "importing"})
		select {
		case <-block:
		case <-ctx.Done():
		}
		return JobResult{RowsAdded: 1}, ctx.Err()
	})
	if err != nil || first.State != JobRunning {
		t.Fatalf("start: %+v %v", first, err)
	}
	if _, err := r.Start(ctx, nil, p.ID, "import_file", nil, 0, nil); !errors.Is(err, ErrJobActive) {
		t.Fatalf("second import: %v", err)
	}
	ok, err := r.Start(ctx, nil, p.ID, "export", nil, 0, func(context.Context, func(Progress)) (JobResult, error) {
		return JobResult{RowsAdded: 3}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, st, ok.ID); j.State != JobSucceeded || j.Result.RowsAdded != 3 {
		t.Fatalf("export: %+v", j)
	}
	j, err := r.Cancel(ctx, first.ID)
	if err != nil || j.State != JobCancelled || j.Result.RowsAdded != 1 {
		t.Fatalf("cancel: %+v %v", j, err)
	}
	select {
	case <-released:
	case <-time.After(time.Second):
		t.Fatal("release not called")
	}
	if again, err := r.Cancel(ctx, first.ID); err != nil || again.State != JobCancelled {
		t.Fatalf("cancel terminal: %+v %v", again, err)
	}
	if _, err := r.Cancel(ctx, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cancel unknown: %v", err)
	}
	bad, err := r.Start(ctx, nil, p.ID, "clean", nil, 0, func(context.Context, func(Progress)) (JobResult, error) {
		return JobResult{}, &JobError{"x", "y"}
	})
	if err != nil {
		t.Fatal(err)
	}
	if j := waitJob(t, st, bad.ID); j.State != JobFailed || j.ErrorCode != "x" || j.Error != "y" {
		t.Fatalf("failed: %+v", j)
	}
	boom, _ := r.Start(ctx, nil, p.ID, "generate", nil, 0, func(context.Context, func(Progress)) (JobResult, error) {
		return JobResult{}, errors.New("secret cause")
	})
	if j := waitJob(t, st, boom.ID); j.ErrorCode != "internal" || j.Error != "The job failed." {
		t.Fatalf("internal: %+v", j)
	}

	seen := 0
	for drained := false; !drained; {
		select {
		case e := <-events:
			if e.Name == "job" {
				seen++
			}
		default:
			drained = true
		}
	}
	if seen < 4 {
		t.Fatalf("job events = %d", seen)
	}
	if list, err := st.ListJobs(ctx, p.ID, false); err != nil || len(list) != 4 {
		t.Fatalf("list: %d %v", len(list), err)
	}
	if list, _ := st.ListJobs(ctx, p.ID, true); len(list) != 0 {
		t.Fatalf("active list: %d", len(list))
	}

	if _, err := st.db.Exec(`INSERT INTO jobs(id,project_id,kind,state,created,updated) VALUES('stale',?,'export','running','t','t')`, p.ID); err != nil {
		t.Fatal(err)
	}
	if err := r.Recover(ctx); err != nil {
		t.Fatal(err)
	}
	if j, _ := st.GetJob(ctx, "stale"); j.State != JobFailed || j.ErrorCode != "interrupted" {
		t.Fatalf("recover: %+v", j)
	}

	full, cancelFull := hub.Subscribe("other")
	defer cancelFull()
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			hub.Publish("other", Event{Name: "rows", Data: i})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Publish blocked on a full subscriber")
	}
	if len(full) != 32 {
		t.Fatalf("buffered = %d", len(full))
	}
}
