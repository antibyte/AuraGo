package dockerstacks

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

func TestEngineContainerLifecycleAndLogs(t *testing.T) {
	fe := newFakeEngine(t)
	fe.prepared()
	fe.runnerLines = []string{`{"id":"Container web-1","status":"Working","text":"Starting"}`, "plain line"}
	fe.runnerErr = "boom\n"
	fe.runnerExit = 3
	e := newEngine(fe.client(t))
	ctx := context.Background()

	op := Operation{ID: "op_0123456789abcdef", Stack: "web", Kind: KindUp}
	spec, _ := runnerSpec(RunnerImage, fe.host(), op, "compose.yaml")
	id, err := e.createContainer(ctx, runnerName(op.ID), spec)
	if err != nil || id == "" {
		t.Fatalf("createContainer = (%q, %v)", id, err)
	}
	if got := fe.container("aurago-dm-op-op_0123456789abcdef"); got == nil {
		t.Fatal("the runner was not created under its operation name")
	}
	if _, err := e.createContainer(ctx, runnerName(op.ID), spec); dockerhost.StatusOf(err) != http.StatusConflict {
		t.Fatalf("a second create with the same name = %v, want 409", err)
	}
	if err := e.startContainer(ctx, id); err != nil {
		t.Fatalf("startContainer: %v", err)
	}
	var stdout, stderr bytes.Buffer
	if err := e.followLogs(ctx, id, 0, &stdout, &stderr); err != nil {
		t.Fatalf("followLogs: %v", err)
	}
	if want := "{\"id\":\"Container web-1\",\"status\":\"Working\",\"text\":\"Starting\"}\nplain line\nboom\n"; stderr.String() != want || stdout.Len() != 0 {
		t.Fatalf("stderr = %q stdout = %q, want the runner lines on stderr", stderr.String(), stdout.String())
	}
	st, err := e.inspectContainer(ctx, id)
	if err != nil {
		t.Fatal(err)
	}
	if st.Running || st.Status != "exited" || st.ExitCode != 3 || st.Labels[LabelOperation] != op.ID {
		t.Fatalf("inspect = %+v, want exited with code 3 and the operation label", st)
	}
	if err := e.removeContainer(ctx, id); err != nil {
		t.Fatalf("removeContainer: %v", err)
	}
	if err := e.removeContainer(ctx, id); err != nil {
		t.Fatalf("removing a container that is gone must succeed, got %v", err)
	}
	if _, err := e.inspectContainer(ctx, id); !isNotFound(err) {
		t.Fatalf("inspect after remove = %v, want 404", err)
	}
}

func TestEngineKillTreatsAnExitedContainerAsDone(t *testing.T) {
	fe := newFakeEngine(t)
	fe.prepared()
	e := newEngine(fe.client(t))
	ctx := context.Background()
	fc := fe.container(IOContainerName) // created, never started
	if err := e.killContainer(ctx, fc.id); err != nil {
		t.Fatalf("kill of a container that is not running = %v, want nil", err)
	}
}

func TestStreamingRequestsIgnoreTheClientTimeout(t *testing.T) {
	fe := newFakeEngine(t)
	fe.prepared()
	gate := make(chan struct{})
	fe.runnerGate = gate
	fe.runnerLines = []string{"started"}
	// The client times out after 100 ms; the log follow below lasts about 400 ms.
	c := dockerutil.NewClient("tcp://"+fe.server.Listener.Addr().String(), 100*time.Millisecond)
	t.Cleanup(c.CloseIdleConnections)
	e := newEngine(c)
	ctx := context.Background()

	spec, _ := runnerSpec(RunnerImage, fe.host(), Operation{ID: "op_1", Stack: "web", Kind: KindStop}, "compose.yaml")
	id, err := e.createContainer(ctx, "slow-runner", spec)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.startContainer(ctx, id); err != nil {
		t.Fatal(err)
	}
	go func() {
		time.Sleep(400 * time.Millisecond)
		close(gate)
	}()
	var out bytes.Buffer
	if err := e.followLogs(ctx, id, 0, &out, &out); err != nil {
		t.Fatalf("followLogs over the client timeout: %v", err)
	}
	if out.String() != "started\n" {
		t.Fatalf("logs = %q", out.String())
	}
	// A plain request still honours the client timeout.
	slow := newEngine(dockerutil.NewClient("tcp://"+fe.server.Listener.Addr().String(), 1*time.Nanosecond))
	if _, err := slow.version(ctx); err == nil {
		t.Fatal("a plain request must keep the client timeout")
	}
}

