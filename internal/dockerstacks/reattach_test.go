package dockerstacks

import (
	"context"
	"database/sql"
	"strings"
	"testing"
	"time"
)

// reopen opens a second service on the database and hosts of env, as the next
// process would after a restart.
func reopen(t *testing.T, env *testEnv, tune func(*Options)) *Service {
	t.Helper()
	opts := Options{
		Logger:       env.svc.log,
		Now:          env.clock.Now,
		HealthWait:   300 * time.Millisecond,
		HealthPoll:   10 * time.Millisecond,
		MaxDuration:  time.Minute,
		ListCacheTTL: time.Minute,
	}
	if tune != nil {
		tune(&opts)
	}
	s, err := Open(env.svc.db, env.hosts, opts)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	t.Cleanup(s.Close)
	return s
}

// leftBehind records what the previous process left in the journal: an
// operation in the given state, with the output it had flushed.
func leftBehind(t *testing.T, db *sql.DB, id, stack, kind, state, output string) {
	t.Helper()
	j, err := newJournal(db)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now().UTC()
	op := Operation{ID: id, HostID: testHostID, Stack: stack, Kind: kind, State: state, Output: output,
		RunnerID: "old-runner-id", CreatedAt: started, StartedAt: &started}
	if err := j.insert(context.Background(), op); err != nil {
		t.Fatal(err)
	}
}

// leftRunner puts the runner container of an operation on the fake host.
func leftRunner(env *testEnv, id, stack, kind, state string, exit int, lines ...string) *fakeContainer {
	op := Operation{ID: id, Stack: stack, Kind: kind}
	spec, _ := runnerSpec(RunnerImage, env.host, op, "compose.yaml")
	fc := env.fe.addContainer(runnerName(id), spec, state, exit)
	for _, line := range lines {
		env.fe.emit(fc, 2, line+"\n")
	}
	return fc
}

func TestAnOperationWhoseRunnerFinishedWhileAuraGoWasDownIsFinishedFromItsExitCode(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 1 minute", "", time.Now().Unix(),
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"})
	leftBehind(t, env.svc.db, "op_1111111111111111", "web", KindUp, StateRunning, "stale output that the log replaces\n")
	leftRunner(env, "op_1111111111111111", "web", KindUp, "exited", 0,
		`{"id":"Container web-web-1","status":"Done","text":"Started"}`)

	s2 := reopen(t, env, nil)
	if _, err := s2.Start(context.Background(), testHostID, "web", KindDown, OpOptions{}); CodeOf(err) != CodeOperationBusy {
		t.Fatalf("the adopted operation must keep its stack locked from the start, got %v", err)
	}
	done := waitDone(t, s2, "op_1111111111111111")
	if done.State != StateSucceeded || done.ExitCode == nil || *done.ExitCode != 0 || len(done.Ports) != 1 {
		t.Fatalf("done = %+v", done)
	}
	if done.Output != "Container web-web-1 Done Started\n" {
		t.Fatalf("output = %q, want it rebuilt from the runner's log", done.Output)
	}
	waitUntil(t, "the runner to be removed after its result was recorded", 5*time.Second, func() bool {
		return len(env.fe.containersWithRole(RoleRunner)) == 0
	})
	if s2.activeRun(testHostID, "web") != nil {
		t.Fatal("the stack must be free again")
	}
}

func TestAFailedRunnerFoundAfterARestartKeepsItsErrorLine(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	leftBehind(t, env.svc.db, "op_2222222222222222", "web", KindUp, StateRunning, "")
	leftRunner(env, "op_2222222222222222", "web", KindUp, "exited", 15, "yaml: line 4: did not find expected key")

	done := waitDone(t, reopen(t, env, nil), "op_2222222222222222")
	if done.State != StateFailed || done.Code != opCodeRunnerFailed || done.ErrorLine != 4 || *done.ExitCode != 15 {
		t.Fatalf("done = %+v", done)
	}
}

