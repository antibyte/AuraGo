package cydcreator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
	"unicode/utf8"
)

func TestBuildRunsAndStoresArtifacts(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Build")
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatalf("StartBuild: %v", err)
	}
	if b.Status != BuildQueued || b.Origin != BuildOriginUser || b.Board != "esp32-2432s028r" || b.ProjectVersion != 1 || b.JobID != "" {
		t.Fatalf("queued build = %+v", b)
	}
	done := waitBuild(t, env.svc, b.ID)
	if done.Status != BuildSucceeded || done.ErrorCode != "" || done.StartedAt == "" || done.FinishedAt == "" {
		t.Fatalf("finished build = %+v", done)
	}
	if done.Usage != (BuildUsage{FlashUsed: 1000, FlashTotal: 1966080, RAMUsed: 200, RAMTotal: 327680, DurationMS: 1234, Warnings: 1}) {
		t.Fatalf("usage = %+v", done.Usage)
	}
	var names []string
	for _, a := range done.Artifacts {
		names = append(names, a.Name)
	}
	if !slices.Equal(names, []string{ArtifactMergedBin, ArtifactFirmwareBin, ArtifactResultJSON, ArtifactBuildLog}) {
		t.Fatalf("artifacts = %v (firmware.elf must stay internal)", names)
	}
	if len(done.Diagnostics) != 1 || done.Diagnostics[0].File != "src/app/app_main.cpp" || done.Diagnostics[0].Severity != SeverityWarning {
		t.Fatalf("diagnostics = %+v", done.Diagnostics)
	}
	req := env.builder.request(0)
	if req.BuildID != b.ID || req.Env != "esp32-2432s028r" || req.CacheKey != p.ID+"-esp32-2432s028r" ||
		string(req.Tree["src/generated/ui.cpp"]) != "// Build\n" || len(req.Tree["src/app/app_main.cpp"]) == 0 {
		t.Fatalf("build request = %+v", req)
	}
	r, info, err := env.svc.OpenArtifact(ctx, b.ID, ArtifactMergedBin)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(r)
	_ = r.Close()
	if string(data) != "merged-"+b.ID || info.SHA256 != sha256Hex(data) || info.Size != int64(len(data)) {
		t.Fatalf("merged.bin = %q %+v", data, info)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts", b.ID, ArtifactFirmwareELF)); err != nil {
		t.Fatalf("firmware.elf must be stored for backtraces: %v", err)
	}
	_, _, err = env.svc.OpenArtifact(ctx, b.ID, ArtifactFirmwareELF)
	wantCode(t, err, CodeNotFound)
	log, err := env.svc.BuildLog(ctx, b.ID)
	if err != nil || !strings.Contains(log, "warning: unused variable") {
		t.Fatalf("BuildLog = %q, %v", log, err)
	}
	summary, _ := env.svc.GetProject(ctx, p.ID)
	if summary.LastBuild == nil || summary.LastBuild.ID != b.ID {
		t.Fatalf("last_build = %+v", summary.LastBuild)
	}
	statuses := collectEvents(t, events, 3, 2*time.Second, EventBuildStatus)
	var seen []string
	for _, ev := range statuses {
		seen = append(seen, eventBuild(t, ev).Status)
	}
	if !slices.Equal(seen, []string{BuildQueued, BuildRunning, BuildSucceeded}) {
		t.Fatalf("build_status sequence = %v", seen)
	}
	list, err := env.svc.ListBuilds(ctx, p.ID)
	if err != nil || len(list) != 1 || list[0].ID != b.ID {
		t.Fatalf("ListBuilds = %+v, %v", list, err)
	}
}

func TestBuildQueueRunsOneBuildAtATimeInOrder(t *testing.T) {
	env := newTestService(t)
	var running, maxRunning atomic.Int32
	release := make(chan struct{})
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		n := running.Add(1)
		for {
			old := maxRunning.Load()
			if n <= old || maxRunning.CompareAndSwap(old, n) {
				break
			}
		}
		<-release
		running.Add(-1)
		return successOutput(req), nil
	}
	var ids []string
	for i := 0; i < 3; i++ {
		p := env.createProject(t, fmt.Sprintf("Queue %d", i))
		b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		ids = append(ids, b.ID)
	}
	if n := env.svc.activeBuilds(); n != 3 {
		t.Fatalf("active builds = %d, want 3", n)
	}
	close(release)
	for _, id := range ids {
		if b := waitBuild(t, env.svc, id); b.Status != BuildSucceeded {
			t.Fatalf("build %s = %s", id, b.Status)
		}
	}
	if maxRunning.Load() != 1 {
		t.Fatalf("%d builds ran at the same time, want 1", maxRunning.Load())
	}
	for i, id := range ids {
		if got := env.builder.request(i).BuildID; got != id {
			t.Fatalf("build %d ran as %s, want FIFO order %v", i, got, ids)
		}
	}
}

