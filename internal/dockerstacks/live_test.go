package dockerstacks

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"os"
	"path"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
	"aurago/internal/dockerutil"
)

// The live test runs the whole stack cycle against a real Docker Engine:
// prepare, create, up, running, down, delete. It is gated by environment variables:
//
//	AURAGO_DOCKER_MANAGER_LIVE=1                 switch the test on
//	AURAGO_DOCKER_MANAGER_LIVE_DIR=<path>        throw-away stacks directory as the ENGINE sees it
//	                                             (its last element must start with "aurago-dm-live")
//	AURAGO_DOCKER_MANAGER_LIVE_ENDPOINT=<host>   Engine endpoint (default: dockerutil.DefaultHost())
//	AURAGO_DOCKER_MANAGER_LIVE_SOCKET=<path>     Docker socket path inside the engine host
//	                                             (default /var/run/docker.sock)
//
// The test service is the runner image itself with `sleep 300`, so no further image is
// pulled. Project names start with "aurago-dm-live-". The engine's IO container is
// created for the throw-away directory and removed again, so run the test on an engine
// whose Docker Manager is not in use (the test skips when it finds an IO container of
// another stacks directory). With -v the test prints every raw line of the runner's
// `--progress json` output ("RAW ..."): compare the field names with the mapping in
// parseProgressLine.
//
//	AURAGO_DOCKER_MANAGER_LIVE=1 AURAGO_DOCKER_MANAGER_LIVE_DIR=/tmp/aurago-dm-live \
//	  go test ./internal/dockerstacks/ -run TestLiveStackCycle -v -count=1 -timeout 30m

// liveHosts serves one host, "live", from a real engine client.
type liveHosts struct {
	host   dockerhost.Host
	client *dockerutil.Client
}

func (l liveHosts) Host(_ context.Context, id string) (dockerhost.Host, error) {
	if id != l.host.ID {
		return dockerhost.Host{}, dockerhost.ErrHostNotFound
	}
	return l.host, nil
}

func (l liveHosts) Client(_ context.Context, id string) (*dockerutil.Client, error) {
	if id != l.host.ID {
		return nil, dockerhost.ErrHostNotFound
	}
	return l.client, nil
}

