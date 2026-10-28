package dockerstacks

import (
	"context"
	"reflect"
	"sync"
	"testing"
	"time"
)

// opLog collects what an observer is told.
type opLog struct {
	mu  sync.Mutex
	ops []Operation
}

func (l *opLog) add(op Operation) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.ops = append(l.ops, op)
}

func (l *opLog) states() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]string, 0, len(l.ops))
	for _, op := range l.ops {
		out = append(out, op.State)
	}
	return out
}

func TestObserveSeesEveryStateChangeAndNeverTheOutput(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	web := env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 2 seconds", "starting", time.Now().Unix(),
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"})
	env.fe.runnerLines = []string{`{"id":"Network web_default","status":"Working","text":"Creating"}`}

	var first, second opLog
	cancelFirst := env.svc.Observe(first.add)
	defer cancelFirst()
	cancelSecond := env.svc.Observe(second.add)

	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the health_wait state", 5*time.Second, func() bool {
		cur, _ := env.svc.Operation(context.Background(), op.ID)
		return cur.State == StateHealthWait
	})
	env.fe.mu.Lock()
	web.health = "healthy"
	env.fe.mu.Unlock()
	done := waitDone(t, env.svc, op.ID)
	if done.State != StateSucceeded || done.Output == "" {
		t.Fatalf("done = %+v, want a succeeded operation that has output", done)
	}

	waitUntil(t, "the final state to be reported", 5*time.Second, func() bool {
		return len(first.states()) == 4
	})
	want := []string{StateQueued, StateRunning, StateHealthWait, StateSucceeded}
	if got := first.states(); !reflect.DeepEqual(got, want) {
		t.Fatalf("states = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(second.states(), want) {
		t.Fatalf("second observer states = %v, want %v", second.states(), want)
	}
	for i, seen := range first.ops {
		if seen.ID != op.ID || seen.HostID != testHostID || seen.Stack != "web" || seen.Kind != KindUp {
			t.Fatalf("notification %d = %+v, want the operation's identity", i, seen)
		}
		if seen.Output != "" {
			t.Fatalf("notification %d carries output %q; output is never forwarded", i, seen.Output)
		}
	}
	last := first.ops[3]
	if last.FinishedAt == nil || last.ExitCode == nil || *last.ExitCode != 0 || len(last.Ports) != 1 {
		t.Fatalf("final notification = %+v, want the finished operation with exit code and ports", last)
	}

	// A cancelled observer hears nothing more; the other one still does.
	cancelSecond()
	cancelSecond()
	next := startAndWait(t, env, "web", KindStop, OpOptions{})
	waitUntil(t, "the stop operation to be reported", 5*time.Second, func() bool {
		return len(first.states()) == 7
	})
	if len(second.states()) != 4 {
		t.Fatalf("a cancelled observer was told %v", second.states())
	}
	if first.ops[6].ID != next.ID || first.ops[6].State != StateSucceeded {
		t.Fatalf("last notification = %+v", first.ops[6])
	}
}

func TestObserveReportsAFailedOperationAndAnObserverMayBeNil(t *testing.T) {
	env := newTestEnv(t, true)
	cancel := env.svc.Observe(nil)
	cancel()
	var log opLog
	defer env.svc.Observe(log.add)()

	// No compose file: the operation fails after it started.
	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitDone(t, env.svc, op.ID); done.State != StateFailed || done.Code != CodeStackNotFound {
		t.Fatalf("done = %+v, want failed with %s", done, CodeStackNotFound)
	}
	waitUntil(t, "the failure to be reported", 5*time.Second, func() bool { return len(log.states()) == 3 })
	if got := log.states(); !reflect.DeepEqual(got, []string{StateQueued, StateRunning, StateFailed}) {
		t.Fatalf("states = %v", got)
	}
	if last := log.ops[2]; last.Code != CodeStackNotFound || last.Error == "" {
		t.Fatalf("failure notification = %+v, want the code and the one-line error", last)
	}
}