func TestCancelQueuedAndRunningBuilds(t *testing.T) {
	env := newTestService(t)
	started := make(chan struct{}, 1)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		started <- struct{}{}
		<-ctx.Done()
		return BuildOutput{}, ctx.Err()
	}
	p := env.createProject(t, "Cancel")
	ctx := context.Background()
	first, _ := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	<-started
	second, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err := env.svc.CancelBuild(ctx, second.ID); err != nil {
		t.Fatal(err)
	}
	if b, _ := env.svc.GetBuild(ctx, second.ID); b.Status != BuildCancelled || b.ErrorCode != CodeBuildCancelled {
		t.Fatalf("queued build after cancel = %+v", b)
	}
	if err := env.svc.CancelBuild(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	b := waitBuild(t, env.svc, first.ID)
	if b.Status != BuildCancelled || b.ErrorCode != CodeBuildCancelled {
		t.Fatalf("running build after cancel = %+v", b)
	}
	if env.builder.requestCount() != 1 {
		t.Fatalf("the cancelled queued build reached the builder")
	}
	if err := env.svc.CancelBuild(ctx, first.ID); err != nil {
		t.Fatalf("cancelling a finished build must be a no-op: %v", err)
	}
	wantCode(t, env.svc.CancelBuild(ctx, "ccb_missing"), CodeNotFound)
	if log, _ := env.svc.BuildLog(ctx, first.ID); log != "" {
		t.Fatalf("log of a silent build = %q", log)
	}
}

func TestBuildTimeoutColdAndWarm(t *testing.T) {
	env := newTestService(t, testServiceOptions{limits: ServiceLimits{BuildTimeout: 2 * time.Second, ColdBuildTimeout: 3 * time.Second}})
	ctx := context.Background()
	p := env.createProject(t, "Timeout")
	var fail atomic.Bool
	fail.Store(true)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		if fail.Load() {
			return BuildOutput{ExitCode: 1, Log: "src/app/app_main.cpp:1:1: error: boom"}, nil
		}
		return successOutput(req), nil
	}
	run := func() BuildRequest {
		t.Helper()
		b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		waitBuild(t, env.svc, b.ID)
		return env.builder.request(env.builder.requestCount() - 1)
	}
	if req := run(); req.Timeout != 3*time.Second {
		t.Fatalf("first build timeout = %v, want the cold 3s", req.Timeout)
	}
	if req := run(); req.Timeout != 3*time.Second {
		t.Fatalf("a failed build must not warm the cache: %v", req.Timeout)
	}
	fail.Store(false)
	run()
	if req := run(); req.Timeout != 2*time.Second {
		t.Fatalf("build after a success = %v, want the warm 2s", req.Timeout)
	}

	env.svc.UpdateLimits(ServiceLimits{BuildTimeout: 50 * time.Millisecond, ColdBuildTimeout: 50 * time.Millisecond})
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		onLog("Compiling…")
		<-ctx.Done()
		return BuildOutput{}, ctx.Err()
	}
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	done := waitBuild(t, env.svc, b.ID)
	if done.Status != BuildFailed || done.ErrorCode != CodeBuildTimeout {
		t.Fatalf("timed out build = %+v", done)
	}
	if log, _ := env.svc.BuildLog(ctx, b.ID); !strings.Contains(log, "Compiling…") {
		t.Fatalf("log of the timed out build = %q", log)
	}
}

func TestBuildOutcomeMapping(t *testing.T) {
	appError := "/work/%s/src/app/app_main.cpp:7:3: error: 'foo' was not declared in this scope\n"
	genError := "/work/%s/src/generated/ui.cpp:12:1: error: expected ';' before '}' token\n"
	cases := []struct {
		name      string
		out       func(req BuildRequest) (BuildOutput, error)
		status    string
		code      string
		note      string
		artifacts []string
	}{
		{"compile error in app code", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{ExitCode: 1, Log: fmt.Sprintf(appError, req.BuildID)}, nil
		}, BuildFailed, CodeBuildFailed, "", []string{ArtifactBuildLog}},
		{"compile error in generated code", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{ExitCode: 1, Log: fmt.Sprintf(genError, req.BuildID)}, nil
		}, BuildFailed, CodeGeneratorError, "", []string{ArtifactBuildLog}},
		{"app and generated errors", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{ExitCode: 1, Log: fmt.Sprintf(genError+appError, req.BuildID, req.BuildID)}, nil
		}, BuildFailed, CodeBuildFailed, "", []string{ArtifactBuildLog}},
		{"tooling error", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{ExitCode: 2, Log: "pio: internal error"}, nil
		}, BuildFailed, CodeBuildFailed, "exit code 2", []string{ArtifactBuildLog}},
		{"flash budget", func(req BuildRequest) (BuildOutput, error) {
			out := successOutput(req)
			out.ExitCode, out.Files = 3, nil
			out.Result.OK, out.Result.Exit = false, 3
			return out, nil
		}, BuildFailed, CodeFlashBudgetExceeded, "", []string{ArtifactResultJSON, ArtifactBuildLog}},
		{"unknown exit code", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{ExitCode: 137}, nil
		}, BuildFailed, CodeBuildFailed, "exit code 137", []string{ArtifactBuildLog}},
		{"success without elf", func(req BuildRequest) (BuildOutput, error) {
			out := successOutput(req)
			delete(out.Files, ArtifactFirmwareELF)
			return out, nil
		}, BuildFailed, CodeBuildFailed, "firmware.elf", []string{ArtifactResultJSON, ArtifactBuildLog}},
		{"checksum mismatch", func(req BuildRequest) (BuildOutput, error) {
			out := successOutput(req)
			out.Files[ArtifactMergedBin] = []byte("tampered")
			return out, nil
		}, BuildFailed, CodeBuildFailed, "could not be stored", []string{ArtifactResultJSON, ArtifactBuildLog}},
		{"image missing", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{}, NewError(CodeImageMissing, "the toolchain image is missing", nil)
		}, BuildFailed, CodeImageMissing, "toolchain image is missing", []string{ArtifactBuildLog}},
		{"untyped builder failure", func(req BuildRequest) (BuildOutput, error) {
			return BuildOutput{}, errors.New("socket closed")
		}, BuildFailed, CodeBuildFailed, "server log", []string{ArtifactBuildLog}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestService(t)
			env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
				return tc.out(req)
			}
			p := env.createProject(t, "Outcome")
			b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			done := waitBuild(t, env.svc, b.ID)
			if done.Status != tc.status || done.ErrorCode != tc.code {
				t.Fatalf("build = %s/%s, want %s/%s (diagnostics %+v)", done.Status, done.ErrorCode, tc.status, tc.code, done.Diagnostics)
			}
			if tc.note != "" {
				found := false
				for _, d := range done.Diagnostics {
					found = found || strings.Contains(d.Message, tc.note)
				}
				if !found {
					t.Fatalf("diagnostics %+v lack a note containing %q", done.Diagnostics, tc.note)
				}
			}
			var names []string
			for _, a := range done.Artifacts {
				names = append(names, a.Name)
			}
			if !slices.Equal(names, tc.artifacts) {
				t.Fatalf("artifacts = %v, want %v", names, tc.artifacts)
			}
		})
	}
}