func TestLiveStackCycle(t *testing.T) {
	if os.Getenv("AURAGO_DOCKER_MANAGER_LIVE") != "1" {
		t.Skip("set AURAGO_DOCKER_MANAGER_LIVE=1 (and AURAGO_DOCKER_MANAGER_LIVE_DIR) to run against a real Docker Engine")
	}
	dir := path.Clean(os.Getenv("AURAGO_DOCKER_MANAGER_LIVE_DIR"))
	if dir == "." || !strings.HasPrefix(path.Base(dir), "aurago-dm-live") {
		t.Fatalf("AURAGO_DOCKER_MANAGER_LIVE_DIR must name a throw-away directory whose last element starts with aurago-dm-live, got %q", dir)
	}
	endpoint := os.Getenv("AURAGO_DOCKER_MANAGER_LIVE_ENDPOINT")
	if endpoint == "" {
		endpoint = dockerutil.DefaultHost()
	}
	socket := os.Getenv("AURAGO_DOCKER_MANAGER_LIVE_SOCKET")
	if socket == "" {
		socket = dockerhost.DefaultHostSocketPath
	}
	client := dockerutil.NewClient(endpoint, 30*time.Second)
	t.Cleanup(client.CloseIdleConnections)
	host := dockerhost.Host{ID: "live", Name: "live", Kind: dockerhost.KindLocal, Address: endpoint, StacksDir: dir, HostSocketPath: socket, Color: "blue"}
	check := host // the rules for stacks_dir and host_socket_path apply here too
	check.Kind, check.Address = dockerhost.KindTCP, "127.0.0.1"
	if err := dockerhost.ValidateHost(check.Normalized()); err != nil {
		t.Fatalf("the stacks directory or socket path is not usable: %v", err)
	}

	ctx := context.Background()
	e := newEngine(client)
	if state, st, err := e.ioStatus(ctx, host); err != nil {
		t.Fatalf("cannot reach the engine at %s: %v", endpoint, err)
	} else if state == ioMismatch {
		t.Skipf("the engine has a container %s for another stacks directory (%q); run on a scratch engine", IOContainerName, st.Labels[LabelStacksDir])
	}

	svc, err := Open(openTestDB(t), liveHosts{host: host, client: client}, Options{
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug})),
		HealthWait: 30 * time.Second,
		HealthPoll: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(svc.Close)
	svc.rawLine = func(operationID, line string) { t.Logf("RAW %s %s", operationID, line) }

	wait := func(op Operation) Operation {
		t.Helper()
		deadline := time.Now().Add(10 * time.Minute)
		for time.Now().Before(deadline) {
			cur, err := svc.Operation(ctx, op.ID)
			if err != nil {
				t.Fatalf("Operation %s: %v", op.ID, err)
			}
			if cur.Done() {
				t.Logf("operation %s %s %s: %s %s\n%s", cur.ID, cur.Kind, cur.Stack, cur.State, cur.Error, cur.Output)
				return cur
			}
			time.Sleep(time.Second)
		}
		t.Fatalf("operation %s did not finish in 10 minutes", op.ID)
		return Operation{}
	}
	start := func(name, kind string, opts OpOptions) Operation {
		t.Helper()
		op, err := svc.Start(ctx, host.ID, name, kind, opts)
		if err != nil {
			t.Fatalf("Start %s %s: %v", kind, name, err)
		}
		return wait(op)
	}

	// 1. Prepare: pulls the runner image (about 64 MB), creates the IO container, runs the lister once.
	createdIO := false
	if state, _, _ := e.ioStatus(ctx, host); state == ioMissing {
		createdIO = true
	}
	t.Cleanup(func() {
		if createdIO {
			removeCtx, cancel := context.WithTimeout(context.Background(), cleanupTimeout)
			defer cancel()
			_ = e.removeContainer(removeCtx, IOContainerName)
		}
	})
	prep, err := svc.Prepare(ctx, host.ID)
	if err != nil {
		t.Fatal(err)
	}
	if done := wait(prep); done.State != StateSucceeded {
		t.Fatalf("prepare = %s: %s (%s)", done.State, done.Error, done.Code)
	}
	helpers, err := svc.Helpers(ctx, host.ID)
	if err != nil || !helpers.Prepared {
		t.Fatalf("helpers after prepare = %+v, %v", helpers, err)
	}

	// 2. Create a stack whose only service is the runner image running `sleep`.
	var rnd [3]byte
	_, _ = rand.Read(rnd[:])
	name := "aurago-dm-live-" + hex.EncodeToString(rnd[:])
	compose := "services:\n  sleeper:\n    image: " + RunnerImage + "\n    command: [\"sleep\", \"300\"]\n    restart: \"no\"\n"
	t.Cleanup(func() { // best effort: a failed test must not leave the project behind
		if _, err := svc.Get(context.Background(), host.ID, name); err == nil {
			if op, err := svc.Start(context.Background(), host.ID, name, KindDelete, OpOptions{Volumes: true}); err == nil {
				for i := 0; i < 120; i++ {
					if cur, _ := svc.Operation(context.Background(), op.ID); cur.Done() {
						break
					}
					time.Sleep(time.Second)
				}
			}
		}
	})
	created, err := svc.Create(ctx, host.ID, CreateInput{Name: name, Compose: compose})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.Status != StatusDraft || created.Compose.Revision != Revision([]byte(compose)) {
		t.Fatalf("created = %+v", created.StackSummary)
	}

	// 3. config, then up: the stack must be running afterwards.
	if done := start(name, KindConfig, OpOptions{}); done.State != StateSucceeded {
		t.Fatalf("config = %s: %s (%s)", done.State, done.Error, done.Code)
	}
	if done := start(name, KindUp, OpOptions{}); done.State != StateSucceeded {
		t.Fatalf("up = %s: %s (%s)", done.State, done.Error, done.Code)
	}
	listing, err := svc.List(ctx, host.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	var found *StackSummary
	for i := range listing.Stacks {
		if listing.Stacks[i].Name == name {
			found = &listing.Stacks[i]
		}
	}
	if found == nil || found.Status != StatusRunning || found.Services.Running != 1 {
		t.Fatalf("after up the stack must be running, got %+v", found)
	}
	detail, err := svc.Get(ctx, host.ID, name)
	if err != nil || len(detail.ServiceList) != 1 || detail.ServiceList[0].Name != "sleeper" || detail.ServiceList[0].State != "running" {
		t.Fatalf("Get after up = %+v, %v", detail.ServiceList, err)
	}
	if left := len(listingHelpers(t, e)); left != 0 {
		t.Logf("note: %d runner or lister containers are still on the engine", left)
	}

	// 4. down, then delete with its volumes: the folder must be gone.
	if done := start(name, KindDown, OpOptions{}); done.State != StateSucceeded {
		t.Fatalf("down = %s: %s (%s)", done.State, done.Error, done.Code)
	}
	if got, err := svc.Get(ctx, host.ID, name); err != nil || got.Status != StatusDraft {
		t.Fatalf("after down the stack is a draft again, got %+v, %v", got.StackSummary, err)
	}
	if done := start(name, KindDelete, OpOptions{Volumes: true}); done.State != StateSucceeded {
		t.Fatalf("delete = %s: %s (%s)", done.State, done.Error, done.Code)
	}
	if _, err := svc.Get(ctx, host.ID, name); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("Get after delete = %v, want %s", err, CodeStackNotFound)
	}
}

// listingHelpers lists the runner and lister containers that are still on the engine.
func listingHelpers(t *testing.T, e engine) []string {
	t.Helper()
	containers, err := dockerhost.ListContainers(context.Background(), e.c)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range containers {
		if role := c.Labels[LabelRole]; role == RoleRunner || role == RoleLister {
			names = append(names, c.Names[0])
		}
	}
	return names
}
