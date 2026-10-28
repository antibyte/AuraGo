package dockerstacks

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

// testEnv is a service over one fake engine, registered as host "h_fake".
type testEnv struct {
	t     *testing.T
	fe    *fakeEngine
	hosts *fakeHosts
	svc   *Service
	host  dockerhost.Host
	clock *fakeClock
}

const testHostID = "h_fake"

// newTestEnv returns a service with one fake host. prepared gives the host the
// runner image and the IO container.
func newTestEnv(t *testing.T, prepared bool) *testEnv {
	t.Helper()
	fe := newFakeEngine(t)
	if prepared {
		fe.prepared()
	}
	hosts := newFakeHosts()
	host := hosts.add(t, testHostID, fe)
	clock := newFakeClock()
	svc := newTestService(t, hosts, func(o *Options) { o.Now = clock.Now })
	return &testEnv{t: t, fe: fe, hosts: hosts, svc: svc, host: host, clock: clock}
}

func TestParseListerOutput(t *testing.T) {
	out := strings.Join([]string{
		"/opt/stacks/web/compose.yaml",
		"/opt/stacks/web/.env",
		"/opt/stacks/web/docker-compose.yml", // loses against compose.yaml
		"/opt/stacks/db/docker-compose.yaml",
		"/opt/stacks/db/compose.yml", // wins against docker-compose.yaml
		"/opt/stacks/envonly/.env",
		"/opt/stacks/My Stack/compose.yaml", // not a valid name, has a compose file: ignored
		"/opt/stacks/My Stack/.env",
		"/opt/stacks/Upper/.env", // not a valid name, no compose file: not reported
		"/opt/stacks/web/deeper/compose.yaml",
		"/elsewhere/other/compose.yaml",
		"/opt/stacks/web/README.md",
		"",
	}, "\n")
	folders, ignored := parseListerOutput("/opt/stacks", out)
	want := map[string]scanFolder{
		"web":     {composeFile: "compose.yaml", hasEnv: true},
		"db":      {composeFile: "compose.yml"},
		"envonly": {hasEnv: true},
	}
	if !reflect.DeepEqual(folders, want) {
		t.Fatalf("folders = %+v, want %+v", folders, want)
	}
	if len(ignored) != 1 || ignored[0] != (IgnoredFolder{Name: "My Stack", Reason: "name_invalid"}) {
		t.Fatalf("ignored = %+v", ignored)
	}
}

func TestHelpersReportWhatTheHostHas(t *testing.T) {
	env := newTestEnv(t, false)
	ctx := context.Background()

	st, err := env.svc.Helpers(ctx, testHostID)
	if err != nil {
		t.Fatal(err)
	}
	want := HelperStatus{APIVersion: "1.45", APISupported: true, RunnerImage: "docker:29.9.2-cli", IOContainer: "missing"}
	if st != want {
		t.Fatalf("unprepared host: %+v, want %+v", st, want)
	}

	env.fe.addImage(RunnerImage, "sha256:runnerimage")
	if st, _ = env.svc.Helpers(ctx, testHostID); !st.RunnerImagePresent || st.Prepared || st.IOContainer != "missing" {
		t.Fatalf("image only: %+v", st)
	}

	env.fe.addContainer(IOContainerName, ioSpec(RunnerImage, env.host), "created", 0)
	if st, _ = env.svc.Helpers(ctx, testHostID); !st.Prepared || st.IOContainer != "ok" {
		t.Fatalf("prepared: %+v", st)
	}

	// An IO container made for another stacks directory is a mismatch.
	env.fe.mu.Lock()
	for _, fc := range env.fe.containers {
		fc.labels[LabelStacksDir] = "/srv/other"
	}
	env.fe.mu.Unlock()
	if st, _ = env.svc.Helpers(ctx, testHostID); st.Prepared || st.IOContainer != "mismatch" {
		t.Fatalf("other stacks directory: %+v", st)
	}

	env.fe.setVersion("1.40", "linux", "amd64")
	st, err = env.svc.Helpers(ctx, testHostID)
	if err != nil || st.APISupported || st.Prepared || st.APIVersion != "1.40" {
		t.Fatalf("API 1.40: %+v, %v", st, err)
	}
	if _, err := env.svc.Helpers(ctx, "h_unknown"); !errors.Is(err, dockerhost.ErrHostNotFound) {
		t.Fatalf("unknown host = %v, want ErrHostNotFound", err)
	}
}