func TestBuildLogEventsAreThrottled(t *testing.T) {
	env := newTestService(t)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		for i := 0; i < 80; i++ {
			onLog(fmt.Sprintf("line %03d", i))
		}
		time.Sleep(450 * time.Millisecond)
		for i := 80; i < 400; i++ {
			onLog(fmt.Sprintf("line %03d", i))
		}
		out := successOutput(req)
		out.Log = "" // the service then keeps the lines it received
		return out, nil
	}
	p := env.createProject(t, "Logs")
	events, cancel := env.svc.Subscribe(p.ID)
	defer cancel()
	b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	type received struct {
		at    time.Time
		lines []string
	}
	var logs []received
	deadline := time.After(10 * time.Second)
	for finished := false; !finished; {
		select {
		case ev := <-events:
			switch ev.Type {
			case EventBuildLog:
				var data struct {
					BuildID string   `json:"build_id"`
					Lines   []string `json:"lines"`
				}
				if err := json.Unmarshal(ev.Data, &data); err != nil || data.BuildID != b.ID || ev.Seq != 0 {
					t.Fatalf("build_log event = %+v (%v)", ev, err)
				}
				logs = append(logs, received{time.Now(), data.Lines})
			case EventBuildStatus:
				finished = eventBuild(t, ev).Status == BuildSucceeded
			}
		case <-deadline:
			t.Fatal("build did not finish")
		}
	}
	if len(logs) < 2 {
		t.Fatalf("got %d build_log events, want several", len(logs))
	}
	skipped := false
	for i, l := range logs {
		if len(l.lines) > buildLogMaxLines {
			t.Fatalf("event %d has %d lines", i, len(l.lines))
		}
		skipped = skipped || strings.Contains(l.lines[0], "lines skipped")
		if i > 0 && l.at.Sub(logs[i-1].at) < buildLogInterval-40*time.Millisecond {
			t.Fatalf("events %d and %d are only %v apart", i-1, i, l.at.Sub(logs[i-1].at))
		}
	}
	if !skipped {
		t.Fatal("a burst above 50 lines must carry the skipped marker")
	}
	last := logs[len(logs)-1].lines
	if last[len(last)-1] != "line 399" {
		t.Fatalf("last event ends with %q, want the newest line", last[len(last)-1])
	}
	if stored, _ := env.svc.BuildLog(context.Background(), b.ID); !strings.Contains(stored, "line 000") {
		t.Fatal("the stored log must stay complete")
	}
}

func TestBuildRetentionKeepsTheNewestBuilds(t *testing.T) {
	env := newTestService(t, testServiceOptions{limits: ServiceLimits{KeepBuildsPerProject: 2}})
	ctx := context.Background()
	p := env.createProject(t, "Retention")
	var ids []string
	for i := 0; i < 4; i++ {
		b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		waitBuild(t, env.svc, b.ID)
		ids = append(ids, b.ID)
		if i == 0 {
			if _, err := env.svc.db.Exec(`INSERT INTO cc_devices(id,project_id,name,board,status,token_hash,target_build_id,created_at)
				VALUES('ccd_1',?,'Device','esp32-2432s028r','active','x',?,'2026-10-10T08:00:00.000Z')`, p.ID, b.ID); err != nil {
				t.Fatal(err)
			}
		}
	}
	artifacts := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts")
	for i, id := range ids {
		_, err := env.svc.GetBuild(ctx, id)
		_, statErr := os.Stat(filepath.Join(artifacts, id))
		keep := i == 0 || i >= 2 // device target, then the two newest
		if keep && (err != nil || statErr != nil) {
			t.Errorf("build %d (%s) should be kept: %v / %v", i, id, err, statErr)
		}
		if !keep && (ErrorCode(err) != CodeNotFound || !os.IsNotExist(statErr)) {
			t.Errorf("build %d (%s) should be deleted: %v / %v", i, id, err, statErr)
		}
	}
}

