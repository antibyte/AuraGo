package server

import (
	"net/http"
	"strings"
	"testing"
)

// dmContainerNamed finds a container view by its first name.
func dmContainerNamed(t *testing.T, list []map[string]any, name string) map[string]any {
	t.Helper()
	for _, item := range list {
		if names, _ := item["names"].([]any); len(names) > 0 && names[0] == name {
			return item
		}
	}
	t.Fatalf("no container named %q in %v", name, list)
	return nil
}

func TestDockerManagerContainerListCarriesProtection(t *testing.T) {
	f := newDMFixture(t)

	// Local host: the Containers page's own classification plus the compose fields.
	code, body := f.do(http.MethodGet, "/hosts/local/containers", nil)
	if code != http.StatusOK {
		t.Fatalf("local list = %d %s", code, f.lastRaw)
	}
	list := dmArray(t, body, "containers")
	if len(list) != 2 {
		t.Fatalf("local list has %d containers, want 2: %s", len(list), f.lastRaw)
	}
	web, rv := dmContainerNamed(t, list, "web1"), dmContainerNamed(t, list, "rv")
	if web["compose_project"] != "shop" || web["protected_owner"] != nil {
		t.Fatalf("web1 = %v, want compose_project shop and no owner", web)
	}
	if rv["protected_owner"] != "ruview" || rv["state"] != "exited" {
		t.Fatalf("rv = %v, want protected_owner ruview", rv)
	}

	// Remote host: only the owner from names and labels.
	id := f.addRemote("NAS")
	code, body = f.do(http.MethodGet, "/hosts/"+id+"/containers", nil)
	if code != http.StatusOK {
		t.Fatalf("remote list = %d %s", code, f.lastRaw)
	}
	list = dmArray(t, body, "containers")
	cams, blog := dmContainerNamed(t, list, "cams"), dmContainerNamed(t, list, "blog")
	if cams["protected_owner"] != "ruview" || blog["protected_owner"] != nil || cams["self"] != nil {
		t.Fatalf("remote containers = %v / %v, want an owner only on cams and no self flag", cams, blog)
	}
}

func TestDockerManagerContainerActions(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/"

	code, body := f.do(http.MethodPost, base+"web1/restart", nil)
	if code != http.StatusOK || body["status"] != "ok" || !f.remote.called("POST /containers/web1/restart") {
		t.Fatalf("restart = %d %s, calls %v", code, f.lastRaw, f.remote.calls)
	}
	code, body = f.do(http.MethodPost, base+"missing/stop", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "container_not_found")
	code, body = f.do(http.MethodPost, base+"web1/explode", nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "invalid_action")
	code, body = f.do(http.MethodGet, base+"web1/start", nil)
	dmExpectError(t, f, code, body, http.StatusMethodNotAllowed, "method_not_allowed")
	code, body = f.do(http.MethodPost, base+"bad%20name/start", nil)
	dmExpectError(t, f, code, body, http.StatusBadRequest, "bad_request")

	f.remote.force("POST /containers/web1/pause", http.StatusConflict)
	code, body = f.do(http.MethodPost, base+"web1/pause", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "docker_error")
}

func TestDockerManagerInspectAndLogs(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/web1/"

	code, body := f.do(http.MethodGet, base+"inspect", nil)
	if inspect := dmObject(t, body, "inspect"); code != http.StatusOK || len(inspect) == 0 || !strings.Contains(f.lastRaw, "/web1") {
		t.Fatalf("inspect = %d %s, want the inspect object of web1", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, base+"logs?tail=50", nil)
	logs, _ := body["logs"].(string)
	if code != http.StatusOK || !strings.Contains(logs, "hello from web1") {
		t.Fatalf("logs = %d %s, want the demultiplexed log line", code, f.lastRaw)
	}
	code, body = f.do(http.MethodGet, "/hosts/"+id+"/containers/missing/logs", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "container_not_found")
}

func TestDockerManagerReadOnlyBlocksOnlyContainerMutations(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.setDocker(true, true)

	code, body := f.do(http.MethodPost, "/hosts/"+id+"/containers/web1/start", nil)
	dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
	code, body = f.do(http.MethodDelete, "/hosts/"+id+"/containers/web1", nil)
	dmExpectError(t, f, code, body, http.StatusForbidden, "read_only")
	if f.remote.called("POST /containers/web1/start") || f.remote.called("DELETE /containers/web1") {
		t.Fatalf("read-only mode reached the engine: %v", f.remote.calls)
	}
	if code, _ = f.do(http.MethodGet, "/hosts/"+id+"/containers", nil); code != http.StatusOK {
		t.Fatalf("read-only list = %d, want 200", code)
	}
	// Host management changes AuraGo's own configuration, not Docker.
	if code, _ = f.do(http.MethodPost, "/hosts", map[string]any{"name": "Second", "kind": "tcp", "address": "127.0.0.1", "port": f.remote.port()}); code != http.StatusCreated {
		t.Fatalf("read-only host create = %d %s, want 201", code, f.lastRaw)
	}
}

func TestDockerManagerRemoveAsksForProtectedConfirmation(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	base := "/hosts/" + id + "/containers/"

	code, body := f.do(http.MethodDelete, base+"cams", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
	if body["owner"] != "ruview" || f.remote.called("DELETE /containers/cams") {
		t.Fatalf("unconfirmed remove = %s, calls %v; want owner ruview and no engine delete", f.lastRaw, f.remote.calls)
	}
	code, body = f.do(http.MethodDelete, base+"cams?confirm=protected&force=1", nil)
	if code != http.StatusOK || !f.remote.called("DELETE /containers/cams") {
		t.Fatalf("confirmed remove = %d %s, calls %v", code, f.lastRaw, f.remote.calls)
	}
	if code, _ = f.do(http.MethodDelete, base+"blog", nil); code != http.StatusOK || !f.remote.called("DELETE /containers/blog") {
		t.Fatalf("unprotected remove = %d %s", code, f.lastRaw)
	}

	// The local host runs the full classification of the Containers API.
	defer replaceContainerProtection(containerProtection{Owner: "go2rtc"})()
	code, body = f.do(http.MethodDelete, "/hosts/local/containers/rv", nil)
	dmExpectError(t, f, code, body, http.StatusConflict, containerCodeConfirmationRequired)
	if body["owner"] != "go2rtc" || f.local.called("DELETE /containers/rv") {
		t.Fatalf("local unconfirmed remove = %s, calls %v", f.lastRaw, f.local.calls)
	}
	if code, _ = f.do(http.MethodDelete, "/hosts/local/containers/rv?confirm=protected", nil); code != http.StatusOK || !f.local.called("DELETE /containers/rv") {
		t.Fatalf("local confirmed remove = %d %s, calls %v", code, f.lastRaw, f.local.calls)
	}
}

func TestDockerManagerUnreachableHostIsABadGateway(t *testing.T) {
	f := newDMFixture(t)
	id := f.addRemote("NAS")
	f.remote.srv.Close()

	code, body := f.do(http.MethodGet, "/hosts/"+id+"/containers", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	if reason, _ := body["reason"].(string); reason == "" {
		t.Fatalf("answer %s carries no reason", f.lastRaw)
	}
	code, body = f.do(http.MethodPost, "/hosts/"+id+"/containers/web1/restart", nil)
	dmExpectError(t, f, code, body, http.StatusBadGateway, "host_unreachable")
	code, body = f.do(http.MethodGet, "/hosts/h_000000000000/containers", nil)
	dmExpectError(t, f, code, body, http.StatusNotFound, "host_not_found")
}
