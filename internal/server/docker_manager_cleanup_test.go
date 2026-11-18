package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"aurago/internal/dockerstacks"
)

func TestDockerManagerCleanupRechecksSelection(t *testing.T) {
	id := func(letter string) string { return strings.Repeat(letter, 64) }
	imageID := func(letter string) string { return "sha256:" + id(letter) }
	var nowUsed, badInventory, replaced atomic.Bool
	var mu sync.Mutex
	var deletes []string
	engine := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := dmEngineVersionPrefix.ReplaceAllString(r.URL.Path, "")
		write := func(v any) { _ = json.NewEncoder(w).Encode(v) }
		switch {
		case p == "/version":
			write(map[string]string{"ApiVersion": "1.45", "MinAPIVersion": "1.25"})
		case r.Method == http.MethodDelete:
			mu.Lock()
			deletes = append(deletes, p+"?"+r.URL.RawQuery)
			mu.Unlock()
			if p == "/images/"+imageID("d") {
				http.Error(w, "image has multiple tags", http.StatusConflict)
			} else if p == "/images/"+imageID("e") {
				http.Error(w, "engine error", http.StatusInternalServerError)
			} else {
				w.WriteHeader(http.StatusNoContent)
			}
		case p == "/containers/json":
			if !r.URL.Query().Has("all") {
				t.Error("cleanup must include stopped containers")
			}
			if badInventory.Load() {
				write(nil)
				return
			}
			mounts := []map[string]any{{"Type": "volume", "Name": "in-use"}}
			rows := []map[string]any{
				dmContainerJSON('1', "stopped", imageID("a"), "exited", "in-use", nil, mounts),
				dmContainerJSON('2', "helper", imageID("a"), "exited", "bridge", map[string]string{dockerstacks.LabelRole: "io"}, nil),
				dmContainerJSON('3', "running", imageID("a"), "running", "bridge", nil, nil),
			}
			if nowUsed.Load() {
				rows = append(rows, dmContainerJSON('4', "new", imageID("c"), "created", "bridge", nil, nil))
			}
			write(rows)
		case p == "/images/json":
			var rows []map[string]any
			for i, tag := range []string{"in-use:1", "download:1", "retagged:1", "multi:1", "broken:1", dockerstacks.RunnerImageTag} {
				rows = append(rows, map[string]any{"Id": imageID(string("abcdef"[i])), "RepoTags": []string{tag}, "Size": 2048})
			}
			write(rows)
		case p == "/volumes":
			var rows []map[string]any
			for _, name := range []string{"archive", "in-use", "aurago_data", "managed"} {
				labels := map[string]string{}
				if name == "managed" {
					labels["aurago.managed"] = "storage"
				}
				rows = append(rows, map[string]any{"Name": name, "Driver": "local", "CreatedAt": "2026-10-01T00:00:00Z", "Labels": labels})
			}
			write(map[string]any{"Volumes": rows})
		case p == "/volumes/archive":
			stamp := "2026-10-01T00:00:00Z"
			if replaced.Load() {
				stamp = "2026-11-01T00:00:00Z"
			}
			write(map[string]string{"Name": "archive", "CreatedAt": stamp})
		case p == "/networks":
			var rows []map[string]any
			for i, name := range []string{"bridge", "host", "none", "in-use", "spare", "ingress", "docker_gwbridge"} {
				rows = append(rows, map[string]any{"Id": id(string("1234567"[i])), "Name": name, "Scope": "local", "Ingress": name == "ingress"})
			}
			write(rows)
		case p == "/system/df":
			write(map[string]any{"Volumes": []any{map[string]any{"Name": "archive", "UsageData": map[string]int{"Size": 8192}}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer engine.Close()
	f := newDMFixture(t)
	f.remote = &dmFakeEngine{srv: engine}
	base := "/hosts/" + f.addRemote("Cleanup") + "/cleanup"
	preview := func(kind string) []map[string]any {
		t.Helper()
		code, body := f.do(http.MethodPost, base+"/preview", map[string]any{"kind": kind})
		if code != http.StatusOK {
			t.Fatalf("%s preview: %s", kind, f.lastRaw)
		}
		return dmArray(t, body, "items")
	}
	apply := func(kind string, choices []map[string]any, status int) []map[string]any {
		t.Helper()
		code, body := f.do(http.MethodPost, base, map[string]any{"kind": kind, "items": choices})
		if code != status {
			t.Fatalf("%s apply: %d %s", kind, code, f.lastRaw)
		}
		if code != http.StatusOK {
			return nil
		}
		return dmArray(t, body, "results")
	}
	checkDeletes := func(want ...string) {
		t.Helper()
		mu.Lock()
		defer mu.Unlock()
		if !slices.Equal(deletes, want) {
			t.Fatalf("Engine deletes = %v, want %v", deletes, want)
		}
		deletes = nil
	}
	images := preview("images")
	if len(images) != 4 {
		t.Fatalf("unused image list = %v", images)
	}
	f.setDocker(true, true)
	apply("images", images, http.StatusForbidden)
	stopped := preview("containers")
	if len(stopped) != 1 || stopped[0]["id"] != id("1") {
		t.Fatalf("stopped container list = %v", stopped)
	}
	f.setDocker(true, false)
	nowUsed.Store(true)
	forged := map[string]any{"id": imageID("a"), "revision": images[0]["revision"]}
	command := append([]map[string]any{forged}, images...)
	result := apply("images", command, http.StatusOK)
	for letter, state := range map[string]string{"a": "skipped", "b": "removed", "c": "skipped", "d": "skipped", "e": "failed"} {
		if got := dmFind(t, result, "id", imageID(letter)); got["state"] != state {
			t.Fatalf("image %s = %v, want %s", letter, got, state)
		}
	}
	mu.Lock()
	slices.Sort(deletes)
	mu.Unlock()
	checkDeletes("/images/"+imageID("b")+"?noprune=true", "/images/"+imageID("d")+"?noprune=true", "/images/"+imageID("e")+"?noprune=true")
	badInventory.Store(true)
	apply("images", images, http.StatusBadGateway)
	checkDeletes()
	badInventory.Store(false)
	apply("containers", stopped, http.StatusOK)
	checkDeletes("/containers/" + id("1") + "?")
	networks := preview("networks")
	if len(networks) != 1 || networks[0]["id"] != id("5") {
		t.Fatalf("unused networks = %v", networks)
	}
	apply("networks", networks, http.StatusOK)
	checkDeletes("/networks/" + id("5") + "?")
	volumes := preview("volumes")
	if len(volumes) != 1 || volumes[0]["id"] != "archive" || volumes[0]["size"] != float64(8192) {
		t.Fatalf("unused volumes = %v", volumes)
	}
	apply("volumes", volumes, http.StatusBadRequest)
	volumes[0]["confirm_name"] = "archive"
	replaced.Store(true)
	if got := apply("volumes", volumes, http.StatusOK)[0]["state"]; got != "skipped" {
		t.Fatalf("replaced volume = %v", got)
	}
	checkDeletes()
	replaced.Store(false)
	apply("volumes", volumes, http.StatusOK)
	checkDeletes("/volumes/archive?")
	if f.local.called("GET /images/json") || f.local.called("GET /volumes") {
		t.Fatal("remote cleanup reached the local Engine")
	}
}

func TestDockerManagerCleanupRejectsInvalidRequests(t *testing.T) {
	f := newDMFixture(t)
	path := "/hosts/local/cleanup"
	for _, in := range []any{
		map[string]any{"kind": "all"},
		map[string]any{"kind": "images"},
		map[string]any{"kind": "containers", "items": []any{map[string]string{"id": "short", "revision": "a"}}},
		map[string]any{"kind": "volumes", "items": []any{map[string]string{"id": "../data", "revision": "a", "confirm_name": "../data"}}},
	} {
		code, body := f.do(http.MethodPost, path, in)
		dmExpectError(t, f, code, body, http.StatusBadRequest, dmCodeBadRequest)
	}
	code, body := f.do(http.MethodGet, path, nil)
	dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, dmCodeMethod)
	f.setDocker(false, false)
	code, body = f.do(http.MethodPost, path+"/preview", map[string]string{"kind": "images"})
	dmExpectError(t, f, code, body, http.StatusServiceUnavailable, dmCodeDisabled)
}
