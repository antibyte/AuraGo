package render

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	projA = "11111111-1111-4111-8111-111111111111"
	projB = "22222222-2222-4222-8222-222222222222"
)

func newTestManager(t *testing.T, dir string, run RunFunc) *Manager {
	t.Helper()
	m, err := NewManager(ManagerOptions{Dir: dir, Run: run})
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	t.Cleanup(m.Close)
	return m
}

func waitJob(t *testing.T, m *Manager, id string, pred func(Job) bool) Job {
	t.Helper()
	return waitJobFor(t, m, id, pred, 15*time.Second)
}

func waitJobFor(t *testing.T, m *Manager, id string, pred func(Job) bool, limit time.Duration) Job {
	t.Helper()
	deadline := time.Now().Add(limit)
	for time.Now().Before(deadline) {
		if j, ok := m.Get(id); ok && pred(j) {
			return j
		}
		time.Sleep(5 * time.Millisecond)
	}
	j, _ := m.Get(id)
	t.Fatalf("job %s did not reach the expected state, last: %+v", id, j)
	return Job{}
}

func terminal(j Job) bool { return j.Terminal() }

func TestJobLifecycleSuccessAndPersistence(t *testing.T) {
	dir := t.TempDir()
	m := newTestManager(t, dir, func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		h.SetPrefixHash(HashPrefix("secret-prefix-123"))
		h.SetPhase(PhaseRendering)
		h.SetFrames(5, 10)
		return Output{Path: "Documents/Storymaker/p/exports/x.mp4", Size: 42}, nil
	})
	sub, unsub := m.Subscribe(projA)
	defer unsub()
	job, replayed, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if err != nil || replayed {
		t.Fatalf("Enqueue = %v, replayed %v", err, replayed)
	}
	if job.Status != StatusQueued || job.Kind != KindRender || !validJobID(job.ID) {
		t.Fatalf("queued job = %+v", job)
	}
	done := waitJob(t, m, job.ID, terminal)
	if done.Status != StatusSucceeded || done.Progress != 1 || done.Output == nil || done.Output.Size != 42 || done.Error != nil {
		t.Fatalf("finished job = %+v", done)
	}
	select {
	case <-sub:
	default:
		t.Fatal("the subscriber was never signalled")
	}
	changed, rev := m.ChangedSince(projA, 0)
	if len(changed) != 1 || changed[0].ID != job.ID || rev == 0 {
		t.Fatalf("ChangedSince = %+v, %d", changed, rev)
	}
	if again, _ := m.ChangedSince(projA, rev); len(again) != 0 {
		t.Fatalf("a second ChangedSince returned %+v", again)
	}
	if other, _ := m.ChangedSince(projB, 0); len(other) != 0 {
		t.Fatalf("another project saw %+v", other)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "jobs.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-prefix-123") {
		t.Fatal("jobs.json holds the render prefix")
	}
	if !strings.Contains(string(raw), HashPrefix("secret-prefix-123")) {
		t.Fatal("jobs.json does not hold the prefix hash")
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("jobs.json is not JSON: %v", err)
	}

	m.Close()
	m2 := newTestManager(t, dir, func(context.Context, Job, any, *Handle) (Output, error) { return Output{}, nil })
	reloaded, ok := m2.Get(job.ID)
	if !ok || reloaded.Status != StatusSucceeded || reloaded.Output == nil || reloaded.Output.Path != done.Output.Path {
		t.Fatalf("reloaded job = %+v, %v", reloaded, ok)
	}
	if m2.PrefixHash(job.ID) != HashPrefix("secret-prefix-123") {
		t.Fatal("the prefix hash did not survive a restart")
	}
}

func TestJobFailureCodes(t *testing.T) {
	var n atomic.Int32
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		switch n.Add(1) {
		case 1:
			return Output{}, Fail(CodeCodeError, "scene s3 is broken")
		case 2:
			return Output{}, errors.New("disk exploded")
		default:
			panic("boom")
		}
	})
	want := []string{CodeCodeError, CodeRenderFailed, CodeRenderFailed}
	for i := range want {
		job, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
		if err != nil {
			t.Fatal(err)
		}
		done := waitJob(t, m, job.ID, terminal)
		if done.Status != StatusFailed || done.Error == nil || done.Error.Code != want[i] {
			t.Fatalf("job %d = %+v", i, done)
		}
	}
}

