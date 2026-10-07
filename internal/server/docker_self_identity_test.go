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

// selfListAlone lists the container the self signals name with nobody joining
// its network namespace: self is proven.
const selfListAlone = `[{"Id":"` + selfContainerID + `","Names":["/aurago"],"Labels":{"com.docker.compose.project":"aurago"},"HostConfig":{"NetworkMode":"aurago_default"}},` +
	`{"Id":"` + otherContainerID + `","Names":["/web"],"Labels":{"com.docker.compose.project":"vpn"},"HostConfig":{"NetworkMode":"bridge"}}]`

// selfListShared lists the named container as a network provider (Gluetun,
// Tailscale) that another container joins: the signals cannot tell AuraGo from
// its provider.
func selfListShared(providerProject, joinerProject string) string {
	return `[{"Id":"` + selfContainerID + `","Names":["/gluetun"],"Labels":{"com.docker.compose.project":"` + providerProject + `"},"HostConfig":{"NetworkMode":"bridge"}},` +
		`{"Id":"` + otherContainerID + `","Names":["/aurago"],"Labels":{"com.docker.compose.project":"` + joinerProject + `"},"HostConfig":{"NetworkMode":"container:` + selfContainerID + `"}}]`
}

type selfIdentityAPI struct {
	inspect func(w http.ResponseWriter)
	list    func(w http.ResponseWriter)
}

func newSelfIdentityServer(t *testing.T, api selfIdentityAPI) (*Server, *atomic.Int64, *atomic.Int64) {
	t.Helper()
	var inspects, lists atomic.Int64
	host := newContainerDockerAPI(t, func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/"+selfContainerID+"/json"):
			inspects.Add(1)
			api.inspect(w)
		case strings.HasSuffix(r.URL.Path, "/containers/json"):
			lists.Add(1)
			api.list(w)
		default:
			http.NotFound(w, r)
		}
	})
	s := testContainerServer(true, false)
	s.Cfg.Docker.Host = host
	s.Cfg.Runtime.IsDocker = true
	s.Cfg.Directories.DataDir = "/app/data"
	return s, &inspects, &lists
}

func writeBody(body string) func(http.ResponseWriter) {
	return func(w http.ResponseWriter) { _, _ = w.Write([]byte(body)) }
}

func TestDockerSelfIdentityReadsComposeProjectAndDataMounts(t *testing.T) {
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerSelfHostname("aurago-test")()
	s, inspects, _ := newSelfIdentityServer(t, selfIdentityAPI{inspect: writeBody(selfInspectWithComposeProject), list: writeBody(selfListAlone)})
	s.bindDockerSelfIdentity()
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })

	cfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
	want := tools.DockerSelfIdentity{
		ComposeProject:   "aurago",
		Proven:           true,
		StateVolumes:     []string{"aurago_aurago_data", "aurago_models"},
		StateBindSources: []string{"/srv/aurago/extra"},
		StateHostPaths:   []string{"/var/lib/docker/volumes/aurago_aurago_data/_data", "/var/lib/docker/volumes/aurago_models/_data"},
	}
	if got := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(got, want) {
		t.Fatalf("identity = %+v, want %+v (the workdir volume and the secrets bind are not AuraGo state)", got, want)
	}
	if again := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(again, want) || inspects.Load() != 1 {
		t.Fatalf("second lookup = %+v after %d inspects, want the cached identity after one inspect", again, inspects.Load())
	}
}

// With network_mode container:/service: the self signals name the network
// provider, as K12's containerSelfInList knows. Its project is AuraGo's only
// when every container of the shared namespace carries the same project, and
// its mounts are never AuraGo's proven state.
func TestDockerSelfIdentityDoesNotTakeASharedNetworkProviderForAuraGo(t *testing.T) {
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerSelfHostname("aurago-test")()
	for _, tc := range []struct {
		name, provider, joiner, want string
	}{
		{"provider of another project", "vpn", "aurago", ""},
		{"joiner of another project", "aurago", "tools", ""},
		{"one project", "aurago", "aurago", "aurago"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, _, lists := newSelfIdentityServer(t, selfIdentityAPI{inspect: writeBody(selfInspectWithComposeProject), list: writeBody(selfListShared(tc.provider, tc.joiner))})
			s.bindDockerSelfIdentity()
			t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
			got := tools.DockerSelfIdentityFor(context.Background(), tools.DockerConfig{Host: s.Cfg.Docker.Host})
			if want := (tools.DockerSelfIdentity{ComposeProject: tc.want}); !reflect.DeepEqual(got, want) || lists.Load() != 1 {
				t.Fatalf("identity = %+v after %d lists, want %+v: a shared namespace proves no mounts and only a common project", got, lists.Load(), want)
			}
		})
	}
}

func TestDockerSelfIdentityIsEmptyOnNativeRuntime(t *testing.T) {
	s, inspects, lists := newSelfIdentityServer(t, selfIdentityAPI{inspect: writeBody(selfInspectWithComposeProject), list: writeBody(selfListAlone)})
	s.Cfg.Runtime.IsDocker = false
	s.bindDockerSelfIdentity()
	t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
	if got := tools.DockerSelfIdentityFor(context.Background(), tools.DockerConfig{Host: s.Cfg.Docker.Host}); !reflect.DeepEqual(got, tools.DockerSelfIdentity{}) || inspects.Load() != 0 || lists.Load() != 0 {
		t.Fatalf("native identity = %+v after %d inspects and %d lists, want zero and no Docker request", got, inspects.Load(), lists.Load())
	}
}

func TestDockerSelfIdentityRetriesAFailedInspectOnlyLater(t *testing.T) {
	defer replaceContainerSelfProcFiles(map[string]string{"/proc/self/mountinfo": selfMountinfoFixture})()
	defer replaceContainerSelfHostname("aurago-test")()
	failed := func(w http.ResponseWriter) { w.WriteHeader(http.StatusInternalServerError) }
	for _, tc := range []struct {
		name string
		api  selfIdentityAPI
	}{
		{"inspect", selfIdentityAPI{inspect: failed, list: writeBody(selfListAlone)}},
		{"list", selfIdentityAPI{inspect: writeBody(selfInspectWithComposeProject), list: failed}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, inspects, _ := newSelfIdentityServer(t, tc.api)
			s.bindDockerSelfIdentity()
			t.Cleanup(func() { tools.SetDockerSelfIdentityResolver(nil) })
			cfg := tools.DockerConfig{Host: s.Cfg.Docker.Host}
			for i := 0; i < 3; i++ {
				if got := tools.DockerSelfIdentityFor(context.Background(), cfg); !reflect.DeepEqual(got, tools.DockerSelfIdentity{}) {
					t.Fatalf("identity after a failed %s = %+v, want zero", tc.name, got)
				}
			}
			if inspects.Load() != 1 {
				t.Fatalf("inspects = %d, want 1 within the retry interval", inspects.Load())
			}
		})
	}
}
