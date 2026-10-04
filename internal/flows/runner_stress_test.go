package flows

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// stressFixture runs flows whose single "stress.work" node records how many runs
// execute at once, overall and per exclusive (queue or skip) flow. The node waits
// a random time below 3 ms so runs overlap and queues fill; no assertion depends on
// how long it waits.
type stressFixture struct {
	r        *Runner
	store    *Store
	finished chan string // one run id per OnRunFinished call
	// active counts the work nodes executing now, maxActive the highest count seen.
	active, maxActive atomic.Int64
	perFlow           sync.Map // flow id -> *atomic.Int64: live runs of an exclusive flow in its node
	problem           atomic.Pointer[string]
	startedMu         sync.Mutex
	started           map[string]int
}

func newStressFixture(t *testing.T, cfg RunnerConfig, maxRuns int) *stressFixture {
	t.Helper()
	sx := &stressFixture{finished: make(chan string, maxRuns), started: map[string]int{}}
	reg := newTestRegistry(t)
	reg.MustRegister(&NodeDef{Type: "stress.work", DefaultTimeout: 10 * time.Second,
		Execute: func(ctx context.Context, in ExecInput) (ExecResult, error) {
			n := sx.active.Add(1)
			defer sx.active.Add(-1)
			for m := sx.maxActive.Load(); n > m && !sx.maxActive.CompareAndSwap(m, n); m = sx.maxActive.Load() {
			}
			if in.Run.Mode != ModeTest && in.Flow.Settings.Concurrency != ConcurrencyParallel {
				v, _ := sx.perFlow.LoadOrStore(in.Run.FlowID, new(atomic.Int64))
				c := v.(*atomic.Int64)
				if c.Add(1) > 1 {
					sx.fail("two live runs of exclusive flow " + in.Run.FlowID + " executed at once")
				}
				defer c.Add(-1)
			}
			select {
			case <-time.After(time.Duration(rand.IntN(3000)) * time.Microsecond):
				return ExecResult{}, nil
			case <-ctx.Done():
				return ExecResult{}, ctx.Err()
			}
		}})
	sx.store = openTestStore(t)
	sx.r = NewRunner(newTestEngine(reg, nil, 4), sx.store, RunnerHooks{
		OnRunStarted: func(rec RunRecord) {
			sx.startedMu.Lock()
			defer sx.startedMu.Unlock()
			sx.started[rec.ID]++
		},
		OnRunFinished: func(rec RunRecord, _ RunResult) { sx.finished <- rec.ID },
	}, cfg, discardLogger())
	t.Cleanup(func() { _ = sx.r.Shutdown(context.Background()) })
	return sx
}

func (sx *stressFixture) fail(msg string) { sx.problem.CompareAndSwap(nil, &msg) }

func (sx *stressFixture) flow(t *testing.T, id string, policy ConcurrencyPolicy) *Flow {
	t.Helper()
	b := newFlow("Stress " + id)
	tr := b.node("start", "test.trigger", nil)
	work := b.node("work", "stress.work", nil)
	b.edge(tr, PortOut, work)
	f := b.build()
	f.ID = id
	f.Settings.Concurrency = policy
	if _, err := sx.store.CreateFlow(context.Background(), f, "", storeNow); err != nil {
		t.Fatalf("CreateFlow: %v", err)
	}
	return f
}

// checkState verifies the runner's bookkeeping under its lock: the global limit,
// one cancel func per occupied slot, and at most one admitted live run per exclusive flow.
func (sx *stressFixture) checkState(exclusive map[string]bool) {
	r := sx.r
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.slots > r.cfg.MaxParallelRuns || r.slots != len(r.cancels) {
		sx.fail(fmt.Sprintf("slots=%d cancels=%d limit=%d", r.slots, len(r.cancels), r.cfg.MaxParallelRuns))
	}
	for flowID, n := range r.live {
		if exclusive[flowID] && n > 1 {
			sx.fail(fmt.Sprintf("exclusive flow %s has %d admitted live runs", flowID, n))
		}
	}
}