func TestJobsRunOneAtATimeAndTheQueueHasACap(t *testing.T) {
	release := make(chan struct{})
	var running, peak atomic.Int32
	started := make(chan string, 64)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		cur := running.Add(1)
		for {
			p := peak.Load()
			if cur <= p || peak.CompareAndSwap(p, cur) {
				break
			}
		}
		started <- job.ID
		select {
		case <-release:
		case <-ctx.Done():
		}
		running.Add(-1)
		return Output{}, nil
	})
	first, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if err != nil {
		t.Fatal(err)
	}
	<-started // the first job runs and blocks
	var queued []string
	for i := 0; i < DefaultMaxQueued; i++ {
		j, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA})
		if err != nil {
			t.Fatalf("queued job %d: %v", i, err)
		}
		queued = append(queued, j.ID)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA}); !errors.Is(err, ErrBusy) {
		t.Fatalf("job 17 = %v, want ErrBusy", err)
	}
	// Cancelling a queued job frees a place.
	if j, ok := m.Cancel(queued[0]); !ok || j.Status != StatusCancelled {
		t.Fatalf("Cancel(queued) = %+v, %v", j, ok)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projB}); err != nil {
		t.Fatalf("enqueue after a cancel: %v", err)
	}
	close(release)
	waitJob(t, m, first.ID, terminal)
	for _, id := range queued[1:] {
		waitJob(t, m, id, terminal)
	}
	if peak.Load() != 1 {
		t.Fatalf("%d jobs ran at once", peak.Load())
	}
}

func TestCancelRunningJob(t *testing.T) {
	started := make(chan struct{})
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		close(started)
		<-ctx.Done()
		return Output{}, ctx.Err()
	})
	job, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	if j, ok := m.Cancel(job.ID); !ok || j.Status != StatusRunning {
		t.Fatalf("Cancel(running) = %+v, %v", j, ok)
	}
	done := waitJob(t, m, job.ID, terminal)
	if done.Status != StatusCancelled || done.Error == nil || done.Error.Code != CodeCancelled {
		t.Fatalf("cancelled job = %+v", done)
	}
	// Cancelling again changes nothing.
	if j, ok := m.Cancel(job.ID); !ok || j.Status != StatusCancelled {
		t.Fatalf("second Cancel = %+v", j)
	}
	if _, ok := m.Cancel("00000000-0000-4000-8000-000000000000"); ok {
		t.Fatal("Cancel of an unknown job succeeded")
	}
}

func TestRestartMarksUnfinishedJobsInterruptedAndClearsStaging(t *testing.T) {
	dir := t.TempDir()
	staging := filepath.Join(StagingRoot(dir), "leftover")
	if err := os.MkdirAll(staging, 0o700); err != nil {
		t.Fatal(err)
	}
	// Write a jobs.json by hand, as a crashed server would have left it.
	now := time.Now().UTC().Format(time.RFC3339Nano)
	doc := `{"version":1,"jobs":[` +
		`{"id":"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa","project_id":"` + projA + `","kind":"render","status":"running","progress":0.4,"phase":"rendering","frames_done":4,"frames_total":10,"created_at":"` + now + `","updated_at":"` + now + `","rev":3},` +
		`{"id":"bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb","project_id":"` + projA + `","kind":"render","status":"queued","progress":0,"frames_done":0,"frames_total":0,"created_at":"` + now + `","updated_at":"` + now + `","rev":4},` +
		`{"id":"not-a-uuid","project_id":"` + projA + `","kind":"render","status":"queued","created_at":"` + now + `","updated_at":"` + now + `"}]}`
	if err := os.WriteFile(filepath.Join(dir, "jobs.json"), []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	m := newTestManager(t, dir, func(context.Context, Job, any, *Handle) (Output, error) { return Output{}, nil })
	for _, id := range []string{"aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa", "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"} {
		j, ok := m.Get(id)
		if !ok || j.Status != StatusInterrupted || j.Error == nil || j.Error.Code != CodeInterrupted || j.FinishedAt == nil {
			t.Fatalf("job %s = %+v, %v", id, j, ok)
		}
	}
	if _, ok := m.Get("not-a-uuid"); ok {
		t.Fatal("a job with an invalid id was loaded")
	}
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Fatalf("the staging folder survived the restart: %v", err)
	}
	// New revisions continue after the loaded ones.
	j, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	if changed, _ := m.ChangedSince(projA, 4); len(changed) == 0 || changed[len(changed)-1].ID != j.ID {
		t.Fatalf("revisions did not continue: %+v", changed)
	}
}