func TestStartBuildPreconditions(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Pre")
	env.builder.setStatus(ToolchainStatus{DockerAvailable: false, Error: "no socket"})
	_, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	wantCode(t, err, CodeDockerUnavailable)
	env.builder.setStatus(ToolchainStatus{DockerAvailable: true, Image: "x"})
	_, err = env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	wantCode(t, err, CodeImageMissing)
	env.builder.setStatus(ToolchainStatus{DockerAvailable: true, Image: "x", Present: true, Pulling: true})
	_, err = env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	wantCode(t, err, CodeImageMissing)
	env.builder.setStatus(ToolchainStatus{DockerAvailable: true, Image: "x", Present: true})

	layout, version, _, _ := env.svc.GetLayout(ctx, p.ID)
	layout.Screens[0].Widgets = []Widget{{ID: "w_x", Type: "label", Name: "bad name", W: 10, H: 10}}
	if _, _, err := env.svc.PutLayout(ctx, p.ID, layout, version, OriginUser); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	wantCode(t, err, CodeInvalidLayout)
	if list, _ := env.svc.ListBuilds(ctx, p.ID); len(list) != 0 {
		t.Fatalf("an invalid layout created builds: %+v", list)
	}

	ok := env.createProject(t, "Busy")
	release := make(chan struct{})
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		<-release
		return successOutput(req), nil
	}
	defer close(release)
	for i := 0; i <= maxQueuedBuilds; i++ { // one running plus maxQueuedBuilds waiting
		if _, err := env.svc.StartBuild(ctx, ok.ID, BuildOptions{}); err != nil {
			t.Fatalf("build %d: %v", i, err)
		}
		if i == 0 {
			for env.builder.requestCount() == 0 {
				time.Sleep(5 * time.Millisecond)
			}
		}
	}
	_, err = env.svc.StartBuild(ctx, ok.ID, BuildOptions{})
	wantCode(t, err, CodeBusy)

	noBuilder, err := NewService(ServiceConfig{WorkspaceDir: t.TempDir(), DataDir: t.TempDir(), DBPath: filepath.Join(t.TempDir(), "x.db"), Generate: fakeGenerate})
	if err != nil {
		t.Fatal(err)
	}
	defer noBuilder.Close()
	_, err = noBuilder.StartBuild(ctx, "ccp_x", BuildOptions{})
	wantCode(t, err, CodeDockerUnavailable)
}

func TestCloseInterruptsRunningAndQueuedBuilds(t *testing.T) {
	dir := t.TempDir()
	env := newTestService(t, testServiceOptions{dir: dir})
	started := make(chan struct{}, 1)
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		started <- struct{}{}
		<-ctx.Done()
		return BuildOutput{}, ctx.Err()
	}
	p := env.createProject(t, "Shutdown")
	running, _ := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	<-started
	queued, _ := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	if err := env.svc.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(env.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	for _, id := range []string{running.ID, queued.ID} {
		b, err := restarted.GetBuild(context.Background(), id)
		if err != nil || b.Status != BuildInterrupted || b.ErrorCode != CodeBuildInterrupted {
			t.Fatalf("build %s after Close = %+v, %v", id, b, err)
		}
	}
}

func TestDecodeBacktrace(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	env.builder.frames = []Frame{{Address: "0x400d1234", Function: "app_loop()", File: "src/app/app_main.cpp", Line: 42}}
	p := env.createProject(t, "Trace")
	b, _ := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	waitBuild(t, env.svc, b.ID)
	_, err := env.svc.DecodeBacktrace(ctx, b.ID, nil)
	wantCode(t, err, CodeBadRequest)
	_, err = env.svc.DecodeBacktrace(ctx, b.ID, []string{"400d1234"})
	wantCode(t, err, CodeBadRequest)
	tooMany := make([]string, 65)
	for i := range tooMany {
		tooMany[i] = "0x1"
	}
	_, err = env.svc.DecodeBacktrace(ctx, b.ID, tooMany)
	wantCode(t, err, CodeBadRequest)
	frames, err := env.svc.DecodeBacktrace(ctx, b.ID, []string{"0x400d1234"})
	if err != nil || len(frames) != 1 || frames[0].Line != 42 {
		t.Fatalf("frames = %+v, %v", frames, err)
	}
	req := env.builder.a2l[0]
	if req.Chip != ChipESP32 || req.BuildID != b.ID || string(req.ELF) != "elf-"+b.ID || !slices.Equal(req.Addresses, []string{"0x400d1234"}) {
		t.Fatalf("addr2line request = %+v", req)
	}
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		return BuildOutput{ExitCode: 1}, nil
	}
	failed, _ := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	waitBuild(t, env.svc, failed.ID)
	_, err = env.svc.DecodeBacktrace(ctx, failed.ID, []string{"0x400d1234"})
	wantCode(t, err, CodeNotFound)
}

