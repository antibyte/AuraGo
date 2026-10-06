package tools

import (
	"encoding/json"
	"net/http"
	"slices"
	"strings"
	"testing"

	"aurago/internal/acestep"
	"aurago/internal/dockerutil"
)

func TestDockerListContainerEntriesMatchesDockerListContainers(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || !strings.HasSuffix(r.URL.Path, "/containers/json") {
			t.Errorf("unexpected Docker request %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"Id":"0123456789abcdef0123","Names":["/web"],"Image":"nginx:1","State":"running","Status":"Up 1 minute","Labels":{"com.docker.compose.service":"web"},"HostConfig":{"NetworkMode":"container:vpn"},"NetworkSettings":{"Networks":{"bridge":{"IPAddress":"172.17.0.2","GlobalIPv6Address":""}}}},
			{"Id":"short","Names":["/aurago-local-llm"],"Image":"llama","State":"exited","Status":"Exited (0)","Labels":{"aurago.managed":"local-llm"}}
		]`))
	})
	cfg := DockerConfig{Host: host}
	for _, excluded := range [][]string{nil, {dockerutil.LocalLLMOwner}} {
		entries, failure := DockerListContainerEntries(cfg, true, excluded...)
		if failure != "" {
			t.Fatalf("failure = %s", failure)
		}
		encoded, err := json.Marshal(map[string]interface{}{"status": "ok", "count": len(entries), "containers": entries})
		if err != nil {
			t.Fatalf("marshal entries: %v", err)
		}
		if got, want := string(encoded), DockerListContainers(cfg, true, excluded...); got != want {
			t.Fatalf("entries drifted from DockerListContainers (excluded %v):\n got %s\nwant %s", excluded, got, want)
		}
	}
	entries, _ := DockerListContainerEntries(cfg, true)
	if len(entries) != 2 {
		t.Fatalf("entries = %+v, want 2", entries)
	}
	if entries[0].FullID != "0123456789abcdef0123" || entries[0].Labels["com.docker.compose.service"] != "web" || !slices.Equal(entries[0].NetworkIPs, []string{"172.17.0.2"}) || entries[0].NetworkMode != "container:vpn" {
		t.Fatalf("first entry metadata = %+v", entries[0])
	}
	if entries[1].FullID != "short" || entries[1].Labels["aurago.managed"] != "local-llm" || len(entries[1].NetworkIPs) != 0 || entries[1].NetworkMode != "" {
		t.Fatalf("second entry metadata = %+v", entries[1])
	}
}

func TestDockerListContainerEntriesReturnsTheListErrorJSON(t *testing.T) {
	host := fakeDockerHost(t, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"engine refused"}`, http.StatusConflict)
	})
	cfg := DockerConfig{Host: host}
	entries, failure := DockerListContainerEntries(cfg, true)
	if entries != nil || !strings.Contains(failure, "engine refused") || failure != DockerListContainers(cfg, true) {
		t.Fatalf("entries = %+v, failure = %q; want the DockerListContainers error JSON", entries, failure)
	}
}

func TestDockerContainerOwnersFromMetadataMatchesReservedNamesAndLabels(t *testing.T) {
	owners := []string{dockerutil.AppOwner, acestep.Owner, dockerutil.HomepageOwner, "go2rtc", dockerutil.LocalLLMOwner, dockerutil.BoringGarageOwner}
	for _, tc := range []struct {
		name   string
		names  []string
		labels map[string]string
		want   string
	}{
		{"app container name", []string{"/aurago"}, nil, dockerutil.AppOwner},
		{"compose replica name", []string{"/stack-aurago-1"}, nil, dockerutil.AppOwner},
		{"go2rtc label", []string{"/cams"}, map[string]string{"aurago.managed": "go2rtc"}, "go2rtc"},
		{"legacy local llm labels", []string{"/renamed"}, map[string]string{"com.aurago.managed": "true", "com.aurago.owner": "local-llm"}, dockerutil.LocalLLMOwner},
		{"homepage web name", []string{"/aurago-homepage-web"}, nil, dockerutil.HomepageOwner},
		{"garage name", []string{"/aurago-boring-garage"}, nil, dockerutil.BoringGarageOwner},
		{"acestep name", []string{"/" + acestep.ContainerName}, nil, acestep.Owner},
		{"unmanaged", []string{"/nginx"}, map[string]string{"com.docker.compose.service": "web"}, ""},
	} {
		owned := DockerContainerOwnersFromMetadata(tc.names, tc.labels, owners...)
		if tc.want == "" {
			if len(owned) != 0 {
				t.Fatalf("%s: owners = %v, want none", tc.name, owned)
			}
			continue
		}
		if !owned[tc.want] || len(owned) != 1 {
			t.Fatalf("%s: owners = %v, want only %s", tc.name, owned, tc.want)
		}
	}
}