func TestARunnerThatIsStillRunningIsFollowedToItsEnd(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	leftBehind(t, env.svc.db, "op_3333333333333333", "web", KindUp, StateRunning, "")
	runner := leftRunner(env, "op_3333333333333333", "web", KindUp, "running", 0, "first line before the restart")

	s2 := reopen(t, env, nil)
	waitUntil(t, "the first line", 5*time.Second, func() bool {
		op, _ := s2.Operation(context.Background(), "op_3333333333333333")
		return strings.Contains(op.Output, "first line before the restart")
	})
	op, _ := s2.Operation(context.Background(), "op_3333333333333333")
	if op.Done() {
		t.Fatal("the runner is still running")
	}
	env.fe.emit(runner, 2, "second line after the restart\n")
	env.fe.finish(runner, 0)
	done := waitDone(t, s2, "op_3333333333333333")
	if done.State != StateSucceeded || done.Output != "first line before the restart\nsecond line after the restart\n" {
		t.Fatalf("done = %+v", done)
	}
}

func TestAnOperationWithoutARunnerIsInterruptedUnlessItOnlyWaitedForHealth(t *testing.T) {
	env := newTestEnv(t, true)
	leftBehind(t, env.svc.db, "op_4444444444444441", "a", KindUp, StateRunning, "partial output\n")
	leftBehind(t, env.svc.db, "op_4444444444444442", "b", KindUp, StateQueued, "")
	leftBehind(t, env.svc.db, "op_4444444444444443", "c", KindUpdate, StateHealthWait, "all done\n")
	leftBehind(t, env.svc.db, "op_4444444444444444", "", KindPrepare, StateRunning, "")

	s2 := reopen(t, env, nil)
	for _, id := range []string{"op_4444444444444441", "op_4444444444444442", "op_4444444444444444"} {
		op := waitDone(t, s2, id)
		if op.State != StateInterrupted || op.Code != opCodeInterrupted || !strings.Contains(op.Error, "restarted") || op.FinishedAt == nil {
			t.Fatalf("%s = %+v, want interrupted", id, op)
		}
	}
	if op, _ := s2.Operation(context.Background(), "op_4444444444444441"); op.Output != "partial output\n" {
		t.Fatalf("an interrupted operation keeps the output it had, got %q", op.Output)
	}
	if op := waitDone(t, s2, "op_4444444444444443"); op.State != StateSucceeded {
		t.Fatalf("an operation in health_wait without a runner had finished its work, got %+v", op)
	}
}

func TestAnUnreachableHostParksTheOperationUntilTheNextCall(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	leftBehind(t, env.svc.db, "op_5555555555555555", "web", KindUp, StateRunning, "")
	leftRunner(env, "op_5555555555555555", "web", KindUp, "exited", 0, "done")
	env.hosts.setDown(testHostID, true)

	s2 := reopen(t, env, nil)
	waitUntil(t, "the operation to be parked", 5*time.Second, func() bool {
		s2.mu.Lock()
		defer s2.mu.Unlock()
		return len(s2.parked) == 1
	})
	op, _ := s2.Operation(context.Background(), "op_5555555555555555") // the host is still down: stays parked
	if op.Done() {
		t.Fatalf("an operation whose host does not answer must stay as it is, got %+v", op)
	}
	if _, err := s2.Start(context.Background(), testHostID, "web", KindDown, OpOptions{}); CodeOf(err) != CodeOperationBusy {
		t.Fatalf("a parked operation still holds its stack, got %v", err)
	}

	env.hosts.setDown(testHostID, false)
	if _, err := s2.List(context.Background(), testHostID, false); err != nil { // reaching the host retries the operation
		t.Fatal(err)
	}
	done := waitDone(t, s2, "op_5555555555555555")
	if done.State != StateSucceeded {
		t.Fatalf("done = %+v", done)
	}
}

