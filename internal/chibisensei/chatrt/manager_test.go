package chatrt

import (
	"context"
	"errors"
	"os"
	"runtime"
	"testing"
	"time"
)

func TestManagerReusesInstanceAndWaitsWhileBusy(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	first := fx.acquire("a")
	if got := len(l.launchSpecs()); got != 1 {
		t.Fatalf("launches %d", got)
	}

	got := make(chan *Instance, 1)
	go func() {
		lease, err := fx.m.Acquire(context.Background(), fx.model("a"))
		if err != nil {
			t.Errorf("second Acquire: %v", err)
		}
		got <- lease
	}()
	select {
	case <-got:
		t.Fatal("second Acquire returned while the first lease was held")
	case <-time.After(100 * time.Millisecond):
	}
	first.Release()
	var second *Instance
	select {
	case second = <-got:
	case <-time.After(5 * time.Second):
		t.Fatal("second Acquire did not return after Release")
	}
	if second == nil {
		t.Fatal("no second lease")
	}
	if got := len(l.launchSpecs()); got != 1 {
		t.Fatalf("the same key launched %d times", got)
	}

	// With no release the wait ends in ErrBusy.
	fx.m.busyWait = 50 * time.Millisecond
	start := time.Now()
	if _, err := fx.m.Acquire(context.Background(), fx.model("a")); !errors.Is(err, ErrBusy) {
		t.Fatalf("Acquire on a held instance: %v", err)
	}
	if time.Since(start) > 3*time.Second {
		t.Fatal("ErrBusy took too long")
	}
	second.Release()
	second.Release() // idempotent
}

func TestManagerEvictsLeastRecentlyUsedIdle(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	for _, key := range []string{"a", "b", "a"} {
		fx.acquire(key).Release()
		fx.advance(time.Second)
	}
	fx.acquire("c").Release()

	st := fx.m.Status(context.Background())
	keys := []string{}
	for _, in := range st.Instances {
		keys = append(keys, in.Key)
	}
	if len(keys) != 2 || keys[0] != "a" || keys[1] != "c" {
		t.Fatalf("instances %v", keys)
	}
	if n := len(l.launchSpecs()); n != 3 {
		t.Fatalf("launches %d", n)
	}
	l.mu.Lock()
	stoppedB := l.procs[1].isStopped()
	l.mu.Unlock()
	if !stoppedB {
		t.Fatal("the evicted instance b is still running")
	}
}

func TestManagerBusyWhenNoIdleSlot(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	a, b := fx.acquire("a"), fx.acquire("b")
	defer a.Release()
	defer b.Release()
	start := time.Now()
	if _, err := fx.m.Acquire(context.Background(), fx.model("c")); !errors.Is(err, ErrBusy) {
		t.Fatalf("Acquire c: %v", err)
	}
	if time.Since(start) >= fx.m.busyWait {
		t.Fatal("ErrBusy waited for busyWait")
	}
	if err := fx.m.Admit("c"); !errors.Is(err, ErrBusy) {
		t.Fatalf("Admit c: %v", err)
	}
	if err := fx.m.Admit("a"); err != nil {
		t.Fatalf("Admit a: %v", err)
	}
	if err := fx.m.Admit("a", "b"); err != nil {
		t.Fatalf("Admit a, b: %v", err)
	}
	b.Release()
	if err := fx.m.Admit("c"); err != nil {
		t.Fatalf("Admit c with an idle slot: %v", err)
	}
}

func TestManagerIdleReaper(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	fx.setConfig(func(c *Config) { c.IdleMinutes = 1 })
	idle := fx.acquire("a")
	idle.Release()
	held := fx.acquire("b")
	defer held.Release()

	fx.advance(61 * time.Second)
	fx.m.reapOnce()
	st := fx.m.Status(context.Background())
	if len(st.Instances) != 1 || st.Instances[0].Key != "b" || st.Instances[0].State != "busy" {
		t.Fatalf("instances after reap: %+v", st.Instances)
	}

	fx.setConfig(func(c *Config) { c.Enabled = false })
	fx.m.reapOnce()
	if st := fx.m.Status(context.Background()); len(st.Instances) != 0 {
		t.Fatalf("a disabled runtime keeps instances: %+v", st.Instances)
	}
}

