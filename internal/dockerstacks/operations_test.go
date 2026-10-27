package dockerstacks

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

// waitDone polls an operation until it has finished and returns it.
func waitDone(t *testing.T, svc *Service, id string) Operation {
	t.Helper()
	var op Operation
	waitUntil(t, "operation "+id+" to finish", 10*time.Second, func() bool {
		var err error
		if op, err = svc.Operation(context.Background(), id); err != nil {
			t.Fatalf("Operation: %v", err)
		}
		return op.Done()
	})
	return op
}

// seedStack puts a stack with a compose file on the host.
func seedComposeStack(env *testEnv, name string) {
	env.fe.putFile("/opt/stacks/"+name+"/compose.yaml", stacksTestCompose)
}

// startAndWait starts an operation and waits for its end.
func startAndWait(t *testing.T, env *testEnv, stack, kind string, opts OpOptions) Operation {
	t.Helper()
	op, err := env.svc.Start(context.Background(), testHostID, stack, kind, opts)
	if err != nil {
		t.Fatalf("Start %s: %v", kind, err)
	}
	return waitDone(t, env.svc, op.ID)
}

func TestUpRunsTheRunnerFollowsItAndRecordsThePorts(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	env.fe.runnerLines = []string{
		`{"id":"Network web_default","status":"Working","text":"Creating"}`,
		`{"id":"layer1","parent_id":"Image nginx","status":"Working","text":"Downloading","current":50,"total":100}`,
		`{"id":"Container web-web-1","status":"Done","text":"Started"}`,
	}
	env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 2 seconds", "", time.Now().Unix(),
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"})
	var raw []string
	env.svc.rawLine = func(_, line string) { raw = append(raw, line) }

	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{Volumes: true})
	if err != nil {
		t.Fatal(err)
	}
	if op.State != StateQueued && op.State != StateRunning {
		t.Fatalf("Start returned state %q, want queued or running", op.State)
	}
	if !strings.HasPrefix(op.ID, "op_") || len(op.ID) != 19 || op.Options.Volumes {
		t.Fatalf("operation = %+v, want an op_ id of 16 hex characters and volumes ignored for up", op)
	}
	_, events, cancel, err := env.svc.Subscribe(op.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	var types []string
	var progress []Progress
	for ev := range events {
		types = append(types, ev.Type)
		if ev.Type == EventProgress {
			progress = append(progress, *ev.Progress)
		}
	}
	if len(types) == 0 || types[len(types)-1] != EventDone {
		t.Fatalf("event types = %v, want a stream that ends with done", types)
	}

	done := waitDone(t, env.svc, op.ID)
	if done.State != StateSucceeded || done.ExitCode == nil || *done.ExitCode != 0 || done.Code != "" || done.Error != "" || done.Warning != "" {
		t.Fatalf("done = %+v", done)
	}
	if done.StartedAt == nil || done.FinishedAt == nil || done.RunnerID == "" {
		t.Fatalf("timestamps / runner id = %+v", done)
	}
	if len(done.Ports) != 1 || done.Ports[0].PublicPort != 8080 || done.Ports[0].Service != "web" {
		t.Fatalf("ports = %+v", done.Ports)
	}
	for _, want := range []string{"Network web_default Working Creating", "Container web-web-1 Done Started"} {
		if !strings.Contains(done.Output, want) {
			t.Fatalf("output lacks %q:\n%s", want, done.Output)
		}
	}
	if strings.Contains(done.Output, "Downloading") {
		t.Fatalf("a byte count must not become an output line:\n%s", done.Output)
	}
	if len(progress) != 3 || progress[1].Parent != "Image nginx" || progress[1].Total != 100 || progress[1].Percent != 50 {
		t.Fatalf("progress events = %+v, want the three items, the second at 50%%", progress)
	}
	if strings.Join(raw, "\n") != strings.Join(env.fe.runnerLines, "\n") {
		t.Fatalf("raw lines = %q", raw)
	}
	waitUntil(t, "the runner to be removed once its result is recorded", 5*time.Second, func() bool {
		return len(env.fe.containersWithRole(RoleRunner)) == 0
	})
	// The row in the journal is the final state, and the stack is free again.
	row, _ := env.svc.journal.get(context.Background(), op.ID)
	if row.State != StateSucceeded || row.Output != done.Output {
		t.Fatalf("journal row = %+v", row)
	}
	if env.svc.activeRun(testHostID, "web") != nil {
		t.Fatal("the stack must be free after the operation")
	}
}