// takeFinished returns the run ids already reported finished, without waiting.
func (sx *stressFixture) takeFinished() []string {
	var ids []string
	for {
		select {
		case id := <-sx.finished:
			ids = append(ids, id)
		default:
			return ids
		}
	}
}

// expectFinishedOnce fails unless got holds every id of want exactly once and nothing else.
func expectFinishedOnce(t *testing.T, want map[string]bool, got []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, id := range got {
		if !want[id] {
			t.Fatalf("run %s finished, but no Start returned it", id)
		}
		if seen[id] {
			t.Fatalf("run %s finished twice", id)
		}
		seen[id] = true
	}
	if len(seen) != len(want) {
		t.Fatalf("%d of %d runs finished", len(seen), len(want))
	}
}

func (sx *stressFixture) assertClean(t *testing.T, maxParallel int64) {
	t.Helper()
	if p := sx.problem.Load(); p != nil {
		t.Fatal(*p)
	}
	if m := sx.maxActive.Load(); m > maxParallel {
		t.Fatalf("%d runs executed at once; the limit is %d", m, maxParallel)
	}
	sx.startedMu.Lock()
	defer sx.startedMu.Unlock()
	for id, n := range sx.started {
		if n != 1 {
			t.Fatalf("run %s started %d times", id, n)
		}
	}
}

// Random Start, Cancel and IsBusy calls from several goroutines across queue, skip
// and parallel flows, in live and test mode. Every run finishes exactly once, the
// global limit and the exclusive policies hold throughout, and the runner's state is
// empty once all runs ended.
func TestRunnerStress(t *testing.T) {
	const workers, ops, maxParallel = 6, 20, 3
	sx := newStressFixture(t, RunnerConfig{MaxParallelRuns: maxParallel, MaxQueuedPerFlow: 3}, workers*ops)
	flowList := []*Flow{
		sx.flow(t, "flow_aaaaaaaaea", ConcurrencyQueue),
		sx.flow(t, "flow_aaaaaaaaeb", ConcurrencySkip),
		sx.flow(t, "flow_aaaaaaaaec", ConcurrencyParallel),
		sx.flow(t, "flow_aaaaaaaaed", ConcurrencyQueue),
	}
	exclusive := map[string]bool{flowList[0].ID: true, flowList[1].ID: true, flowList[3].ID: true}
	var idsMu sync.Mutex
	ids := map[string]bool{}
	var idList []string
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(seed uint64) {
			defer wg.Done()
			rng := rand.New(rand.NewPCG(seed, 16))
			for i := 0; i < ops; i++ {
				switch op := rng.IntN(10); {
				case op < 6:
					f := flowList[rng.IntN(len(flowList))]
					mode := ModeLive
					if rng.IntN(4) == 0 {
						mode = ModeTest
					}
					res, err := sx.r.Start(StartRequest{Flow: f, Mode: mode, TriggerNode: f.Nodes[0].ID, TriggerType: "manual"})
					switch {
					case errors.Is(err, ErrQueueFull):
					case err != nil:
						sx.fail("Start: " + err.Error())
					case res.Status == StartSkipped && res.RunID != "":
						sx.fail("a skipped start returned a run id")
					case res.RunID != "":
						idsMu.Lock()
						ids[res.RunID] = true
						idList = append(idList, res.RunID)
						idsMu.Unlock()
					}
				case op < 9:
					idsMu.Lock()
					var id string
					if len(idList) > 0 {
						id = idList[rng.IntN(len(idList))]
					}
					idsMu.Unlock()
					if id != "" {
						sx.r.Cancel(id)
					}
				default:
					sx.r.IsBusy(flowList[rng.IntN(len(flowList))].ID)
					sx.checkState(exclusive)
				}
			}
		}(uint64(w + 1))
	}
	wg.Wait()

	// All ids are known now; the finished channel holds an id per hook call.
	got := make([]string, 0, len(ids))
	timeout := time.After(10 * time.Second)
	for len(got) < len(ids) {
		select {
		case id := <-sx.finished:
			got = append(got, id)
		case <-timeout:
			t.Fatalf("%d of %d runs finished", len(got), len(ids))
		}
	}
	expectFinishedOnce(t, ids, got)
	sx.assertClean(t, maxParallel)
	r := sx.r
	r.mu.Lock()
	state := fmt.Sprintf("slots=%d live=%v queues=%d waiting=%d cancels=%d",
		r.slots, r.live, len(r.flowQueue), len(r.waiting), len(r.cancels))
	clean := r.slots == 0 && len(r.live) == 0 && len(r.flowQueue) == 0 && len(r.waiting) == 0 && len(r.cancels) == 0
	r.mu.Unlock()
	if !clean {
		t.Fatalf("runner state after all runs ended: %s", state)
	}
	// Shutdown waits for every run goroutine, so a second finish would show up now.
	if err := r.Shutdown(context.Background()); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if extra := sx.takeFinished(); len(extra) != 0 {
		t.Fatalf("runs finished again: %v", extra)
	}
	t.Logf("%d runs, at most %d at once", len(ids), sx.maxActive.Load())
}

