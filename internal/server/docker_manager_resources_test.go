package server

import (
	"net/http"
	"testing"
)

func TestDockerManagerOverview(t *testing.T) {
	f := newDMFixture(t)
	code, body := f.do(http.MethodGet, "/hosts/local/overview", nil)
	info := dmObject(t, body, "info")
	if code != http.StatusOK || info["server_version"] != "27.0.1" || info["ncpu"] != float64(4) || info["containers_running"] != float64(1) {
		t.Fatalf("overview = %d %s, want the engine figures in snake_case", code, f.lastRaw)
	}
}

func TestDockerManagerResourceListsCarryUsage(t *testing.T) {
	f := newDMFixture(t)

	code, body := f.do(http.MethodGet, "/hosts/local/images", nil)
	images := dmArray(t, body, "images")
	if code != http.StatusOK || len(images) != 3 {
		t.Fatalf("images = %d %s", code, f.lastRaw)
	}
	if got := dmFind(t, images, "id", "sha256:img1")["containers"]; got != float64(1) {
		t.Fatalf("img1 containers = %v, want 1", got)
	}
	old := dmFind(t, images, "id", "sha256:old")
	if old["containers"] != float64(0) || old["dangling"] != true {
		t.Fatalf("old image = %v, want unused and dangling", old)
	}
	if tags, ok := old["tags"].([]any); !ok || len(tags) != 0 {
		t.Fatalf("old image tags = %v, want an empty array, never null", old["tags"])
	}

	code, body = f.do(http.MethodGet, "/hosts/local/volumes", nil)
	volumes := dmArray(t, body, "volumes")
	used, idle := dmFind(t, volumes, "name", "data"), dmFind(t, volumes, "name", "idle")
	if usedBy, _ := used["used_by"].([]any); code != http.StatusOK || len(usedBy) != 1 || usedBy[0] != "web1" {
		t.Fatalf("volume data = %v, want used_by [web1]", used)
	}
	if usedBy, ok := idle["used_by"].([]any); !ok || len(usedBy) != 0 {
		t.Fatalf("volume idle used_by = %v, want an empty array, never null", idle["used_by"])
	}

	code, body = f.do(http.MethodGet, "/hosts/local/networks", nil)
	networks := dmArray(t, body, "networks")
	bridge, spare := dmFind(t, networks, "name", "bridge"), dmFind(t, networks, "name", "idle")
	if code != http.StatusOK || bridge["containers"] != float64(1) || bridge["builtin"] != true || spare["containers"] != float64(0) {
		t.Fatalf("networks = %s, want bridge builtin with 1 container and idle with 0", f.lastRaw)
	}
}

func TestDockerManagerResourceListsRejectOtherMethods(t *testing.T) {
	f := newDMFixture(t)
	for _, path := range []string{"/hosts/local/images", "/hosts/local/volumes", "/hosts/local/networks", "/hosts/local/overview", "/hosts/local/containers"} {
		code, body := f.do(http.MethodPost, path, nil)
		dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	}
}

func TestDockerManagerResourceListsOfAnUnreachableHost(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.srv.Close()
	for _, path := range []string{"/overview", "/images", "/volumes", "/networks"} {
		code, body := f.do(http.MethodGet, "/hosts/"+id+path, nil)
		dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	}
}