func TestTheRunnerIsCreatedFromTheGoldenPayloadAndStartedOnce(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	gate := make(chan struct{})
	env.fe.runnerGate = gate
	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the runner", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleRunner)) == 1 })
	runner := env.fe.containersWithRole(RoleRunner)[0]
	if runner.name != "aurago-dm-op-"+op.ID {
		t.Fatalf("runner name = %q", runner.name)
	}
	want, _ := runnerSpec(RunnerImage, env.host, Operation{ID: op.ID, Stack: "web", Kind: KindUp}, "compose.yaml")
	assertSameJSON(t, env.fe.specOf(runner), mustJSON(want))
	waitUntil(t, "the runner to be started", 5*time.Second, func() bool { return env.fe.isRunning(runner) })
	waitUntil(t, "the running state with the runner id", 5*time.Second, func() bool {
		cur, _ := env.svc.Operation(context.Background(), op.ID)
		return cur.State == StateRunning && cur.RunnerID == runner.id
	})
	if env.fe.requestCount("POST /containers/"+runner.id+"/start") != 1 {
		t.Fatalf("the runner must be started exactly once: %v", env.fe.requests())
	}
	close(gate)
	if done := waitDone(t, env.svc, op.ID); done.State != StateSucceeded {
		t.Fatalf("done = %+v", done)
	}
}

func TestEveryKindRunsItsOwnScript(t *testing.T) {
	cases := []struct {
		kind    string
		volumes bool
		script  string
	}{
		{KindConfig, false, scriptConfig},
		{KindUp, false, scriptUp},
		{KindDown, false, scriptDown},
		{KindDown, true, scriptDownVolumes},
		{KindStop, false, scriptStop},
		{KindRestart, false, scriptRestart},
		{KindPull, false, scriptPull},
		{KindUpdate, false, scriptUpdate},
		{KindDelete, false, scriptDelete},
		{KindDelete, true, scriptDeleteVols},
	}
	for _, c := range cases {
		env := newTestEnv(t, true)
		seedComposeStack(env, "web")
		gate := make(chan struct{})
		env.fe.runnerGate = gate
		op, err := env.svc.Start(context.Background(), testHostID, "web", c.kind, OpOptions{Volumes: c.volumes})
		if err != nil {
			t.Fatal(err)
		}
		waitUntil(t, c.kind+" runner", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleRunner)) == 1 })
		cmd := env.fe.specOf(env.fe.containersWithRole(RoleRunner)[0])["Cmd"].([]any)
		if cmd[0] != c.script {
			t.Errorf("%s volumes=%v: script = %q, want %q", c.kind, c.volumes, cmd[0], c.script)
		}
		close(gate)
		waitDone(t, env.svc, op.ID)
	}
}