func TestEnginePullReportsMessagesAndStreamErrors(t *testing.T) {
	fe := newFakeEngine(t)
	e := newEngine(fe.client(t))
	ctx := context.Background()

	var messages []jsonMessage
	if err := e.pullImage(ctx, runnerImageRepo, runnerImagePullTag, func(m jsonMessage) { messages = append(messages, m) }); err != nil {
		t.Fatalf("pullImage: %v", err)
	}
	if len(messages) != 6 || messages[2].Progress.Total != 4000 || messages[2].ID != "layer1" {
		t.Fatalf("messages = %+v, want the six fake pull lines with the layer progress", messages)
	}
	if !fe.hasImage(RunnerImage) {
		t.Fatalf("the pull must make %s present; requests: %v", RunnerImage, fe.requests())
	}
	found := false
	for _, r := range fe.requests() {
		if strings.Contains(r, "POST /v1.45/images/create?fromImage=docker&tag=sha256%3A5e6baf") {
			found = true
		}
	}
	if !found {
		t.Fatalf("pull request not as the contract says (fromImage=docker&tag=sha256:...): %v", fe.requests())
	}

	fe.pullFails = "pull access denied for docker, repository does not exist"
	err := e.pullImage(ctx, runnerImageRepo, runnerImagePullTag, nil)
	if dockerhost.StatusOf(err) != http.StatusInternalServerError || !strings.Contains(err.Error(), "pull access denied") {
		t.Fatalf("a failed pull = %v, want a 500 StatusError with the Engine message", err)
	}

	err = readJSONMessages(strings.NewReader(`{"status":"Pulling"}`+"\n"+`{"errorDetail":{"message":"manifest unknown"},"error":"manifest unknown"}`+"\n"), nil)
	var jsonErr *dockerutil.JSONMessageError
	if !errors.As(err, &jsonErr) || jsonErr.Message != "manifest unknown" {
		t.Fatalf("an error event = %v, want *JSONMessageError", err)
	}
}

func TestEngineSaveAndLoadStreamTheImage(t *testing.T) {
	fe := newFakeEngine(t)
	fe.addImage(RunnerImage, "sha256:runnerimage")
	e := newEngine(fe.client(t))
	ctx := context.Background()

	body, err := e.saveImage(ctx, RunnerImage)
	if err != nil {
		t.Fatalf("saveImage: %v", err)
	}
	loaded, err := e.loadImage(ctx, body)
	_ = body.Close()
	if err != nil {
		t.Fatalf("loadImage: %v", err)
	}
	if loaded != "sha256:loadedimage" {
		t.Fatalf("loaded = %q, want the id from the Engine's answer", loaded)
	}
	fe.mu.Lock()
	got := string(fe.loadedBody)
	fe.mu.Unlock()
	if got != "FAKE-IMAGE-TAR:"+RunnerImage {
		t.Fatalf("the load body = %q, want the saved stream copied through", got)
	}
	if err := e.tagImage(ctx, loaded, "aurago-docker-manager-runner", "x"); err != nil {
		t.Fatalf("tagImage: %v", err)
	}
	if !fe.hasImage("aurago-docker-manager-runner:x") {
		t.Fatal("the tag request did not reach the Engine")
	}
}