func TestHelpersFindACopiedImageByItsLocalTag(t *testing.T) {
	env := newTestEnv(t, false)
	env.fe.addImage(RunnerLocalTag, "sha256:copied")
	env.fe.addContainer(IOContainerName, ioSpec(RunnerLocalTag, env.host), "created", 0)
	st, err := env.svc.Helpers(context.Background(), testHostID)
	if err != nil || !st.Prepared {
		t.Fatalf("a host that has the image only under the local tag is prepared, got %+v, %v", st, err)
	}
}

func TestReadsOnAnUnpreparedHostAnswerHostNotPreparedAndCreateNothing(t *testing.T) {
	env := newTestEnv(t, false)
	_, err := env.svc.List(context.Background(), testHostID, false)
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CodeHostNotPrepared {
		t.Fatalf("List = %v, want %s", err, CodeHostNotPrepared)
	}
	helpers, ok := coded.Fields["helpers"].(HelperStatus)
	if !ok || helpers.Prepared || helpers.RunnerImage != RunnerImageTag {
		t.Fatalf("fields = %+v, want the helper status", coded.Fields)
	}
	for _, prefix := range []string{"POST /containers/create", "POST /images/create"} {
		if n := env.fe.requestCount(prefix); n != 0 {
			t.Fatalf("a read made %d %s requests; reads never create or pull", n, prefix)
		}
	}

	env.fe.prepared()
	env.fe.setVersion("1.40", "linux", "amd64")
	_, err = env.svc.List(context.Background(), testHostID, false)
	if CodeOf(err) != CodeAPITooOld {
		t.Fatalf("List on API 1.40 = %v, want %s", err, CodeAPITooOld)
	}
	if errors.As(err, &coded); coded.Fields["api_version"] != "1.40" {
		t.Fatalf("fields = %+v, want api_version 1.40", coded.Fields)
	}
}