func TestCloseLeavesTheRunnerGoingAndTheNextOpenFinishesTheOperation(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	gate := make(chan struct{})
	env.fe.runnerGate = gate
	env.fe.runnerLines = []string{`{"id":"Container web-web-1","status":"Working","text":"Starting"}`}

	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, events, cancel, _ := env.svc.Subscribe(op.ID)
	defer cancel()
	waitUntil(t, "the runner's first line", 5*time.Second, func() bool {
		cur, _ := env.svc.Operation(context.Background(), op.ID)
		return strings.Contains(cur.Output, "Starting")
	})

	env.svc.Close()
	for range events { // Close ends the event stream without a done event
	}
	if left := env.fe.containersWithRole(RoleRunner); len(left) != 1 || !env.fe.isRunning(left[0]) {
		t.Fatalf("Close must leave the runner running: %v", left)
	}
	row, err := env.svc.journal.get(context.Background(), op.ID)
	if err != nil || row.Done() || !strings.Contains(row.Output, "Starting") || row.RunnerID == "" {
		t.Fatalf("after Close the row must still be unfinished and carry its output, got %+v / %v", row, err)
	}

	s2 := reopen(t, env, nil)
	close(gate) // the runner finishes while the new process follows it
	done := waitDone(t, s2, op.ID)
	if done.State != StateSucceeded || !strings.Contains(done.Output, "Container web-web-1 Working Starting") {
		t.Fatalf("done = %+v", done)
	}
	waitUntil(t, "the runner to be removed", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleRunner)) == 0 })
}

func TestTheFirstUseOfAHostSweepsLeftoverHelpers(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	old := env.clock.Now().Unix() - 3600 // the service reads the test clock
	spec, _ := runnerSpec(RunnerImage, env.host, Operation{ID: "op_6666666666666661", Stack: "web", Kind: KindUp}, "compose.yaml")

	deadRunner := env.fe.addContainer("aurago-dm-op-op_6666666666666661", spec, "exited", 0)
	spec2, _ := runnerSpec(RunnerImage, env.host, Operation{ID: "op_6666666666666662", Stack: "web", Kind: KindUp}, "compose.yaml")
	activeRunnerContainer := env.fe.addContainer("aurago-dm-op-op_6666666666666662", spec2, "exited", 0)
	oldLister := env.fe.addContainer("aurago-dm-ls-00000001", listerSpec(RunnerImage, env.host), "exited", 0)
	oldLister.created = old
	freshLister := env.fe.addContainer("aurago-dm-ls-00000002", listerSpec(RunnerImage, env.host), "exited", 0)
	freshLister.created = env.clock.Now().Unix() - 5
	runningLister := env.fe.addContainer("aurago-dm-ls-00000003", listerSpec(RunnerImage, env.host), "running", 0)
	runningLister.created = old
	registerRun(t, env.svc, Operation{ID: "op_6666666666666662", HostID: testHostID, Stack: "web", Kind: KindUp, State: StateRunning})

	if _, err := env.svc.List(context.Background(), testHostID, false); err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the sweep", 5*time.Second, func() bool { return env.fe.container(deadRunner.name) == nil })
	if env.fe.container(oldLister.name) != nil {
		t.Fatal("an exited lister older than a minute must be removed")
	}
	for _, kept := range []*fakeContainer{activeRunnerContainer, freshLister, runningLister, env.fe.container(IOContainerName)} {
		if kept == nil || env.fe.container(kept.name) == nil {
			t.Fatalf("the sweep removed a container it must keep: %+v", kept)
		}
	}
	// It runs once per host and process.
	listsBefore := env.fe.requestCount("GET /containers/json")
	_, _ = env.svc.List(context.Background(), testHostID, true)
	if env.fe.requestCount("GET /containers/json") != listsBefore+1 {
		t.Fatal("the sweep must not list containers again on the next call")
	}
}