// Shutdown while several goroutines keep starting runs. Every Start that succeeded
// was before the shutdown, so its run has finished exactly once by the time Shutdown
// returned, no node runs any more, and later Starts fail with ErrRunnerClosed.
func TestRunnerStressShutdownRacesStart(t *testing.T) {
	const workers, starts, maxParallel = 4, 15, 2
	sx := newStressFixture(t, RunnerConfig{MaxParallelRuns: maxParallel, MaxQueuedPerFlow: 4}, workers*starts)
	queue := sx.flow(t, "flow_aaaaaaaaee", ConcurrencyQueue)
	parallel := sx.flow(t, "flow_aaaaaaaaef", ConcurrencyParallel)
	var idsMu sync.Mutex
	ids := map[string]bool{}
	var accepted atomic.Int64
	shutdownNow := make(chan struct{})
	var kick sync.Once
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < starts; i++ {
				f := queue
				if (w+i)%2 == 0 {
					f = parallel
				}
				res, err := sx.r.Start(StartRequest{Flow: f, Mode: ModeLive, TriggerNode: f.Nodes[0].ID, TriggerType: "manual"})
				switch {
				case errors.Is(err, ErrRunnerClosed), errors.Is(err, ErrQueueFull):
					continue
				case err != nil:
					sx.fail("Start: " + err.Error())
					continue
				}
				idsMu.Lock()
				ids[res.RunID] = true
				idsMu.Unlock()
				// Each flow takes at least 5 runs before it refuses any, so 8 are always accepted.
				if accepted.Add(1) == 8 {
					kick.Do(func() { close(shutdownNow) })
				}
			}
		}(w)
	}
	select {
	case <-shutdownNow:
	case <-time.After(10 * time.Second):
		t.Fatal("the workers did not get 8 runs accepted")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := sx.r.Shutdown(ctx); err != nil {
		t.Fatalf("Shutdown: %v", err)
	}
	if n := sx.active.Load(); n != 0 {
		t.Fatalf("%d nodes still run after Shutdown returned", n)
	}
	wg.Wait()
	expectFinishedOnce(t, ids, sx.takeFinished())
	sx.assertClean(t, maxParallel)
	if _, err := sx.r.Start(StartRequest{Flow: queue, Mode: ModeLive, TriggerNode: queue.Nodes[0].ID}); !errors.Is(err, ErrRunnerClosed) {
		t.Fatalf("Start after Shutdown = %v, want ErrRunnerClosed", err)
	}
	t.Logf("%d runs accepted before the shutdown", len(ids))
}
