package dockerstacks

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

// twoHostEnv has the local engine ("local") and a remote one ("h_remote").
type twoHostEnv struct {
	local, remote *fakeEngine
	runs          int
	svc           *Service
	remoteHost    dockerhost.Host
}

func newTwoHostEnv(t *testing.T) *twoHostEnv {
	t.Helper()
	local, remote := newFakeEngine(t), newFakeEngine(t)
	hosts := newFakeHosts()
	hosts.add(t, "local", local)
	remoteHost := hosts.add(t, "h_remote", remote)
	return &twoHostEnv{local: local, remote: remote, svc: newTestService(t, hosts, nil), remoteHost: remoteHost}
}

// ensureResult is what ensureHost returned together with the operation's output
// and progress events.
type ensureResult struct {
	image    string
	err      error
	output   string
	progress []Progress
}

func mustJSON(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

// ensure runs ensureHost for a host as an operation would.
func (e *twoHostEnv) ensure(t *testing.T, hostID string, verbose bool) ensureResult {
	t.Helper()
	e.runs++
	id := fmt.Sprintf("op_%016x", e.runs)
	run := registerRun(t, e.svc, Operation{ID: id, HostID: hostID, Kind: KindPrepare})
	_, events, cancel, err := e.svc.Subscribe(id)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()
	h, eng, err := e.svc.target(context.Background(), hostID)
	if err != nil {
		t.Fatal(err)
	}
	image, ensureErr := e.svc.ensureHost(context.Background(), run, eng, h, verbose)
	out := run.snapshot().Output
	run.finish(outcome{state: StateSucceeded})
	var progress []Progress
	for _, ev := range drainEvents(events) {
		if ev.Type == EventProgress {
			progress = append(progress, *ev.Progress)
		}
	}
	return ensureResult{image: image, err: ensureErr, output: out, progress: progress}
}

func TestEnsureHostPullsTheImageAndCreatesTheIOContainer(t *testing.T) {
	env := newTwoHostEnv(t)
	res := env.ensure(t, "h_remote", true)
	out, progress := res.output, res.progress
	if res.err != nil || res.image != RunnerImage {
		t.Fatalf("ensureHost = (%q, %v)", res.image, res.err)
	}
	if !env.remote.hasImage(RunnerImage) {
		t.Fatal("the image was not pulled")
	}
	io := env.remote.container(IOContainerName)
	if io == nil {
		t.Fatal("no IO container")
	}
	assertSameJSON(t, env.remote.specOf(io), mustJSON(ioSpec(RunnerImage, env.remoteHost)))
	if env.remote.isRunning(io) {
		t.Fatal("the IO container must never be started")
	}
	for _, want := range []string{"Pulling the helper image docker:29.9.2-cli", "Creating the IO container aurago-docker-manager-io", "layer1: Pull complete"} {
		if !strings.Contains(out, want) {
			t.Fatalf("output lacks %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Downloading") {
		t.Fatalf("byte ticks must not reach the output:\n%s", out)
	}
	var bar *Progress
	for i := range progress {
		if progress[i].ID == "layer1" && progress[i].Total == 4000 && progress[i].Current == 1000 {
			bar = &progress[i]
		}
	}
	if bar == nil || bar.Percent != 25 || bar.Parent != RunnerImageTag {
		t.Fatalf("progress events = %+v, want the layer's download at 25%%", progress)
	}
	if st, _ := env.svc.Helpers(context.Background(), "h_remote"); !st.Prepared {
		t.Fatalf("after ensureHost the host must be prepared: %+v", st)
	}
}

func TestEnsureHostDoesNothingOnAPreparedHost(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.prepared()
	quiet := env.ensure(t, "h_remote", false)
	if quiet.err != nil || quiet.image != RunnerImage || quiet.output != "" {
		t.Fatalf("ensureHost = (%q, %v) output %q, want silence", quiet.image, quiet.err, quiet.output)
	}
	if env.remote.requestCount("POST /images/create")+env.remote.requestCount("POST /containers/create") != 0 {
		t.Fatal("a prepared host must not be touched")
	}
	verbose := env.ensure(t, "h_remote", true).output
	if !strings.Contains(verbose, "is present") || !strings.Contains(verbose, "is in place") {
		t.Fatalf("a prepare operation reports what it found:\n%s", verbose)
	}
}

func TestAFailedPullOnTheLocalHostNamesTheExactCommand(t *testing.T) {
	env := newTwoHostEnv(t)
	env.local.pullFails = "Get \"https://registry-1.docker.io/v2/\": dial tcp: lookup registry-1.docker.io: no such host"
	err := env.ensure(t, "local", true).err
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CodeRunnerImageMissing {
		t.Fatalf("ensureHost = %v, want %s", err, CodeRunnerImageMissing)
	}
	wantCmd := "docker pull docker:29.9.2-cli@sha256:5e6baf67325a79f64ed3bed6863954d5e05d0f3d4e57cbdb14809346814bbfdb"
	if coded.Fields["command"] != wantCmd || !strings.Contains(coded.Message, wantCmd) || !strings.Contains(coded.Message, "no such host") {
		t.Fatalf("error = %+v, want the command %q and the reason", coded, wantCmd)
	}
	if env.local.container(IOContainerName) != nil {
		t.Fatal("no IO container without the image")
	}
}

func TestARemoteHostThatCannotPullGetsTheImageFromTheLocalEngine(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.pullFails = "network unreachable"
	env.local.addImage(RunnerImage, "sha256:localimage")

	res := env.ensure(t, "h_remote", true)
	out := res.output
	if res.err != nil || res.image != RunnerLocalTag {
		t.Fatalf("ensureHost = (%q, %v), want the image under the local tag", res.image, res.err)
	}
	if !strings.Contains(out, "trying to copy the image from the local engine") || !strings.Contains(out, "Copying the helper image from the local engine (linux/amd64)") {
		t.Fatalf("output:\n%s", out)
	}
	env.local.mu.Lock()
	saved := env.local.savedRefs
	env.local.mu.Unlock()
	if len(saved) != 1 || saved[0] != RunnerImage {
		t.Fatalf("the local engine saved %v, want the pinned reference", saved)
	}
	if !env.remote.hasImage(RunnerLocalTag) {
		t.Fatal("the loaded image must be tagged " + RunnerLocalTag)
	}
	io := env.remote.container(IOContainerName)
	if io == nil || env.remote.specOf(io)["Image"] != RunnerLocalTag {
		t.Fatalf("the IO container must be created from the local tag, got %v", io)
	}
	if st, _ := env.svc.Helpers(context.Background(), "h_remote"); !st.Prepared {
		t.Fatalf("a host with a copied image is prepared: %+v", st)
	}
}

func TestCopyIsRefusedForADifferentArchitectureOrAMissingLocalImage(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.pullFails = "network unreachable"
	env.local.addImage(RunnerImage, "sha256:localimage")
	env.remote.setVersion("1.45", "linux", "arm64")
	err := env.ensure(t, "h_remote", true).err
	if CodeOf(err) != CodeRunnerImageMissing || !strings.Contains(err.Error(), "linux/amd64") || !strings.Contains(err.Error(), "linux/arm64") {
		t.Fatalf("ensureHost across architectures = %v, want %s naming both", err, CodeRunnerImageMissing)
	}
	if env.local.requestCount("GET /images/") > 2 { // inspect only, no save
		t.Fatalf("no image may be saved across architectures: %v", env.local.requests())
	}
	if env.remote.hasImage(RunnerLocalTag) || env.remote.container(IOContainerName) != nil {
		t.Fatal("nothing may be loaded or created")
	}

	env = newTwoHostEnv(t)
	env.remote.pullFails = "network unreachable"
	err = env.ensure(t, "h_remote", true).err
	if CodeOf(err) != CodeRunnerImageMissing || !strings.Contains(err.Error(), "does not have the image either") {
		t.Fatalf("ensureHost without a local copy = %v", err)
	}
}

func TestEnsureHostRecreatesAnIOContainerOfAnotherStacksDirectory(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.addImage(RunnerImage, "sha256:runnerimage")
	other := env.remoteHost
	other.StacksDir = "/srv/old-stacks"
	env.remote.addContainer(IOContainerName, ioSpec(RunnerImage, other), "created", 0)

	res := env.ensure(t, "h_remote", true)
	err, out := res.err, res.output
	if err != nil || !strings.Contains(out, "Recreating the IO container for the stacks directory /opt/stacks") {
		t.Fatalf("ensureHost = %v\n%s", err, out)
	}
	io := env.remote.container(IOContainerName)
	if io.labels[LabelStacksDir] != "/opt/stacks" {
		t.Fatalf("the IO container still has the old directory: %v", io.labels)
	}
}

func TestEnsureHostNeverTouchesAForeignContainerWithTheIOName(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.addImage(RunnerImage, "sha256:runnerimage")
	env.remote.addContainer(IOContainerName, containerSpec{Image: RunnerImage, Labels: map[string]string{"owner": "someone else"}}, "running", 0)

	err := env.ensure(t, "h_remote", true).err
	if CodeOf(err) != CodeStacksDirInvalid || !strings.Contains(err.Error(), IOContainerName) {
		t.Fatalf("ensureHost = %v, want %s naming the container", err, CodeStacksDirInvalid)
	}
	if env.remote.requestCount("DELETE /containers/") != 0 {
		t.Fatal("a container without the IO role label must never be removed")
	}
}

func TestEnsureHostRefusesAnOldEngine(t *testing.T) {
	env := newTwoHostEnv(t)
	env.remote.setVersion("1.40", "linux", "amd64")
	err := env.ensure(t, "h_remote", true).err
	if CodeOf(err) != CodeAPITooOld {
		t.Fatalf("ensureHost on API 1.40 = %v, want %s", err, CodeAPITooOld)
	}
}
