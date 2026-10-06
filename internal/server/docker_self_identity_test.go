package server

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"aurago/internal/tools"
)

const selfInspectWithComposeProject = `{"Id":"` + selfContainerID + `","Config":{"Labels":{"com.docker.compose.project":"aurago","com.docker.compose.service":"aurago"}},"Mounts":[` +
	`{"Type":"volume","Name":"aurago_aurago_data","Source":"/var/lib/docker/volumes/aurago_aurago_data/_data","Destination":"/app/data"},` +
	`{"Type":"volume","Name":"aurago_models","Source":"/var/lib/docker/volumes/aurago_models/_data","Destination":"/app/data/models/aurago-qwen35"},` +
	`{"Type":"volume","Name":"aurago_aurago_workdir","Source":"/var/lib/docker/volumes/aurago_aurago_workdir/_data","Destination":"/app/agent_workspace/workdir"},` +
	`{"Type":"bind","Source":"/srv/aurago/secrets","Destination":"/run/optional-secrets"},` +
	`{"Type":"bind","Source":"/srv/aurago/extra","Destination":"/app/data/extra"}]}`

func newSelfIdentityServer(t *testing.T, inspect func(w http.ResponseWriter)) (*Server, *atomic.Int64) {
	t.Helper()
	var inspects atomic.Int64
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/containers/"+selfContainerID+"/json") {
			inspects.Add(1)
			inspect(w)
			return
		}
		http.NotFound(w, r)
	})
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = host
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Directories.DataDir = "/app/data"
	return s, &inspects
}

func TestDockerSelfIdentityReadsComposeProjectAndDataMounts(t *testing.T) {
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerSelfHostname("aurago-test")()
	s, inspects := newSelfIdentityServer(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte(selfInspectWithComposeProject)) })
	s.bindDockerSelfIdentity()
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })

	cfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
	want := tools.DockerSelfIdentity{
		ComposeProject:   "aurago",
		StateVolumes:     []string{"aurago_aurago_data", "aurago_models"},
		StateBindSources: []string{"/srv/aurago/extra"},
	}
	if got := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("identity = %+v, want %+v (the workdir volume and the secrets bind are not AuraGo state)", got, want)
	}
	if again := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(again, want) || inspects.Load() != 1 {
		t.Fatalf("second lookup = %+v after %d inspects, want the cached identity after one inspect", again, inspects.Load())
	}
}

func TestDockerSelfIdentityIsEmptyOnNativeRuntime(t *testing.T) {
	s, inspects := newSelfIdentityServer(t, func(w http.ResponseWriter) { _, _ = w.Write([]byte(selfInspectWithComposeProject)) })
	s.Cfg.Runtime.IsDocker = false
	s.bindDockerSelfIdentity()
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
	if got := tools.DockerSelfIdentityFor(context.Background(), tools.DockerConfig{Host: s.Cfg.Docker.Host}); !reflect.DeepEqual(got, tools.DockerSelfIdentity{}) || inspects.Load() != 0 {
		t.Fatalf("native identity = %+v after %d inspects, want zero and no Docker request", got, inspects.Load())
	}
}

func TestDockerSelfIdentityRetriesAFailedInspectOnlyLater(t *testing.T) {
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerSelfHostname("aurago-test")()
	s, inspects := newSelfIdentityServer(t, func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) })
	s.bindDockerSelfIdentity()
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
	cfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
	for i := 0; i < 3; i++ {
		if got := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(got, tools.DockerSelfIdentity{}) {
			t.Fatalf("identity after a failed inspect = %+v, want zero", got)
		}
	}
	if inspects.Load() != 1 {
		t.Fatalf("inspects = %d, want 1 within the retry interval", inspects.Load())
	}
}