func TestPullToolchainStartsOnePull(t *testing.T) {
	env := newTestService(t)
	release := make(chan struct{})
	var once sync.Once
	env.builder.pull = func(ctx context.Context) error {
		select {
		case <-release:
		case <-ctx.Done():
		}
		return nil
	}
	defer once.Do(func() { close(release) })
	for i := 0; i < 3; i++ {
		if err := env.svc.PullToolchain(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	time.Sleep(50 * time.Millisecond)
	env.builder.mu.Lock()
	calls := env.builder.pullCalls
	env.builder.mu.Unlock()
	if calls != 1 {
		t.Fatalf("Pull ran %d times, want 1", calls)
	}
	once.Do(func() { close(release) })
	env.builder.setStatus(ToolchainStatus{})
	wantCode(t, env.svc.PullToolchain(context.Background()), CodeDockerUnavailable)
}

func TestWaitBuildHonoursTheContext(t *testing.T) {
	env := newTestService(t)
	release := make(chan struct{})
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		<-release
		return successOutput(req), nil
	}
	defer close(release)
	p := env.createProject(t, "Wait")
	b, _ := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if _, err := env.svc.WaitBuild(ctx, b.ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("WaitBuild = %v, want the context deadline", err)
	}
}

func TestAgentBuildUsesTheStagingTree(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Stage")
	staging := filepath.Join(t.TempDir(), "job")
	if err := os.CopyFS(staging, os.DirFS(filepath.Join(env.cfg.WorkspaceDir, "CYD", "stage"))); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "src", "app", "app_main.cpp"), []byte("// agent version\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	env.lockForJob(t, p.ID, "ccj_0042")
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{Origin: JobOrigin("ccj_0042"), Dir: staging})
	if err != nil {
		t.Fatal(err)
	}
	waitBuild(t, env.svc, b.ID)
	if got := string(env.builder.request(0).Tree["src/app/app_main.cpp"]); got != "// agent version\n" {
		t.Fatalf("built tree = %q, want the staging copy", got)
	}
}

