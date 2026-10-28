package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"aurago/internal/dockerhost"
)

// dmEngineCalls counts the recorded calls of one "METHOD /path" on the fake engine.
func dmEngineCalls(e *dmFakeEngine, call string) int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, got := range e.calls {
		if got == call {
			n++
		}
	}
	return n
}

// The local host's container list used to list the Engine twice (the list, then the protection classification). One request is one
// listing now, and the answer still carries the classification.
func TestDockerManagerLocalContainerListListsTheEngineOnce(t *testing.T) {
	f := newDMFixture(t)

	before := dmEngineCalls(f.local, "GET /containers/json")
	code, body := f.do(http.MethodGet, "/hosts/local/containers", nil)
	if code != http.StatusOK {
		t.Fatalf("local list = %d %s", code, f.lastRaw)
	}
	if got := dmEngineCalls(f.local, "GET /containers/json") - before; got != 1 {
		t.Fatalf("GET /hosts/local/containers made %d Engine listings, want 1", got)
	}
	rv := dmContainerNamed(t, dmArray(t, body, "containers"), "rv")
	if rv["protected_owner"] != "ruview" {
		t.Fatalf("rv = %v, want the protection from the single listing", rv)
	}

	// A remote host never read the local classification and still lists once.
	id := f.addRemote("NAS")
	before = dmEngineCalls(f.remote, "GET /containers/json")
	if code, _ = f.do(http.MethodGet, "/hosts/"+id+"/containers", nil); code != http.StatusOK {
		t.Fatalf("remote list = %d %s", code, f.lastRaw)
	}
	if got := dmEngineCalls(f.remote, "GET /containers/json") - before; got != 1 {
		t.Fatalf("remote list made %d Engine listings, want 1", got)
	}
}

// The classification of the Docker Manager's list is the Containers page's (/api/containers): for the same Engine answer, entries
// read from the Docker Manager's listing give exactly the JSON that adminContainerListJSON builds from its own listing. The
// fixture has a container that others join (shared network), the one AuraGo runs in (self), one at the Docker endpoint (both
// address families) and one that another AuraGo feature owns.
func TestDockerManagerListingEntriesClassifyLikeTheContainersPage(t *testing.T) {
	const (
		idA = "aaaaaaaaaaaa00000000000000000000000000000000000000000000000000aa"
		idB = "bbbbbbbbbbbb00000000000000000000000000000000000000000000000000bb"
		idC = "cccccccccccc00000000000000000000000000000000000000000000000000cc"
		idD = "dddddddddddd00000000000000000000000000000000000000000000000000dd"
		idE = "eeeeeeeeeeee00000000000000000000000000000000000000000000000000ee"
	)
	engineList := `[
		{"Id":"` + idA + `","Names":["/gateway"],"Image":"gw:1","State":"running","Status":"Up 1 hour","Labels":{"com.docker.compose.service":"gw"},
		 "HostConfig":{"NetworkMode":"bridge"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2","GlobalIPv6Address":""}}}},
		{"Id":"` + idB + `","Names":["/vpn-client"],"Image":"vpn:1","State":"running","Status":"Up 1 hour","Labels":{},
		 "HostConfig":{"NetworkMode":"container:` + idA + `"},"NetworkSettings":{"Networks":{}}},
		{"Id":"` + idC + `","Names":["/docker-proxy"],"Image":"proxy:1","State":"running","Status":"Up 1 hour","Labels":null,
		 "HostConfig":{"NetworkMode":" appnet "},"NetworkSettings":{"Networks":{"appnet":{"IPAddress":"172.30.0.5"},"v6":{"IPAddress":"","GlobalIPv6Address":"fd00::5"}}}},
		{"Id":"` + idD + `","Names":["/rv","/rv-alias"],"Image":"rv:1","State":"exited","Status":"Exited (0) 2 days ago","Labels":{"aurago.managed":"ruview"},
		 "HostConfig":{"NetworkMode":"default"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.9"}}}},
		{"Id":"` + idE + `","Names":["/aurago"],"Image":"aurago:1","State":"running","Status":"Up 1 day","Labels":{},
		 "HostConfig":{"NetworkMode":"bridge"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.3"}}}}
	]`
	for name, hostname := range map[string]string{"shared network": "aaaaaaaaaaaa", "self": "eeeeeeeeeeee"} {
		t.Run(name, func(t *testing.T) {
			engine := newDMFakeEngine(t, engineList)
			s := newDMServer(t, engine.endpoint())
			s.Cfg.Runtime.IsDocker = true
			defer replaceContainerSelfHostname(hostname)()
			defer replaceContainerSelfProcFiles(nil)()
			defer replaceContainerEndpointAddresses("172.30.0.5", "fd00::5")()
			cfg, _, _ := containerDockerConfig(s)
			ctx := context.Background()

			want := adminContainerListJSON(ctx, s, cfg)
			if !strings.Contains(want, `"status":"ok"`) || !strings.Contains(want, `"docker_endpoint":true`) || !strings.Contains(want, `"protected_owner":"ruview"`) {
				t.Fatalf("the fixture must produce an endpoint flag and an owner, got %s", want)
			}
			if flag := map[string]string{"shared network": `"shared_network":true`, "self": `"self":true`}[name]; !strings.Contains(want, flag) {
				t.Fatalf("the fixture must produce %s, got %s", flag, want)
			}

			client, err := s.DockerHosts.Client(ctx, dockerhost.LocalHostID)
			if err != nil {
				t.Fatal(err)
			}
			list, entries, err := dockerhost.ListContainersWithEntries(ctx, client)
			if err != nil {
				t.Fatal(err)
			}
			if len(list) != 5 || len(entries) != 5 {
				t.Fatalf("listing gave %d summaries and %d entries, want 5 each", len(list), len(entries))
			}
			got := adminContainerListJSONOf(adminContainerEntries(ctx, s, cfg, entries))
			if got != want {
				t.Fatalf("classification of the Docker Manager's listing differs from the Containers page's:\n got %s\nwant %s", got, want)
			}
			if _, err := dockerhost.ListContainers(ctx, client); err != nil {
				t.Fatal(err)
			}
		})
	}
}

// adminContainerListJSONOf encodes classified entries the way adminContainerListJSON answers.
func adminContainerListJSONOf(entries []adminContainerEntry) string {
	encoded, _ := json.Marshal(map[string]interface{}{"status": "ok", "count": len(entries), "containers": entries})
	return string(encoded)
}
