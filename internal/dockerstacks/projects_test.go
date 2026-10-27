package dockerstacks

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"aurago/internal/dockerhost"
)

// addLifecycleRoutes teaches the fake the stop and restart actions of project
// containers (the fake itself serves start and kill).
func addLifecycleRoutes(fe *fakeEngine) {
	set := func(state string, exit int) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			fe.mu.Lock()
			fc := fe.lookupLocked(r.PathValue("id"))
			fe.mu.Unlock()
			if fc == nil {
				fakeMessage(w, http.StatusNotFound, "No such container: %s", r.PathValue("id"))
				return
			}
			if state == "exited" {
				fe.finish(fc, exit)
			} else {
				fe.mu.Lock()
				fc.state = state
				fe.mu.Unlock()
			}
			w.WriteHeader(http.StatusNoContent)
		}
	}
	fe.mux.HandleFunc("POST /v1.45/containers/{id}/stop", set("exited", 0))
	fe.mux.HandleFunc("POST /v1.45/containers/{id}/restart", set("running", 0))
}

func TestProjectActionStartsStopsAndRestartsEveryContainer(t *testing.T) {
	env := newTestEnv(t, true)
	addLifecycleRoutes(env.fe)
	now := time.Now().Unix()
	env.fe.addProjectContainer("legacy-app-1", "legacy", "app", "running", "Up", "", now)
	env.fe.addProjectContainer("legacy-db-1", "legacy", "db", "running", "Up", "", now)
	env.fe.addProjectContainer("other-app-1", "other", "app", "running", "Up", "", now)
	ctx := context.Background()
	states := func() string {
		var s []string
		for _, n := range []string{"legacy-app-1", "legacy-db-1", "other-app-1"} {
			fc := env.fe.container(n)
			env.fe.mu.Lock()
			s = append(s, fc.state)
			env.fe.mu.Unlock()
		}
		return strings.Join(s, ",")
	}

	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "stop"); err != nil {
		t.Fatal(err)
	}
	if got := states(); got != "exited,exited,running" {
		t.Fatalf("after stop: %s, want only the legacy project stopped", got)
	}
	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "start"); err != nil {
		t.Fatal(err)
	}
	if got := states(); got != "running,running,running" {
		t.Fatalf("after start: %s", got)
	}
	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "restart"); err != nil {
		t.Fatal(err)
	}
	if env.fe.requestCount("POST /containers/") < 6 {
		t.Fatalf("each action must reach both containers: %v", env.fe.requests())
	}
	// Starting what already runs is fine: the Engine answers 304.
	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "start"); err != nil {
		t.Fatalf("start of a running project: %v", err)
	}
}

func TestProjectActionRefusals(t *testing.T) {
	env := newTestEnv(t, true)
	addLifecycleRoutes(env.fe)
	env.fe.addProjectContainer("legacy-app-1", "legacy", "app", "running", "Up", "", time.Now().Unix())
	ctx := context.Background()
	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "pause"); CodeOf(err) != CodeInvalidOperation {
		t.Fatalf("pause = %v", err)
	}
	if err := env.svc.ProjectAction(ctx, testHostID, "ghost", "stop"); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("unknown project = %v", err)
	}
	if err := env.svc.ProjectAction(ctx, testHostID, "", "stop"); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("empty project = %v", err)
	}
	if err := env.svc.ProjectAction(ctx, "h_unknown", "legacy", "stop"); !errors.Is(err, dockerhost.ErrHostNotFound) {
		t.Fatalf("unknown host = %v", err)
	}
	registerRun(t, env.svc, Operation{ID: "op_7777777777777777", HostID: testHostID, Stack: "legacy", Kind: KindUp, State: StateRunning})
	var busy *BusyError
	if err := env.svc.ProjectAction(ctx, testHostID, "legacy", "stop"); !errors.As(err, &busy) || busy.OperationID != "op_7777777777777777" {
		t.Fatalf("a project with a running operation = %v, want a BusyError", err)
	}
	if env.fe.container("legacy-app-1").state != "running" {
		t.Fatal("nothing may be touched while an operation runs")
	}
}

func TestProjectActionTriesEveryContainerAndReturnsTheFirstFailure(t *testing.T) {
	env := newTestEnv(t, true)
	// No stop route: every stop answers 404, but both containers must be asked.
	now := time.Now().Unix()
	env.fe.addProjectContainer("legacy-a-1", "legacy", "a", "running", "Up", "", now)
	env.fe.addProjectContainer("legacy-b-1", "legacy", "b", "running", "Up", "", now)
	err := env.svc.ProjectAction(context.Background(), testHostID, "legacy", "stop")
	if dockerhost.StatusOf(err) != http.StatusNotFound {
		t.Fatalf("err = %v, want the Engine's 404", err)
	}
	if n := env.fe.requestCount("POST /containers/"); n != 2 {
		t.Fatalf("%d stop requests, want one per container", n)
	}
}