func TestASecondOperationOnTheSameStackIsBusy(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	seedComposeStack(env, "db")
	gate := make(chan struct{})
	env.fe.runnerGate = gate
	ctx := context.Background()

	first, err := env.svc.Start(ctx, testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.Start(ctx, testHostID, "web", KindDown, OpOptions{})
	var busy *BusyError
	if !errors.As(err, &busy) || busy.OperationID != first.ID || busy.Kind != KindUp || CodeOf(err) != CodeOperationBusy {
		t.Fatalf("second Start = %v, want a BusyError naming %s (up)", err, first.ID)
	}
	other, err := env.svc.Start(ctx, testHostID, "db", KindUp, OpOptions{})
	if err != nil {
		t.Fatalf("an operation on another stack must not be blocked: %v", err)
	}
	if _, err := env.svc.Prepare(ctx, testHostID); err != nil {
		t.Fatalf("prepare locks the host, not a stack: %v", err)
	}
	if _, err := env.svc.Prepare(ctx, testHostID); CodeOf(err) != CodeOperationBusy {
		t.Fatalf("a second prepare = %v, want busy", err)
	}
	listing, err := env.svc.List(ctx, testHostID, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range listing.Stacks {
		if s.Operation == nil || s.Operation.Kind != KindUp {
			t.Fatalf("stack %s should show its running operation, got %+v", s.Name, s.Operation)
		}
	}
	close(gate)
	waitDone(t, env.svc, first.ID)
	waitDone(t, env.svc, other.ID)
	if _, err := env.svc.Start(ctx, testHostID, "web", KindDown, OpOptions{}); err != nil {
		t.Fatalf("after the first finished the stack is free: %v", err)
	}
}

func TestAFailedRunnerNamesTheLineOfTheComposeFile(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	env.fe.runnerLines = []string{`{"id":"Network web_default","status":"Working","text":"Creating"}`}
	env.fe.runnerErr = "yaml: line 12: did not find expected key\n"
	env.fe.runnerExit = 15

	done := startAndWait(t, env, "web", KindUp, OpOptions{})
	if done.State != StateFailed || done.Code != opCodeRunnerFailed || done.ExitCode == nil || *done.ExitCode != 15 {
		t.Fatalf("done = %+v", done)
	}
	if done.Error != "yaml: line 12: did not find expected key" || done.ErrorLine != 12 {
		t.Fatalf("error = %q line %d", done.Error, done.ErrorLine)
	}
	if len(done.Ports) != 0 || done.Warning != "" {
		t.Fatalf("a failed up has no ports: %+v", done)
	}
	waitUntil(t, "the runner of a failed operation to be removed too", 5*time.Second, func() bool {
		return len(env.fe.containersWithRole(RoleRunner)) == 0
	})
	if !strings.Contains(done.Output, "Network web_default Working Creating") {
		t.Fatalf("output = %q", done.Output)
	}

	env.fe.runnerErr, env.fe.runnerExit = "", 3
	silent := startAndWait(t, env, "web", KindStop, OpOptions{})
	if silent.State != StateFailed || silent.Error != "docker compose exited with code 3." || silent.ErrorLine != 0 {
		t.Fatalf("a runner that explains nothing = %+v, want the exit code as the message", silent)
	}
}

func TestStartRefusalsAndMissingStacks(t *testing.T) {
	env := newTestEnv(t, true)
	ctx := context.Background()
	if _, err := env.svc.Start(ctx, testHostID, "web", "explode", OpOptions{}); CodeOf(err) != CodeInvalidOperation {
		t.Fatalf("unknown kind = %v", err)
	}
	if _, err := env.svc.Start(ctx, testHostID, "", KindPrepare, OpOptions{}); CodeOf(err) != CodeInvalidOperation {
		t.Fatalf("prepare through Start = %v", err)
	}
	if _, err := env.svc.Start(ctx, testHostID, "../x", KindUp, OpOptions{}); CodeOf(err) != CodeStackNameInvalid {
		t.Fatalf("bad name = %v", err)
	}
	if _, err := env.svc.Start(ctx, "h_unknown", "web", KindUp, OpOptions{}); !errors.Is(err, dockerhost.ErrHostNotFound) {
		t.Fatalf("unknown host = %v", err)
	}
	if n := env.fe.requestCount("POST /containers/create"); n != 0 {
		t.Fatalf("refused starts must not touch the host, %d containers created", n)
	}

	missing := startAndWait(t, env, "ghost", KindUp, OpOptions{})
	if missing.State != StateFailed || missing.Code != CodeStackNotFound || !strings.Contains(missing.Error, "ghost") {
		t.Fatalf("up of a stack without a compose file = %+v", missing)
	}
	gone := startAndWait(t, env, "ghost", KindDelete, OpOptions{})
	if gone.State != StateFailed || gone.Code != CodeStackNotFound {
		t.Fatalf("delete of a stack without a folder = %+v", gone)
	}
	if env.fe.requestCount("POST /containers/create") != 0 {
		t.Fatal("no runner for a stack that does not exist")
	}
}

func TestAnOperationOnAnUnpreparedHostPreparesItFirst(t *testing.T) {
	env := newTestEnv(t, false)
	seedComposeStack(env, "web")
	done := startAndWait(t, env, "web", KindConfig, OpOptions{})
	if done.State != StateSucceeded {
		t.Fatalf("done = %+v\n%s", done, done.Output)
	}
	if !strings.Contains(done.Output, "Pulling the helper image") || !strings.Contains(done.Output, "Creating the IO container") {
		t.Fatalf("the implicit preparation must show in the output:\n%s", done.Output)
	}
	if !env.fe.hasImage(RunnerImage) || env.fe.container(IOContainerName) == nil {
		t.Fatal("host not prepared")
	}
}

func TestPrepareOperationPreparesTheHostAndWarmsTheScan(t *testing.T) {
	env := newTestEnv(t, false)
	seedComposeStack(env, "web")
	op, err := env.svc.Prepare(context.Background(), testHostID)
	if err != nil {
		t.Fatal(err)
	}
	if op.Kind != KindPrepare || op.Stack != "" {
		t.Fatalf("operation = %+v", op)
	}
	done := waitDone(t, env.svc, op.ID)
	if done.State != StateSucceeded {
		t.Fatalf("done = %+v\n%s", done, done.Output)
	}
	for _, want := range []string{"Docker Engine 27.3.1, API 1.45", "Pulling the helper image", "Creating the IO container", "Checking the stacks directory /opt/stacks", "Ready: 1 stack folder(s) found in /opt/stacks"} {
		if !strings.Contains(done.Output, want) {
			t.Fatalf("output lacks %q:\n%s", want, done.Output)
		}
	}
	if st, _ := env.svc.Helpers(context.Background(), testHostID); !st.Prepared {
		t.Fatalf("helpers = %+v", st)
	}
	listing, err := env.svc.List(context.Background(), testHostID, false)
	if err != nil || !listing.Cached || len(listing.Stacks) != 1 {
		t.Fatalf("the scan of the prepare operation must serve the next List: cached=%v err=%v", listing.Cached, err)
	}
	// Preparing again is cheap and says what it found.
	again := startPrepare(t, env)
	if again.State != StateSucceeded || !strings.Contains(again.Output, "is present") {
		t.Fatalf("second prepare = %+v", again)
	}
	if env.fe.requestCount("POST /images/create") != 1 {
		t.Fatal("the image is pulled once")
	}
}

func startPrepare(t *testing.T, env *testEnv) Operation {
	t.Helper()
	op, err := env.svc.Prepare(context.Background(), testHostID)
	if err != nil {
		t.Fatal(err)
	}
	return waitDone(t, env.svc, op.ID)
}

func TestPrepareFailureOnAnOldEngineAndAPullThatCannotBeCopied(t *testing.T) {
	env := newTestEnv(t, false)
	env.fe.setVersion("1.40", "linux", "amd64")
	done := startPrepare(t, env)
	if done.State != StateFailed || done.Code != CodeAPITooOld {
		t.Fatalf("prepare on API 1.40 = %+v", done)
	}

	env = newTestEnv(t, false)
	env.fe.pullFails = "dial tcp: lookup registry-1.docker.io: no such host"
	done = startPrepare(t, env)
	wantCmd := "docker pull docker:29.9.2-cli@sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb"
	if done.State != StateFailed || done.Code != CodeRunnerImageMissing || !strings.Contains(done.Error, wantCmd) {
		t.Fatalf("prepare without a registry = %+v, want %s naming the command", done, CodeRunnerImageMissing)
	}
}

func TestPrepareReportsAStacksDirectoryTheHostCannotBind(t *testing.T) {
	env := newTestEnv(t, false)
	env.fe.startFails = "invalid mount config for type \"bind\": bind source path does not exist: /opt/stacks"
	done := startPrepare(t, env)
	if done.State != StateFailed || done.Code != CodeStacksDirInvalid || !strings.Contains(done.Error, "/opt/stacks") {
		t.Fatalf("prepare = %+v", done)
	}
}

func TestHealthWaitWatchesServicesUntilTheyAreHealthy(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	web := env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 2 seconds", "starting", time.Now().Unix(),
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"})
	op, err := env.svc.Start(context.Background(), testHostID, "web", KindUp, OpOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitUntil(t, "the health_wait state", 5*time.Second, func() bool {
		cur, _ := env.svc.Operation(context.Background(), op.ID)
		return cur.State == StateHealthWait
	})
	cur, _ := env.svc.Operation(context.Background(), op.ID)
	if cur.ExitCode == nil || *cur.ExitCode != 0 {
		t.Fatalf("the exit code is recorded before the health wait: %+v", cur)
	}
	if left := env.fe.containersWithRole(RoleRunner); len(left) != 0 {
		t.Fatal("the runner is removed once its result is recorded, before the health wait")
	}
	env.fe.mu.Lock()
	web.health = "healthy"
	env.fe.mu.Unlock()

	done := waitDone(t, env.svc, op.ID)
	if done.State != StateSucceeded || done.Warning != "" || len(done.Ports) != 1 {
		t.Fatalf("done = %+v", done)
	}
}

func TestHealthWaitEndsWithAWarning(t *testing.T) {
	cases := []struct {
		name    string
		prepare func(env *testEnv)
		warning string
	}{
		{"unhealthy", func(env *testEnv) {
			env.fe.addProjectContainer("web-db-1", "web", "db", "running", "Up", "unhealthy", time.Now().Unix())
		}, "db is unhealthy"},
		{"never healthy", func(env *testEnv) {
			env.fe.addProjectContainer("web-api-1", "web", "api", "running", "Up", "starting", time.Now().Unix())
		}, "api did not become healthy within 150ms"},
		{"crashed", func(env *testEnv) {
			env.fe.addProjectContainer("web-job-1", "web", "job", "exited", "Exited (2) 1 second ago", "", time.Now().Unix())
			env.fe.addProjectContainer("web-ok-1", "web", "ok", "exited", "Exited (0) 1 second ago", "", time.Now().Unix())
		}, "job exited with code 2"},
		{"crash loop", func(env *testEnv) {
			env.fe.addProjectContainer("web-w-1", "web", "w", "restarting", "Restarting (1) 2 seconds ago", "", time.Now().Unix())
		}, "w keeps restarting"},
	}
	for _, c := range cases {
		env := newTestEnv(t, true)
		env.svc.opts.HealthWait = 150 * time.Millisecond
		seedComposeStack(env, "web")
		c.prepare(env)
		done := startAndWait(t, env, "web", KindUp, OpOptions{})
		if done.State != StateSucceeded || done.Warning != c.warning {
			t.Errorf("%s: state %s warning %q, want succeeded with %q", c.name, done.State, done.Warning, c.warning)
		}
	}
}

func TestAssessHealth(t *testing.T) {
	mk := func(service, state, status, health string) dockerhost.ContainerSummary {
		return dockerhost.ContainerSummary{ComposeService: service, State: state, Status: status, Health: health}
	}
	cases := []struct {
		name     string
		members  []dockerhost.ContainerSummary
		timedOut bool
		settled  bool
		warning  string
	}{
		{"nothing", nil, false, true, ""},
		{"no health checks", []dockerhost.ContainerSummary{mk("a", "running", "Up", "")}, false, true, ""},
		{"healthy", []dockerhost.ContainerSummary{mk("a", "running", "Up", "healthy")}, false, true, ""},
		{"starting is not settled", []dockerhost.ContainerSummary{mk("a", "running", "Up", "starting")}, false, false, ""},
		{"starting after the time", []dockerhost.ContainerSummary{mk("a", "running", "Up", "starting"), mk("b", "running", "Up", "starting")}, true, true, "a, b did not become healthy within 1s"},
		{"two unhealthy", []dockerhost.ContainerSummary{mk("a", "running", "Up", "unhealthy"), mk("b", "running", "Up", "unhealthy")}, false, true, "a, b are unhealthy"},
		{"restarting is not settled", []dockerhost.ContainerSummary{mk("a", "restarting", "Restarting", "")}, false, false, ""},
		{"exit 0 is fine, dead is not", []dockerhost.ContainerSummary{mk("a", "exited", "Exited (0) 1s ago", ""), mk("b", "dead", "Dead", "")}, false, true, "b is dead"},
	}
	for _, c := range cases {
		settled, warning := assessHealth(c.members, c.timedOut, time.Second)
		if settled != c.settled || warning != c.warning {
			t.Errorf("%s: (%v, %q), want (%v, %q)", c.name, settled, warning, c.settled, c.warning)
		}
	}
}

func TestAnOperationThatRunsTooLongIsKilledAndCalledATimeout(t *testing.T) {
	env := newTestEnv(t, true)
	env.svc.opts.MaxDuration = 300 * time.Millisecond
	seedComposeStack(env, "web")
	gate := make(chan struct{})
	env.fe.runnerGate = gate
	t.Cleanup(func() { close(gate) })

	done := startAndWait(t, env, "web", KindPull, OpOptions{})
	if done.State != StateFailed || done.Code != opCodeTimeout || !strings.Contains(done.Error, "took longer than") {
		t.Fatalf("done = %+v", done)
	}
	runnerKills := 0
	for _, r := range env.fe.requests() {
		if strings.HasPrefix(r, "POST /v1.45/containers/") && strings.Contains(r, "/kill") {
			runnerKills++
		}
	}
	if runnerKills != 1 {
		t.Fatalf("the runner must be killed once: %v", env.fe.requests())
	}
	waitUntil(t, "the runner to be removed", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleRunner)) == 0 })
}

func TestAnUnreachableHostFailsTheOperationWithAClassifiedReason(t *testing.T) {
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	env.hosts.setDown(testHostID, true)
	done := startAndWait(t, env, "web", KindUp, OpOptions{})
	if done.State != StateFailed || done.Code != opCodeHostUnreachable || !strings.Contains(done.Error, "refused") {
		t.Fatalf("done = %+v, want host_unreachable with the reason refused", done)
	}
}

func TestRunnerLogStreamThatDropsIsPickedUpWithoutRepeatingLines(t *testing.T) {
	run := registerRun(t, newTestService(t, newFakeHosts(), nil), Operation{ID: "op_0123456789abcdef", HostID: "h", Stack: "web", Kind: KindUp})
	sink := &replay{run: run}
	for _, l := range []string{"one", "two"} {
		sink.emit(l)
	}
	sink.seen = 0 // the stream is read again from its start
	for _, l := range []string{"one", "two", "three"} {
		sink.emit(l)
	}
	if got := run.snapshot().Output; got != "one\ntwo\nthree\n" {
		t.Fatalf("output = %q, want each line once", got)
	}
}
