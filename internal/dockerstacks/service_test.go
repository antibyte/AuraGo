package dockerstacks

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"
)

// fakeClock is a manually advanced clock for Options.Now.
type fakeClock struct {
	mu sync.Mutex
	t  time.Time
}

func newFakeClock() *fakeClock {
	return &fakeClock{t: time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)}
}

func (c *fakeClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *fakeClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// newTestService opens a service on a fresh database with short timings. tune
// may change the options.
func newTestService(t *testing.T, hosts Hosts, tune func(*Options)) *Service {
	t.Helper()
	opts := Options{
		Logger:       slog.New(slog.NewTextHandler(io.Discard, nil)),
		HealthWait:   400 * time.Millisecond,
		HealthPoll:   10 * time.Millisecond,
		MaxDuration:  time.Minute,
		ListCacheTTL: time.Minute,
	}
	if tune != nil {
		tune(&opts)
	}
	s, err := Open(openTestDB(t), hosts, opts)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// registerRun stores an operation row and registers its live run, as Start does.
func registerRun(t *testing.T, s *Service, op Operation) *opRun {
	t.Helper()
	if op.CreatedAt.IsZero() {
		op.CreatedAt = s.opts.Now().UTC()
	}
	if op.State == "" {
		op.State = StateQueued
	}
	if err := s.journal.insert(context.Background(), op); err != nil {
		t.Fatal(err)
	}
	run := newOpRun(s, op)
	s.mu.Lock()
	s.runs[op.ID] = run
	s.locks[lockKey{op.HostID, op.Stack}] = op.ID
	s.mu.Unlock()
	return run
}

func drainEvents(ch <-chan Event) []Event {
	var events []Event
	for ev := range ch {
		events = append(events, ev)
	}
	return events
}

func TestOpenRejectsMissingArguments(t *testing.T) {
	if _, err := Open(nil, newFakeHosts(), Options{}); err == nil {
		t.Fatal("Open without a database must fail")
	}
	if _, err := Open(openTestDB(t), nil, Options{}); err == nil {
		t.Fatal("Open without hosts must fail")
	}
	o := Options{}.withDefaults()
	if o.HealthWait != 120*time.Second || o.HealthPoll != 2*time.Second || o.MaxDuration != 60*time.Minute ||
		o.ListCacheTTL != 30*time.Second || o.LocalHostID != "local" || o.Now == nil || o.Logger == nil {
		t.Fatalf("defaults = %+v", o)
	}
}

func TestSubscribeDeliversSnapshotThenEventsInOrderAndEndsWithDone(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	run := registerRun(t, s, Operation{ID: "op_aaaaaaaaaaaaaaaa", HostID: "h_1", Stack: "web", Kind: KindUp})
	run.setState(StateRunning)
	run.line("before the subscriber")

	op, events, cancel, err := s.Subscribe("op_aaaaaaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	if op.State != StateRunning || op.Output != "before the subscriber\n" || op.StartedAt == nil {
		t.Fatalf("snapshot = %+v, want the running state with the output so far", op)
	}

	run.composeLine(`{"id":"Container web-1","status":"Working","text":"Starting"}`)
	run.composeLine(`{"id":"layer1","status":"Working","text":"Downloading","current":10,"total":100}`)
	run.composeLine("plain line")
	run.setState(StateHealthWait)
	exit := 0
	run.finish(outcome{state: StateSucceeded, exit: &exit, warning: "db is unhealthy", ports: []PortLink{{Service: "web", PublicPort: 8080, PrivatePort: 80, Protocol: "tcp"}}})

	got := drainEvents(events)
	types := make([]string, len(got))
	for i, ev := range got {
		types[i] = ev.Type
		// Seq numbers the events of the whole operation: the state change and the line
		// before the subscription took numbers 1 and 2.
		if ev.Seq != int64(i+3) {
			t.Fatalf("event %d has seq %d, want %d", i, ev.Seq, i+3)
		}
	}
	// Progress and output for the first item, progress only for the byte tick, the plain
	// line, the health_wait state, done.
	want := []string{EventProgress, EventOutput, EventProgress, EventOutput, EventState, EventDone}
	if strings.Join(types, ",") != strings.Join(want, ",") {
		t.Fatalf("event types = %v, want %v", types, want)
	}
	if got[0].Progress == nil || got[0].Progress.ID != "Container web-1" || got[1].Text != "Container web-1 Working Starting\n" {
		t.Fatalf("first item = %+v / %+v", got[0], got[1])
	}
	if got[2].Progress.Total != 100 || got[3].Text != "plain line\n" {
		t.Fatalf("byte tick / plain line = %+v / %+v", got[2], got[3])
	}
	if got[4].Operation == nil || got[4].Operation.State != StateHealthWait || got[4].Operation.Output != "" {
		t.Fatalf("state event = %+v, want health_wait without output", got[4])
	}
	done := got[5].Operation
	if done == nil || done.State != StateSucceeded || done.Warning != "db is unhealthy" || done.FinishedAt == nil ||
		done.ExitCode == nil || *done.ExitCode != 0 || !strings.Contains(done.Output, "plain line") || len(done.Ports) != 1 {
		t.Fatalf("done event = %+v", done)
	}

	// The lock is released and the journal holds the final row.
	if s.activeRun("h_1", "web") != nil {
		t.Fatal("a finished operation must release its stack")
	}
	row, err := s.Operation(context.Background(), "op_aaaaaaaaaaaaaaaa")
	if err != nil || row.State != StateSucceeded || !strings.Contains(row.Output, "before the subscriber") {
		t.Fatalf("journal row = %+v, %v", row, err)
	}
	// A subscriber after the end gets the final snapshot and a closed channel.
	op2, ch2, _, err := s.Subscribe("op_aaaaaaaaaaaaaaaa")
	if err != nil || op2.State != StateSucceeded {
		t.Fatalf("late Subscribe = %+v, %v", op2, err)
	}
	if _, open := <-ch2; open {
		t.Fatal("the channel of a finished operation must already be closed")
	}
	if _, _, _, err := s.Subscribe("op_unknown"); CodeOf(err) != CodeOperationNotFound {
		t.Fatalf("Subscribe of an unknown id = %v, want %s", err, CodeOperationNotFound)
	}
}

func TestASubscriberThatFalls256EventsBehindIsDropped(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	run := registerRun(t, s, Operation{ID: "op_bbbbbbbbbbbbbbbb", HostID: "h_1", Stack: "web", Kind: KindPull})
	_, slow, cancelSlow, _ := s.Subscribe("op_bbbbbbbbbbbbbbbb")
	defer cancelSlow()
	_, fast, cancelFast, _ := s.Subscribe("op_bbbbbbbbbbbbbbbb")
	defer cancelFast()

	fastCount := 0
	fastDone := make(chan struct{})
	go func() {
		defer close(fastDone)
		for range fast {
			fastCount++
		}
	}()
	for i := 0; i < 300; i++ {
		run.line("line")
		if i%50 == 49 {
			time.Sleep(20 * time.Millisecond) // let the fast reader keep up
		}
	}

	slowCount := 0
	for range slow { // closed by the drop, not by the end of the operation
		slowCount++
	}
	if slowCount != subscriberBuffer {
		t.Fatalf("the slow subscriber received %d events, want exactly the %d that fit its buffer", slowCount, subscriberBuffer)
	}
	run.finish(outcome{state: StateSucceeded})
	<-fastDone
	if fastCount != 301 { // 300 lines and the done event
		t.Fatalf("the fast subscriber received %d events, want 301", fastCount)
	}
}

func TestCancelEndsASubscription(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	run := registerRun(t, s, Operation{ID: "op_cccccccccccccccc", HostID: "h_1", Stack: "web", Kind: KindPull})
	_, events, cancel, _ := s.Subscribe("op_cccccccccccccccc")
	cancel()
	cancel() // twice is fine
	if _, open := <-events; open {
		t.Fatal("cancel must close the channel")
	}
	run.line("after cancel") // must not panic on the closed channel
}

func TestOutputKeepsTheLast64KiBAtACharacterBoundary(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	run := registerRun(t, s, Operation{ID: "op_dddddddddddddddd", HostID: "h_1", Stack: "web", Kind: KindPull})
	chunk := strings.Repeat("ü", 1000) + "\n" // 2001 bytes
	for i := 0; i < 200; i++ {
		run.addOutput(chunk)
	}
	run.addOutput("the very last line\n")
	out := run.snapshot().Output
	if len(out) > OutputTailBytes || len(out) < OutputTailBytes-4 {
		t.Fatalf("output is %d bytes, want about %d", len(out), OutputTailBytes)
	}
	if !strings.HasSuffix(out, "the very last line\n") {
		t.Fatal("the tail must end with the latest output")
	}
	if strings.ContainsRune(out, utf8.RuneError) {
		t.Fatal("the tail was cut inside a multi-byte character")
	}
}

func TestOutputReachesTheRowAtMostEveryTwoSeconds(t *testing.T) {
	clock := newFakeClock()
	s := newTestService(t, newFakeHosts(), func(o *Options) { o.Now = clock.Now })
	run := registerRun(t, s, Operation{ID: "op_eeeeeeeeeeeeeeee", HostID: "h_1", Stack: "web", Kind: KindPull})
	rowOutput := func() string {
		row, err := s.journal.get(context.Background(), "op_eeeeeeeeeeeeeeee")
		if err != nil {
			t.Fatal(err)
		}
		return row.Output
	}

	run.line("one")
	clock.Advance(time.Second)
	run.line("two")
	if got := rowOutput(); got != "" {
		t.Fatalf("row output after 1 s = %q, want it still empty", got)
	}
	clock.Advance(1500 * time.Millisecond)
	run.line("three")
	if got := rowOutput(); got != "one\ntwo\nthree\n" {
		t.Fatalf("row output after 2.5 s = %q, want all three lines flushed", got)
	}
	run.line("four")
	run.finish(outcome{state: StateFailed, code: opCodeRunnerFailed, message: "boom"})
	row, _ := s.journal.get(context.Background(), "op_eeeeeeeeeeeeeeee")
	if row.Output != "one\ntwo\nthree\nfour\n" || row.State != StateFailed || row.Error != "boom" {
		t.Fatalf("final row = %+v, want the full output written at the end", row)
	}
}

func TestComposeLinesKeepTheOutputReadable(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	var raw []string
	s.rawLine = func(id, line string) { raw = append(raw, line) }
	run := registerRun(t, s, Operation{ID: "op_ffffffffffffffff", HostID: "h_1", Stack: "web", Kind: KindUp})
	for _, line := range []string{
		`{"id":"layer1","status":"Working","text":"Downloading","current":10,"total":100}`, // byte count: event only
		`{"id":"layer1","status":"Working","text":"Downloading","current":100,"total":100}`,
		`{"id":"layer1","status":"Working","text":"Pulling"}`,
		`{"id":"layer1","status":"Working","text":"Pulling"}`, // repeat
		`{"id":"layer1","status":"Done","text":"Pulled"}`,
		"yaml: line 12: did not find expected key",
	} {
		run.composeLine(line)
	}
	if len(raw) != 6 {
		t.Fatalf("the raw line hook saw %d lines, want all 6", len(raw))
	}
	want := "layer1 Working Pulling\nlayer1 Done Pulled\nyaml: line 12: did not find expected key\n"
	if got := run.snapshot().Output; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
	if msg := run.scan.message(); msg != "yaml: line 12: did not find expected key" {
		t.Fatalf("error scan = %q", msg)
	}
}

func TestOperationsAreListedNewestFirstWithoutOutput(t *testing.T) {
	s := newTestService(t, newFakeHosts(), nil)
	base := time.Date(2026, 10, 27, 11, 0, 0, 0, time.UTC)
	registerRun(t, s, Operation{ID: "op_1111111111111111", HostID: "h_1", Stack: "a", Kind: KindUp, State: StateSucceeded, CreatedAt: base, Output: "old"})
	live := registerRun(t, s, Operation{ID: "op_2222222222222222", HostID: "h_1", Stack: "b", Kind: KindUp, CreatedAt: base.Add(time.Minute)})
	live.setState(StateRunning)
	live.line("working")

	ops, err := s.Operations(context.Background(), OperationFilter{HostID: "h_1"})
	if err != nil || len(ops) != 2 || ops[0].ID != "op_2222222222222222" {
		t.Fatalf("Operations = %+v, %v", ops, err)
	}
	if ops[0].Output != "" || ops[1].Output != "" {
		t.Fatal("a list must not carry the output")
	}
	if ops[0].State != StateRunning {
		t.Fatalf("the running operation must show its in-memory state, got %q", ops[0].State)
	}
	one, err := s.Operation(context.Background(), "op_2222222222222222")
	if err != nil || one.Output != "working\n" {
		t.Fatalf("Operation of a running one = %+v, %v, want the output so far", one, err)
	}
	active, _ := s.Operations(context.Background(), OperationFilter{Active: true})
	if len(active) != 1 || active[0].ID != "op_2222222222222222" {
		t.Fatalf("active = %+v", active)
	}
}