func TestProjectVolumesListsLabelledVolumesWithSizes(t *testing.T) {
	env := newTestEnv(t, true)
	env.fe.volumes = []fakeVolume{
		{Name: "shop_pg", Driver: "local", Project: "shop", Size: 4096},
		{Name: "shop_cache", Driver: "local", Project: "shop", Size: -1},
		{Name: "other_data", Driver: "local", Project: "other", Size: 7},
	}
	ctx := context.Background()
	volumes, err := env.svc.ProjectVolumes(ctx, testHostID, "shop")
	if err != nil {
		t.Fatal(err)
	}
	want := []ProjectVolume{{Name: "shop_cache", Driver: "local", Size: -1}, {Name: "shop_pg", Driver: "local", Size: 4096}}
	if len(volumes) != 2 || volumes[0] != want[0] || volumes[1] != want[1] {
		t.Fatalf("volumes = %+v, want %+v", volumes, want)
	}

	// An Engine below API 1.42 has no type=volume: the sizes stay unknown and df is not asked.
	env.fe.setVersion("1.41", "linux", "amd64")
	dfBefore := env.fe.requestCount("GET /system/df")
	volumes, err = env.svc.ProjectVolumes(ctx, testHostID, "shop")
	if err != nil || volumes[1].Size != -1 || env.fe.requestCount("GET /system/df") != dfBefore {
		t.Fatalf("API 1.41: %+v, %v, want unknown sizes and no df request", volumes, err)
	}

	// A failing df does not fail the call.
	env.fe.setVersion("1.45", "linux", "amd64")
	env.fe.dfStatus = http.StatusInternalServerError
	volumes, err = env.svc.ProjectVolumes(ctx, testHostID, "shop")
	if err != nil || len(volumes) != 2 || volumes[1].Size != -1 {
		t.Fatalf("a failing df: %+v, %v, want the volumes with unknown sizes", volumes, err)
	}
	none, err := env.svc.ProjectVolumes(ctx, testHostID, "nothing")
	if err != nil || len(none) != 0 || none == nil {
		t.Fatalf("a project without volumes = %+v, %v, want an empty list", none, err)
	}
	if _, err := env.svc.ProjectVolumes(ctx, testHostID, "../x"); CodeOf(err) != CodeStackNameInvalid {
		t.Fatalf("bad name = %v", err)
	}
}

func TestPublishedPortsSkipHelpersStoppedContainersAndTheExcludedProject(t *testing.T) {
	env := newTestEnv(t, true)
	now := time.Now().Unix()
	env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up", "", now,
		fakePort{IP: "0.0.0.0", Private: 80, Public: 8080, Proto: "tcp"}, fakePort{IP: "::", Private: 80, Public: 8080, Proto: "tcp"},
		fakePort{IP: "127.0.0.1", Private: 9000, Public: 9000, Proto: "tcp"})
	env.fe.addProjectContainer("dns-dns-1", "dns", "dns", "running", "Up", "", now, fakePort{Private: 53, Public: 53, Proto: "udp"}, fakePort{Private: 53, Public: 53, Proto: "tcp"})
	env.fe.addProjectContainer("old-app-1", "old", "app", "exited", "Exited (0) 1 hour ago", "", now, fakePort{Private: 80, Public: 8081, Proto: "tcp"})
	solo := env.fe.addProjectContainer("solo", "", "solo", "running", "Up", "", now, fakePort{Private: 22, Public: 2222, Proto: "tcp"})
	solo.labels = map[string]string{}
	lister := env.fe.addContainer("aurago-dm-ls-1", listerSpec(RunnerImage, env.host), "running", 0)
	lister.ports = []fakePort{{Private: 1, Public: 1, Proto: "tcp"}}

	ports, err := env.svc.PublishedPorts(context.Background(), testHostID, "")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range ports {
		got = append(got, p.Container+":"+p.Protocol+":"+p.HostIP+":"+strconv.Itoa(p.Port)+":"+p.Project)
	}
	want := "dns-dns-1:tcp::53:dns,dns-dns-1:udp::53:dns,solo:tcp::2222:,web-web-1:tcp::8080:web,web-web-1:tcp:127.0.0.1:9000:web"
	if strings.Join(got, ",") != want {
		t.Fatalf("ports =\n%s\nwant\n%s", strings.Join(got, "\n"), want)
	}
	excluded, _ := env.svc.PublishedPorts(context.Background(), testHostID, "web")
	for _, p := range excluded {
		if p.Project == "web" {
			t.Fatalf("the excluded project's own ports are not conflicts: %+v", p)
		}
	}
	if len(excluded) != 3 {
		t.Fatalf("excluded = %+v", excluded)
	}
}

func TestProjectActionNeverTouchesTheContainersOfAStackOfTheSameName(t *testing.T) {
	env := newTestEnv(t, true)
	addLifecycleRoutes(env.fe)
	seedComposeStack(env, "web")
	now := time.Now().Unix()
	env.fe.addProjectContainer("web-web-1", "web", "web", "running", "Up", "", now) // the stack's own, from /opt/stacks/web
	foreign := env.fe.addProjectContainer("web-db-1", "web", "db", "running", "Up", "", now)
	env.fe.setWorkingDir(foreign, "/home/andi/web") // an external project that happens to have the stack's name
	ctx := context.Background()
	state := func(name string) string {
		fc := env.fe.container(name)
		env.fe.mu.Lock()
		defer env.fe.mu.Unlock()
		return fc.state
	}

	if err := env.svc.ProjectAction(ctx, testHostID, "web", "stop"); err != nil {
		t.Fatal(err)
	}
	if state("web-db-1") != "exited" || state("web-web-1") != "running" {
		t.Fatalf("states: foreign %s, own %s; want only the external project stopped", state("web-db-1"), state("web-web-1"))
	}

	// Without an external project of that name there is nothing for the action to do.
	env.fe.setWorkingDir(foreign, "/opt/stacks/web")
	before := env.fe.requestCount("POST /containers/")
	if err := env.svc.ProjectAction(ctx, testHostID, "web", "start"); CodeOf(err) != CodeStackNotFound {
		t.Fatalf("a project action on a stack's own containers = %v, want %s", err, CodeStackNotFound)
	}
	if env.fe.requestCount("POST /containers/") != before {
		t.Fatal("no container may be touched")
	}
}