func eventBuild(t *testing.T, ev Event) Build {
	t.Helper()
	var data struct {
		Build Build `json:"build"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		t.Fatalf("decode build event: %v", err)
	}
	return data.Build
}

func waitBuild(t *testing.T, svc *Service, id string) Build {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	b, err := svc.WaitBuild(ctx, id)
	if err != nil {
		t.Fatalf("WaitBuild(%s): %v", id, err)
	}
	return b
}

// fontOutput is successOutput plus the out/fonts/ files cyd-build copies.
func fontOutput(req BuildRequest) BuildOutput {
	out := successOutput(req)
	out.Result.Fonts = map[string]BuildResultArtifact{}
	for name, data := range map[string][]byte{
		"fonts.json":              []byte(`{"fonts":[]}` + "\n"),
		"ui_font_montserrat_14.c": []byte("// font of " + req.BuildID + "\n"),
	} {
		out.Files[ArtifactFontsDir+"/"+name] = data
		out.Result.Fonts[name] = BuildResultArtifact{Size: int64(len(data)), SHA256: sha256Hex(data)}
	}
	return out
}

func TestBuildKeepsConvertedFonts(t *testing.T) {
	env := newTestService(t)
	env.builder.build = func(_ context.Context, req BuildRequest, _ func(string)) (BuildOutput, error) {
		return fontOutput(req), nil
	}
	ctx := context.Background()
	p := env.createProject(t, "Fonts")
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	done := waitBuild(t, env.svc, b.ID)
	if done.Status != BuildSucceeded {
		t.Fatalf("build = %s %s", done.Status, done.ErrorCode)
	}
	for _, a := range done.Artifacts {
		if strings.HasPrefix(a.Name, ArtifactFontsDir) || a.Name == "fonts.json" {
			t.Fatalf("fonts must not be listed as downloadable artefacts: %+v", done.Artifacts)
		}
	}
	sha, files, err := env.svc.BuildFonts(ctx, b.ID)
	if err != nil {
		t.Fatal(err)
	}
	if sha != sha256Hex([]byte(`{"fonts":[]}`+"\n")) || len(files) != 1 || string(files["ui_font_montserrat_14.c"]) != "// font of "+b.ID+"\n" {
		t.Fatalf("BuildFonts = %q %v", sha, files)
	}
	if _, _, err := env.svc.OpenArtifact(ctx, b.ID, "fonts.json"); ErrorCode(err) != CodeNotFound {
		t.Fatalf("fonts.json must not be served: %v", err)
	}
	_, _, err = env.svc.BuildFonts(ctx, "ccb_missing")
	wantCode(t, err, CodeNotFound)
}

func TestBuildWithoutFontsReportsNone(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "NoFonts")
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitBuild(t, env.svc, b.ID)
	if sha, files, err := env.svc.BuildFonts(ctx, b.ID); err != nil || sha != "" || files != nil {
		t.Fatalf("BuildFonts without fonts = %q %v %v", sha, files, err)
	}
}

func TestBuildFontChecksumMismatchFails(t *testing.T) {
	env := newTestService(t)
	env.builder.build = func(_ context.Context, req BuildRequest, _ func(string)) (BuildOutput, error) {
		out := fontOutput(req)
		out.Files[ArtifactFontsDir+"/ui_font_montserrat_14.c"] = []byte("tampered")
		return out, nil
	}
	p := env.createProject(t, "BadFonts")
	b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if done := waitBuild(t, env.svc, b.ID); done.Status != BuildFailed || done.ErrorCode != CodeBuildFailed {
		t.Fatalf("build = %s %s", done.Status, done.ErrorCode)
	}
}

// TestStartBuildKeepsTypedGeneratorErrors: Generate refusals (invalid layout,
// bad asset data or app paths) reach the caller with their own code; only
// untyped failures become CYD_GENERATOR_ERROR. No build record is created.
func TestStartBuildKeepsTypedGeneratorErrors(t *testing.T) {
	for _, tc := range []struct {
		err  error
		want string
	}{
		{NewError(CodeBadRequest, "asset sun has no data", map[string]any{"asset": "sun"}), CodeBadRequest},
		{NewError(CodeInvalidLayout, "template version 9.0.0 is not supported", nil), CodeInvalidLayout},
		{fmt.Errorf("boom"), CodeGeneratorError},
	} {
		env := newTestService(t, testServiceOptions{generate: func(GenerateInput) (GenerateOutput, error) {
			return GenerateOutput{}, tc.err
		}})
		p := env.createProject(t, "Gen "+tc.want)
		_, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
		wantCode(t, err, tc.want)
		if builds, err := env.svc.ListBuilds(context.Background(), p.ID); err != nil || len(builds) != 0 {
			t.Fatalf("%s: builds = %+v, %v", tc.want, builds, err)
		}
	}
}

func TestDeleteProjectRemovesBuildsAndArtifacts(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Doomed")
	b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitBuild(t, env.svc, b.ID)
	if err := env.svc.DeleteProject(ctx, p.ID); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := os.Stat(filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts", b.ID)); !os.IsNotExist(err) {
		t.Fatalf("artefacts still exist: %v", err)
	}
	_, err = env.svc.GetBuild(ctx, b.ID)
	wantCode(t, err, CodeNotFound)
}

func TestDeleteProjectRefusesAnActiveBuild(t *testing.T) {
	env := newTestService(t)
	release := make(chan struct{})
	env.builder.build = func(ctx context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
		<-release
		return successOutput(req), nil
	}
	p := env.createProject(t, "Busy")
	b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	wantCode(t, env.svc.DeleteProject(context.Background(), p.ID), CodeBusy)
	close(release)
	waitBuild(t, env.svc, b.ID)
	if err := env.svc.DeleteProject(context.Background(), p.ID); err != nil {
		t.Fatalf("delete after the build: %v", err)
	}
}

// TestStartBuildKeepsTypedProjectTreeErrors: a damaged project tree (missing
// directory, oversized image file) is refused with its own code before a
// build record exists.
func TestStartBuildKeepsTypedProjectTreeErrors(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	p := env.createProject(t, "Big")
	big := append(pngBytes(t, 4, 4, false), make([]byte, MaxAssetUploadBytes)...)
	if err := os.MkdirAll(filepath.Join(env.cfg.WorkspaceDir, "CYD", "big", "assets"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(env.cfg.WorkspaceDir, "CYD", "big", "assets", "big.png"), big, 0o640); err != nil {
		t.Fatal(err)
	}
	rawAssetEntry(t, env, p.ID, Asset{ID: "a_big", File: "assets/big.png", W: 4, H: 4})
	_, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
	if typed := wantCode(t, err, CodeAssetTooLarge); typed.Details["asset"] != "a_big" || typed.Details["reason"] != "file_size" {
		t.Fatalf("details = %v", typed.Details)
	}

	gone := env.createProject(t, "Gone")
	if err := os.RemoveAll(filepath.Join(env.cfg.WorkspaceDir, "CYD", "gone")); err != nil {
		t.Fatal(err)
	}
	_, err = env.svc.StartBuild(ctx, gone.ID, BuildOptions{})
	if typed := wantCode(t, err, CodeInvalidLayout); typed.Details["reason"] != "missing" {
		t.Fatalf("details = %v", typed.Details)
	}
	for _, id := range []string{p.ID, gone.ID} {
		if builds, err := env.svc.ListBuilds(ctx, id); err != nil || len(builds) != 0 {
			t.Fatalf("builds of %s = %+v, %v", id, builds, err)
		}
	}
}

// TestBuildFailuresTheAgentMustNotRepair: cyd-build exit code 2 (OOM kill,
// tooling or build-tree failure), an unknown exit code and a builder failure
// after a partial log carry no error diagnostics (the lines stay as notes), so
// plan 3 never treats them as repairable, and a compile failure without a
// parsable diagnostic still says where to look.
func TestBuildFailuresTheAgentMustNotRepair(t *testing.T) {
	appError := "/work/%s/src/app/app_main.cpp:7:3: error: 'foo' was not declared in this scope"
	killed := "xtensa-esp32-elf-g++: fatal error: Killed signal terminated program cc1plus"
	streamed := func(code int, fail error) func(BuildRequest, func(string)) (BuildOutput, error) {
		return func(req BuildRequest, onLog func(string)) (BuildOutput, error) {
			onLog(fmt.Sprintf(appError, req.BuildID))
			onLog(killed)
			return BuildOutput{ExitCode: code}, fail
		}
	}
	for _, tc := range []struct {
		name    string
		run     func(req BuildRequest, onLog func(string)) (BuildOutput, error)
		code    string
		note    string
		demoted bool
	}{
		{"killed compiler", streamed(BuildExitToolingError, nil), CodeBuildFailed, "exit code 2", true},
		{"unknown exit code", streamed(137, nil), CodeBuildFailed, "exit code 137", true},
		{"untyped builder failure after a partial log", streamed(0, errors.New("download artefacts: unexpected EOF")),
			CodeBuildFailed, "server log", true},
		{"typed builder failure after a partial log", streamed(0, NewError(CodeDockerUnavailable, "the Docker Engine went away", nil)),
			CodeDockerUnavailable, "Docker Engine went away", true},
		{"unattributed link error", func(BuildRequest, func(string)) (BuildOutput, error) {
			return BuildOutput{ExitCode: BuildExitCompileError, Log: "region `dram0_0_seg' overflowed by 1234 bytes\n"}, nil
		}, CodeBuildFailed, ArtifactBuildLog, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := newTestService(t)
			env.builder.build = func(_ context.Context, req BuildRequest, onLog func(string)) (BuildOutput, error) {
				return tc.run(req, onLog)
			}
			p := env.createProject(t, "Tooling")
			b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
			if err != nil {
				t.Fatal(err)
			}
			done := waitBuild(t, env.svc, b.ID)
			if done.Status != BuildFailed || done.ErrorCode != tc.code {
				t.Fatalf("build = %s/%s, want %s/%s", done.Status, done.ErrorCode, BuildFailed, tc.code)
			}
			note, demoted := false, false
			for _, d := range done.Diagnostics {
				if d.Severity == SeverityError {
					t.Fatalf("diagnostic %+v would make the build agent-repairable", d)
				}
				note = note || d.Severity == SeverityNote && strings.Contains(d.Message, tc.note)
				demoted = demoted || d.Severity == SeverityNote && d.File == "src/app/app_main.cpp"
			}
			if !note {
				t.Fatalf("diagnostics %+v lack a note containing %q", done.Diagnostics, tc.note)
			}
			if demoted != tc.demoted {
				t.Fatalf("diagnostics %+v: compile error kept as a note = %v, want %v", done.Diagnostics, demoted, tc.demoted)
			}
		})
	}
}