// leftBehindSince records an unfinished operation that started `ago` before the
// test clock, so that its time budget (MaxDuration) is spent when it is adopted.
func leftBehindSince(t *testing.T, env *testEnv, id, stack, kind, state string, ago time.Duration) {
	t.Helper()
	j, err := newJournal(env.svc.db)
	if err != nil {
		t.Fatal(err)
	}
	started := env.clock.Now().UTC().Add(-ago)
	op := Operation{ID: id, HostID: testHostID, Stack: stack, Kind: kind, State: state,
		RunnerID: "old-runner-id", CreatedAt: started, StartedAt: &started}
	if err := j.insert(context.Background(), op); err != nil {
		t.Fatal(err)
	}
}

func TestAnAdoptedOperationWhoseTimeRanOutWhileAuraGoWasDownIsStillFinishedFromItsExitCode(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	seedComposeStack(env, "api")
	leftBehindSince(t, env, "op_7777777777777771", "web", KindUp, StateRunning, 2*time.Hour)
	leftRunner(env, "op_7777777777777771", "web", KindUp, "exited", 0, `{"id":"Container web-web-1","status":"Done","text":"Started"}`)
	leftBehindSince(t, env, "op_7777777777777772", "api", KindUp, StateRunning, 2*time.Hour)
	leftRunner(env, "op_7777777777777772", "api", KindUp, "exited", 15, "yaml: line 4: did not find expected key")

	s2 := reopen(t, env, nil)
	if web := waitDone(t, s2, "op_7777777777777771"); web.State != StateSucceeded || web.ExitCode == nil || *web.ExitCode != 0 {
		t.Fatalf("an exited runner keeps its result although the time is spent, got %+v", web)
	}
	if api := waitDone(t, s2, "op_7777777777777772"); api.State != StateFailed || api.Code != opCodeRunnerFailed || api.ExitCode == nil || *api.ExitCode != 15 {
		t.Fatalf("an exited runner keeps its exit code although the time is spent, got %+v", api)
	}
	for _, stack := range []string{"web", "api"} {
		if s2.activeRun(testHostID, stack) != nil {
			t.Fatalf("the stack %s must be free again", stack)
		}
	}
}

func TestAnAdoptedOperationWithSpentTimeKillsARunnerThatIsStillGoing(t *testing.T) {
	prev := spentBudgetWindow
	spentBudgetWindow = 300 * time.Millisecond
	t.Cleanup(func() { spentBudgetWindow = prev })
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	leftBehindSince(t, env, "op_7777777777777773", "web", KindUp, StateRunning, 2*time.Hour)
	leftRunner(env, "op_7777777777777773", "web", KindUp, "running", 0, "still working")

	done := waitDone(t, reopen(t, env, nil), "op_7777777777777773")
	if done.State != StateFailed || done.Code != opCodeTimeout {
		t.Fatalf("done = %+v, want a timeout", done)
	}
	kills := 0
	for _, r := range env.fe.requests() {
		if strings.HasPrefix(r, "POST /v1.45/containers/") && strings.Contains(r, "/kill") {
			kills++
		}
	}
	if kills != 1 {
		t.Fatalf("the runner must be killed once: %v", env.fe.requests())
	}
	waitUntil(t, "the runner to be removed", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleRunner)) == 0 })
}

func TestAnAdoptedOperationWithSpentTimeEndsInsteadOfWaitingForAHostThatDoesNotAnswer(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	leftBehindSince(t, env, "op_7777777777777774", "web", KindUp, StateRunning, 2*time.Hour)
	leftRunner(env, "op_7777777777777774", "web", KindUp, "exited", 0, "done")
	env.hosts.setDown(testHostID, true)

	done := waitDone(t, reopen(t, env, nil), "op_7777777777777774")
	if done.State != StateFailed || done.Code != opCodeTimeout {
		t.Fatalf("done = %+v, want a timeout and not a parked operation", done)
	}
}