func TestManagerReaperLoopStopsWithClose(t *testing.T) {
	fx := newFixture(t, newFakeLauncher(true))
	fx.m.reapEvery = 5 * time.Millisecond
	fx.m.Start(context.Background())
	fx.m.Start(context.Background()) // idempotent
	fx.m.Close()
	waitClosed(t, fx.m.reaperDone, "reaper end")
}

func TestManagerChatCancelStopsGeneration(t *testing.T) {
	l := newFakeLauncher(true)
	started := make(chan struct{})
	l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
		p := &fakeProc{done: make(chan struct{})}
		p.chatFn = func(ctx context.Context, _ ChatRequest, _ func(string)) (ChatStats, error) {
			close(started)
			<-ctx.Done()
			return ChatStats{}, ctx.Err()
		}
		return p, nil
	}
	fx := newFixture(t, l)
	lease := fx.acquire("a")
	ctx, cancel := context.WithCancel(context.Background())
	errc := make(chan error, 1)
	go func() {
		_, err := lease.Chat(ctx, ChatRequest{}, func(string) {})
		errc <- err
	}()
	waitClosed(t, started, "chat start")
	cancel()
	select {
	case err := <-errc:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Chat error %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("cancel did not end the generation")
	}
	lease.Release()
	fx.acquire("a").Release()
}

func TestManagerChatFillsBackendAndRejectsReleasedLease(t *testing.T) {
	fx := newFixture(t, newFakeLauncher(true))
	lease := fx.acquire("a")
	var text string
	stats, err := lease.Chat(context.Background(), ChatRequest{}, func(s string) { text += s })
	if err != nil || text != "ok" || stats.Backend != "cpu" || lease.Backend() != "cpu" {
		t.Fatalf("Chat: %v %q %+v", err, text, stats)
	}
	lease.Release()
	if _, err := lease.Chat(context.Background(), ChatRequest{}, nil); err == nil {
		t.Fatal("Chat on a released lease succeeded")
	}
}

func TestManagerChatPassesContextErrorAndDropsCrashedInstance(t *testing.T) {
	l := newFakeLauncher(true)
	var proc *fakeProc
	l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
		proc = &fakeProc{done: make(chan struct{})}
		return proc, nil
	}
	fx := newFixture(t, l)
	lease := fx.acquire("a")
	proc.chatFn = func(context.Context, ChatRequest, func(string)) (ChatStats, error) { return ChatStats{}, ErrContext }
	if _, err := lease.Chat(context.Background(), ChatRequest{}, nil); !errors.Is(err, ErrContext) {
		t.Fatalf("ErrContext lost: %v", err)
	}
	proc.chatFn = func(context.Context, ChatRequest, func(string)) (ChatStats, error) {
		_ = proc.Stop() // the process dies mid-answer
		return ChatStats{}, errProcessExited
	}
	if _, err := lease.Chat(context.Background(), ChatRequest{}, nil); err == nil || errors.Is(err, ErrStopped) {
		t.Fatalf("crash error %v", err)
	}
	lease.Release()
	if st := fx.m.Status(context.Background()); len(st.Instances) != 0 {
		t.Fatalf("crashed instance still listed: %+v", st.Instances)
	}
	fx.acquire("a").Release() // starts a fresh process
	if n := len(l.launchSpecs()); n != 2 {
		t.Fatalf("launches %d", n)
	}
}