func TestMaxFontArtifactBytesFollowsTheLargestAppSlot(t *testing.T) {
	slot, err := AppSlotSize(16)
	if err != nil || maxFontArtifactBytes != 7*slot {
		t.Fatalf("maxFontArtifactBytes = %d, want 7 x the 16 MB app slot %d (%v)", maxFontArtifactBytes, slot, err)
	}
}

// TestRetentionKeepsTheNewestSucceededBuild: failed builds beyond the limit
// never prune the project's only good firmware.
func TestRetentionKeepsTheNewestSucceededBuild(t *testing.T) {
	env := newTestService(t, testServiceOptions{limits: ServiceLimits{KeepBuildsPerProject: 2}})
	ctx := context.Background()
	p := env.createProject(t, "Good")
	var ids []string
	for i := 0; i < 4; i++ {
		if i == 1 {
			env.builder.build = func(context.Context, BuildRequest, func(string)) (BuildOutput, error) {
				return BuildOutput{ExitCode: BuildExitCompileError, Log: "src/app/app_main.cpp:1:1: error: boom\n"}, nil
			}
		}
		b, err := env.svc.StartBuild(ctx, p.ID, BuildOptions{})
		if err != nil {
			t.Fatal(err)
		}
		waitBuild(t, env.svc, b.ID)
		ids = append(ids, b.ID)
	}
	artifacts := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts")
	for i, id := range ids {
		b, err := env.svc.GetBuild(ctx, id)
		_, statErr := os.Stat(filepath.Join(artifacts, id))
		keep := i != 1 // newest succeeded (0), last_build (3) and the newest other (2)
		if keep && (err != nil || statErr != nil) {
			t.Errorf("build %d (%s, %s) should be kept: %v / %v", i, id, b.Status, err, statErr)
		}
		if !keep && (ErrorCode(err) != CodeNotFound || !os.IsNotExist(statErr)) {
			t.Errorf("build %d (%s) should be deleted: %v / %v", i, id, err, statErr)
		}
	}
	if _, _, err := env.svc.OpenArtifact(ctx, ids[0], ArtifactMergedBin); err != nil {
		t.Fatalf("the good firmware is gone: %v", err)
	}
}