func TestCloseInterruptsTheRunningJob(t *testing.T) {
	started := make(chan struct{})
	m, err := NewManager(ManagerOptions{Dir: t.TempDir(), Run: func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		close(started)
		<-ctx.Done()
		return Output{}, ctx.Err()
	}})
	if err != nil {
		t.Fatal(err)
	}
	running, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	waiting, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	m.Close()
	for _, id := range []string{running.ID, waiting.ID} {
		if j, _ := m.Get(id); j.Status != StatusInterrupted {
			t.Fatalf("job %s after Close = %+v", id, j)
		}
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Enqueue after Close = %v", err)
	}
}

func TestIdempotencyKey(t *testing.T) {
	gate := make(chan struct{})
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		<-gate
		return Output{}, nil
	})
	defer close(gate)
	a, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || replay {
		t.Fatalf("first = %v %v", err, replay)
	}
	b, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || !replay || b.ID != a.ID {
		t.Fatalf("replay = %+v %v %v", b, replay, err)
	}
	if _, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Key: "k1", Fingerprint: "story-v2"}); !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("same key, other story = %v", err)
	}
	// The same key in another project is a different request.
	c, replay, err := m.Enqueue(EnqueueRequest{ProjectID: projB, Key: "k1", Fingerprint: "story-v1"})
	if err != nil || replay || c.ID == a.ID {
		t.Fatalf("other project = %+v %v %v", c, replay, err)
	}
}

func TestCancelProjectCancelsRunningAndQueued(t *testing.T) {
	started := make(chan struct{}, 4)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		started <- struct{}{}
		<-ctx.Done()
		return Output{}, ctx.Err()
	})
	a, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	<-started
	b, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	c, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projB})
	m.CancelProject(projA)
	for _, id := range []string{a.ID, b.ID} {
		if j := waitJob(t, m, id, terminal); j.Status != StatusCancelled {
			t.Fatalf("job %s = %+v", id, j)
		}
	}
	// The other project's job gets its turn and keeps running.
	<-started
	if j, _ := m.Get(c.ID); j.Status != StatusRunning {
		t.Fatalf("other project's job = %+v", j)
	}
}

func TestProgressUpdatesAreThrottledButTheFinalStateIsAlwaysAnnounced(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, _ any, h *Handle) (Output, error) {
		defer wg.Done()
		for i := 1; i <= 1000; i++ {
			h.SetFrames(i, 1000)
		}
		return Output{Path: "p", Size: 1}, nil
	})
	sub, unsub := m.Subscribe("")
	defer unsub()
	job, _, _ := m.Enqueue(EnqueueRequest{ProjectID: projA})
	wg.Wait()
	done := waitJob(t, m, job.ID, terminal)
	if done.FramesDone != 1000 || done.FramesTotal != 1000 || done.Status != StatusSucceeded {
		t.Fatalf("done = %+v", done)
	}
	if rev := m.Revision(); rev > 40 {
		t.Fatalf("1000 progress ticks produced %d revisions", rev)
	}
	select {
	case <-sub:
	case <-time.After(time.Second):
		t.Fatal("no signal")
	}
}

func TestPayloadReachesTheRunFuncAndIsReleased(t *testing.T) {
	got := make(chan any, 1)
	m := newTestManager(t, t.TempDir(), func(ctx context.Context, job Job, payload any, h *Handle) (Output, error) {
		got <- payload
		return Output{}, nil
	})
	job, _, err := m.Enqueue(EnqueueRequest{ProjectID: projA, Payload: "snapshot-1"})
	if err != nil {
		t.Fatal(err)
	}
	if v := <-got; v != "snapshot-1" {
		t.Fatalf("payload = %v", v)
	}
	waitJob(t, m, job.ID, terminal)
	m.mu.Lock()
	left := len(m.payload)
	m.mu.Unlock()
	if left != 0 {
		t.Fatalf("%d payloads kept after the job ended", left)
	}
}