func seedStacks(env *testEnv) {
	fe := env.fe
	for _, f := range []string{
		"/opt/stacks/web/compose.yaml", "/opt/stacks/web/.env", "/opt/stacks/db/docker-compose.yml",
		"/opt/stacks/draft/compose.yaml", "/opt/stacks/My Stack/compose.yaml", "/opt/stacks/envonly/.env",
	} {
		fe.putFile(f, "x")
	}
	now := time.Now().Unix()
	fe.addProjectContainer("web-web-1", "web", "web", "running", "Up 2 hours (healthy)", "healthy", now-7200,
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"}, fakePort{IP: "::", Private: 80, Public: 8080, Proto: "tcp"})
	fe.addProjectContainer("web-cache-1", "web", "cache", "running", "Up 2 hours (unhealthy)", "unhealthy", now-7300)
	fe.addProjectContainer("db-db-1", "db", "db", "exited", "Exited (1) 3 minutes ago", "", now-600)
	fe.addProjectContainer("legacy-app-1", "legacy", "app", "running", "Up 1 day", "", now-86400,
		fakePort{IP: "0.0.0.0", Private: 3000, Public: 3000, Proto: "tcp"})
}

func TestListBuildsStacksExternalProjectsAndIgnoredFolders(t *testing.T) {
	env := newTestEnv(t, true)
	seedStacks(env)
	listing, err := env.svc.List(context.Background(), testHostID, false)
	if err != nil {
		t.Fatal(err)
	}
	if listing.HostID != testHostID || listing.StacksDir != "/opt/stacks" || listing.Cached {
		t.Fatalf("listing header = %+v", listing)
	}
	names := make([]string, len(listing.Stacks))
	for i, s := range listing.Stacks {
		names[i] = s.Name
	}
	if strings.Join(names, ",") != "db,draft,web" {
		t.Fatalf("stacks = %v, want db, draft, web sorted (envonly has no compose file)", names)
	}
	db, draft, web := listing.Stacks[0], listing.Stacks[1], listing.Stacks[2]
	if db.Status != StatusError || db.ComposeFile != "docker-compose.yml" || db.HasEnv || db.Dir != "/opt/stacks/db" {
		t.Fatalf("db = %+v", db)
	}
	if draft.Status != StatusDraft || draft.Created != 0 || len(draft.ContainerIDs) != 0 || draft.Ports == nil {
		t.Fatalf("draft = %+v", draft)
	}
	if web.Status != StatusPartial || web.Services.Total != 2 || web.Services.Running != 2 || !web.HasEnv || web.ComposeFile != "compose.yaml" {
		t.Fatalf("web = %+v", web)
	}
	if len(web.Ports) != 1 || web.Ports[0].PublicPort != 8080 || web.Ports[0].Service != "web" {
		t.Fatalf("web ports = %+v, want the IPv4 and IPv6 binding collapsed", web.Ports)
	}
	if len(web.ContainerIDs) != 2 || len(web.ContainerIDs[0]) != 12 || web.Created == 0 {
		t.Fatalf("web ids/created = %v / %d", web.ContainerIDs, web.Created)
	}
	if len(listing.External) != 1 || listing.External[0].Name != "legacy" || listing.External[0].Status != StatusRunning ||
		listing.External[0].WorkingDir != "/opt/stacks/legacy" || len(listing.External[0].Ports) != 1 {
		t.Fatalf("external = %+v", listing.External)
	}
	if len(listing.Ignored) != 1 || listing.Ignored[0].Name != "My Stack" {
		t.Fatalf("ignored = %+v", listing.Ignored)
	}
	// The IO container carries a role label and must not show up anywhere.
	for _, s := range listing.Stacks {
		for _, id := range s.ContainerIDs {
			if strings.HasPrefix(env.fe.container(IOContainerName).id, id) {
				t.Fatalf("the IO container is listed in stack %s", s.Name)
			}
		}
	}
}

func TestListCachesTheFolderScanButReadsStatusLive(t *testing.T) {
	env := newTestEnv(t, true)
	seedStacks(env)
	ctx := context.Background()
	listerRuns := func() int { return env.fe.requestCount("POST /containers/create?name=aurago-dm-ls-") }

	first, err := env.svc.List(ctx, testHostID, false)
	if err != nil || first.Cached || listerRuns() != 1 {
		t.Fatalf("first List: cached=%v runs=%d err=%v", first.Cached, listerRuns(), err)
	}
	if left := env.fe.containersWithRole(RoleLister); len(left) != 0 {
		t.Fatalf("%d lister containers left behind", len(left))
	}

	// A new folder is not seen within the TTL, a container state change is.
	env.fe.putFile("/opt/stacks/fresh/compose.yaml", "x")
	cache := env.fe.container("web-cache-1")
	env.fe.mu.Lock() // the first List started a background sweep that reads the same container
	cache.state = "exited"
	env.fe.mu.Unlock()
	second, err := env.svc.List(ctx, testHostID, false)
	if err != nil || !second.Cached || listerRuns() != 1 || len(second.Stacks) != 3 {
		t.Fatalf("second List: cached=%v runs=%d stacks=%d err=%v", second.Cached, listerRuns(), len(second.Stacks), err)
	}
	if !second.ScannedAt.Equal(first.ScannedAt) {
		t.Fatal("a cached listing keeps the time of its scan")
	}
	if web := second.Stacks[2]; web.Services.Running != 1 {
		t.Fatalf("status must be live, web = %+v", web)
	}

	refreshed, err := env.svc.List(ctx, testHostID, true)
	if err != nil || refreshed.Cached || listerRuns() != 2 || len(refreshed.Stacks) != 4 {
		t.Fatalf("refresh: cached=%v runs=%d stacks=%d err=%v", refreshed.Cached, listerRuns(), len(refreshed.Stacks), err)
	}

	env.fe.putFile("/opt/stacks/later/compose.yaml", "x")
	env.clock.Advance(31 * time.Minute)
	if expired, _ := env.svc.List(ctx, testHostID, false); expired.Cached || len(expired.Stacks) != 5 {
		t.Fatalf("after the TTL the scan must run again, cached=%v stacks=%d", expired.Cached, len(expired.Stacks))
	}

	env.svc.invalidateScan(testHostID)
	if again, _ := env.svc.List(ctx, testHostID, false); again.Cached {
		t.Fatal("invalidateScan must force a new scan")
	}
}

func TestListerFailures(t *testing.T) {
	ctx := context.Background()

	env := newTestEnv(t, true)
	env.fe.startFails = "invalid mount config for type \"bind\": bind source path does not exist"
	_, err := env.svc.List(ctx, testHostID, false)
	var coded *Error
	if !errors.As(err, &coded) || coded.Code != CodeStacksDirInvalid || !strings.Contains(coded.Message, "/opt/stacks") ||
		!strings.Contains(coded.Message, "invalid mount config") || dockerhost.StatusOf(err) != http.StatusInternalServerError {
		t.Fatalf("a refused bind = %v, want %s naming the directory and the Engine's reason", err, CodeStacksDirInvalid)
	}
	if left := env.fe.containersWithRole(RoleLister); len(left) != 0 {
		t.Fatal("a failed lister must still be removed")
	}

	env = newTestEnv(t, true)
	env.fe.listerExit, env.fe.listerErr = 1, "find: '/opt/stacks': Permission denied\n"
	if _, err = env.svc.List(ctx, testHostID, false); CodeOf(err) != CodeStacksDirInvalid || !strings.Contains(err.Error(), "Permission denied") {
		t.Fatalf("find that found nothing and complained = %v, want %s with find's message", err, CodeStacksDirInvalid)
	}

	env = newTestEnv(t, true)
	seedStacks(env)
	env.fe.listerExit, env.fe.listerErr = 1, "find: '/opt/stacks/web/data': Permission denied\n"
	listing, err := env.svc.List(ctx, testHostID, false)
	if err != nil || len(listing.Stacks) != 3 {
		t.Fatalf("find that exits 1 after printing stacks is not a failure, got %v / %v", listing.Stacks, err)
	}
}

func TestListAttachesTheRunningOperationToItsStack(t *testing.T) {
	env := newTestEnv(t, true)
	seedStacks(env)
	registerRun(t, env.svc, Operation{ID: "op_9999999999999999", HostID: testHostID, Stack: "web", Kind: KindUpdate, State: StateRunning})
	listing, err := env.svc.List(context.Background(), testHostID, false)
	if err != nil {
		t.Fatal(err)
	}
	web := listing.Stacks[2]
	if web.Operation == nil || web.Operation.ID != "op_9999999999999999" || web.Operation.Kind != KindUpdate || listing.Stacks[0].Operation != nil {
		t.Fatalf("operation refs: web=%+v db=%+v", web.Operation, listing.Stacks[0].Operation)
	}
}

func TestAListerThatNeverFinishesIsStoppedAndListAnswersWithAnError(t *testing.T) {
	prev := listerTimeout
	listerTimeout = 300 * time.Millisecond
	t.Cleanup(func() { listerTimeout = prev })
	env := newTestEnv(t, true)
	seedComposeStack(env, "web")
	env.fe.mu.Lock()
	env.fe.listerGate = make(chan struct{}) // the find hangs, as on a stalled mount
	env.fe.mu.Unlock()

	// The caller's context has no deadline, as with a request that sets none.
	started := time.Now()
	_, err := env.svc.List(context.Background(), testHostID, true)
	if CodeOf(err) != CodeStacksDirInvalid || !strings.Contains(err.Error(), "could not be listed within") {
		t.Fatalf("a hung lister = %v, want stacks_dir_unavailable that names the time", err)
	}
	if took := time.Since(started); took > 5*time.Second {
		t.Fatalf("List took %s; the lister must be bounded by its own deadline", took)
	}
	kills := 0
	for _, r := range env.fe.requests() {
		if strings.HasPrefix(r, "POST /v1.45/containers/") && strings.HasSuffix(r, "/kill") {
			kills++
		}
	}
	if kills != 1 {
		t.Fatalf("the lister must be killed once: %v", env.fe.requests())
	}
	waitUntil(t, "the lister to be removed", 5*time.Second, func() bool { return len(env.fe.containersWithRole(RoleLister)) == 0 })
}