func TestManagerStopAllAndClose(t *testing.T) {
	l := newFakeLauncher(true)
	parked := make(chan struct{})
	l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
		p := &fakeProc{done: make(chan struct{})}
		p.chatFn = func(ctx context.Context, _ ChatRequest, _ func(string)) (ChatStats, error) {
			close(parked)
			select {
			case <-p.done:
				return ChatStats{}, errProcessExited
			case <-ctx.Done():
				return ChatStats{}, ctx.Err()
			}
		}
		return p, nil
	}
	fx := newFixture(t, l)
	fx.m.reapEvery = time.Hour
	fx.m.Start(context.Background())
	lease := fx.acquire("a")
	errc := make(chan error, 1)
	go func() {
		_, err := lease.Chat(context.Background(), ChatRequest{}, nil)
		errc <- err
	}()
	waitClosed(t, parked, "chat start")
	fx.m.StopAll(context.Background())
	fx.m.StopAll(context.Background())
	select {
	case err := <-errc:
		if !errors.Is(err, ErrStopped) {
			t.Fatalf("busy chat ended with %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("busy chat did not end")
	}
	if st := fx.m.Status(context.Background()); len(st.Instances) != 0 {
		t.Fatalf("instances after StopAll: %+v", st.Instances)
	}
	fx.m.Close()
	fx.m.Close()
	lease.Release() // after Close: no panic
	if _, err := fx.m.Acquire(context.Background(), fx.model("a")); !errors.Is(err, ErrClosed) {
		t.Fatalf("Acquire after Close: %v", err)
	}
	if err := fx.m.Admit("a"); !errors.Is(err, ErrClosed) {
		t.Fatalf("Admit after Close: %v", err)
	}
	waitClosed(t, fx.m.reaperDone, "reaper end")
}

func TestManagerModelMissing(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	ref := fx.model("a")
	missing := ref
	missing.Path = ref.Path + ".gone"
	if _, err := fx.m.Acquire(context.Background(), missing); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("missing file: %v", err)
	}
	wrong := ref
	wrong.Bytes++
	if _, err := fx.m.Acquire(context.Background(), wrong); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("wrong size: %v", err)
	}
	dir := ref
	dir.Path, dir.Bytes = fx.dir, 0
	if _, err := fx.m.Acquire(context.Background(), dir); !errors.Is(err, ErrModelMissing) {
		t.Fatalf("directory: %v", err)
	}
	if n := len(l.launchSpecs()); n != 0 {
		t.Fatalf("a missing model launched %d times", n)
	}
	if err := os.Remove(ref.Path); err != nil {
		t.Fatal(err)
	}
}

func TestManagerStartFailureNotCached(t *testing.T) {
	l := newFakeLauncher(true)
	calls := 0
	l.launchFn = func(context.Context, LaunchSpec) (Process, error) {
		calls++
		if calls == 1 {
			return nil, errors.New("boom")
		}
		return &fakeProc{done: make(chan struct{})}, nil
	}
	fx := newFixture(t, l)
	if _, err := fx.m.Acquire(context.Background(), fx.model("a")); err == nil || errors.Is(err, ErrBusy) {
		t.Fatalf("first Acquire: %v", err)
	}
	if st := fx.m.Status(context.Background()); len(st.Instances) != 0 {
		t.Fatalf("failed start left an instance: %+v", st.Instances)
	}
	fx.acquire("a").Release()
	if calls != 2 {
		t.Fatalf("launch calls %d", calls)
	}
}

func TestManagerLaunchSpec(t *testing.T) {
	l := newFakeLauncher(true)
	fx := newFixture(t, l)
	fx.setConfig(func(c *Config) { c.ContextSize = 2048 })
	fx.acquire("tuned:run1").Release()
	specs := l.launchSpecs()
	if len(specs) != 1 {
		t.Fatalf("launches %d", len(specs))
	}
	s := specs[0]
	if s.Threads != max(1, runtime.NumCPU()/2) || s.ContextSize != 2048 || s.Key != "tuned:run1" || s.Backend != "cpu" ||
		s.CacheDir != fx.config().CacheDir || len(s.Assets) != 1 {
		t.Fatalf("spec %+v", s)
	}
}