// TestArtifactCleanupIgnoresMalformedBuildIDs: damaged rows with an empty or
// escaping id never make DeleteProject or the startup recovery remove the
// artefact directory or anything outside it.
func TestArtifactCleanupIgnoresMalformedBuildIDs(t *testing.T) {
	dir := t.TempDir()
	env := newTestService(t, testServiceOptions{dir: dir})
	p := env.createProject(t, "Damaged Rows")
	for _, id := range []string{"", "../x"} {
		if _, err := env.svc.db.Exec(`INSERT INTO cc_builds(id,project_id,origin,status,board,project_version,created_at)
			VALUES(?,?,'user','queued','esp32-2432s028r',1,'2026-10-10T08:00:00.000Z')`, id, p.ID); err != nil {
			t.Fatal(err)
		}
	}
	artifacts := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts")
	inside := filepath.Join(artifacts, "keep.txt")
	outside := filepath.Join(env.cfg.DataDir, "cyd_creator", "x", "keep.txt")
	for _, f := range []string{inside, outside} {
		if err := os.MkdirAll(filepath.Dir(f), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(f, []byte("keep"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	check := func(when string) {
		t.Helper()
		for _, f := range []string{inside, outside} {
			if _, err := os.Stat(f); err != nil {
				t.Fatalf("%s removed %s: %v", when, f, err)
			}
		}
	}
	if err := env.svc.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(env.cfg) // recovers both rows as interrupted
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	check("the startup recovery")
	if err := restarted.DeleteProject(context.Background(), p.ID); err != nil {
		t.Fatal(err)
	}
	check("DeleteProject")
}

// TestStartupSweepsOrphanedArtifacts: an artefact directory named like a
// build without a cc_builds row (a removal that failed) is removed at the next
// start; directories of stored builds and other entries stay.
func TestStartupSweepsOrphanedArtifacts(t *testing.T) {
	dir := t.TempDir()
	env := newTestService(t, testServiceOptions{dir: dir})
	p := env.createProject(t, "Sweep")
	b, err := env.svc.StartBuild(context.Background(), p.ID, BuildOptions{})
	if err != nil {
		t.Fatal(err)
	}
	waitBuild(t, env.svc, b.ID)
	artifacts := filepath.Join(env.cfg.DataDir, "cyd_creator", "artifacts")
	for _, f := range []string{"ccb_orphan/merged.bin", "ccb_9999.tmp/build.log", "notes/keep.txt"} {
		path := filepath.Join(artifacts, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte("x"), 0o640); err != nil {
			t.Fatal(err)
		}
	}
	if err := env.svc.Close(); err != nil {
		t.Fatal(err)
	}
	restarted, err := NewService(env.cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	for name, want := range map[string]bool{b.ID: true, "notes": true, "ccb_orphan": false, "ccb_9999.tmp": false} {
		_, err := os.Stat(filepath.Join(artifacts, name))
		if exists := err == nil; exists != want {
			t.Errorf("%s exists = %v, want %v (%v)", name, exists, want, err)
		}
	}
}

// TestPullAndClose: Close waits for a running pull, and a pull after Close is
// refused.
func TestPullAndClose(t *testing.T) {
	env := newTestService(t)
	var finished atomic.Bool
	started := make(chan struct{})
	env.builder.pull = func(ctx context.Context) error {
		close(started)
		<-ctx.Done()
		time.Sleep(50 * time.Millisecond)
		finished.Store(true)
		return ctx.Err()
	}
	if err := env.svc.PullToolchain(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-started
	if err := env.svc.Close(); err != nil {
		t.Fatal(err)
	}
	if !finished.Load() {
		t.Fatal("Close returned before the running pull ended")
	}
	wantCode(t, env.svc.PullToolchain(context.Background()), CodeBusy)
}

func TestClipBuildLogKeepsTheCap(t *testing.T) {
	if got := clipBuildLog("short\n"); got != "short\n" {
		t.Fatalf("short log = %q", got)
	}
	long := strings.Repeat("ä", MaxBuildLogBytes) // 2 bytes per rune
	got := clipBuildLog(long)
	if len(got) > MaxBuildLogBytes || !strings.HasSuffix(got, buildLogTruncated) || !utf8.ValidString(got) {
		t.Fatalf("clipped log: %d bytes, suffix %q, valid %v", len(got), got[len(got)-len(buildLogTruncated):], utf8.ValidString(got))
	}
}

// TestArtifactLookupsRefuseForeignBuildIDs: a build id that the service could
// not have issued never becomes an artefact path, even when a (damaged) row
// carries it.
func TestArtifactLookupsRefuseForeignBuildIDs(t *testing.T) {
	env := newTestService(t)
	ctx := context.Background()
	ids := []string{"../escape", "ccb_x/../../escape", `ccb_x\..\..\escape`, "build.log", ""}
	for _, id := range ids {
		if _, err := env.svc.db.Exec(`INSERT INTO cc_builds(id,project_id,origin,status,board,project_version,artifacts,created_at)
			VALUES(?,'ccp_x','user','succeeded','esp32-2432s028r',1,?,'2026-10-10T08:00:00.000Z')`,
			id, `[{"name":"build.log","size":1,"sha256":""}]`); err != nil {
			t.Fatal(err)
		}
	}
	for _, id := range ids {
		_, _, err := env.svc.OpenArtifact(ctx, id, ArtifactBuildLog)
		wantCode(t, err, CodeNotFound)
		_, err = env.svc.BuildLog(ctx, id)
		wantCode(t, err, CodeNotFound)
		_, _, err = env.svc.BuildFonts(ctx, id)
		wantCode(t, err, CodeNotFound)
		_, err = env.svc.DecodeBacktrace(ctx, id, []string{"0x1"})
		wantCode(t, err, CodeNotFound)
	}
}