func TestEngineVersionAndImageInspect(t *testing.T) {
	fe := newFakeEngine(t)
	fe.setVersion("1.40", "linux", "arm64")
	e := newEngine(fe.client(t))
	ctx := context.Background()

	v, err := e.version(ctx)
	if err != nil || v.APIVersion != "1.40" || v.OS != "linux" || v.Arch != "arm64" {
		t.Fatalf("version = (%+v, %v)", v, err)
	}
	if supportsStacks("1.40") || !supportsStacks("1.41") || !supportsStacks("1.45") || supportsStacks("") || supportsStacks("2.0") {
		t.Fatal("supportsStacks must accept exactly API 1.41 and newer")
	}
	if _, found, err := e.inspectImage(ctx, RunnerImage); found || err != nil {
		t.Fatalf("inspectImage of a missing image = (_, %v, %v), want not found without an error", found, err)
	}
	fe.addImage(RunnerImage, "sha256:runnerimage")
	info, found, err := e.inspectImage(ctx, RunnerImage)
	if !found || err != nil || info.ID != "sha256:runnerimage" || info.Arch != "arm64" {
		t.Fatalf("inspectImage = (%+v, %v, %v)", info, found, err)
	}
}

func TestEngineProjectVolumesAndSizes(t *testing.T) {
	fe := newFakeEngine(t)
	fe.volumes = []fakeVolume{
		{Name: "shop_data", Driver: "local", Project: "shop", Size: 2048},
		{Name: "shop_cache", Driver: "local", Project: "shop", Size: -1},
		{Name: "other_data", Driver: "local", Project: "other", Size: 5},
	}
	e := newEngine(fe.client(t))
	ctx := context.Background()

	volumes, err := e.projectVolumes(ctx, "shop")
	if err != nil || len(volumes) != 2 || volumes[0].Name != "shop_data" || volumes[0].Size != -1 {
		t.Fatalf("projectVolumes = (%+v, %v), want the two shop volumes with unknown sizes", volumes, err)
	}
	sizes, err := e.volumeSizes(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if sizes["shop_data"] != 2048 || sizes["other_data"] != 5 {
		t.Fatalf("sizes = %v", sizes)
	}
	if _, ok := sizes["shop_cache"]; ok {
		t.Fatal("a volume the Engine could not measure (-1) must be left out")
	}
}

func TestEngineRequestKeepsNonSuccessAsStatusError(t *testing.T) {
	fe := newFakeEngine(t)
	e := newEngine(fe.client(t))
	_, err := e.request(context.Background(), http.MethodGet, "containers/ghost/archive?path=/x", nil, "", false)
	if dockerhost.StatusOf(err) != http.StatusNotFound || !strings.Contains(err.Error(), "No such container: ghost") {
		t.Fatalf("request = %v, want a 404 StatusError with the Engine's message", err)
	}
}

func TestAnImagePullOrLoadThatGoesSilentEndsAfterTheIdleTimeout(t *testing.T) {
	prev := imageIdleTimeout
	imageIdleTimeout = 300 * time.Millisecond
	t.Cleanup(func() { imageIdleTimeout = prev })
	fe := newFakeEngine(t)
	fe.addImage(RunnerImage, "sha256:runnerimage")
	e := newEngine(fe.client(t))
	ctx := context.Background() // no deadline of its own: only the idle timeout can end the call

	fe.pullStall = true
	messages := 0
	err := e.pullImage(ctx, runnerImageRepo, runnerImagePullTag, func(jsonMessage) { messages++ })
	if err == nil || !strings.Contains(err.Error(), "stalled") || messages != 1 {
		t.Fatalf("a pull that went silent = %v after %d message(s), want a stalled error after the first one", err, messages)
	}

	fe.loadStall = true
	_, err = e.loadImage(ctx, strings.NewReader("FAKE-IMAGE-TAR"))
	if err == nil || !strings.Contains(err.Error(), "stalled") {
		t.Fatalf("a load that never answers = %v, want a stalled error", err)
	}
}
